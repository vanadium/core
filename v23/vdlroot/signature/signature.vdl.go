// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: signature

// Package signature defines types representing interface and method signatures.
//
//nolint:revive
package signature

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
	vdlTypeStruct1   *vdl.Type = nil
	vdlTypeStruct2   *vdl.Type = nil
	vdlTypeStruct3   *vdl.Type = nil
	vdlTypeList4     *vdl.Type = nil
	vdlTypeOptional5 *vdl.Type = nil
	vdlTypeList6     *vdl.Type = nil
	vdlTypeStruct7   *vdl.Type = nil
	vdlTypeList8     *vdl.Type = nil
	vdlTypeList9     *vdl.Type = nil
)

// Type definitions
// ================
// Embed describes the signature of an embedded interface.
type Embed struct {
	Name    string
	PkgPath string
	Doc     string
}

func (Embed) VDLReflect(struct {
	Name string `vdl:"signature.Embed"`
}) {
}

func (x Embed) VDLIsZero() bool { //nolint:gocyclo
	return x == Embed{}
}

func (x Embed) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.PkgPath != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.PkgPath); err != nil {
			return err
		}
	}
	if x.Doc != "" {
		if err := enc.NextFieldValueString(2, vdl.StringType, x.Doc); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Embed) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Embed{}
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
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.PkgPath = value
			}
		case 2:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Doc = value
			}
		}
	}
}

// Arg describes the signature of a single argument.
type Arg struct {
	Name string
	Doc  string
	Type *vdl.Type // Type of the argument.
}

func (Arg) VDLReflect(struct {
	Name string `vdl:"signature.Arg"`
}) {
}

func (x Arg) VDLIsZero() bool { //nolint:gocyclo
	if x.Name != "" {
		return false
	}
	if x.Doc != "" {
		return false
	}
	if x.Type != nil && x.Type != vdl.AnyType {
		return false
	}
	return true
}

func (x Arg) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.Doc != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Doc); err != nil {
			return err
		}
	}
	if x.Type != nil && x.Type != vdl.AnyType {
		if err := enc.NextFieldValueTypeObject(2, x.Type); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Arg) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Arg{
		Type: vdl.AnyType,
	}
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
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Doc = value
			}
		case 2:
			switch value, err := dec.ReadValueTypeObject(); {
			case err != nil:
				return err
			default:
				x.Type = value
			}
		}
	}
}

// Method describes the signature of an interface method.
type Method struct {
	Name      string
	Doc       string
	InArgs    []Arg        // Input arguments
	OutArgs   []Arg        // Output arguments
	InStream  *Arg         // Input stream (optional)
	OutStream *Arg         // Output stream (optional)
	Tags      []*vdl.Value // Method tags
}

func (Method) VDLReflect(struct {
	Name string `vdl:"signature.Method"`
}) {
}

func (x Method) VDLIsZero() bool { //nolint:gocyclo
	if x.Name != "" {
		return false
	}
	if x.Doc != "" {
		return false
	}
	if len(x.InArgs) != 0 {
		return false
	}
	if len(x.OutArgs) != 0 {
		return false
	}
	if x.InStream != nil {
		return false
	}
	if x.OutStream != nil {
		return false
	}
	if len(x.Tags) != 0 {
		return false
	}
	return true
}

func (x Method) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.Doc != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Doc); err != nil {
			return err
		}
	}
	if len(x.InArgs) != 0 {
		if err := enc.NextField(2); err != nil {
			return err
		}
		if err := vdlWriteAnonList1(enc, x.InArgs); err != nil {
			return err
		}
	}
	if len(x.OutArgs) != 0 {
		if err := enc.NextField(3); err != nil {
			return err
		}
		if err := vdlWriteAnonList1(enc, x.OutArgs); err != nil {
			return err
		}
	}
	if x.InStream != nil {
		if err := enc.NextField(4); err != nil {
			return err
		}
		enc.SetNextStartValueIsOptional()
		if err := x.InStream.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.OutStream != nil {
		if err := enc.NextField(5); err != nil {
			return err
		}
		enc.SetNextStartValueIsOptional()
		if err := x.OutStream.VDLWrite(enc); err != nil {
			return err
		}
	}
	if len(x.Tags) != 0 {
		if err := enc.NextField(6); err != nil {
			return err
		}
		if err := vdlWriteAnonList2(enc, x.Tags); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList1(enc vdl.Encoder, x []Arg) error {
	if err := enc.StartValue(vdlTypeList4); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := elem.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList2(enc vdl.Encoder, x []*vdl.Value) error {
	if err := enc.StartValue(vdlTypeList6); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if elem == nil {
			if err := enc.NilValue(vdl.AnyType); err != nil {
				return err
			}
		} else {
			if err := elem.VDLWrite(enc); err != nil {
				return err
			}
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Method) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Method{}
	if err := dec.StartValue(vdlTypeStruct3); err != nil {
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
		if decType != vdlTypeStruct3 {
			index = vdlTypeStruct3.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Doc = value
			}
		case 2:
			if err := vdlReadAnonList1(dec, &x.InArgs); err != nil {
				return err
			}
		case 3:
			if err := vdlReadAnonList1(dec, &x.OutArgs); err != nil {
				return err
			}
		case 4:
			if err := dec.StartValue(vdlTypeOptional5); err != nil {
				return err
			}
			if dec.IsNil() {
				x.InStream = nil
				if err := dec.FinishValue(); err != nil {
					return err
				}
			} else {
				x.InStream = new(Arg)
				dec.IgnoreNextStartValue()
				if err := x.InStream.VDLRead(dec); err != nil {
					return err
				}
			}
		case 5:
			if err := dec.StartValue(vdlTypeOptional5); err != nil {
				return err
			}
			if dec.IsNil() {
				x.OutStream = nil
				if err := dec.FinishValue(); err != nil {
					return err
				}
			} else {
				x.OutStream = new(Arg)
				dec.IgnoreNextStartValue()
				if err := x.OutStream.VDLRead(dec); err != nil {
					return err
				}
			}
		case 6:
			if err := vdlReadAnonList2(dec, &x.Tags); err != nil {
				return err
			}
		}
	}
}

func vdlReadAnonList1(dec vdl.Decoder, x *[]Arg) error {
	if err := dec.StartValue(vdlTypeList4); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]Arg, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			var elem Arg
			if err := elem.VDLRead(dec); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
}

func vdlReadAnonList2(dec vdl.Decoder, x *[]*vdl.Value) error {
	if err := dec.StartValue(vdlTypeList6); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]*vdl.Value, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			var elem *vdl.Value
			elem = new(vdl.Value)
			if err := elem.VDLRead(dec); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
}

// Interface describes the signature of an interface.
type Interface struct {
	Name    string
	PkgPath string
	Doc     string
	Embeds  []Embed  // No special ordering.
	Methods []Method // Ordered by method name.
}

func (Interface) VDLReflect(struct {
	Name string `vdl:"signature.Interface"`
}) {
}

func (x Interface) VDLIsZero() bool { //nolint:gocyclo
	if x.Name != "" {
		return false
	}
	if x.PkgPath != "" {
		return false
	}
	if x.Doc != "" {
		return false
	}
	if len(x.Embeds) != 0 {
		return false
	}
	if len(x.Methods) != 0 {
		return false
	}
	return true
}

func (x Interface) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct7); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.PkgPath != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.PkgPath); err != nil {
			return err
		}
	}
	if x.Doc != "" {
		if err := enc.NextFieldValueString(2, vdl.StringType, x.Doc); err != nil {
			return err
		}
	}
	if len(x.Embeds) != 0 {
		if err := enc.NextField(3); err != nil {
			return err
		}
		if err := vdlWriteAnonList3(enc, x.Embeds); err != nil {
			return err
		}
	}
	if len(x.Methods) != 0 {
		if err := enc.NextField(4); err != nil {
			return err
		}
		if err := vdlWriteAnonList4(enc, x.Methods); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList3(enc vdl.Encoder, x []Embed) error {
	if err := enc.StartValue(vdlTypeList8); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := elem.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func vdlWriteAnonList4(enc vdl.Encoder, x []Method) error {
	if err := enc.StartValue(vdlTypeList9); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntry(false); err != nil {
			return err
		}
		if err := elem.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Interface) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Interface{}
	if err := dec.StartValue(vdlTypeStruct7); err != nil {
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
		if decType != vdlTypeStruct7 {
			index = vdlTypeStruct7.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.PkgPath = value
			}
		case 2:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Doc = value
			}
		case 3:
			if err := vdlReadAnonList3(dec, &x.Embeds); err != nil {
				return err
			}
		case 4:
			if err := vdlReadAnonList4(dec, &x.Methods); err != nil {
				return err
			}
		}
	}
}

func vdlReadAnonList3(dec vdl.Decoder, x *[]Embed) error {
	if err := dec.StartValue(vdlTypeList8); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]Embed, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			var elem Embed
			if err := elem.VDLRead(dec); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
}

func vdlReadAnonList4(dec vdl.Decoder, x *[]Method) error {
	if err := dec.StartValue(vdlTypeList9); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make([]Method, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, err := dec.NextEntry(); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			var elem Method
			if err := elem.VDLRead(dec); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
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
	vdl.Register((*Embed)(nil))
	vdl.Register((*Arg)(nil))
	vdl.Register((*Method)(nil))
	vdl.Register((*Interface)(nil))

	// Initialize type definitions.
	vdlTypeStruct1 = vdl.TypeOf((*Embed)(nil)).Elem()
	vdlTypeStruct2 = vdl.TypeOf((*Arg)(nil)).Elem()
	vdlTypeStruct3 = vdl.TypeOf((*Method)(nil)).Elem()
	vdlTypeList4 = vdl.TypeOf((*[]Arg)(nil))
	vdlTypeOptional5 = vdl.TypeOf((*Arg)(nil))
	vdlTypeList6 = vdl.TypeOf((*[]*vdl.Value)(nil))
	vdlTypeStruct7 = vdl.TypeOf((*Interface)(nil)).Elem()
	vdlTypeList8 = vdl.TypeOf((*[]Embed)(nil))
	vdlTypeList9 = vdl.TypeOf((*[]Method)(nil))

	return struct{}{}
}
