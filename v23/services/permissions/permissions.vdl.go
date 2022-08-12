// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: permissions
// Package permissions defines an interface for managing access control
// permissions.
//
//nolint:revive
package permissions

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
)

var initializeVDLCalled = false
var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Interface definitions
// =====================

// ObjectClientMethods is the client interface
// containing Object methods.
//
// Object provides access control for Vanadium objects.
//
// Vanadium services implementing dynamic access control would typically embed
// this interface and tag additional methods defined by the service with one of
// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
// object would be:
//
//	package mypackage
//
//	import "v.io/v23/security/access"
//	import "v.io/v23/services/permissions"
//
//	type MyObject interface {
//	  permissions.Object
//	  MyRead() (string, error) {access.Read}
//	  MyWrite(string) error    {access.Write}
//	}
//
// If the set of pre-defined tags is insufficient, services may define their
// own tag type and annotate all methods with this new type.
//
// Instead of embedding this Object interface, define SetPermissions and
// GetPermissions in their own interface. Authorization policies will typically
// respect annotations of a single type. For example, the VDL definition of an
// object would be:
//
//	package mypackage
//
//	import "v.io/v23/security/access"
//
//	type MyTag string
//
//	const (
//	  Blue = MyTag("Blue")
//	  Red  = MyTag("Red")
//	)
//
//	type MyObject interface {
//	  MyMethod() (string, error) {Blue}
//
//	  // Allow clients to change access via the access.Object interface:
//	  SetPermissions(perms access.Permissions, version string) error         {Red}
//	  GetPermissions() (perms access.Permissions, version string, err error) {Blue}
//	}
type ObjectClientMethods interface {
	// SetPermissions replaces the current Permissions for an object.  version
	// allows for optional, optimistic concurrency control.  If non-empty,
	// version's value must come from GetPermissions.  If any client has
	// successfully called SetPermissions in the meantime, the version will be
	// stale and SetPermissions will fail.  If empty, SetPermissions performs an
	// unconditional update.
	//
	// Permissions objects are expected to be small.  It is up to the
	// implementation to define the exact limit, though it should probably be
	// around 100KB.  Large lists of principals can be represented concisely using
	// blessings.
	//
	// There is some ambiguity when calling SetPermissions on a mount point.
	// Does it affect the mount itself or does it affect the service endpoint
	// that the mount points to?  The chosen behavior is that it affects the
	// service endpoint.  To modify the mount point's Permissions, use
	// ResolveToMountTable to get an endpoint and call SetPermissions on that.
	// This means that clients must know when a name refers to a mount point to
	// change its Permissions.
	SetPermissions(_ *context.T, perms access.Permissions, version string, _ ...rpc.CallOpt) error
	// GetPermissions returns the complete, current Permissions for an object. The
	// returned version can be passed to a subsequent call to SetPermissions for
	// optimistic concurrency control. A successful call to SetPermissions will
	// invalidate version, and the client must call GetPermissions again to get
	// the current version.
	GetPermissions(*context.T, ...rpc.CallOpt) (perms access.Permissions, version string, _ error)
}

// ObjectClientStub embeds ObjectClientMethods and is a
// placeholder for additional management operations.
type ObjectClientStub interface {
	ObjectClientMethods
}

// ObjectClient returns a client stub for Object.
func ObjectClient(name string) ObjectClientStub {
	return implObjectClientStub{name}
}

type implObjectClientStub struct {
	name string
}

func (c implObjectClientStub) SetPermissions(ctx *context.T, i0 access.Permissions, i1 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "SetPermissions", []interface{}{i0, i1}, nil, opts...)
	return
}

func (c implObjectClientStub) GetPermissions(ctx *context.T, opts ...rpc.CallOpt) (o0 access.Permissions, o1 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "GetPermissions", nil, []interface{}{&o0, &o1}, opts...)
	return
}

// ObjectServerMethods is the interface a server writer
// implements for Object.
//
// Object provides access control for Vanadium objects.
//
// Vanadium services implementing dynamic access control would typically embed
// this interface and tag additional methods defined by the service with one of
// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
// object would be:
//
//	package mypackage
//
//	import "v.io/v23/security/access"
//	import "v.io/v23/services/permissions"
//
//	type MyObject interface {
//	  permissions.Object
//	  MyRead() (string, error) {access.Read}
//	  MyWrite(string) error    {access.Write}
//	}
//
// If the set of pre-defined tags is insufficient, services may define their
// own tag type and annotate all methods with this new type.
//
// Instead of embedding this Object interface, define SetPermissions and
// GetPermissions in their own interface. Authorization policies will typically
// respect annotations of a single type. For example, the VDL definition of an
// object would be:
//
//	package mypackage
//
//	import "v.io/v23/security/access"
//
//	type MyTag string
//
//	const (
//	  Blue = MyTag("Blue")
//	  Red  = MyTag("Red")
//	)
//
//	type MyObject interface {
//	  MyMethod() (string, error) {Blue}
//
//	  // Allow clients to change access via the access.Object interface:
//	  SetPermissions(perms access.Permissions, version string) error         {Red}
//	  GetPermissions() (perms access.Permissions, version string, err error) {Blue}
//	}
type ObjectServerMethods interface {
	// SetPermissions replaces the current Permissions for an object.  version
	// allows for optional, optimistic concurrency control.  If non-empty,
	// version's value must come from GetPermissions.  If any client has
	// successfully called SetPermissions in the meantime, the version will be
	// stale and SetPermissions will fail.  If empty, SetPermissions performs an
	// unconditional update.
	//
	// Permissions objects are expected to be small.  It is up to the
	// implementation to define the exact limit, though it should probably be
	// around 100KB.  Large lists of principals can be represented concisely using
	// blessings.
	//
	// There is some ambiguity when calling SetPermissions on a mount point.
	// Does it affect the mount itself or does it affect the service endpoint
	// that the mount points to?  The chosen behavior is that it affects the
	// service endpoint.  To modify the mount point's Permissions, use
	// ResolveToMountTable to get an endpoint and call SetPermissions on that.
	// This means that clients must know when a name refers to a mount point to
	// change its Permissions.
	SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error
	// GetPermissions returns the complete, current Permissions for an object. The
	// returned version can be passed to a subsequent call to SetPermissions for
	// optimistic concurrency control. A successful call to SetPermissions will
	// invalidate version, and the client must call GetPermissions again to get
	// the current version.
	GetPermissions(*context.T, rpc.ServerCall) (perms access.Permissions, version string, _ error)
}

// ObjectServerStubMethods is the server interface containing
// Object methods, as expected by rpc.Server.
// There is no difference between this interface and ObjectServerMethods
// since there are no streaming methods.
type ObjectServerStubMethods ObjectServerMethods

// ObjectServerStub adds universal methods to ObjectServerStubMethods.
type ObjectServerStub interface {
	ObjectServerStubMethods
	// DescribeInterfaces the Object interfaces.
	Describe__() []rpc.InterfaceDesc
}

// ObjectServer returns a server stub for Object.
// It converts an implementation of ObjectServerMethods into
// an object that may be used by rpc.Server.
func ObjectServer(impl ObjectServerMethods) ObjectServerStub {
	stub := implObjectServerStub{
		impl: impl,
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := rpc.NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := rpc.NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type implObjectServerStub struct {
	impl ObjectServerMethods
	gs   *rpc.GlobState
}

func (s implObjectServerStub) SetPermissions(ctx *context.T, call rpc.ServerCall, i0 access.Permissions, i1 string) error {
	return s.impl.SetPermissions(ctx, call, i0, i1)
}

func (s implObjectServerStub) GetPermissions(ctx *context.T, call rpc.ServerCall) (access.Permissions, string, error) {
	return s.impl.GetPermissions(ctx, call)
}

func (s implObjectServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implObjectServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{ObjectDesc}
}

// ObjectDesc describes the Object interface.
var ObjectDesc rpc.InterfaceDesc = descObject

// descObject hides the desc to keep godoc clean.
var descObject = rpc.InterfaceDesc{
	Name:    "Object",
	PkgPath: "v.io/v23/services/permissions",
	Doc:     "// Object provides access control for Vanadium objects.\n//\n// Vanadium services implementing dynamic access control would typically embed\n// this interface and tag additional methods defined by the service with one of\n// Admin, Read, Write, Resolve etc. For example, the VDL definition of the\n// object would be:\n//\n//   package mypackage\n//\n//   import \"v.io/v23/security/access\"\n//   import \"v.io/v23/services/permissions\"\n//\n//   type MyObject interface {\n//     permissions.Object\n//     MyRead() (string, error) {access.Read}\n//     MyWrite(string) error    {access.Write}\n//   }\n//\n// If the set of pre-defined tags is insufficient, services may define their\n// own tag type and annotate all methods with this new type.\n//\n// Instead of embedding this Object interface, define SetPermissions and\n// GetPermissions in their own interface. Authorization policies will typically\n// respect annotations of a single type. For example, the VDL definition of an\n// object would be:\n//\n//  package mypackage\n//\n//  import \"v.io/v23/security/access\"\n//\n//  type MyTag string\n//\n//  const (\n//    Blue = MyTag(\"Blue\")\n//    Red  = MyTag(\"Red\")\n//  )\n//\n//  type MyObject interface {\n//    MyMethod() (string, error) {Blue}\n//\n//    // Allow clients to change access via the access.Object interface:\n//    SetPermissions(perms access.Permissions, version string) error         {Red}\n//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}\n//  }",
	Methods: []rpc.MethodDesc{
		{
			Name: "SetPermissions",
			Doc:  "// SetPermissions replaces the current Permissions for an object.  version\n// allows for optional, optimistic concurrency control.  If non-empty,\n// version's value must come from GetPermissions.  If any client has\n// successfully called SetPermissions in the meantime, the version will be\n// stale and SetPermissions will fail.  If empty, SetPermissions performs an\n// unconditional update.\n//\n// Permissions objects are expected to be small.  It is up to the\n// implementation to define the exact limit, though it should probably be\n// around 100KB.  Large lists of principals can be represented concisely using\n// blessings.\n//\n// There is some ambiguity when calling SetPermissions on a mount point.\n// Does it affect the mount itself or does it affect the service endpoint\n// that the mount points to?  The chosen behavior is that it affects the\n// service endpoint.  To modify the mount point's Permissions, use\n// ResolveToMountTable to get an endpoint and call SetPermissions on that.\n// This means that clients must know when a name refers to a mount point to\n// change its Permissions.",
			InArgs: []rpc.ArgDesc{
				{Name: "perms", Doc: ``},   // access.Permissions
				{Name: "version", Doc: ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Admin"))},
		},
		{
			Name: "GetPermissions",
			Doc:  "// GetPermissions returns the complete, current Permissions for an object. The\n// returned version can be passed to a subsequent call to SetPermissions for\n// optimistic concurrency control. A successful call to SetPermissions will\n// invalidate version, and the client must call GetPermissions again to get\n// the current version.",
			OutArgs: []rpc.ArgDesc{
				{Name: "perms", Doc: ``},   // access.Permissions
				{Name: "version", Doc: ``}, // string
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Admin"))},
		},
	},
}

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
// var _ = initializeVDL()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func initializeVDL() struct{} {
	if initializeVDLCalled {
		return struct{}{}
	}
	initializeVDLCalled = true

	return struct{}{}
}
