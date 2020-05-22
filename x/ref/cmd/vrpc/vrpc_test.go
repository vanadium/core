// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/vlog"
	"v.io/x/ref/cmd/vrpc/internal"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type server struct{}

// TypeTester interface implementation

func (*server) EchoBool(_ *context.T, _ rpc.ServerCall, i1 bool) (bool, error) {
	vlog.VI(2).Info("EchoBool(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoFloat32(_ *context.T, _ rpc.ServerCall, i1 float32) (float32, error) {
	vlog.VI(2).Info("EchoFloat32(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoFloat64(_ *context.T, _ rpc.ServerCall, i1 float64) (float64, error) {
	vlog.VI(2).Info("EchoFloat64(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoInt32(_ *context.T, _ rpc.ServerCall, i1 int32) (int32, error) {
	vlog.VI(2).Info("EchoInt32(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoInt64(_ *context.T, _ rpc.ServerCall, i1 int64) (int64, error) {
	vlog.VI(2).Info("EchoInt64(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoString(_ *context.T, _ rpc.ServerCall, i1 string) (string, error) {
	vlog.VI(2).Info("EchoString(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoByte(_ *context.T, _ rpc.ServerCall, i1 byte) (byte, error) {
	vlog.VI(2).Info("EchoByte(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoUint32(_ *context.T, _ rpc.ServerCall, i1 uint32) (uint32, error) {
	vlog.VI(2).Info("EchoUint32(%v) was called.", i1)
	return i1, nil
}

func (*server) EchoUint64(_ *context.T, _ rpc.ServerCall, i1 uint64) (uint64, error) {
	vlog.VI(2).Info("EchoUint64(%v) was called.", i1)
	return i1, nil
}

func (*server) XEchoArray(_ *context.T, _ rpc.ServerCall, i1 internal.Array2Int) (internal.Array2Int, error) {
	vlog.VI(2).Info("XEchoArray(%v) was called.", i1)
	return i1, nil
}

func (*server) XEchoMap(_ *context.T, _ rpc.ServerCall, i1 map[int32]string) (map[int32]string, error) {
	vlog.VI(2).Info("XEchoMap(%v) was called.", i1)
	return i1, nil
}

func (*server) XEchoSet(_ *context.T, _ rpc.ServerCall, i1 map[int32]struct{}) (map[int32]struct{}, error) {
	vlog.VI(2).Info("XEchoSet(%v) was called.", i1)
	return i1, nil
}

func (*server) XEchoSlice(_ *context.T, _ rpc.ServerCall, i1 []int32) ([]int32, error) {
	vlog.VI(2).Info("XEchoSlice(%v) was called.", i1)
	return i1, nil
}

func (*server) XEchoStruct(_ *context.T, _ rpc.ServerCall, i1 internal.Struct) (internal.Struct, error) {
	vlog.VI(2).Info("XEchoStruct(%v) was called.", i1)
	return i1, nil
}

func (*server) YMultiArg(_ *context.T, _ rpc.ServerCall, i1, i2 int32) (int32, int32, error) {
	vlog.VI(2).Info("YMultiArg(%v,%v) was called.", i1, i2)
	return i1, i2, nil
}

func (*server) YNoArgs(_ *context.T, _ rpc.ServerCall) error {
	vlog.VI(2).Info("YNoArgs() was called.")
	return nil
}

func (*server) ZStream(_ *context.T, call internal.TypeTesterZStreamServerCall, nStream int32, item bool) error {
	vlog.VI(2).Info("ZStream(%v,%v) was called.", nStream, item)
	sender := call.SendStream()
	for i := int32(0); i < nStream; i++ {
		sender.Send(item) //nolint:errcheck
	}
	return nil
}

func initTest(t *testing.T) (ctx *context.T, name string, shutdown v23.Shutdown) {
	ctx, shutdown = test.V23Init()
	obj := internal.TypeTesterServer(&server{})
	ctx, server, err := v23.WithNewServer(ctx, "", obj, nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
		return
	}
	name = server.Status().Endpoints[0].Name()
	return
}

func testSignature(t *testing.T, showReserved bool, wantSig string) {
	ctx, name, shutdown := initTest(t)
	defer shutdown()
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	args := []string{"signature", "-s", fmt.Sprintf("-show-reserved=%v", showReserved), name}
	if err := v23cmd.ParseAndRunForTest(cmdVRPC, ctx, env, args); err != nil {
		t.Fatalf("%s: %v", args, err)
	}

	if got, want := stdout.String(), wantSig; got != want {
		t.Errorf("%s: got stdout %s, want %s", args, got, want)
	}
	if got, want := stderr.String(), ""; got != want {
		t.Errorf("%s: got stderr %s, want %s", args, got, want)
	}
}

func TestSignatureWithReserved(t *testing.T) {
	wantSig := `// TypeTester methods are listed in alphabetical order, to make it easier to
// test Signature output, which sorts methods alphabetically.
type "v.io/x/ref/cmd/vrpc/internal".TypeTester interface {
	// Methods to test support for primitive types.
	EchoBool(I1 bool) (O1 bool | error)
	EchoByte(I1 byte) (O1 byte | error)
	EchoFloat32(I1 float32) (O1 float32 | error)
	EchoFloat64(I1 float64) (O1 float64 | error)
	EchoInt32(I1 int32) (O1 int32 | error)
	EchoInt64(I1 int64) (O1 int64 | error)
	EchoString(I1 string) (O1 string | error)
	EchoUint32(I1 uint32) (O1 uint32 | error)
	EchoUint64(I1 uint64) (O1 uint64 | error)
	// Methods to test support for composite types.
	XEchoArray(I1 "v.io/x/ref/cmd/vrpc/internal".Array2Int) (O1 "v.io/x/ref/cmd/vrpc/internal".Array2Int | error)
	XEchoMap(I1 map[int32]string) (O1 map[int32]string | error)
	XEchoSet(I1 set[int32]) (O1 set[int32] | error)
	XEchoSlice(I1 []int32) (O1 []int32 | error)
	XEchoStruct(I1 "v.io/x/ref/cmd/vrpc/internal".Struct) (O1 "v.io/x/ref/cmd/vrpc/internal".Struct | error)
	// Methods to test support for different number of arguments.
	YMultiArg(I1 int32, I2 int32) (O1 int32, O2 int32 | error)
	YNoArgs() error
	// Methods to test support for streaming.
	ZStream(NumStreamItems int32, StreamItem bool) stream<_, bool> error
}

// Reserved methods implemented by the RPC framework.  Each method name is prefixed with a double underscore "__".
type __Reserved interface {
	// Glob returns all entries matching the pattern.
	__Glob(pattern string) stream<any, any> error
	// MethodSignature returns the signature for the given method.
	__MethodSignature(method string) ("signature".Method | error)
	// Signature returns all interface signatures implemented by the object.
	__Signature() ([]"signature".Interface | error)
}

type "signature".Arg struct {
	Name string
	Doc string
	Type typeobject
}

type "signature".Embed struct {
	Name string
	PkgPath string
	Doc string
}

type "signature".Interface struct {
	Name string
	PkgPath string
	Doc string
	Embeds []"signature".Embed
	Methods []"signature".Method
}

type "signature".Method struct {
	Name string
	Doc string
	InArgs []"signature".Arg
	OutArgs []"signature".Arg
	InStream ?"signature".Arg
	OutStream ?"signature".Arg
	Tags []any
}

type "v.io/x/ref/cmd/vrpc/internal".Array2Int [2]int32

type "v.io/x/ref/cmd/vrpc/internal".Struct struct {
	X int32
	Y int32
}
`
	testSignature(t, true, wantSig)
}

func TestSignatureNoReserved(t *testing.T) {
	wantSig := `// TypeTester methods are listed in alphabetical order, to make it easier to
// test Signature output, which sorts methods alphabetically.
type "v.io/x/ref/cmd/vrpc/internal".TypeTester interface {
	// Methods to test support for primitive types.
	EchoBool(I1 bool) (O1 bool | error)
	EchoByte(I1 byte) (O1 byte | error)
	EchoFloat32(I1 float32) (O1 float32 | error)
	EchoFloat64(I1 float64) (O1 float64 | error)
	EchoInt32(I1 int32) (O1 int32 | error)
	EchoInt64(I1 int64) (O1 int64 | error)
	EchoString(I1 string) (O1 string | error)
	EchoUint32(I1 uint32) (O1 uint32 | error)
	EchoUint64(I1 uint64) (O1 uint64 | error)
	// Methods to test support for composite types.
	XEchoArray(I1 "v.io/x/ref/cmd/vrpc/internal".Array2Int) (O1 "v.io/x/ref/cmd/vrpc/internal".Array2Int | error)
	XEchoMap(I1 map[int32]string) (O1 map[int32]string | error)
	XEchoSet(I1 set[int32]) (O1 set[int32] | error)
	XEchoSlice(I1 []int32) (O1 []int32 | error)
	XEchoStruct(I1 "v.io/x/ref/cmd/vrpc/internal".Struct) (O1 "v.io/x/ref/cmd/vrpc/internal".Struct | error)
	// Methods to test support for different number of arguments.
	YMultiArg(I1 int32, I2 int32) (O1 int32, O2 int32 | error)
	YNoArgs() error
	// Methods to test support for streaming.
	ZStream(NumStreamItems int32, StreamItem bool) stream<_, bool> error
}

type "v.io/x/ref/cmd/vrpc/internal".Array2Int [2]int32

type "v.io/x/ref/cmd/vrpc/internal".Struct struct {
	X int32
	Y int32
}
`
	testSignature(t, false, wantSig)
}

func TestMethodSignature(t *testing.T) {
	ctx, name, shutdown := initTest(t)
	defer shutdown()

	tests := []struct {
		Method, Want string
	}{
		// Spot-check some individual methods.
		{"EchoByte", `EchoByte(I1 byte) (O1 byte | error)`},
		{"EchoFloat32", `EchoFloat32(I1 float32) (O1 float32 | error)`},
		{"XEchoStruct", `
XEchoStruct(I1 "v.io/x/ref/cmd/vrpc/internal".Struct) (O1 "v.io/x/ref/cmd/vrpc/internal".Struct | error)

type "v.io/x/ref/cmd/vrpc/internal".Struct struct {
	X int32
	Y int32
}
`},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		if err := v23cmd.ParseAndRunForTest(cmdVRPC, ctx, env, []string{"signature", name, test.Method}); err != nil {
			t.Errorf("%q failed: %v", test.Method, err)
			continue
		}
		if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(test.Want); got != want {
			t.Errorf("got stdout %q, want %q", got, want)
		}
		if got, want := stderr.String(), ""; got != want {
			t.Errorf("got stderr %q, want %q", got, want)
		}
	}
}

func TestCall(t *testing.T) {
	ctx, name, shutdown := initTest(t)
	defer shutdown()

	tests := []struct {
		Method, InArgs, Want string
	}{
		{"EchoBool", `true`, `true`},
		{"EchoBool", `false`, `false`},
		{"EchoFloat32", `1.2`, `float32(1.2)`},
		{"EchoFloat64", `-3.4`, `float64(-3.4)`},
		{"EchoInt32", `11`, `int32(11)`},
		{"EchoInt64", `-22`, `int64(-22)`},
		{"EchoString", `"abc"`, `"abc"`},
		{"EchoByte", `33`, `byte(33)`},
		{"EchoUint32", `44`, `uint32(44)`},
		{"EchoUint64", `55`, `uint64(55)`},
		{"XEchoArray", `{1,2}`, `"v.io/x/ref/cmd/vrpc/internal".Array2Int{1, 2}`},
		{"XEchoMap", `{1:"a"}`, `map[int32]string{1: "a"}`},
		{"XEchoSet", `{1}`, `set[int32]{1}`},
		{"XEchoSlice", `{1,2}`, `[]int32{1, 2}`},
		{"XEchoStruct", `{1,2}`, `"v.io/x/ref/cmd/vrpc/internal".Struct{X: 1, Y: 2}`},
		{"YMultiArg", `1,2`, `int32(1) int32(2)`},
		{"YNoArgs", ``, ``},
		{"ZStream", `2,true`, `<< true
<< true`},
	}
	for _, test := range tests {
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		if err := v23cmd.ParseAndRunForTest(cmdVRPC, ctx, env, []string{"call", name, test.Method, test.InArgs}); err != nil {
			t.Errorf("%q(%s) failed: %v", test.Method, test.InArgs, err)
			continue
		}
		if got, want := strings.TrimSpace(stdout.String()), strings.TrimSpace(test.Want); got != want {
			t.Errorf("got stdout %q, want %q", got, want)
		}
		if got, want := stderr.String(), ""; got != want {
			t.Errorf("got stderr %q, want %q", got, want)
		}
	}
}
