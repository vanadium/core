// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: internal

// Package internal provides a VDL specification for a service used in the
// unittest of the access package.
//
//nolint:revive
package internal

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
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
// Any package can define tags (of arbitrary types) to be attached to methods.
// This type can be used to index into a Permissions.
type MyTag string

func (MyTag) VDLReflect(struct {
	Name string `vdl:"v.io/v23/security/access/internal.MyTag"`
}) {
}

func (x MyTag) VDLIsZero() bool { //nolint:gocyclo
	return x == ""
}

func (x MyTag) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueString(vdlTypeString1, string(x)); err != nil {
		return err
	}
	return nil
}

func (x *MyTag) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		*x = MyTag(value)
	}
	return nil
}

// Const definitions
// =================

// For this example/unittest, there are three possible values of MyTag, each
// represented by a single-character string.
const Read = MyTag("R")
const Write = MyTag("W")
const Execute = MyTag("X")

// Interface definitions
// =====================

// MyObjectClientMethods is the client interface
// containing MyObject methods.
//
// MyObject demonstrates how tags are attached to methods.
type MyObjectClientMethods interface {
	Get(*context.T, ...rpc.CallOpt) error
	Put(*context.T, ...rpc.CallOpt) error
	Resolve(*context.T, ...rpc.CallOpt) error
	NoTags(*context.T, ...rpc.CallOpt) error // No tags attached to this.
}

// MyObjectClientStub embeds MyObjectClientMethods and is a
// placeholder for additional management operations.
type MyObjectClientStub interface {
	MyObjectClientMethods
}

// MyObjectClient returns a client stub for MyObject.
func MyObjectClient(name string) MyObjectClientStub {
	return implMyObjectClientStub{name}
}

type implMyObjectClientStub struct {
	name string
}

func (c implMyObjectClientStub) Get(ctx *context.T, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Get", nil, nil, opts...)
	return
}

func (c implMyObjectClientStub) Put(ctx *context.T, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Put", nil, nil, opts...)
	return
}

func (c implMyObjectClientStub) Resolve(ctx *context.T, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Resolve", nil, nil, opts...)
	return
}

func (c implMyObjectClientStub) NoTags(ctx *context.T, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "NoTags", nil, nil, opts...)
	return
}

// MyObjectServerMethods is the interface a server writer
// implements for MyObject.
//
// MyObject demonstrates how tags are attached to methods.
type MyObjectServerMethods interface {
	Get(*context.T, rpc.ServerCall) error
	Put(*context.T, rpc.ServerCall) error
	Resolve(*context.T, rpc.ServerCall) error
	NoTags(*context.T, rpc.ServerCall) error // No tags attached to this.
}

// MyObjectServerStubMethods is the server interface containing
// MyObject methods, as expected by rpc.Server.
// There is no difference between this interface and MyObjectServerMethods
// since there are no streaming methods.
type MyObjectServerStubMethods MyObjectServerMethods

// MyObjectServerStub adds universal methods to MyObjectServerStubMethods.
type MyObjectServerStub interface {
	MyObjectServerStubMethods
	// DescribeInterfaces the MyObject interfaces.
	Describe__() []rpc.InterfaceDesc
}

// MyObjectServer returns a server stub for MyObject.
// It converts an implementation of MyObjectServerMethods into
// an object that may be used by rpc.Server.
func MyObjectServer(impl MyObjectServerMethods) MyObjectServerStub {
	stub := implMyObjectServerStub{
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

type implMyObjectServerStub struct {
	impl MyObjectServerMethods
	gs   *rpc.GlobState
}

func (s implMyObjectServerStub) Get(ctx *context.T, call rpc.ServerCall) error {
	return s.impl.Get(ctx, call)
}

func (s implMyObjectServerStub) Put(ctx *context.T, call rpc.ServerCall) error {
	return s.impl.Put(ctx, call)
}

func (s implMyObjectServerStub) Resolve(ctx *context.T, call rpc.ServerCall) error {
	return s.impl.Resolve(ctx, call)
}

func (s implMyObjectServerStub) NoTags(ctx *context.T, call rpc.ServerCall) error {
	return s.impl.NoTags(ctx, call)
}

func (s implMyObjectServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implMyObjectServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{MyObjectDesc}
}

// MyObjectDesc describes the MyObject interface.
var MyObjectDesc rpc.InterfaceDesc = descMyObject

// descMyObject hides the desc to keep godoc clean.
var descMyObject = rpc.InterfaceDesc{
	Name:    "MyObject",
	PkgPath: "v.io/v23/security/access/internal",
	Doc:     "// MyObject demonstrates how tags are attached to methods.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Get",
			Tags: []*vdl.Value{vdl.ValueOf(MyTag("R"))},
		},
		{
			Name: "Put",
			Tags: []*vdl.Value{vdl.ValueOf(MyTag("W"))},
		},
		{
			Name: "Resolve",
			Tags: []*vdl.Value{vdl.ValueOf(MyTag("X"))},
		},
		{
			Name: "NoTags",
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
	vdl.Register((*MyTag)(nil))

	// Initialize type definitions.
	vdlTypeString1 = vdl.TypeOf((*MyTag)(nil))

	return struct{}{}
}
