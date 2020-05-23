// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: nativedep2

package nativedep2

import (
	"time"
	"v.io/v23/vdl"
	_ "v.io/x/ref/lib/vdl/testdata/nativetest"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

//////////////////////////////////////////////////
// Type definitions

type MyTime time.Time

func (MyTime) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/lib/vdl/testdata/nativedep2.MyTime"`
}) {
}

func (x MyTime) VDLIsZero() bool { //nolint:gocyclo
	return x == 0
}

func (x MyTime) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueInt(vdlTypeInt321, int64(x)); err != nil {
		return err
	}
	return nil
}

func (x *MyTime) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueInt(32); {
	case err != nil:
		return err
	default:
		*x = MyTime(value)
	}
	return nil
}

// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (
	vdlTypeInt321 *vdl.Type
)

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

	// Register types.
	vdl.Register((*MyTime)(nil))

	// Initialize type definitions.
	vdlTypeInt321 = vdl.TypeOf((*MyTime)(nil))

	return struct{}{}
}
