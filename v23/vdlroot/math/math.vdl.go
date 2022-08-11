// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: math

//nolint:revive
package math

import (
	"v.io/v23/vdl"
)

var initializeVDLCalled = false
var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Hold type definitions in package-level variables, for better performance.
// Declare and initialize with default values here so that the initializeVDL
// method will be considered ready to initialize before any of the type
// definitions that appear below.
//
//nolint:unused
var (
	vdlTypeStruct1 *vdl.Type = nil
	vdlTypeStruct2 *vdl.Type = nil
)

// Type definitions
// ================
// Complex64 is a complex number composed of 32-bit real and imaginary parts.
type Complex64 struct {
	Real float32
	Imag float32
}

func (Complex64) VDLReflect(struct {
	Name string `vdl:"math.Complex64"`
}) {
}

func (x Complex64) VDLIsZero() bool { //nolint:gocyclo
	return x == Complex64{}
}

func (x Complex64) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	if x.Real != 0 {
		if err := enc.NextFieldValueFloat(0, vdl.Float32Type, float64(x.Real)); err != nil {
			return err
		}
	}
	if x.Imag != 0 {
		if err := enc.NextFieldValueFloat(1, vdl.Float32Type, float64(x.Imag)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Complex64) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Complex64{}
	if err := dec.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct1 {
			index = vdlTypeStruct1.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueFloat(32); {
			case err != nil:
				return err
			default:
				x.Real = float32(value)
			}
		case 1:
			switch value, err := dec.ReadValueFloat(32); {
			case err != nil:
				return err
			default:
				x.Imag = float32(value)
			}
		}
	}
}

// Complex128 is a complex number composed of 64-bit real and imaginary parts.
type Complex128 struct {
	Real float64
	Imag float64
}

func (Complex128) VDLReflect(struct {
	Name string `vdl:"math.Complex128"`
}) {
}

func (x Complex128) VDLIsZero() bool { //nolint:gocyclo
	return x == Complex128{}
}

func (x Complex128) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	if x.Real != 0 {
		if err := enc.NextFieldValueFloat(0, vdl.Float64Type, x.Real); err != nil {
			return err
		}
	}
	if x.Imag != 0 {
		if err := enc.NextFieldValueFloat(1, vdl.Float64Type, x.Imag); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Complex128) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Complex128{}
	if err := dec.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct2 {
			index = vdlTypeStruct2.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueFloat(64); {
			case err != nil:
				return err
			default:
				x.Real = value
			}
		case 1:
			switch value, err := dec.ReadValueFloat(64); {
			case err != nil:
				return err
			default:
				x.Imag = value
			}
		}
	}
}

// Type-check native conversion functions.
var (
	_ func(Complex128, *complex128) error = Complex128ToNative
	_ func(*Complex128, complex128) error = Complex128FromNative
	_ func(Complex64, *complex64) error   = Complex64ToNative
	_ func(*Complex64, complex64) error   = Complex64FromNative
)

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

	// Register native type conversions first, so that vdl.TypeOf works.
	vdl.RegisterNative(Complex128ToNative, Complex128FromNative)
	vdl.RegisterNative(Complex64ToNative, Complex64FromNative)

	// Register types.
	vdl.Register((*Complex64)(nil))
	vdl.Register((*Complex128)(nil))

	// Initialize type definitions.
	vdlTypeStruct1 = vdl.TypeOf((*Complex64)(nil)).Elem()
	vdlTypeStruct2 = vdl.TypeOf((*Complex128)(nil)).Elem()

	return struct{}{}
}
