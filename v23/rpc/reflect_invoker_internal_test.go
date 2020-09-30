// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// reflect_invoker_internal_test uses internals of reflect_invoker (and hence is in the rpc package),
// whereas reflect_invoker_test is it the rpc_test package.

package rpc

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

// test tags.
var (
	tagAlpha   = vdl.StringValue(nil, "alpha")
	tagBeta    = vdl.StringValue(nil, "beta")
	tagGamma   = vdl.StringValue(nil, "gamma")
	tagDelta   = vdl.StringValue(nil, "delta")
	tagEpsilon = vdl.StringValue(nil, "epsilon")
)

var errApp = errors.New("app error")

type notags struct{}

func (o *notags) Method1(*context.T, ServerCall) error             { return nil }
func (o *notags) Method2(*context.T, ServerCall) (int, error)      { return 0, nil }
func (o *notags) Method3(*context.T, ServerCall, int) error        { return nil }
func (o *notags) Method4(*context.T, ServerCall, int) (int, error) { return 0, nil }
func (o *notags) Error(*context.T, ServerCall) error               { return errApp }

type tags struct{}

func (o *tags) Alpha(*context.T, ServerCall) error                               { return nil }
func (o *tags) Beta(*context.T, ServerCall) (int, error)                         { return 0, nil }
func (o *tags) Gamma(*context.T, ServerCall, int) error                          { return nil }
func (o *tags) Delta(*context.T, ServerCall, int) (int, error)                   { return 0, nil }
func (o *tags) Epsilon(*context.T, ServerCall, int, string) (int, string, error) { return 0, "", nil }
func (o *tags) Error(*context.T, ServerCall) error                               { return errApp }

//nolint:golint // API change required.
func (o *tags) Describe__() []InterfaceDesc {
	return []InterfaceDesc{{
		Methods: []MethodDesc{
			{Name: "Alpha", Tags: []*vdl.Value{tagAlpha}},
			{Name: "Beta", Tags: []*vdl.Value{tagBeta}},
			{Name: "Gamma", Tags: []*vdl.Value{tagGamma}},
			{Name: "Delta", Tags: []*vdl.Value{tagDelta}},
			{Name: "Epsilon", Tags: []*vdl.Value{tagEpsilon}},
		},
	}}
}

type (
	badcall        struct{}
	noInitCall     struct{ ServerCall }
	badInit1Call   struct{ ServerCall }
	badInit2Call   struct{ ServerCall }
	badInit3Call   struct{ ServerCall }
	noSendRecvCall struct{ ServerCall }
	badSend1Call   struct{ ServerCall }
	badSend2Call   struct{ ServerCall }
	badSend3Call   struct{ ServerCall }
	badRecv1Call   struct{ ServerCall }
	badRecv2Call   struct{ ServerCall }
	badRecv3Call   struct{ ServerCall }

	badoutargs struct{}

	badGlobber       struct{}
	badGlob1         struct{}
	badGlob2         struct{}
	badGlob3         struct{}
	badGlob4         struct{}
	badGlob5         struct{}
	badGlob6         struct{}
	badGlob7         struct{}
	badGlobChildren1 struct{}
	badGlobChildren2 struct{}
	badGlobChildren3 struct{}
	badGlobChildren4 struct{}
	badGlobChildren5 struct{}
	badGlobChildren6 struct{}
)

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

func (*badInit1Call) Init()                      {}
func (*badInit2Call) Init(int)                   {}
func (*badInit3Call) Init(StreamServerCall, int) {}
func (*noSendRecvCall) Init(StreamServerCall)    {}
func (*badSend1Call) Init(StreamServerCall)      {}
func (*badSend1Call) SendStream()                {}
func (*badSend2Call) Init(StreamServerCall)      {}
func (*badSend2Call) SendStream() interface {
	Send() error
} {
	return nil
}
func (*badSend3Call) Init(StreamServerCall) {}
func (*badSend3Call) SendStream() interface {
	Send(int)
} {
	return nil
}
func (*badRecv1Call) Init(StreamServerCall) {}
func (*badRecv1Call) RecvStream()           {}
func (*badRecv2Call) Init(StreamServerCall) {}
func (*badRecv2Call) RecvStream() interface {
	Advance() bool
	Value() int
} {
	return nil
}
func (*badRecv3Call) Init(StreamServerCall) {}
func (*badRecv3Call) RecvStream() interface {
	Advance()
	Value() int
	Error() error
} {
	return nil
}

func (badoutargs) NoFinalError1(*context.T, ServerCall)                {}
func (badoutargs) NoFinalError2(*context.T, ServerCall) string         { return "" }
func (badoutargs) NoFinalError3(*context.T, ServerCall) (bool, string) { return false, "" }

//nolint:golint // API change required.
func (badoutargs) NoFinalError4(*context.T, ServerCall) (error, string) { return nil, "" }

func (badGlobber) Globber() {}

//nolint:golint // API change required.
func (badGlob1) Glob__() {}

//nolint:golint // API change required.
func (badGlob2) Glob__(*context.T) {}

//nolint:golint // API change required.
func (badGlob3) Glob__(*context.T, ServerCall) {}

//nolint:golint // API change required.
func (badGlob4) Glob__(*context.T, ServerCall, string) {}

//nolint:golint // API change required.
func (badGlob5) Glob__(*context.T, ServerCall, string) <-chan naming.GlobReply { return nil }

//nolint:golint // API change required.
func (badGlob6) Glob__(*context.T, ServerCall, string) error { return nil }

//nolint:golint // API change required.
func (badGlob7) Glob__() (<-chan naming.GlobReply, error) { return nil, nil }

//nolint:golint // API change required.
func (badGlobChildren1) GlobChildren__() {}

//nolint:golint // API change required.
func (badGlobChildren2) GlobChildren__(*context.T) {}

//nolint:golint // API change required.
func (badGlobChildren3) GlobChildren__(*context.T, ServerCall) {}

//nolint:golint // API change required.
func (badGlobChildren4) GlobChildren__(*context.T, ServerCall) <-chan string { return nil }

//nolint:golint // API change required.
func (badGlobChildren5) GlobChildren__(*context.T, ServerCall) error { return nil }

//nolint:golint // API change required.
func (badGlobChildren6) GlobChildren__() (<-chan string, error) { return nil, nil }

const (
	badInit = `Init must have signature`
	badSend = `SendStream must have signature`
	badRecv = `RecvStream must have signature`
)

func TestTypeCheckMethods(t *testing.T) {
	type testcase struct {
		obj  interface{}
		want map[string]string
	}
	tests := []testcase{
		{struct{}{}, nil},
		{&notags{}, map[string]string{
			"Method1": "",
			"Method2": "",
			"Method3": "",
			"Method4": "",
			"Error":   "",
		}},
		{&tags{}, map[string]string{
			"Alpha":      "",
			"Beta":       "",
			"Gamma":      "",
			"Delta":      "",
			"Epsilon":    "",
			"Error":      "",
			"Describe__": verror.ErrInternal.Errorf(nil, "internal error: %v", errReservedMethod).Error(),
		}},

		{badcall{}, map[string]string{
			"NonRPC1":    errNonRPCMethod("NonRPC1").Error(),
			"NonRPC2":    errNonRPCMethod("NonRPC2").Error(),
			"NonRPC3":    errNonRPCMethod("NonRPC3").Error(),
			"NonRPC4":    errNonRPCMethod("NonRPC4").Error(),
			"NonRPC5":    errNonRPCMethod("NonRPC5").Error(),
			"NonRPC6":    errNonRPCMethod("NonRPC6").Error(),
			"NoInit":     "must have Init method",
			"BadInit1":   badInit,
			"BadInit2":   badInit,
			"BadInit3":   badInit,
			"NoSendRecv": "must have at least one of RecvStream or SendStream",
			"BadSend1":   badSend,
			"BadSend2":   badSend,
			"BadSend3":   badSend,
			"BadRecv1":   badRecv,
			"BadRecv2":   badRecv,
			"BadRecv3":   badRecv,
		}},
		{badoutargs{}, map[string]string{
			"NoFinalError1": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError2": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError3": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError4": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
		}},

		{badGlobber{}, map[string]string{
			"Globber": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobber).Error(),
		}},
		{badGlob1{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob2{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob3{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob4{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob5{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob6{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob7{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlobChildren1{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren2{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren3{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren4{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren5{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren6{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},

		// Test for backwards compatibility.
		{&tags{}, map[string]string{
			"Alpha":      "",
			"Beta":       "",
			"Gamma":      "",
			"Delta":      "",
			"Epsilon":    "",
			"Error":      "",
			"Describe__": verror.ErrInternal.Errorf(nil, "internal error: %v", errReservedMethod).Error(),
		}},
		{badoutargs{}, map[string]string{
			"NoFinalError1": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError2": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError3": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
			"NoFinalError4": verror.ErrAborted.Errorf(nil, "aborted: %v", errNoFinalErrorOutArg).Error(),
		}},
		{badGlobber{}, map[string]string{
			"Globber": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobber).Error(),
		}},
		{badGlob1{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob2{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob3{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob4{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob5{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob6{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlob7{}, map[string]string{
			"Glob__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlob).Error(),
		}},
		{badGlobChildren1{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren2{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren3{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren4{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren5{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
		{badGlobChildren6{}, map[string]string{
			"GlobChildren__": verror.ErrAborted.Errorf(nil, "aborted: %v", errBadGlobChildren).Error(),
		}},
	}

	for _, test := range tests {
		typecheck := TypeCheckMethods(test.obj)
		if got, want := typecheck, test.want; (got == nil) != (want == nil) {
			t.Errorf("TypeCheckMethods(%T) got %v, want %v", test.obj, got, want)
		}
		if got, want := len(typecheck), len(test.want); got != want {
			t.Errorf("TypeCheckMethods(%T) got len %d, want %d (got %q, want %q)", test.obj, got, want, typecheck, test.want)
		}
		for wantKey, wantVal := range test.want {
			gotVal, ok := typecheck[wantKey]
			if !ok {
				t.Errorf("TypeCheckMethods(%T) got %v, want key %q", test.obj, typecheck, wantKey)
			}
			if wantVal == "" && gotVal != nil {
				t.Errorf("TypeCheckMethods(%T) got method %q %q, want %q", test.obj, wantKey, gotVal, wantVal)
			}
			if got, want := strings.ToLower(fmt.Sprint(gotVal)), strings.ToLower(wantVal); !strings.Contains(got, want) {
				t.Errorf("TypeCheckMethods(%T) got method %q %q, want substr %q", test.obj, wantKey, got, want)
			}
		}
	}

}
