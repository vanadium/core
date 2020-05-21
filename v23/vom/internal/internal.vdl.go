// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: internal

package internal

import (
	"fmt"
	"v.io/v23/vdl"
	"v.io/v23/vom"
)

var _ = __VDLInit() // Must be first; see __VDLInit comments for details.

//////////////////////////////////////////////////
// Type definitions

type VNumber int32

func (VNumber) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VNumber"`
}) {
}

func (x VNumber) VDLIsZero() bool {
	return x == 0
}

func (x VNumber) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueInt(__VDLType_int32_1, int64(x)); err != nil {
		return err
	}
	return nil
}

func (x *VNumber) VDLRead(dec vdl.Decoder) error {
	switch value, err := dec.ReadValueInt(32); {
	case err != nil:
		return err
	default:
		*x = VNumber(value)
	}
	return nil
}

type VString string

func (VString) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VString"`
}) {
}

func (x VString) VDLIsZero() bool {
	return x == ""
}

func (x VString) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueString(__VDLType_string_2, string(x)); err != nil {
		return err
	}
	return nil
}

func (x *VString) VDLRead(dec vdl.Decoder) error {
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		*x = VString(value)
	}
	return nil
}

type VEnum int

const (
	VEnumA VEnum = iota
	VEnumB
	VEnumC
)

// VEnumAll holds all labels for VEnum.
var VEnumAll = [...]VEnum{VEnumA, VEnumB, VEnumC}

// VEnumFromString creates a VEnum from a string label.
// nolint: deadcode, unused
func VEnumFromString(label string) (x VEnum, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *VEnum) Set(label string) error {
	switch label {
	case "A", "a":
		*x = VEnumA
		return nil
	case "B", "b":
		*x = VEnumB
		return nil
	case "C", "c":
		*x = VEnumC
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in internal.VEnum", label)
}

// String returns the string label of x.
func (x VEnum) String() string {
	switch x {
	case VEnumA:
		return "A"
	case VEnumB:
		return "B"
	case VEnumC:
		return "C"
	}
	return ""
}

func (VEnum) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VEnum"`
	Enum struct{ A, B, C string }
}) {
}

func (x VEnum) VDLIsZero() bool {
	return x == VEnumA
}

func (x VEnum) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueString(__VDLType_enum_3, x.String()); err != nil {
		return err
	}
	return nil
}

func (x *VEnum) VDLRead(dec vdl.Decoder) error {
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		if err := x.Set(value); err != nil {
			return err
		}
	}
	return nil
}

type VByteList []byte

func (VByteList) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VByteList"`
}) {
}

func (x VByteList) VDLIsZero() bool {
	return len(x) == 0
}

func (x VByteList) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueBytes(__VDLType_list_4, []byte(x)); err != nil {
		return err
	}
	return nil
}

func (x *VByteList) VDLRead(dec vdl.Decoder) error {
	var bytes []byte
	if err := dec.ReadValueBytes(-1, &bytes); err != nil {
		return err
	}
	*x = bytes
	return nil
}

type VByteArray [3]byte

func (VByteArray) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VByteArray"`
}) {
}

func (x VByteArray) VDLIsZero() bool {
	return x == VByteArray{}
}

func (x VByteArray) VDLWrite(enc vdl.Encoder) error {
	if err := enc.WriteValueBytes(__VDLType_array_5, x[:]); err != nil {
		return err
	}
	return nil
}

func (x *VByteArray) VDLRead(dec vdl.Decoder) error {
	bytes := x[:]
	if err := dec.ReadValueBytes(3, &bytes); err != nil {
		return err
	}
	return nil
}

type VArray [3]int32

func (VArray) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VArray"`
}) {
}

func (x VArray) VDLIsZero() bool {
	return x == VArray{}
}

func (x VArray) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_array_6); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntryValueInt(vdl.Int32Type, int64(elem)); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VArray) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(__VDLType_array_6); err != nil {
		return err
	}
	for index := 0; index < 3; index++ {
		switch done, elem, err := dec.NextEntryValueInt(32); {
		case err != nil:
			return err
		case done:
			return fmt.Errorf("short array, got len %d < 3 %T)", index, *x)
		default:
			x[index] = int32(elem)
		}
	}
	switch done, err := dec.NextEntry(); {
	case err != nil:
		return err
	case !done:
		return fmt.Errorf("long array, got len > 3 %T", *x)
	}
	return dec.FinishValue()
}

type VList []int32

func (VList) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VList"`
}) {
}

func (x VList) VDLIsZero() bool {
	return len(x) == 0
}

func (x VList) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_list_7); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for _, elem := range x {
		if err := enc.NextEntryValueInt(vdl.Int32Type, int64(elem)); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VList) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(__VDLType_list_7); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make(VList, 0, len)
	} else {
		*x = nil
	}
	for {
		switch done, elem, err := dec.NextEntryValueInt(32); {
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:
			*x = append(*x, int32(elem))
		}
	}
}

type VListAny []*vom.RawBytes

func (VListAny) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VListAny"`
}) {
}

func (x VListAny) VDLIsZero() bool {
	return len(x) == 0
}

func (x VListAny) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_list_8); err != nil {
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

func (x *VListAny) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(__VDLType_list_8); err != nil {
		return err
	}
	if len := dec.LenHint(); len > 0 {
		*x = make(VListAny, 0, len)
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
			var elem *vom.RawBytes
			elem = new(vom.RawBytes)
			if err := elem.VDLRead(dec); err != nil {
				return err
			}
			*x = append(*x, elem)
		}
	}
}

type VSet map[string]struct{}

func (VSet) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VSet"`
}) {
}

func (x VSet) VDLIsZero() bool {
	return len(x) == 0
}

func (x VSet) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_set_9); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for key := range x {
		if err := enc.NextEntryValueString(vdl.StringType, key); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VSet) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(__VDLType_set_9); err != nil {
		return err
	}
	var tmpMap VSet
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(VSet, len)
	}
	for {
		switch done, key, err := dec.NextEntryValueString(); {
		case err != nil:
			return err
		case done:
			*x = tmpMap
			return dec.FinishValue()
		default:
			if tmpMap == nil {
				tmpMap = make(VSet)
			}
			tmpMap[key] = struct{}{}
		}
	}
}

type VMap map[string]bool

func (VMap) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VMap"`
}) {
}

func (x VMap) VDLIsZero() bool {
	return len(x) == 0
}

func (x VMap) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_map_10); err != nil {
		return err
	}
	if err := enc.SetLenHint(len(x)); err != nil {
		return err
	}
	for key, elem := range x {
		if err := enc.NextEntryValueString(vdl.StringType, key); err != nil {
			return err
		}
		if err := enc.WriteValueBool(vdl.BoolType, elem); err != nil {
			return err
		}
	}
	if err := enc.NextEntry(true); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VMap) VDLRead(dec vdl.Decoder) error {
	if err := dec.StartValue(__VDLType_map_10); err != nil {
		return err
	}
	var tmpMap VMap
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(VMap, len)
	}
	for {
		switch done, key, err := dec.NextEntryValueString(); {
		case err != nil:
			return err
		case done:
			*x = tmpMap
			return dec.FinishValue()
		default:
			var elem bool
			switch value, err := dec.ReadValueBool(); {
			case err != nil:
				return err
			default:
				elem = value
			}
			if tmpMap == nil {
				tmpMap = make(VMap)
			}
			tmpMap[key] = elem
		}
	}
}

type VSmallStruct struct {
	A int32
	B string
	C bool
}

func (VSmallStruct) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VSmallStruct"`
}) {
}

func (x VSmallStruct) VDLIsZero() bool {
	return x == VSmallStruct{}
}

func (x VSmallStruct) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_11); err != nil {
		return err
	}
	if x.A != 0 {
		if err := enc.NextFieldValueInt(0, vdl.Int32Type, int64(x.A)); err != nil {
			return err
		}
	}
	if x.B != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.B); err != nil {
			return err
		}
	}
	if x.C {
		if err := enc.NextFieldValueBool(2, vdl.BoolType, x.C); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VSmallStruct) VDLRead(dec vdl.Decoder) error {
	*x = VSmallStruct{}
	if err := dec.StartValue(__VDLType_struct_11); err != nil {
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
		if decType != __VDLType_struct_11 {
			index = __VDLType_struct_11.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.A = int32(value)
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.B = value
			}
		case 2:
			switch value, err := dec.ReadValueBool(); {
			case err != nil:
				return err
			default:
				x.C = value
			}
		}
	}
}

type VLargeStruct struct {
	F1  int32
	F2  int32
	F3  int32
	F4  int32
	F5  int32
	F6  int32
	F7  int32
	F8  int32
	F9  int32
	F10 int32
	F11 int32
	F12 int32
	F13 int32
	F14 int32
	F15 int32
	F16 int32
	F17 int32
	F18 int32
	F19 int32
	F20 int32
	F21 int32
	F22 int32
	F23 int32
	F24 int32
	F25 int32
	F26 int32
	F27 int32
	F28 int32
	F29 int32
	F30 int32
	F31 int32
	F32 int32
	F33 int32
	F34 int32
	F35 int32
	F36 int32
	F37 int32
	F38 int32
	F39 int32
	F40 int32
	F41 int32
	F42 int32
	F43 int32
	F44 int32
	F45 int32
	F46 int32
	F47 int32
	F48 int32
	F49 int32
	F50 int32
}

func (VLargeStruct) VDLReflect(struct {
	Name string `vdl:"v.io/v23/vom/internal.VLargeStruct"`
}) {
}

func (x VLargeStruct) VDLIsZero() bool {
	return x == VLargeStruct{}
}

func (x VLargeStruct) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_struct_12); err != nil {
		return err
	}
	if x.F1 != 0 {
		if err := enc.NextFieldValueInt(0, vdl.Int32Type, int64(x.F1)); err != nil {
			return err
		}
	}
	if x.F2 != 0 {
		if err := enc.NextFieldValueInt(1, vdl.Int32Type, int64(x.F2)); err != nil {
			return err
		}
	}
	if x.F3 != 0 {
		if err := enc.NextFieldValueInt(2, vdl.Int32Type, int64(x.F3)); err != nil {
			return err
		}
	}
	if x.F4 != 0 {
		if err := enc.NextFieldValueInt(3, vdl.Int32Type, int64(x.F4)); err != nil {
			return err
		}
	}
	if x.F5 != 0 {
		if err := enc.NextFieldValueInt(4, vdl.Int32Type, int64(x.F5)); err != nil {
			return err
		}
	}
	if x.F6 != 0 {
		if err := enc.NextFieldValueInt(5, vdl.Int32Type, int64(x.F6)); err != nil {
			return err
		}
	}
	if x.F7 != 0 {
		if err := enc.NextFieldValueInt(6, vdl.Int32Type, int64(x.F7)); err != nil {
			return err
		}
	}
	if x.F8 != 0 {
		if err := enc.NextFieldValueInt(7, vdl.Int32Type, int64(x.F8)); err != nil {
			return err
		}
	}
	if x.F9 != 0 {
		if err := enc.NextFieldValueInt(8, vdl.Int32Type, int64(x.F9)); err != nil {
			return err
		}
	}
	if x.F10 != 0 {
		if err := enc.NextFieldValueInt(9, vdl.Int32Type, int64(x.F10)); err != nil {
			return err
		}
	}
	if x.F11 != 0 {
		if err := enc.NextFieldValueInt(10, vdl.Int32Type, int64(x.F11)); err != nil {
			return err
		}
	}
	if x.F12 != 0 {
		if err := enc.NextFieldValueInt(11, vdl.Int32Type, int64(x.F12)); err != nil {
			return err
		}
	}
	if x.F13 != 0 {
		if err := enc.NextFieldValueInt(12, vdl.Int32Type, int64(x.F13)); err != nil {
			return err
		}
	}
	if x.F14 != 0 {
		if err := enc.NextFieldValueInt(13, vdl.Int32Type, int64(x.F14)); err != nil {
			return err
		}
	}
	if x.F15 != 0 {
		if err := enc.NextFieldValueInt(14, vdl.Int32Type, int64(x.F15)); err != nil {
			return err
		}
	}
	if x.F16 != 0 {
		if err := enc.NextFieldValueInt(15, vdl.Int32Type, int64(x.F16)); err != nil {
			return err
		}
	}
	if x.F17 != 0 {
		if err := enc.NextFieldValueInt(16, vdl.Int32Type, int64(x.F17)); err != nil {
			return err
		}
	}
	if x.F18 != 0 {
		if err := enc.NextFieldValueInt(17, vdl.Int32Type, int64(x.F18)); err != nil {
			return err
		}
	}
	if x.F19 != 0 {
		if err := enc.NextFieldValueInt(18, vdl.Int32Type, int64(x.F19)); err != nil {
			return err
		}
	}
	if x.F20 != 0 {
		if err := enc.NextFieldValueInt(19, vdl.Int32Type, int64(x.F20)); err != nil {
			return err
		}
	}
	if x.F21 != 0 {
		if err := enc.NextFieldValueInt(20, vdl.Int32Type, int64(x.F21)); err != nil {
			return err
		}
	}
	if x.F22 != 0 {
		if err := enc.NextFieldValueInt(21, vdl.Int32Type, int64(x.F22)); err != nil {
			return err
		}
	}
	if x.F23 != 0 {
		if err := enc.NextFieldValueInt(22, vdl.Int32Type, int64(x.F23)); err != nil {
			return err
		}
	}
	if x.F24 != 0 {
		if err := enc.NextFieldValueInt(23, vdl.Int32Type, int64(x.F24)); err != nil {
			return err
		}
	}
	if x.F25 != 0 {
		if err := enc.NextFieldValueInt(24, vdl.Int32Type, int64(x.F25)); err != nil {
			return err
		}
	}
	if x.F26 != 0 {
		if err := enc.NextFieldValueInt(25, vdl.Int32Type, int64(x.F26)); err != nil {
			return err
		}
	}
	if x.F27 != 0 {
		if err := enc.NextFieldValueInt(26, vdl.Int32Type, int64(x.F27)); err != nil {
			return err
		}
	}
	if x.F28 != 0 {
		if err := enc.NextFieldValueInt(27, vdl.Int32Type, int64(x.F28)); err != nil {
			return err
		}
	}
	if x.F29 != 0 {
		if err := enc.NextFieldValueInt(28, vdl.Int32Type, int64(x.F29)); err != nil {
			return err
		}
	}
	if x.F30 != 0 {
		if err := enc.NextFieldValueInt(29, vdl.Int32Type, int64(x.F30)); err != nil {
			return err
		}
	}
	if x.F31 != 0 {
		if err := enc.NextFieldValueInt(30, vdl.Int32Type, int64(x.F31)); err != nil {
			return err
		}
	}
	if x.F32 != 0 {
		if err := enc.NextFieldValueInt(31, vdl.Int32Type, int64(x.F32)); err != nil {
			return err
		}
	}
	if x.F33 != 0 {
		if err := enc.NextFieldValueInt(32, vdl.Int32Type, int64(x.F33)); err != nil {
			return err
		}
	}
	if x.F34 != 0 {
		if err := enc.NextFieldValueInt(33, vdl.Int32Type, int64(x.F34)); err != nil {
			return err
		}
	}
	if x.F35 != 0 {
		if err := enc.NextFieldValueInt(34, vdl.Int32Type, int64(x.F35)); err != nil {
			return err
		}
	}
	if x.F36 != 0 {
		if err := enc.NextFieldValueInt(35, vdl.Int32Type, int64(x.F36)); err != nil {
			return err
		}
	}
	if x.F37 != 0 {
		if err := enc.NextFieldValueInt(36, vdl.Int32Type, int64(x.F37)); err != nil {
			return err
		}
	}
	if x.F38 != 0 {
		if err := enc.NextFieldValueInt(37, vdl.Int32Type, int64(x.F38)); err != nil {
			return err
		}
	}
	if x.F39 != 0 {
		if err := enc.NextFieldValueInt(38, vdl.Int32Type, int64(x.F39)); err != nil {
			return err
		}
	}
	if x.F40 != 0 {
		if err := enc.NextFieldValueInt(39, vdl.Int32Type, int64(x.F40)); err != nil {
			return err
		}
	}
	if x.F41 != 0 {
		if err := enc.NextFieldValueInt(40, vdl.Int32Type, int64(x.F41)); err != nil {
			return err
		}
	}
	if x.F42 != 0 {
		if err := enc.NextFieldValueInt(41, vdl.Int32Type, int64(x.F42)); err != nil {
			return err
		}
	}
	if x.F43 != 0 {
		if err := enc.NextFieldValueInt(42, vdl.Int32Type, int64(x.F43)); err != nil {
			return err
		}
	}
	if x.F44 != 0 {
		if err := enc.NextFieldValueInt(43, vdl.Int32Type, int64(x.F44)); err != nil {
			return err
		}
	}
	if x.F45 != 0 {
		if err := enc.NextFieldValueInt(44, vdl.Int32Type, int64(x.F45)); err != nil {
			return err
		}
	}
	if x.F46 != 0 {
		if err := enc.NextFieldValueInt(45, vdl.Int32Type, int64(x.F46)); err != nil {
			return err
		}
	}
	if x.F47 != 0 {
		if err := enc.NextFieldValueInt(46, vdl.Int32Type, int64(x.F47)); err != nil {
			return err
		}
	}
	if x.F48 != 0 {
		if err := enc.NextFieldValueInt(47, vdl.Int32Type, int64(x.F48)); err != nil {
			return err
		}
	}
	if x.F49 != 0 {
		if err := enc.NextFieldValueInt(48, vdl.Int32Type, int64(x.F49)); err != nil {
			return err
		}
	}
	if x.F50 != 0 {
		if err := enc.NextFieldValueInt(49, vdl.Int32Type, int64(x.F50)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *VLargeStruct) VDLRead(dec vdl.Decoder) error {
	*x = VLargeStruct{}
	if err := dec.StartValue(__VDLType_struct_12); err != nil {
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
		if decType != __VDLType_struct_12 {
			index = __VDLType_struct_12.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F1 = int32(value)
			}
		case 1:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F2 = int32(value)
			}
		case 2:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F3 = int32(value)
			}
		case 3:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F4 = int32(value)
			}
		case 4:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F5 = int32(value)
			}
		case 5:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F6 = int32(value)
			}
		case 6:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F7 = int32(value)
			}
		case 7:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F8 = int32(value)
			}
		case 8:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F9 = int32(value)
			}
		case 9:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F10 = int32(value)
			}
		case 10:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F11 = int32(value)
			}
		case 11:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F12 = int32(value)
			}
		case 12:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F13 = int32(value)
			}
		case 13:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F14 = int32(value)
			}
		case 14:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F15 = int32(value)
			}
		case 15:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F16 = int32(value)
			}
		case 16:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F17 = int32(value)
			}
		case 17:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F18 = int32(value)
			}
		case 18:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F19 = int32(value)
			}
		case 19:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F20 = int32(value)
			}
		case 20:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F21 = int32(value)
			}
		case 21:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F22 = int32(value)
			}
		case 22:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F23 = int32(value)
			}
		case 23:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F24 = int32(value)
			}
		case 24:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F25 = int32(value)
			}
		case 25:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F26 = int32(value)
			}
		case 26:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F27 = int32(value)
			}
		case 27:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F28 = int32(value)
			}
		case 28:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F29 = int32(value)
			}
		case 29:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F30 = int32(value)
			}
		case 30:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F31 = int32(value)
			}
		case 31:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F32 = int32(value)
			}
		case 32:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F33 = int32(value)
			}
		case 33:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F34 = int32(value)
			}
		case 34:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F35 = int32(value)
			}
		case 35:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F36 = int32(value)
			}
		case 36:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F37 = int32(value)
			}
		case 37:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F38 = int32(value)
			}
		case 38:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F39 = int32(value)
			}
		case 39:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F40 = int32(value)
			}
		case 40:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F41 = int32(value)
			}
		case 41:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F42 = int32(value)
			}
		case 42:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F43 = int32(value)
			}
		case 43:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F44 = int32(value)
			}
		case 44:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F45 = int32(value)
			}
		case 45:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F46 = int32(value)
			}
		case 46:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F47 = int32(value)
			}
		case 47:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F48 = int32(value)
			}
		case 48:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F49 = int32(value)
			}
		case 49:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.F50 = int32(value)
			}
		}
	}
}

type (
	// VSmallUnion represents any single field of the VSmallUnion union type.
	VSmallUnion interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// VDLReflect describes the VSmallUnion union type.
		VDLReflect(__VSmallUnionReflect)
		VDLIsZero() bool
		VDLWrite(vdl.Encoder) error
	}
	// VSmallUnionA represents field A of the VSmallUnion union type.
	VSmallUnionA struct{ Value int32 }
	// VSmallUnionB represents field B of the VSmallUnion union type.
	VSmallUnionB struct{ Value string }
	// VSmallUnionC represents field C of the VSmallUnion union type.
	VSmallUnionC struct{ Value bool }
	// __VSmallUnionReflect describes the VSmallUnion union type.
	__VSmallUnionReflect struct {
		Name  string `vdl:"v.io/v23/vom/internal.VSmallUnion"`
		Type  VSmallUnion
		Union struct {
			A VSmallUnionA
			B VSmallUnionB
			C VSmallUnionC
		}
	}
)

func (x VSmallUnionA) Index() int                      { return 0 }
func (x VSmallUnionA) Interface() interface{}          { return x.Value }
func (x VSmallUnionA) Name() string                    { return "A" }
func (x VSmallUnionA) VDLReflect(__VSmallUnionReflect) {}

func (x VSmallUnionB) Index() int                      { return 1 }
func (x VSmallUnionB) Interface() interface{}          { return x.Value }
func (x VSmallUnionB) Name() string                    { return "B" }
func (x VSmallUnionB) VDLReflect(__VSmallUnionReflect) {}

func (x VSmallUnionC) Index() int                      { return 2 }
func (x VSmallUnionC) Interface() interface{}          { return x.Value }
func (x VSmallUnionC) Name() string                    { return "C" }
func (x VSmallUnionC) VDLReflect(__VSmallUnionReflect) {}

func (x VSmallUnionA) VDLIsZero() bool {
	return x.Value == 0
}

func (x VSmallUnionB) VDLIsZero() bool {
	return false
}

func (x VSmallUnionC) VDLIsZero() bool {
	return false
}

func (x VSmallUnionA) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_union_13); err != nil {
		return err
	}
	if err := enc.NextFieldValueInt(0, vdl.Int32Type, int64(x.Value)); err != nil {
		return err
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x VSmallUnionB) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_union_13); err != nil {
		return err
	}
	if err := enc.NextFieldValueString(1, vdl.StringType, x.Value); err != nil {
		return err
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x VSmallUnionC) VDLWrite(enc vdl.Encoder) error {
	if err := enc.StartValue(__VDLType_union_13); err != nil {
		return err
	}
	if err := enc.NextFieldValueBool(2, vdl.BoolType, x.Value); err != nil {
		return err
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func VDLReadVSmallUnion(dec vdl.Decoder, x *VSmallUnion) error {
	if err := dec.StartValue(__VDLType_union_13); err != nil {
		return err
	}
	decType := dec.Type()
	index, err := dec.NextField()
	switch {
	case err != nil:
		return err
	case index == -1:
		return fmt.Errorf("missing field in union %T, from %v", x, decType)
	}
	if decType != __VDLType_union_13 {
		name := decType.Field(index).Name
		index = __VDLType_union_13.FieldIndexByName(name)
		if index == -1 {
			return fmt.Errorf("field %q not in union %T, from %v", name, x, decType)
		}
	}
	switch index {
	case 0:
		var field VSmallUnionA
		switch value, err := dec.ReadValueInt(32); {
		case err != nil:
			return err
		default:
			field.Value = int32(value)
		}
		*x = field
	case 1:
		var field VSmallUnionB
		switch value, err := dec.ReadValueString(); {
		case err != nil:
			return err
		default:
			field.Value = value
		}
		*x = field
	case 2:
		var field VSmallUnionC
		switch value, err := dec.ReadValueBool(); {
		case err != nil:
			return err
		default:
			field.Value = value
		}
		*x = field
	}
	switch index, err := dec.NextField(); {
	case err != nil:
		return err
	case index != -1:
		return fmt.Errorf("extra field %d in union %T, from %v", index, x, dec.Type())
	}
	return dec.FinishValue()
}

// Hold type definitions in package-level variables, for better performance.
// nolint: unused
var (
	__VDLType_int32_1   *vdl.Type
	__VDLType_string_2  *vdl.Type
	__VDLType_enum_3    *vdl.Type
	__VDLType_list_4    *vdl.Type
	__VDLType_array_5   *vdl.Type
	__VDLType_array_6   *vdl.Type
	__VDLType_list_7    *vdl.Type
	__VDLType_list_8    *vdl.Type
	__VDLType_set_9     *vdl.Type
	__VDLType_map_10    *vdl.Type
	__VDLType_struct_11 *vdl.Type
	__VDLType_struct_12 *vdl.Type
	__VDLType_union_13  *vdl.Type
)

var __VDLInitCalled bool

// __VDLInit performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = __VDLInit()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func __VDLInit() struct{} {
	if __VDLInitCalled {
		return struct{}{}
	}
	__VDLInitCalled = true

	// Register types.
	vdl.Register((*VNumber)(nil))
	vdl.Register((*VString)(nil))
	vdl.Register((*VEnum)(nil))
	vdl.Register((*VByteList)(nil))
	vdl.Register((*VByteArray)(nil))
	vdl.Register((*VArray)(nil))
	vdl.Register((*VList)(nil))
	vdl.Register((*VListAny)(nil))
	vdl.Register((*VSet)(nil))
	vdl.Register((*VMap)(nil))
	vdl.Register((*VSmallStruct)(nil))
	vdl.Register((*VLargeStruct)(nil))
	vdl.Register((*VSmallUnion)(nil))

	// Initialize type definitions.
	__VDLType_int32_1 = vdl.TypeOf((*VNumber)(nil))
	__VDLType_string_2 = vdl.TypeOf((*VString)(nil))
	__VDLType_enum_3 = vdl.TypeOf((*VEnum)(nil))
	__VDLType_list_4 = vdl.TypeOf((*VByteList)(nil))
	__VDLType_array_5 = vdl.TypeOf((*VByteArray)(nil))
	__VDLType_array_6 = vdl.TypeOf((*VArray)(nil))
	__VDLType_list_7 = vdl.TypeOf((*VList)(nil))
	__VDLType_list_8 = vdl.TypeOf((*VListAny)(nil))
	__VDLType_set_9 = vdl.TypeOf((*VSet)(nil))
	__VDLType_map_10 = vdl.TypeOf((*VMap)(nil))
	__VDLType_struct_11 = vdl.TypeOf((*VSmallStruct)(nil)).Elem()
	__VDLType_struct_12 = vdl.TypeOf((*VLargeStruct)(nil)).Elem()
	__VDLType_union_13 = vdl.TypeOf((*VSmallUnion)(nil))

	return struct{}{}
}
