// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package securityflag implements utilities for creating security objects based
// on flags.
package securityflag

import (
	"bytes"
	"os"

	v23 "v.io/v23"
	"v.io/x/lib/vlog"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
)

// NewAuthorizerOrDie returns an authorizer configured in the same way
// as that for 'runtime', preferring a literal (ie. on the command line)
// vs a file based specification, and assuming the typical set of access tags.
// It will panic if it fails to create an authorizer.
func NewAuthorizerOrDie(ctx *context.T) security.Authorizer {
	auth, err := NewAuthorizer(ctx, "")
	if err != nil {
		vlog.Fatalf("%v", err)
	}
	return auth
}

// NewAuthorizer constructs an Authorizer based on the PermissionsSpec stored
// in the context for the specified 'name' (which defaults to
// "runtime" for an empty string value). It will preferentially use
// literal permissions over file stored ones.
func NewAuthorizer(ctx *context.T, name string) (security.Authorizer, error) {
	if len(name) == 0 {
		name = "runtime"
	}
	return access.AuthorizerFromSpec(
		v23.GetPermissionsSpec(ctx),
		name,
		access.TypicalTagType())
}

// PermissionsFromSpec returns the permissions specified by the supplied
// PermissionsSpec for 'runtime' (ie. as derived) from command line flags
// specified by flags.PermissionsFlags originally.
// It is intended for callers that need more control of how Permissions are
// managed.
// TODO(rjkroege): Refactor these two functions into one by making an Authorizer
// use a Permissions accessor interface.
func PermissionsFromSpec(spec access.PermissionsSpec, name string) (access.Permissions, error) {
	if len(name) == 0 {
		name = "runtime"
	}
	fname := spec.Files[name]
	literal := spec.Literal
	if fname == "" && literal == "" {
		return nil, nil
	}
	if literal == "" {
		file, err := os.Open(fname)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return access.ReadPermissions(file)
	}
	return access.ReadPermissions(bytes.NewBufferString(literal))
}
