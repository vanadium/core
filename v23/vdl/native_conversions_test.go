// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl_test

import (
	"bytes"
	"os"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

type verrorConverter struct {
	toNative, fromNative bool
}

func (c *verrorConverter) ToNative(wire, native interface{}) error {
	c.toNative = true
	return verror.WireToNative(wire.(*vdl.WireError), native.(*error))
}

func (c *verrorConverter) FromNative(wire, native interface{}) error {
	c.fromNative = true
	return verror.WireFromNative(wire.(**vdl.WireError), native.(error))
}

func TestVerror(t *testing.T) {
	// Make sure to restore the original converter.
	defer vdl.RegisterNativeConverter(&vdl.WireError{}, verror.VDLNativeConverter{})

	ve1 := verror.NewIDAction("someNewError", verror.RetryBackoff)
	converter := &verrorConverter{}
	testSingleError := func() {
		verr := ve1.Errorf(nil, "oh my: %v", os.ErrClosed)
		var err error
		if err := vdl.Convert(&err, verr); err != nil {
			t.Fatal(err)
		}
		cerr, ok := err.(*verror.E)
		if !ok {
			t.Fatalf("type conversion to *verror.E failed for %T", err)
		}
		cverr := err.(*verror.E)
		if got, want := cerr.ID, cverr.ID; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := cerr.Action, cverr.Action; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := cerr.Msg, cverr.Msg; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := len(cerr.ParamList), 3; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := cerr.ParamList[0], "vdl.test"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := cerr.ParamList[1], ""; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := cerr.ParamList[2].(error).Error(), "file already closed"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	converterAssert := func(c *verrorConverter, val bool) {
		if got, want := c.toNative, val; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := c.fromNative, val; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Use conversions installed by the verror package.
	testSingleError()
	converterAssert(converter, false)
	// Use our test converter above.
	vdl.RegisterNativeConverter(&vdl.WireError{}, converter)
	testSingleError()
	converterAssert(converter, true)

	converter = &verrorConverter{}
	vdl.RegisterNativeConverter(&vdl.WireError{}, nil)
	testSingleError()
	converterAssert(converter, false)

}

type (
	nativeTestType struct {
		A int
	}
	nativeTestTypePtr struct {
		called bool
		A      int
	}

	nativeTestTypeStruct struct {
		A nativeTestType
		B nativeTestTypePtr
		C *nativeTestTypePtr
	}
)

var nativeTestTypeCalled bool

func (n nativeTestType) IsZero() bool {
	nativeTestTypeCalled = true
	return n.A == 0
}

func (n *nativeTestTypePtr) IsZero() bool {
	n.called = true
	return n.A == 0
}

func init() {
	vdl.RegisterNativeIsZero(nativeTestType{})
	vdl.RegisterNativeIsZero(&nativeTestTypePtr{})
}

func TestIsZero(t *testing.T) {
	wr := &bytes.Buffer{}
	enc := vom.NewEncoder(wr)
	nt := nativeTestTypeStruct{
		A: nativeTestType{A: 1},
		B: nativeTestTypePtr{A: 2},
		C: &nativeTestTypePtr{A: 3},
	}
	if err := enc.Encode(&nt); err != nil {
		t.Fatal(err)
	}
	if !nativeTestTypeCalled {
		t.Errorf("IsZero was not called for nativeTestType")
	}
	if !nt.B.called {
		t.Errorf("IsZero was not called for nativeTestTypePtr")
	}
	if nt.C.called {
		// IsZero is not called for optional types, i.e. pointers.
		t.Errorf("IsZero was called for *nativeTestTypePtr")
	}
}
