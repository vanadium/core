// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math_test

import (
	"testing"

	"v.io/v23/vom"
)

func TestComplex64EncodeDecode(t *testing.T) {
	cplxIn := complex64(-1 - 3i)
	data, err := vom.Encode(cplxIn)
	if err != nil {
		t.Fatalf("error encoding %T: %v", cplxIn, err)
	}
	var out interface{}
	if err := vom.Decode(data, &out); err != nil {
		t.Fatalf("error decoding %T: %v", cplxIn, err)
	}
	if cplxOut, ok := out.(complex64); ok {
		if got, want := cplxOut, cplxIn; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	} else {
		t.Fatalf("incorrect output type: got %T, want %T", out, cplxIn)
	}
}

func TestComplex128EncodeDecode(t *testing.T) {
	cplxIn := complex128(-1 - 3i)
	data, err := vom.Encode(cplxIn)
	if err != nil {
		t.Fatalf("error encoding %T: %v", cplxIn, err)
	}
	var out interface{}
	if err := vom.Decode(data, &out); err != nil {
		t.Fatalf("error decoding %T: %v", cplxIn, err)
	}
	if cplxOut, ok := out.(complex128); ok {
		if got, want := cplxOut, cplxIn; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	} else {
		t.Fatalf("incorrect output type: got %T, want %T", out, cplxIn)
	}
}
