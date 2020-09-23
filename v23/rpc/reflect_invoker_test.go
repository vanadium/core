// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// reflect_invoker_test is the main unit test for relfelct_invoker.go, but see also
// reflect_invoker_internal_test, which tests some things internal to the module.

package rpc_test

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

// TODO(toddw): Add multi-goroutine tests of reflectCache locking.

func testContext() (*context.T, context.CancelFunc) {
	return context.RootContext()
}

type FakeStreamServerCall struct{}

var _ rpc.StreamServerCall = (*FakeStreamServerCall)(nil)

func (*FakeStreamServerCall) Server() rpc.Server                              { return nil }
func (*FakeStreamServerCall) GrantedBlessings() security.Blessings            { return security.Blessings{} }
func (*FakeStreamServerCall) Closed() <-chan struct{}                         { return nil }
func (*FakeStreamServerCall) IsClosed() bool                                  { return false }
func (*FakeStreamServerCall) Send(item interface{}) error                     { return nil }
func (*FakeStreamServerCall) Recv(itemptr interface{}) error                  { return nil }
func (*FakeStreamServerCall) Timestamp() time.Time                            { return time.Time{} }
func (*FakeStreamServerCall) Method() string                                  { return "" }
func (*FakeStreamServerCall) MethodTags() []*vdl.Value                        { return nil }
func (*FakeStreamServerCall) Suffix() string                                  { return "" }
func (*FakeStreamServerCall) LocalDischarges() map[string]security.Discharge  { return nil }
func (*FakeStreamServerCall) RemoteDischarges() map[string]security.Discharge { return nil }
func (*FakeStreamServerCall) LocalPrincipal() security.Principal              { return nil }
func (*FakeStreamServerCall) LocalBlessings() security.Blessings              { return security.Blessings{} }
func (*FakeStreamServerCall) RemoteBlessings() security.Blessings             { return security.Blessings{} }
func (*FakeStreamServerCall) LocalEndpoint() naming.Endpoint                  { return naming.Endpoint{} }
func (*FakeStreamServerCall) RemoteEndpoint() naming.Endpoint                 { return naming.Endpoint{} }
func (*FakeStreamServerCall) RemoteAddr() net.Addr                            { return nil }
func (*FakeStreamServerCall) Security() security.Call                         { return nil }

var (
	// save the CancelFuncs too, to silence the CancelFunc leak detector
	ctx1, _ = testContext()
	ctx2, _ = testContext()
	ctx3, _ = testContext()
	ctx4, _ = testContext()
	ctx5, _ = testContext()
	ctx6, _ = testContext()
	call1   = &FakeStreamServerCall{}
	call2   = &FakeStreamServerCall{}
	call3   = &FakeStreamServerCall{}
	call4   = &FakeStreamServerCall{}
	call5   = &FakeStreamServerCall{}
	call6   = &FakeStreamServerCall{}
)

// test tags.
var (
	tagAlpha   = vdl.StringValue(nil, "alpha")
	tagBeta    = vdl.StringValue(nil, "beta")
	tagGamma   = vdl.StringValue(nil, "gamma")
	tagDelta   = vdl.StringValue(nil, "delta")
	tagEpsilon = vdl.StringValue(nil, "epsilon")
)

// All objects used for success testing are based on testObj, which captures the
// state from each invocation, so that we may test it against our expectations.
type testObj struct {
	ctx  *context.T
	call rpc.ServerCall
}

func (o testObj) LastContext() *context.T  { return o.ctx }
func (o testObj) LastCall() rpc.ServerCall { return o.call }

type testObjIface interface {
	LastContext() *context.T
	LastCall() rpc.ServerCall
}

var errApp = errors.New("app error")

type notags struct{ testObj }

func (o *notags) Method1(ctx *context.T, call rpc.ServerCall) error {
	o.ctx = ctx
	o.call = call
	return nil
}
func (o *notags) Method2(ctx *context.T, call rpc.ServerCall) (int, error) {
	o.ctx = ctx
	o.call = call
	return 0, nil
}
func (o *notags) Method3(ctx *context.T, call rpc.ServerCall, _ int) error {
	o.ctx = ctx
	o.call = call
	return nil
}
func (o *notags) Method4(ctx *context.T, call rpc.ServerCall, i int) (int, error) {
	o.ctx = ctx
	o.call = call
	return i, nil
}
func (o *notags) Error(ctx *context.T, call rpc.ServerCall) error {
	o.ctx = ctx
	o.call = call
	return errApp
}

type tags struct{ testObj }

func (o *tags) Alpha(ctx *context.T, call rpc.ServerCall) error {
	o.ctx = ctx
	o.call = call
	return nil
}
func (o *tags) Beta(ctx *context.T, call rpc.ServerCall) (int, error) {
	o.ctx = ctx
	o.call = call
	return 0, nil
}
func (o *tags) Gamma(ctx *context.T, call rpc.ServerCall, _ int) error {
	o.ctx = ctx
	o.call = call
	return nil
}
func (o *tags) Delta(ctx *context.T, call rpc.ServerCall, i int) (int, error) {
	o.ctx = ctx
	o.call = call
	return i, nil
}
func (o *tags) Epsilon(ctx *context.T, call rpc.ServerCall, i int, s string) (int, string, error) {
	o.ctx = ctx
	o.call = call
	return i, s, nil
}
func (o *tags) Error(ctx *context.T, call rpc.ServerCall) error {
	o.ctx = ctx
	o.call = call
	return errApp
}

//nolint:golint // API change required.
func (o *tags) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{{
		Methods: []rpc.MethodDesc{
			{Name: "Alpha", Tags: []*vdl.Value{tagAlpha}},
			{Name: "Beta", Tags: []*vdl.Value{tagBeta}},
			{Name: "Gamma", Tags: []*vdl.Value{tagGamma}},
			{Name: "Delta", Tags: []*vdl.Value{tagDelta}},
			{Name: "Epsilon", Tags: []*vdl.Value{tagEpsilon}},
		},
	}}
}

func TestReflectInvoker(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()
	type v []interface{}
	type testcase struct {
		obj    testObjIface
		method string
		ctx    *context.T
		call   rpc.StreamServerCall
		// Expected results:
		tag     *vdl.Value
		args    v
		results v
		err     error
	}
	tests := []testcase{
		{&notags{}, "Method1", ctx1, call1, nil, nil, nil, nil},
		{&notags{}, "Method2", ctx2, call2, nil, nil, v{0}, nil},
		{&notags{}, "Method3", ctx3, call3, nil, v{0}, nil, nil},
		{&notags{}, "Method4", ctx4, call4, nil, v{11}, v{11}, nil},
		{&notags{}, "Error", ctx5, call5, nil, nil, nil, errApp},
		{&tags{}, "Alpha", ctx1, call1, tagAlpha, nil, nil, nil},
		{&tags{}, "Beta", ctx2, call2, tagBeta, nil, v{0}, nil},
		{&tags{}, "Gamma", ctx3, call3, tagGamma, v{0}, nil, nil},
		{&tags{}, "Delta", ctx4, call4, tagDelta, v{11}, v{11}, nil},
		{&tags{}, "Epsilon", ctx5, call5, tagEpsilon, v{11, "b"}, v{11, "b"}, nil},
		{&tags{}, "Error", ctx6, call6, nil, nil, nil, errApp},
	}
	name := func(test testcase) string {
		return fmt.Sprintf("%T.%s()", test.obj, test.method)
	}
	testInvoker := func(test testcase, invoker rpc.Invoker) {
		// Call Invoker.Prepare and check results.
		argptrs, tags, err := invoker.Prepare(ctx, test.method, len(test.args))
		if err != nil {
			t.Errorf("%s Prepare unexpected error: %v", name(test), err)
		}
		if !equalPtrValTypes(argptrs, test.args) {
			t.Errorf("%s Prepare got argptrs %v, want args %v", name(test), printTypes(argptrs), printTypes(toPtrs(test.args)))
		}
		var tag *vdl.Value
		if len(tags) > 0 {
			tag = tags[0]
		}
		if tag != test.tag {
			t.Errorf("%s Prepare got tags %v, want %v", name(test), tags, []*vdl.Value{test.tag})
		}
		// Call Invoker.Invoke and check results.
		results, err := invoker.Invoke(test.ctx, test.call, test.method, toPtrs(test.args))
		if err != test.err {
			t.Errorf(`%s Invoke got error "%v", want "%v"`, name(test), err, test.err)
		}
		if !reflect.DeepEqual(v(results), test.results) {
			t.Errorf("%s Invoke got results %v, want %v", name(test), results, test.results)
		}
		if got, want := test.obj.LastContext(), test.ctx; got != want {
			t.Errorf("%s Invoke got ctx %v, want %v", name(test), got, want)
		}
		if got, want := test.obj.LastCall(), test.call; got != want {
			t.Errorf("%s Invoke got call %v, want %v", name(test), got, want)
		}
	}
	for _, test := range tests {
		invoker := rpc.ReflectInvokerOrDie(test.obj)
		testInvoker(test, invoker)
		invoker, err := rpc.ReflectInvoker(test.obj)
		if err != nil {
			t.Errorf("%s ReflectInvoker got error: %v", name(test), err)
		}
		testInvoker(test, invoker)
	}
}

// equalPtrValTypes returns true iff the types of each value in valptrs is
// identical to the types of the pointers to each value in vals.
func equalPtrValTypes(valptrs, vals []interface{}) bool {
	if len(valptrs) != len(vals) {
		return false
	}
	for ix, val := range vals {
		valptr := valptrs[ix]
		if reflect.TypeOf(valptr) != reflect.PtrTo(reflect.TypeOf(val)) {
			return false
		}
	}
	return true
}

// printTypes returns a string representing the type of each value in vals.
func printTypes(vals []interface{}) string {
	s := "["
	for ix, val := range vals {
		if ix > 0 {
			s += ", "
		}
		s += reflect.TypeOf(val).String()
	}
	return s + "]"
}

// toPtrs takes the given vals and returns a new slice, where each item V at
// index I in vals has been copied into a new pointer value P at index I in the
// result.  The type of P is a pointer to the type of V.
func toPtrs(vals []interface{}) []interface{} {
	valptrs := make([]interface{}, len(vals))
	for ix, val := range vals {
		rvValPtr := reflect.New(reflect.TypeOf(val))
		rvValPtr.Elem().Set(reflect.ValueOf(val))
		valptrs[ix] = rvValPtr.Interface()
	}
	return valptrs
}

type (
	sigTest        struct{}
	stringBoolCall struct{ rpc.ServerCall }
)

func (*stringBoolCall) Init(rpc.StreamServerCall) {}
func (*stringBoolCall) RecvStream() interface {
	Advance() bool
	Value() string
	Err() error
} {
	return nil
}
func (*stringBoolCall) SendStream() interface {
	Send(item bool) error
} {
	return nil
}

func (sigTest) Sig1(*context.T, rpc.ServerCall) error                       { return nil }
func (sigTest) Sig2(*context.T, rpc.ServerCall, int32, string) error        { return nil }
func (sigTest) Sig3(*context.T, *stringBoolCall, float64) ([]uint32, error) { return nil, nil }
func (sigTest) Sig4(*context.T, rpc.StreamServerCall, int32, string) (int32, string, error) {
	return 0, "", nil
}

//nolint:golint // API change required.
func (sigTest) Describe__() []rpc.InterfaceDesc {
	return []rpc.InterfaceDesc{
		{
			Name:    "Iface1",
			PkgPath: "a/b/c",
			Doc:     "Doc Iface1",
			Embeds: []rpc.EmbedDesc{
				{Name: "Iface1Embed1", PkgPath: "x/y", Doc: "Doc embed1"},
			},
			Methods: []rpc.MethodDesc{
				{
					Name:      "Sig3",
					Doc:       "Doc Sig3",
					InArgs:    []rpc.ArgDesc{{Name: "i0_3", Doc: "Doc i0_3"}},
					OutArgs:   []rpc.ArgDesc{{Name: "o0_3", Doc: "Doc o0_3"}},
					InStream:  rpc.ArgDesc{Name: "is_3", Doc: "Doc is_3"},
					OutStream: rpc.ArgDesc{Name: "os_3", Doc: "Doc os_3"},
					Tags:      []*vdl.Value{tagAlpha, tagBeta},
				},
			},
		},
		{
			Name:    "Iface2",
			PkgPath: "d/e/f",
			Doc:     "Doc Iface2",
			Methods: []rpc.MethodDesc{
				{
					// The same Sig3 method is described here in a different interface.
					Name:      "Sig3",
					Doc:       "Doc Sig3x",
					InArgs:    []rpc.ArgDesc{{Name: "i0_3x", Doc: "Doc i0_3x"}},
					OutArgs:   []rpc.ArgDesc{{Name: "o0_3x", Doc: "Doc o0_3x"}},
					InStream:  rpc.ArgDesc{Name: "is_3x", Doc: "Doc is_3x"},
					OutStream: rpc.ArgDesc{Name: "os_3x", Doc: "Doc os_3x"},
					// Must have the same tags as every other definition of this method.
					Tags: []*vdl.Value{tagAlpha, tagBeta},
				},
				{
					Name: "Sig4",
					Doc:  "Doc Sig4",
					InArgs: []rpc.ArgDesc{
						{Name: "i0_4", Doc: "Doc i0_4"}, {Name: "i1_4", Doc: "Doc i1_4"}},
					OutArgs: []rpc.ArgDesc{
						{Name: "o0_4", Doc: "Doc o0_4"}, {Name: "o1_4", Doc: "Doc o1_4"}},
				},
			},
		},
	}
}

func TestReflectInvokerSignature(t *testing.T) {
	tests := []struct {
		Method string // empty to invoke Signature rather than MethodSignature
		Want   interface{}
	}{
		// Tests of MethodSignature.
		{"Sig1", signature.Method{Name: "Sig1"}},
		{"Sig2", signature.Method{
			Name:   "Sig2",
			InArgs: []signature.Arg{{Type: vdl.Int32Type}, {Type: vdl.StringType}},
		}},
		{"Sig3", signature.Method{
			Name: "Sig3",
			Doc:  "Doc Sig3",
			InArgs: []signature.Arg{
				{Name: "i0_3", Doc: "Doc i0_3", Type: vdl.Float64Type}},
			OutArgs: []signature.Arg{
				{Name: "o0_3", Doc: "Doc o0_3", Type: vdl.ListType(vdl.Uint32Type)}},
			InStream: &signature.Arg{
				Name: "is_3", Doc: "Doc is_3", Type: vdl.StringType},
			OutStream: &signature.Arg{
				Name: "os_3", Doc: "Doc os_3", Type: vdl.BoolType},
			Tags: []*vdl.Value{tagAlpha, tagBeta},
		}},
		{"Sig4", signature.Method{
			Name: "Sig4",
			Doc:  "Doc Sig4",
			InArgs: []signature.Arg{
				{Name: "i0_4", Doc: "Doc i0_4", Type: vdl.Int32Type},
				{Name: "i1_4", Doc: "Doc i1_4", Type: vdl.StringType}},
			OutArgs: []signature.Arg{
				{Name: "o0_4", Doc: "Doc o0_4", Type: vdl.Int32Type},
				{Name: "o1_4", Doc: "Doc o1_4", Type: vdl.StringType}},
			// Since rpc.StreamServerCall is used, we must assume streaming with any.
			InStream:  &signature.Arg{Type: vdl.AnyType},
			OutStream: &signature.Arg{Type: vdl.AnyType},
		}},
		// Test Signature, which always returns the "true" information collected via
		// reflection, and enhances it with user-provided descriptions.
		{"", []signature.Interface{
			{
				Name:    "Iface1",
				PkgPath: "a/b/c",
				Doc:     "Doc Iface1",
				Embeds: []signature.Embed{
					{Name: "Iface1Embed1", PkgPath: "x/y", Doc: "Doc embed1"},
				},
				Methods: []signature.Method{
					{
						Name: "Sig3",
						Doc:  "Doc Sig3",
						InArgs: []signature.Arg{
							{Name: "i0_3", Doc: "Doc i0_3", Type: vdl.Float64Type},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_3", Doc: "Doc o0_3", Type: vdl.ListType(vdl.Uint32Type)},
						},
						InStream: &signature.Arg{
							Name: "is_3", Doc: "Doc is_3", Type: vdl.StringType},
						OutStream: &signature.Arg{
							Name: "os_3", Doc: "Doc os_3", Type: vdl.BoolType},
						Tags: []*vdl.Value{tagAlpha, tagBeta},
					},
				},
			},
			{
				Name:    "Iface2",
				PkgPath: "d/e/f",
				Doc:     "Doc Iface2",
				Methods: []signature.Method{
					{
						Name: "Sig3",
						Doc:  "Doc Sig3x",
						InArgs: []signature.Arg{
							{Name: "i0_3x", Doc: "Doc i0_3x", Type: vdl.Float64Type},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_3x", Doc: "Doc o0_3x", Type: vdl.ListType(vdl.Uint32Type)},
						},
						InStream: &signature.Arg{
							Name: "is_3x", Doc: "Doc is_3x", Type: vdl.StringType},
						OutStream: &signature.Arg{
							Name: "os_3x", Doc: "Doc os_3x", Type: vdl.BoolType},
						Tags: []*vdl.Value{tagAlpha, tagBeta},
					},
					{
						Name: "Sig4",
						Doc:  "Doc Sig4",
						InArgs: []signature.Arg{
							{Name: "i0_4", Doc: "Doc i0_4", Type: vdl.Int32Type},
							{Name: "i1_4", Doc: "Doc i1_4", Type: vdl.StringType},
						},
						OutArgs: []signature.Arg{
							{Name: "o0_4", Doc: "Doc o0_4", Type: vdl.Int32Type},
							{Name: "o1_4", Doc: "Doc o1_4", Type: vdl.StringType},
						},
						InStream:  &signature.Arg{Type: vdl.AnyType},
						OutStream: &signature.Arg{Type: vdl.AnyType},
					},
				},
			},
			{
				Doc: "The empty interface contains methods not attached to any interface.",
				Methods: []signature.Method{
					{Name: "Sig1"},
					{
						Name:   "Sig2",
						InArgs: []signature.Arg{{Type: vdl.Int32Type}, {Type: vdl.StringType}},
					},
				},
			},
		}},
	}
	for _, test := range tests {
		invoker := rpc.ReflectInvokerOrDie(sigTest{})
		var got interface{}
		var err error
		if test.Method == "" {
			got, err = invoker.Signature(nil, nil)
		} else {
			got, err = invoker.MethodSignature(nil, nil, test.Method)
		}
		if err != nil {
			t.Errorf("%q got error %v", test.Method, err)
		}
		if want := test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("%q got %#v, want %#v", test.Method, got, want)
		}
	}
}

type (
	badcall        struct{}
	arbitraryCall  struct{}
	noInitCall     struct{ rpc.ServerCall }
	badInit1Call   struct{ rpc.ServerCall }
	badInit2Call   struct{ rpc.ServerCall }
	badInit3Call   struct{ rpc.ServerCall }
	noSendRecvCall struct{ rpc.ServerCall }
	badSend1Call   struct{ rpc.ServerCall }
	badSend2Call   struct{ rpc.ServerCall }
	badSend3Call   struct{ rpc.ServerCall }
	badRecv1Call   struct{ rpc.ServerCall }
	badRecv2Call   struct{ rpc.ServerCall }
	badRecv3Call   struct{ rpc.ServerCall }

	badoutargs struct{}

	badGlobber       struct{}
	badGlob1         struct{}
	badGlob2         struct{}
	badGlob3         struct{}
	badGlob4         struct{}
	badGlob5         struct{}
	badGlob6         struct{}
	badGlobChildren1 struct{}
	badGlobChildren2 struct{}
	badGlobChildren3 struct{}
	badGlobChildren4 struct{}
	badGlobChildren5 struct{}
)

func (arbitraryCall) SomeExportedCall() int { return 0 }

//nolint:deadcode,unused
func (badcall) notExported(*context.T, rpc.ServerCall) error { return nil }
func (badcall) NonRPC1() error                               { return nil }
func (badcall) NonRPC2(int) error                            { return nil }
func (badcall) NonRPC3(int, string) error                    { return nil }
func (badcall) NonRPC4(*badcall) error                       { return nil }
func (badcall) NonRPC5(context.T, *badcall) error            { return nil }
func (badcall) NonRPC6(*context.T, *badcall) error           { return nil }
func (badcall) NoInit(*context.T, *noInitCall) error         { return nil }
func (badcall) BadInit1(*context.T, *badInit1Call) error     { return nil }
func (badcall) BadInit2(*context.T, *badInit2Call) error     { return nil }
func (badcall) BadInit3(*context.T, *badInit3Call) error     { return nil }
func (badcall) NoSendRecv(*context.T, *noSendRecvCall) error { return nil }
func (badcall) BadSend1(*context.T, *badSend1Call) error     { return nil }
func (badcall) BadSend2(*context.T, *badSend2Call) error     { return nil }
func (badcall) BadSend3(*context.T, *badSend3Call) error     { return nil }
func (badcall) BadRecv1(*context.T, *badRecv1Call) error     { return nil }
func (badcall) BadRecv2(*context.T, *badRecv2Call) error     { return nil }
func (badcall) BadRecv3(*context.T, *badRecv3Call) error     { return nil }

func (*badInit1Call) Init()                          {}
func (*badInit2Call) Init(int)                       {}
func (*badInit3Call) Init(rpc.StreamServerCall, int) {}
func (*noSendRecvCall) Init(rpc.StreamServerCall)    {}
func (*badSend1Call) Init(rpc.StreamServerCall)      {}
func (*badSend1Call) SendStream()                    {}
func (*badSend2Call) Init(rpc.StreamServerCall)      {}
func (*badSend2Call) SendStream() interface {
	Send() error
} {
	return nil
}
func (*badSend3Call) Init(rpc.StreamServerCall) {}
func (*badSend3Call) SendStream() interface {
	Send(int)
} {
	return nil
}
func (*badRecv1Call) Init(rpc.StreamServerCall) {}
func (*badRecv1Call) RecvStream()               {}
func (*badRecv2Call) Init(rpc.StreamServerCall) {}
func (*badRecv2Call) RecvStream() interface {
	Advance() bool
	Value() int
} {
	return nil
}
func (*badRecv3Call) Init(rpc.StreamServerCall) {}
func (*badRecv3Call) RecvStream() interface {
	Advance()
	Value() int
	Error() error
} {
	return nil
}

func (badoutargs) NoFinalError1(*context.T, rpc.ServerCall)                {}
func (badoutargs) NoFinalError2(*context.T, rpc.ServerCall) string         { return "" }
func (badoutargs) NoFinalError3(*context.T, rpc.ServerCall) (bool, string) { return false, "" }

//nolint:golint // API change required.
func (badoutargs) NoFinalError4(*context.T, rpc.ServerCall) (error, string) { return nil, "" }

func (badGlobber) Globber() {}

//nolint:golint // API change required.
func (badGlob1) Glob__() {}

//nolint:golint // API change required.
func (badGlob2) Glob__(*context.T) {}

//nolint:golint // API change required.
func (badGlob3) Glob__(*context.T, rpc.GlobServerCall) {}

//nolint:golint // API change required.
func (badGlob4) Glob__(*context.T, rpc.GlobServerCall, *glob.Glob) {}

//nolint:golint // API change required.
func (badGlob5) Glob__(*context.T, rpc.ServerCall, *glob.Glob) error { return nil }

//nolint:golint // API change required.
func (badGlob6) Glob__() error { return nil }

//nolint:golint // API change required.
func (badGlobChildren1) GlobChildren__() {}

//nolint:golint // API change required.
func (badGlobChildren2) GlobChildren__(*context.T) {}

//nolint:golint // API change required.
func (badGlobChildren3) GlobChildren__(*context.T, rpc.GlobChildrenServerCall) {}

//nolint:golint // API change required.
func (badGlobChildren4) GlobChildren__(*context.T, rpc.GlobChildrenServerCall, *glob.Element) {}

//nolint:golint // API change required.
func (badGlobChildren5) GlobChildren__(*context.T, rpc.GlobChildrenServerCall) error { return nil }

func TestReflectInvokerPanic(t *testing.T) {
	type testcase struct {
		obj    interface{}
		regexp string
	}
	tests := []testcase{
		{nil, `ReflectInvoker\(nil\) is invalid`},
		{struct{}{}, "no exported methods"},
		{arbitraryCall{}, "no compatible methods"},

		{badcall{}, "invalid streaming call"},
		{badoutargs{}, "final out-arg must be error"},
		{badGlobber{}, "Globber must have signature"},
		{badGlob1{}, "Glob__ must have signature"},
		{badGlob2{}, "Glob__ must have signature"},
		{badGlob3{}, "Glob__ must have signature"},
		{badGlob4{}, "Glob__ must have signature"},
		{badGlob5{}, "Glob__ must have signature"},
		{badGlob6{}, "Glob__ must have signature"},
		{badGlobChildren1{}, "GlobChildren__ must have signature"},
		{badGlobChildren2{}, "GlobChildren__ must have signature"},
		{badGlobChildren3{}, "GlobChildren__ must have signature"},
		{badGlobChildren4{}, "GlobChildren__ must have signature"},
		{badGlobChildren5{}, "GlobChildren__ must have signature"},
	}
	for _, test := range tests {
		re := regexp.MustCompile(test.regexp)
		invoker, err := rpc.ReflectInvoker(test.obj)
		if invoker != nil {
			t.Errorf(`ReflectInvoker(%T) got non-nil invoker`, test.obj)
		}
		if !re.MatchString(fmt.Sprint(err)) {
			t.Errorf(`ReflectInvoker(%T) got error %v, want regexp "%v"`, test.obj, err, test.regexp)
		}
		obj := test.obj
		recov := testutil.CallAndRecover(func() { rpc.ReflectInvokerOrDie(obj) })
		if !re.MatchString(fmt.Sprint(recov)) {
			t.Errorf(`ReflectInvokerOrDie(%T) got panic %v, want regexp "%v"`, test.obj, recov, test.regexp)
		}
	}
}

func TestReflectInvokerErrors(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()
	type v []interface{}
	type testcase struct {
		obj        interface{}
		method     string
		args       v
		prepareErr error
		invokeErr  error
	}
	tests := []testcase{
		{&notags{}, "UnknownMethod", v{}, verror.ErrUnknownMethod, verror.ErrUnknownMethod},
		{&tags{}, "UnknownMethod", v{}, verror.ErrUnknownMethod, verror.ErrUnknownMethod},
	}
	name := func(test testcase) string {
		return fmt.Sprintf("%T.%s()", test.obj, test.method)
	}
	testInvoker := func(test testcase, invoker rpc.Invoker) {
		// Call Invoker.Prepare and check error.
		_, _, err := invoker.Prepare(ctx, test.method, len(test.args))
		if !errors.Is(err, test.prepareErr) {
			t.Errorf(`%s Prepare got error "%v", want id %v`, name(test), err, test.prepareErr)
		}
		// Call Invoker.Invoke and check error.
		_, err = invoker.Invoke(ctx1, call1, test.method, test.args)
		if !errors.Is(err, test.invokeErr) {
			t.Errorf(`%s Invoke got error "%v", want id %v`, name(test), err, test.invokeErr)
		}
	}
	for _, test := range tests {
		invoker := rpc.ReflectInvokerOrDie(test.obj)
		testInvoker(test, invoker)
		invoker, err := rpc.ReflectInvoker(test.obj)
		if err != nil {
			t.Errorf(`%s ReflectInvoker got error %v`, name(test), err)
		}
		testInvoker(test, invoker)
	}
}

type vGlobberObject struct {
	gs *rpc.GlobState
}

func (o *vGlobberObject) Globber() *rpc.GlobState {
	return o.gs
}

type allGlobberObject struct{}

//nolint:golint // API change required.
func (allGlobberObject) Glob__(*context.T, rpc.GlobServerCall, *glob.Glob) error {
	return nil
}

type childrenGlobberObject struct{}

//nolint:golint // API change required.
func (childrenGlobberObject) GlobChildren__(*context.T, rpc.GlobChildrenServerCall, *glob.Element) error {
	return nil
}

func TestReflectInvokerGlobber(t *testing.T) {
	allGlobber := allGlobberObject{}
	childrenGlobber := childrenGlobberObject{}
	gs := &rpc.GlobState{AllGlobber: allGlobber}
	vGlobber := &vGlobberObject{gs}

	testcases := []struct {
		obj      interface{}
		expected *rpc.GlobState
	}{
		{vGlobber, gs},
		{allGlobber, &rpc.GlobState{AllGlobber: allGlobber}},
		{childrenGlobber, &rpc.GlobState{ChildrenGlobber: childrenGlobber}},
	}

	for _, tc := range testcases {
		ri := rpc.ReflectInvokerOrDie(tc.obj)
		if got := ri.Globber(); !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("Unexpected result for %#v. Got %#v, want %#v", tc.obj, got, tc.expected)
		}
	}
}
