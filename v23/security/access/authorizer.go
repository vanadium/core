// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package access

import (
	"bytes"
	"fmt"
	"os"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

const pkgPath = "v.io/v23/security/access"

var (
	errTagNeedsString              = verror.Register(pkgPath+".errTagNeedsString", verror.NoRetry, "{1:}{2:}tag type({3}) must be backed by a string not {4}{:_}")
	errNoMethodTags                = verror.Register(pkgPath+".errNoMethodTags", verror.NoRetry, "{1:}{2:}PermissionsAuthorizer.Authorize called on {3}.{4}, which has no tags of type {5}; this is likely unintentional{:_}")
	errMultipleMethodTags          = verror.Register(pkgPath+".errMultipleMethodTags", verror.NoRetry, "{1:}{2:}PermissionsAuthorizer on {3}.{4} cannot handle multiple tags of type {5} ({6}); this is likely unintentional{:_}")
	errCantReadPermissionsFromFile = verror.Register(pkgPath+".errCantReadPermissionsFromFile", verror.NoRetry, "{1:}{2:}failed to read Permissions from file{:_}")
)

// PermissionsAuthorizer implements an authorization policy where access is
// granted if the remote end presents blessings included in the Access Control
// Lists (AccessLists) associated with the set of relevant tags.
//
// The set of relevant tags is the subset of tags associated with the
// method (security.Call.MethodTags) that have the same type as tagType.
// Currently, tagType.Kind must be reflect.String, i.e., only tags that are
// named string types are supported.
//
// PermissionsAuthorizer expects exactly one tag of tagType to be associated
// with the method. If there are multiple, it fails authorization and returns
// an error. However, if multiple tags become a common occurrence, then this
// behavior may change.
//
// If the Permissions provided is nil, then a nil authorizer is returned.
//
// Sample usage:
//
// (1) Attach tags to methods in the VDL (eg. myservice.vdl)
//   package myservice
//
//   type MyTag string
//   const (
//     ReadAccess  = MyTag("R")
//     WriteAccess = MyTag("W")
//   )
//
//   type MyService interface {
//     Get() ([]string, error)       {ReadAccess}
//     GetIndex(int) (string, error) {ReadAccess}
//
//     Set([]string) error           {WriteAccess}
//     SetIndex(int, string) error   {WriteAccess}
//   }
//
// (2) Configure the rpc.Dispatcher to use the PermissionsAuthorizer
//   import (
//     "reflect"
//
//     "v.io/v23/rpc"
//     "v.io/v23/security"
//     "v.io/v23/security/access"
//   )
//
//   type dispatcher struct{}
//   func (d dispatcher) Lookup(suffix, method) (rpc.Invoker, security.Authorizer, error) {
//      perms := access.Permissions{
//        "R": access.AccessList{In: []security.BlessingPattern{"alice:friends", "alice:family"} },
//        "W": access.AccessList{In: []security.BlessingPattern{"alice:family", "alice:colleagues" } },
//      }
//      typ := reflect.TypeOf(ReadAccess)  // equivalently, reflect.TypeOf(WriteAccess)
//      return newInvoker(), access.PermissionsAuthorizer(perms, typ), nil
//   }
//
// With the above dispatcher, the server will grant access to a peer with the
// blessing "alice:friend:bob" access only to the "Get" and "GetIndex" methods.
// A peer presenting the blessing "alice:colleague:carol" will get access only
// to the "Set" and "SetIndex" methods. A peer presenting "alice:family:mom"
// will get access to all methods.
func PermissionsAuthorizer(perms Permissions, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, errTagType(tagType)
	}
	return &authorizer{perms, tagType}, nil
}

// TypicalTagTypePermissionsAuthorizer is like PermissionsAuthorizer, but
// assumes TypicalTagType and thus avoids returning an error.
func TypicalTagTypePermissionsAuthorizer(perms Permissions) security.Authorizer {
	return &authorizer{perms, TypicalTagType()}
}

// PermissionsAuthorizerFromFile applies the same authorization policy as
// PermissionsAuthorizer, with the Permissions to be used sourced from a file
// named filename.
//
// Changes to the file are monitored and affect subsequent calls to Authorize.
// Currently, this is achieved by re-reading the file on every call to
// Authorize.
// TODO(ashankar,ataly): Use inotify or a similar mechanism to watch for
// changes.
func PermissionsAuthorizerFromFile(filename string, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, errTagType(tagType)
	}
	return &fileAuthorizer{filename, tagType}, nil
}

// PermissionsSpec represents a specification for permissions derived
// from command line flags or some other means.
type PermissionsSpec struct {
	// Files represents a set of named files that contain permissions.
	// The name 'runtime' is reserved for use by the runtime.
	Files map[string]string
	// Literal represents a literal, ie. json, permissions specification.
	Literal string
}

// AuthorizerFromSpec creates an authorizer as specified by the
// supplied specification.
func AuthorizerFromSpec(ps PermissionsSpec, preferLiteral bool, name string, tagType *vdl.Type) (security.Authorizer, error) {
	if !preferLiteral || (preferLiteral && len(ps.Literal) == 0) {
		return PermissionsAuthorizerFromFile(ps.Files[name], tagType)
	}
	perms, err := ReadPermissions(bytes.NewBufferString(ps.Literal))
	if err != nil {
		return nil, fmt.Errorf("failed to parse permissions literal: %v: %v", ps.Literal, err)
	}
	return PermissionsAuthorizer(perms, tagType)
}

// RuntimeAuthorizer returns an authorizer configured in the same way
// as that for 'runtime', preferring a literal (ie. on the command line)
// vs a file based specification, and assuming the typical set of access tags.
func RuntimeAuthorizer(spec PermissionsSpec) security.Authorizer {
	return TypicalTagAuthorizerFromSpec(spec, true, "runtime")
}

// TypicalTagAuthorizerFromSpec assumes TypicalTagType() and thus avoids
// returning an error.
func TypicalTagAuthorizerFromSpec(ps PermissionsSpec, preferLiteral bool, name string) security.Authorizer {
	a, _ := AuthorizerFromSpec(ps, preferLiteral, name, TypicalTagType())
	return a
}

func errTagType(tt *vdl.Type) error {
	return verror.New(errTagNeedsString, nil, verror.New(errTagNeedsString, nil, tt, tt.Kind()))
}

type authorizer struct {
	perms   Permissions
	tagType *vdl.Type
}

func (a *authorizer) Authorize(ctx *context.T, call security.Call) error {
	blessings, invalid := security.RemoteBlessingNames(ctx, call)
	hastag := false
	for _, tag := range call.MethodTags() {
		if tag.Type() == a.tagType {
			if hastag {
				return verror.New(errMultipleMethodTags, ctx, call.Suffix(), call.Method(), a.tagType, call.MethodTags())
			}
			hastag = true
			if acl, exists := a.perms[tag.RawString()]; !exists || !acl.Includes(blessings...) {
				return NewErrNoPermissions(ctx, blessings, invalid, tag.RawString())
			}
		}
	}
	if !hastag {
		return verror.New(errNoMethodTags, ctx, call.Suffix(), call.Method(), a.tagType)
	}
	return nil
}

type fileAuthorizer struct {
	filename string
	tagType  *vdl.Type
}

func (a *fileAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	perms, err := loadPermissionsFromFile(a.filename)
	if err != nil {
		// TODO(ashankar): Information leak?
		return verror.New(errCantReadPermissionsFromFile, ctx, err)
	}
	return (&authorizer{perms, a.tagType}).Authorize(ctx, call)
}

func loadPermissionsFromFile(filename string) (Permissions, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ReadPermissions(file)
}
