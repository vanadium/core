// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: mounttable

// Package mounttable defines interfaces for managing associations between names
// and servers.
//
// A mount table is a tree of names.  Each name may have something mounted on
// it, either a terminal server or another mount table server.  If multiple
// servers are mounted onto the same name, they are considered equivalent, i.e.,
// it is semantically irrelevant which is chosen to pass an RPC to.
//
// Access is determined by AccessLists at each node in the tree.  When resolving
// a name like a/b/c/d, one must check the client blessings against the
// AccessLists of each node traversed ("", a, a/b, a/b/c, and a/b/c/d).
//
//nolint:revive
package mounttable

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/services/permissions"
	"v.io/v23/vdl"
)

var initializeVDLCalled = false
var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Hold type definitions in package-level variables, for better performance.
// Declare and initialize with default values here so that the initializeVDL
// method will be considered ready to initialize before any of the type
// definitions that appear below.
//nolint:unused
var (
	vdlTypeString1 *vdl.Type = nil
)

// Type definitions
// ================
type Tag string

func (Tag) VDLReflect(struct {
	Name string `vdl:"v.io/v23/services/mounttable.Tag"`
}) {
}

func (x Tag) VDLIsZero() bool { //nolint:gocyclo
	return x == ""
}

func (x Tag) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueString(vdlTypeString1, string(x)); err != nil {
		return err
	}
	return nil
}

func (x *Tag) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		*x = Tag(value)
	}
	return nil
}

// Const definitions
// =================

// Admin allow the client to SetPermissions or Delete the receiver.  It also subsumes
// all the other tags.
const Admin = Tag("Admin")

// Mount allows the client to Mount or Unmount at the named receiver.
// For example, to Mount onto a/b/c requires Mount or Admin access
// to a/b/c (and Read, Admin, or Resolve to a and a/b).
const Mount = Tag("Mount")

// Read allows the client to Glob any children of the node.  Thus to
// perform a Glob of a/* one must have Read access to a AND any other
// access to each child of a.  It also allows Resolution through the node.
const Read = Tag("Read")

// Create allows the client to create nodes below the receiver.
const Create = Tag("Create")

// Resolve allows one to resolve through the receiver.  Thus to Resolve
// a/b/c, one needs Admin, Resolve, or Read permission on a, a/b,
// and a/b/c.
const Resolve = Tag("Resolve")

// Interface definitions
// =====================

// MountTableClientMethods is the client interface
// containing MountTable methods.
//
// MountTable defines the interface to talk to a mounttable.
//
// In all methods of MountTable, the receiver is the name bound to.
type MountTableClientMethods interface {
	// Object provides access control for Vanadium objects.
	//
	// Vanadium services implementing dynamic access control would typically embed
	// this interface and tag additional methods defined by the service with one of
	// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
	// object would be:
	//
	//   package mypackage
	//
	//   import "v.io/v23/security/access"
	//   import "v.io/v23/services/permissions"
	//
	//   type MyObject interface {
	//     permissions.Object
	//     MyRead() (string, error) {access.Read}
	//     MyWrite(string) error    {access.Write}
	//   }
	//
	// If the set of pre-defined tags is insufficient, services may define their
	// own tag type and annotate all methods with this new type.
	//
	// Instead of embedding this Object interface, define SetPermissions and
	// GetPermissions in their own interface. Authorization policies will typically
	// respect annotations of a single type. For example, the VDL definition of an
	// object would be:
	//
	//  package mypackage
	//
	//  import "v.io/v23/security/access"
	//
	//  type MyTag string
	//
	//  const (
	//    Blue = MyTag("Blue")
	//    Red  = MyTag("Red")
	//  )
	//
	//  type MyObject interface {
	//    MyMethod() (string, error) {Blue}
	//
	//    // Allow clients to change access via the access.Object interface:
	//    SetPermissions(perms access.Permissions, version string) error         {Red}
	//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}
	//  }
	permissions.ObjectClientMethods
	// Mount Server (a global name) onto the receiver.
	//
	// Subsequent mounts add to the servers mounted there.  The multiple
	// servers are considered equivalent and are meant solely for
	// availability, i.e., no load balancing is guaranteed.
	//
	// The ttl is the number of seconds the mount is to last unless refreshed by
	// another mount of the same server.  A ttl of 0 represents an infinite
	// duration.  A server with an expired ttl should never appear in the results
	// nor affect the operation of any MountTable method, and should act as if it
	// was never present as far as the interface is concerned.
	//
	// The flags represent a bit mask of options.
	Mount(_ *context.T, server string, ttl uint32, flags naming.MountFlag, _ ...rpc.CallOpt) error
	// Unmount removes server from the receiver.  If server is empty, remove all
	// servers mounted there.  Returns a non-nil error iff server remains mounted
	// at the mount point.
	Unmount(_ *context.T, server string, _ ...rpc.CallOpt) error
	// Delete removes the receiver.  If the receiver has children, it will not
	// be removed unless deleteSubtree is true in which case the whole subtree is
	// removed.
	Delete(_ *context.T, deleteSubtree bool, _ ...rpc.CallOpt) error
	// ResolveStep takes the next step in resolving a name.  Returns the next
	// servers to query and the suffix at those servers.
	ResolveStep(*context.T, ...rpc.CallOpt) (naming.MountEntry, error)
}

// MountTableClientStub embeds MountTableClientMethods and is a
// placeholder for additional management operations.
type MountTableClientStub interface {
	MountTableClientMethods
}

// MountTableClient returns a client stub for MountTable.
func MountTableClient(name string) MountTableClientStub {
	return implMountTableClientStub{name, permissions.ObjectClient(name)}
}

type implMountTableClientStub struct {
	name string

	permissions.ObjectClientStub
}

func (c implMountTableClientStub) Mount(ctx *context.T, i0 string, i1 uint32, i2 naming.MountFlag, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Mount", []interface{}{i0, i1, i2}, nil, opts...)
	return
}

func (c implMountTableClientStub) Unmount(ctx *context.T, i0 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Unmount", []interface{}{i0}, nil, opts...)
	return
}

func (c implMountTableClientStub) Delete(ctx *context.T, i0 bool, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Delete", []interface{}{i0}, nil, opts...)
	return
}

func (c implMountTableClientStub) ResolveStep(ctx *context.T, opts ...rpc.CallOpt) (o0 naming.MountEntry, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "ResolveStep", nil, []interface{}{&o0}, opts...)
	return
}

// MountTableServerMethods is the interface a server writer
// implements for MountTable.
//
// MountTable defines the interface to talk to a mounttable.
//
// In all methods of MountTable, the receiver is the name bound to.
type MountTableServerMethods interface {
	// Object provides access control for Vanadium objects.
	//
	// Vanadium services implementing dynamic access control would typically embed
	// this interface and tag additional methods defined by the service with one of
	// Admin, Read, Write, Resolve etc. For example, the VDL definition of the
	// object would be:
	//
	//   package mypackage
	//
	//   import "v.io/v23/security/access"
	//   import "v.io/v23/services/permissions"
	//
	//   type MyObject interface {
	//     permissions.Object
	//     MyRead() (string, error) {access.Read}
	//     MyWrite(string) error    {access.Write}
	//   }
	//
	// If the set of pre-defined tags is insufficient, services may define their
	// own tag type and annotate all methods with this new type.
	//
	// Instead of embedding this Object interface, define SetPermissions and
	// GetPermissions in their own interface. Authorization policies will typically
	// respect annotations of a single type. For example, the VDL definition of an
	// object would be:
	//
	//  package mypackage
	//
	//  import "v.io/v23/security/access"
	//
	//  type MyTag string
	//
	//  const (
	//    Blue = MyTag("Blue")
	//    Red  = MyTag("Red")
	//  )
	//
	//  type MyObject interface {
	//    MyMethod() (string, error) {Blue}
	//
	//    // Allow clients to change access via the access.Object interface:
	//    SetPermissions(perms access.Permissions, version string) error         {Red}
	//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}
	//  }
	permissions.ObjectServerMethods
	// Mount Server (a global name) onto the receiver.
	//
	// Subsequent mounts add to the servers mounted there.  The multiple
	// servers are considered equivalent and are meant solely for
	// availability, i.e., no load balancing is guaranteed.
	//
	// The ttl is the number of seconds the mount is to last unless refreshed by
	// another mount of the same server.  A ttl of 0 represents an infinite
	// duration.  A server with an expired ttl should never appear in the results
	// nor affect the operation of any MountTable method, and should act as if it
	// was never present as far as the interface is concerned.
	//
	// The flags represent a bit mask of options.
	Mount(_ *context.T, _ rpc.ServerCall, server string, ttl uint32, flags naming.MountFlag) error
	// Unmount removes server from the receiver.  If server is empty, remove all
	// servers mounted there.  Returns a non-nil error iff server remains mounted
	// at the mount point.
	Unmount(_ *context.T, _ rpc.ServerCall, server string) error
	// Delete removes the receiver.  If the receiver has children, it will not
	// be removed unless deleteSubtree is true in which case the whole subtree is
	// removed.
	Delete(_ *context.T, _ rpc.ServerCall, deleteSubtree bool) error
	// ResolveStep takes the next step in resolving a name.  Returns the next
	// servers to query and the suffix at those servers.
	ResolveStep(*context.T, rpc.ServerCall) (naming.MountEntry, error)
}

// MountTableServerStubMethods is the server interface containing
// MountTable methods, as expected by rpc.Server.
// There is no difference between this interface and MountTableServerMethods
// since there are no streaming methods.
type MountTableServerStubMethods MountTableServerMethods

// MountTableServerStub adds universal methods to MountTableServerStubMethods.
type MountTableServerStub interface {
	MountTableServerStubMethods
	// DescribeInterfaces the MountTable interfaces.
	Describe__() []rpc.InterfaceDesc
}

// MountTableServer returns a server stub for MountTable.
// It converts an implementation of MountTableServerMethods into
// an object that may be used by rpc.Server.
func MountTableServer(impl MountTableServerMethods) MountTableServerStub {
	stub := implMountTableServerStub{
		impl:             impl,
		ObjectServerStub: permissions.ObjectServer(impl),
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

type implMountTableServerStub struct {
	impl MountTableServerMethods
	permissions.ObjectServerStub
	gs *rpc.GlobState
}

func (s implMountTableServerStub) Mount(ctx *context.T, call rpc.ServerCall, i0 string, i1 uint32, i2 naming.MountFlag) error {
	return s.impl.Mount(ctx, call, i0, i1, i2)
}

func (s implMountTableServerStub) Unmount(ctx *context.T, call rpc.ServerCall, i0 string) error {
	return s.impl.Unmount(ctx, call, i0)
}

func (s implMountTableServerStub) Delete(ctx *context.T, call rpc.ServerCall, i0 bool) error {
	return s.impl.Delete(ctx, call, i0)
}

func (s implMountTableServerStub) ResolveStep(ctx *context.T, call rpc.ServerCall) (naming.MountEntry, error) {
	return s.impl.ResolveStep(ctx, call)
}

func (s implMountTableServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implMountTableServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{MountTableDesc, permissions.ObjectDesc}
}

// MountTableDesc describes the MountTable interface.
var MountTableDesc rpc.InterfaceDesc = descMountTable

// descMountTable hides the desc to keep godoc clean.
var descMountTable = rpc.InterfaceDesc{
	Name:    "MountTable",
	PkgPath: "v.io/v23/services/mounttable",
	Doc:     "// MountTable defines the interface to talk to a mounttable.\n//\n// In all methods of MountTable, the receiver is the name bound to.",
	Embeds: []rpc.EmbedDesc{
		{Name: "Object", PkgPath: "v.io/v23/services/permissions", Doc: "// Object provides access control for Vanadium objects.\n//\n// Vanadium services implementing dynamic access control would typically embed\n// this interface and tag additional methods defined by the service with one of\n// Admin, Read, Write, Resolve etc. For example, the VDL definition of the\n// object would be:\n//\n//   package mypackage\n//\n//   import \"v.io/v23/security/access\"\n//   import \"v.io/v23/services/permissions\"\n//\n//   type MyObject interface {\n//     permissions.Object\n//     MyRead() (string, error) {access.Read}\n//     MyWrite(string) error    {access.Write}\n//   }\n//\n// If the set of pre-defined tags is insufficient, services may define their\n// own tag type and annotate all methods with this new type.\n//\n// Instead of embedding this Object interface, define SetPermissions and\n// GetPermissions in their own interface. Authorization policies will typically\n// respect annotations of a single type. For example, the VDL definition of an\n// object would be:\n//\n//  package mypackage\n//\n//  import \"v.io/v23/security/access\"\n//\n//  type MyTag string\n//\n//  const (\n//    Blue = MyTag(\"Blue\")\n//    Red  = MyTag(\"Red\")\n//  )\n//\n//  type MyObject interface {\n//    MyMethod() (string, error) {Blue}\n//\n//    // Allow clients to change access via the access.Object interface:\n//    SetPermissions(perms access.Permissions, version string) error         {Red}\n//    GetPermissions() (perms access.Permissions, version string, err error) {Blue}\n//  }"},
	},
	Methods: []rpc.MethodDesc{
		{
			Name: "Mount",
			Doc:  "// Mount Server (a global name) onto the receiver.\n//\n// Subsequent mounts add to the servers mounted there.  The multiple\n// servers are considered equivalent and are meant solely for\n// availability, i.e., no load balancing is guaranteed.\n//\n// The ttl is the number of seconds the mount is to last unless refreshed by\n// another mount of the same server.  A ttl of 0 represents an infinite\n// duration.  A server with an expired ttl should never appear in the results\n// nor affect the operation of any MountTable method, and should act as if it\n// was never present as far as the interface is concerned.\n//\n// The flags represent a bit mask of options.",
			InArgs: []rpc.ArgDesc{
				{Name: "server", Doc: ``}, // string
				{Name: "ttl", Doc: ``},    // uint32
				{Name: "flags", Doc: ``},  // naming.MountFlag
			},
		},
		{
			Name: "Unmount",
			Doc:  "// Unmount removes server from the receiver.  If server is empty, remove all\n// servers mounted there.  Returns a non-nil error iff server remains mounted\n// at the mount point.",
			InArgs: []rpc.ArgDesc{
				{Name: "server", Doc: ``}, // string
			},
		},
		{
			Name: "Delete",
			Doc:  "// Delete removes the receiver.  If the receiver has children, it will not\n// be removed unless deleteSubtree is true in which case the whole subtree is\n// removed.",
			InArgs: []rpc.ArgDesc{
				{Name: "deleteSubtree", Doc: ``}, // bool
			},
		},
		{
			Name: "ResolveStep",
			Doc:  "// ResolveStep takes the next step in resolving a name.  Returns the next\n// servers to query and the suffix at those servers.",
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // naming.MountEntry
			},
		},
	},
}

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//	var _ = initializeVDL()
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

	// Register types.
	vdl.Register((*Tag)(nil))

	// Initialize type definitions.
	vdlTypeString1 = vdl.TypeOf((*Tag)(nil))

	return struct{}{}
}
