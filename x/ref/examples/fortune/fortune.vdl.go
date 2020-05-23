// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: fortune

// Package fortune defines the Fortune example interface.
package fortune

import (
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

//////////////////////////////////////////////////
// Interface definitions

// FortuneClientMethods is the client interface
// containing Fortune methods.
//
// Fortune is the interface to a fortune-telling service.
type FortuneClientMethods interface { //nolint:golint
	// Returns a random fortune.
	Get(*context.T, ...rpc.CallOpt) (fortune string, _ error)
	// Adds a fortune to the set used by Get().
	Add(_ *context.T, fortune string, _ ...rpc.CallOpt) error
	// Returns whether or not a fortune exists.
	Has(_ *context.T, fortune string, _ ...rpc.CallOpt) (bool, error)
}

// FortuneClientStub adds universal methods to FortuneClientMethods.
type FortuneClientStub interface { //nolint:golint
	FortuneClientMethods
	rpc.UniversalServiceMethods
}

// FortuneClient returns a client stub for Fortune.
func FortuneClient(name string) FortuneClientStub { //nolint:golint
	return implFortuneClientStub{name}
}

type implFortuneClientStub struct {
	name string
}

func (c implFortuneClientStub) Get(ctx *context.T, opts ...rpc.CallOpt) (o0 string, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Get", nil, []interface{}{&o0}, opts...)
	return
}

func (c implFortuneClientStub) Add(ctx *context.T, i0 string, opts ...rpc.CallOpt) (err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Add", []interface{}{i0}, nil, opts...)
	return
}

func (c implFortuneClientStub) Has(ctx *context.T, i0 string, opts ...rpc.CallOpt) (o0 bool, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Has", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

// FortuneServerMethods is the interface a server writer
// implements for Fortune.
//
// Fortune is the interface to a fortune-telling service.
type FortuneServerMethods interface { //nolint:golint
	// Returns a random fortune.
	Get(*context.T, rpc.ServerCall) (fortune string, _ error)
	// Adds a fortune to the set used by Get().
	Add(_ *context.T, _ rpc.ServerCall, fortune string) error
	// Returns whether or not a fortune exists.
	Has(_ *context.T, _ rpc.ServerCall, fortune string) (bool, error)
}

// FortuneServerStubMethods is the server interface containing
// Fortune methods, as expected by rpc.Server.
// There is no difference between this interface and FortuneServerMethods
// since there are no streaming methods.
// nolint:golint
type FortuneServerStubMethods FortuneServerMethods

// FortuneServerStub adds universal methods to FortuneServerStubMethods.
type FortuneServerStub interface { //nolint:golint
	FortuneServerStubMethods
	// DescribeInterfaces the Fortune interfaces.
	Describe__() []rpc.InterfaceDesc //nolint:golint
}

// FortuneServer returns a server stub for Fortune.
// It converts an implementation of FortuneServerMethods into
// an object that may be used by rpc.Server.
func FortuneServer(impl FortuneServerMethods) FortuneServerStub { //nolint:golint
	stub := implFortuneServerStub{
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

type implFortuneServerStub struct {
	impl FortuneServerMethods
	gs   *rpc.GlobState
}

func (s implFortuneServerStub) Get(ctx *context.T, call rpc.ServerCall) (string, error) {
	return s.impl.Get(ctx, call)
}

func (s implFortuneServerStub) Add(ctx *context.T, call rpc.ServerCall, i0 string) error {
	return s.impl.Add(ctx, call, i0)
}

func (s implFortuneServerStub) Has(ctx *context.T, call rpc.ServerCall, i0 string) (bool, error) {
	return s.impl.Has(ctx, call, i0)
}

func (s implFortuneServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implFortuneServerStub) Describe__() []rpc.InterfaceDesc { //nolint:golint
	return []rpc.InterfaceDesc{FortuneDesc}
}

// FortuneDesc describes the Fortune interface.
var FortuneDesc rpc.InterfaceDesc = descFortune

// descFortune hides the desc to keep godoc clean.
var descFortune = rpc.InterfaceDesc{
	Name:    "Fortune",
	PkgPath: "v.io/x/ref/examples/fortune",
	Doc:     "// Fortune is the interface to a fortune-telling service.",
	Methods: []rpc.MethodDesc{
		{
			Name: "Get",
			Doc:  "// Returns a random fortune.",
			OutArgs: []rpc.ArgDesc{
				{Name: "fortune", Doc: ``}, // string
			},
		},
		{
			Name: "Add",
			Doc:  "// Adds a fortune to the set used by Get().",
			InArgs: []rpc.ArgDesc{
				{Name: "fortune", Doc: ``}, // string
			},
		},
		{
			Name: "Has",
			Doc:  "// Returns whether or not a fortune exists.",
			InArgs: []rpc.ArgDesc{
				{Name: "fortune", Doc: ``}, // string
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // bool
			},
		},
	},
}

var initializeVDLCalled bool

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = initializeVDL()
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
