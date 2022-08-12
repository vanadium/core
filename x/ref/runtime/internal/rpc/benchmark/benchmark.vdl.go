// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: benchmark
// package benchmark provides simple tools to measure the performance of the
// IPC system.
//
//nolint:revive
package benchmark

import (
	"io"

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

// BenchmarkClientMethods is the client interface
// containing Benchmark methods.
type BenchmarkClientMethods interface {
	// Echo returns the payload that it receives.
	Echo(_ *context.T, Payload []byte, _ ...rpc.CallOpt) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(*context.T, ...rpc.CallOpt) (BenchmarkEchoStreamClientCall, error)
}

// BenchmarkClientStub embeds BenchmarkClientMethods and is a
// placeholder for additional management operations.
type BenchmarkClientStub interface {
	BenchmarkClientMethods
}

// BenchmarkClient returns a client stub for Benchmark.
func BenchmarkClient(name string) BenchmarkClientStub {
	return implBenchmarkClientStub{name}
}

type implBenchmarkClientStub struct {
	name string
}

func (c implBenchmarkClientStub) Echo(ctx *context.T, i0 []byte, opts ...rpc.CallOpt) (o0 []byte, err error) {
	err = v23.GetClient(ctx).Call(ctx, c.name, "Echo", []interface{}{i0}, []interface{}{&o0}, opts...)
	return
}

func (c implBenchmarkClientStub) EchoStream(ctx *context.T, opts ...rpc.CallOpt) (ocall BenchmarkEchoStreamClientCall, err error) {
	var call rpc.ClientCall
	if call, err = v23.GetClient(ctx).StartCall(ctx, c.name, "EchoStream", nil, opts...); err != nil {
		return
	}
	ocall = &implBenchmarkEchoStreamClientCall{ClientCall: call}
	return
}

// BenchmarkEchoStreamClientStream is the client stream for Benchmark.EchoStream.
type BenchmarkEchoStreamClientStream interface {
	// RecvStream returns the receiver side of the Benchmark.EchoStream client stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
	// SendStream returns the send side of the Benchmark.EchoStream client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors
		// encountered while sending, or if Send is called after Close or
		// the stream has been canceled.  Blocks if there is no buffer
		// space; will unblock when buffer space is available or after
		// the stream has been canceled.
		Send(item []byte) error
		// Close indicates to the server that no more items will be sent;
		// server Recv calls will receive io.EOF after all sent items.
		// This is an optional call - e.g. a client might call Close if it
		// needs to continue receiving items from the server after it's
		// done sending.  Returns errors encountered while closing, or if
		// Close is called after the stream has been canceled.  Like Send,
		// blocks if there is no buffer space available.
		Close() error
	}
}

// BenchmarkEchoStreamClientCall represents the call returned from Benchmark.EchoStream.
type BenchmarkEchoStreamClientCall interface {
	BenchmarkEchoStreamClientStream
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.
	//
	// Finish returns immediately if the call has been canceled; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless the call
	// has been canceled or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() error
}

type implBenchmarkEchoStreamClientCall struct {
	rpc.ClientCall
	valRecv []byte
	errRecv error
}

func (c *implBenchmarkEchoStreamClientCall) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return implBenchmarkEchoStreamClientCallRecv{c}
}

type implBenchmarkEchoStreamClientCallRecv struct {
	c *implBenchmarkEchoStreamClientCall
}

func (c implBenchmarkEchoStreamClientCallRecv) Advance() bool {
	c.c.errRecv = c.c.Recv(&c.c.valRecv)
	return c.c.errRecv == nil
}
func (c implBenchmarkEchoStreamClientCallRecv) Value() []byte {
	return c.c.valRecv
}
func (c implBenchmarkEchoStreamClientCallRecv) Err() error {
	if c.c.errRecv == io.EOF {
		return nil
	}
	return c.c.errRecv
}
func (c *implBenchmarkEchoStreamClientCall) SendStream() interface {
	Send(item []byte) error
	Close() error
} {
	return implBenchmarkEchoStreamClientCallSend{c}
}

type implBenchmarkEchoStreamClientCallSend struct {
	c *implBenchmarkEchoStreamClientCall
}

func (c implBenchmarkEchoStreamClientCallSend) Send(item []byte) error {
	return c.c.Send(item)
}
func (c implBenchmarkEchoStreamClientCallSend) Close() error {
	return c.c.CloseSend()
}
func (c *implBenchmarkEchoStreamClientCall) Finish() (err error) {
	err = c.ClientCall.Finish()
	return
}

// BenchmarkServerMethods is the interface a server writer
// implements for Benchmark.
type BenchmarkServerMethods interface {
	// Echo returns the payload that it receives.
	Echo(_ *context.T, _ rpc.ServerCall, Payload []byte) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(*context.T, BenchmarkEchoStreamServerCall) error
}

// BenchmarkServerStubMethods is the server interface containing
// Benchmark methods, as expected by rpc.Server.
// The only difference between this interface and BenchmarkServerMethods
// is the streaming methods.
type BenchmarkServerStubMethods interface {
	// Echo returns the payload that it receives.
	Echo(_ *context.T, _ rpc.ServerCall, Payload []byte) ([]byte, error)
	// EchoStream returns the payload that it receives via the stream.
	EchoStream(*context.T, *BenchmarkEchoStreamServerCallStub) error
}

// BenchmarkServerStub adds universal methods to BenchmarkServerStubMethods.
type BenchmarkServerStub interface {
	BenchmarkServerStubMethods
	// DescribeInterfaces the Benchmark interfaces.
	Describe__() []rpc.InterfaceDesc
}

// BenchmarkServer returns a server stub for Benchmark.
// It converts an implementation of BenchmarkServerMethods into
// an object that may be used by rpc.Server.
func BenchmarkServer(impl BenchmarkServerMethods) BenchmarkServerStub {
	stub := implBenchmarkServerStub{
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

type implBenchmarkServerStub struct {
	impl BenchmarkServerMethods
	gs   *rpc.GlobState
}

func (s implBenchmarkServerStub) Echo(ctx *context.T, call rpc.ServerCall, i0 []byte) ([]byte, error) {
	return s.impl.Echo(ctx, call, i0)
}

func (s implBenchmarkServerStub) EchoStream(ctx *context.T, call *BenchmarkEchoStreamServerCallStub) error {
	return s.impl.EchoStream(ctx, call)
}

func (s implBenchmarkServerStub) Globber() *rpc.GlobState {
	return s.gs
}

func (s implBenchmarkServerStub) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{BenchmarkDesc}
}

// BenchmarkDesc describes the Benchmark interface.
var BenchmarkDesc rpc.InterfaceDesc = descBenchmark

// descBenchmark hides the desc to keep godoc clean.
var descBenchmark = rpc.InterfaceDesc{
	Name:    "Benchmark",
	PkgPath: "v.io/x/ref/runtime/internal/rpc/benchmark",
	Methods: []rpc.MethodDesc{
		{
			Name: "Echo",
			Doc:  "// Echo returns the payload that it receives.",
			InArgs: []rpc.ArgDesc{
				{Name: "Payload", Doc: ``}, // []byte
			},
			OutArgs: []rpc.ArgDesc{
				{Name: "", Doc: ``}, // []byte
			},
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
		{
			Name: "EchoStream",
			Doc:  "// EchoStream returns the payload that it receives via the stream.",
			Tags: []*vdl.Value{vdl.ValueOf(access.Tag("Read"))},
		},
	},
}

// BenchmarkEchoStreamServerStream is the server stream for Benchmark.EchoStream.
type BenchmarkEchoStreamServerStream interface {
	// RecvStream returns the receiver side of the Benchmark.EchoStream server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() []byte
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	}
	// SendStream returns the send side of the Benchmark.EchoStream server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item []byte) error
	}
}

// BenchmarkEchoStreamServerCall represents the context passed to Benchmark.EchoStream.
type BenchmarkEchoStreamServerCall interface {
	rpc.ServerCall
	BenchmarkEchoStreamServerStream
}

// BenchmarkEchoStreamServerCallStub is a wrapper that converts rpc.StreamServerCall into
// a typesafe stub that implements BenchmarkEchoStreamServerCall.
type BenchmarkEchoStreamServerCallStub struct {
	rpc.StreamServerCall
	valRecv []byte
	errRecv error
}

// Init initializes BenchmarkEchoStreamServerCallStub from rpc.StreamServerCall.
func (s *BenchmarkEchoStreamServerCallStub) Init(call rpc.StreamServerCall) {
	s.StreamServerCall = call
}

// RecvStream returns the receiver side of the Benchmark.EchoStream server stream.
func (s *BenchmarkEchoStreamServerCallStub) RecvStream() interface {
	Advance() bool
	Value() []byte
	Err() error
} {
	return implBenchmarkEchoStreamServerCallRecv{s}
}

type implBenchmarkEchoStreamServerCallRecv struct {
	s *BenchmarkEchoStreamServerCallStub
}

func (s implBenchmarkEchoStreamServerCallRecv) Advance() bool {
	s.s.errRecv = s.s.Recv(&s.s.valRecv)
	return s.s.errRecv == nil
}
func (s implBenchmarkEchoStreamServerCallRecv) Value() []byte {
	return s.s.valRecv
}
func (s implBenchmarkEchoStreamServerCallRecv) Err() error {
	if s.s.errRecv == io.EOF {
		return nil
	}
	return s.s.errRecv
}

// SendStream returns the send side of the Benchmark.EchoStream server stream.
func (s *BenchmarkEchoStreamServerCallStub) SendStream() interface {
	Send(item []byte) error
} {
	return implBenchmarkEchoStreamServerCallSend{s}
}

type implBenchmarkEchoStreamServerCallSend struct {
	s *BenchmarkEchoStreamServerCallStub
}

func (s implBenchmarkEchoStreamServerCallSend) Send(item []byte) error {
	return s.s.Send(item)
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
