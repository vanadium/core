// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"math"

	"v.io/v23/vdl"
)

const (
	// TODO(toddw): Move these constants to a common place, they already exist in
	// the vdl and vom packages.  Also think about how to factor out the number
	// conversion logic.
	float64MaxInt = (1 << 53)
	float64MinInt = -(1 << 53)
	float32MaxInt = (1 << 24)
	float32MinInt = -(1 << 24)
)

// MimicValue returns a value of type tt that, when converted to the type of the
// base value, results in exactly the base value.  If tt is Any, the type of the
// returned value is the type of the base value.
//
// Returns nil if no such value is possible.  E.g. if tt is uint32 and base is
// int32(-1), no value of type tt can be converted to the base value.
func MimicValue(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	// Handle nil first, to get rid of pesky corner cases.  After this is done,
	// we've flattened the base value to its non-nil element.
	baseWasAny := false
	if base.Kind() == vdl.Any && !base.IsNil() {
		baseWasAny = true
		base = base.Elem()
	}
	if base.IsNil() {
		return mimicNilValue(tt, base)
	}
	// Now we know we're dealing with a non-nil non-any base.
	if tt == vdl.AnyType {
		// We've been asked to build a value of any type, so we build exactly the
		// same type as the base.  There are two cases:
		//   1) base: int64      tt: any
		//   2) base: any(int64) tt: any
		//
		// For case 1 we could actually fill in tt with any type compatible with the
		// base type, but for case 2 we must fill in tt with the exact base type;
		// see case 4 below.  For simplicity we just always use the exact base type.
		tt = base.Type()
	}
	if baseWasAny && tt != base.Type() {
		// If base was an any value, tt must be exactly the same type as the base.
		// There are two cases:
		//   3) base: any(int64) tt: int64
		//   4) base: any(int64) tt: any(int64)
		//
		// Note that it's not good enough to mimic a value of a compatible type tt;
		// if we changed tt to int16 above, we wouldn't end up with a result of the
		// right type.  Case 4 was ensured by case 2 above.
		return nil
	}
	value := mimicNonNilValue(tt.NonOptional(), base.NonOptional())
	if value == nil {
		return nil
	}
	if tt.Kind() == vdl.Optional {
		value = vdl.OptionalValue(value)
	}
	return value
}

// mimicNilValue implements MimicValue for nil base values.
//
// REQUIRES: base.IsNil() is true
func mimicNilValue(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	switch {
	case tt == vdl.AnyType:
		// We can convert from any(nil) to all nil base values.
		return vdl.ZeroValue(vdl.AnyType)
	case tt.Kind() != vdl.Optional:
		// Can't convert from a non-any non-optional type to a nil value.
		return nil
	}
	// Now we know that tt is optional, and base is either any(nil) or
	// optional(nil).
	switch {
	case base.Type() == vdl.AnyType:
		// Can't convert from optional(nil) to any(nil).
		return nil
	case !vdl.Compatible(tt, base.Type()):
		// Can't convert incompatible optional types.
		return nil
	}
	// Now we know that tt and base are both optional and compatible.
	return vdl.ZeroValue(tt)
}

// mimicNonNilValue implements MimicValue for non-nil base values.
//
// REQUIRES: tt and base cannot be Optional or Any.
func mimicNonNilValue(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	if !vdl.Compatible(tt, base.Type()) {
		return nil
	}
	switch tt.Kind() {
	case vdl.Bool:
		return vdl.BoolValue(tt, base.Bool())
	case vdl.String:
		return vdl.StringValue(tt, stringOrEnumLabel(base))
	case vdl.Enum:
		index := tt.EnumIndex(stringOrEnumLabel(base))
		if index == -1 {
			return nil
		}
		return vdl.EnumValue(tt, index)
	case vdl.TypeObject:
		return base // TypeObject is only convertible from itself.
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return mimicUint(tt, base)
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return mimicInt(tt, base)
	case vdl.Float32, vdl.Float64:
		return mimicFloat(tt, base)
	case vdl.Array, vdl.List:
		return mimicArrayList(tt, base)
	case vdl.Set:
		return mimicSet(tt, base)
	case vdl.Map:
		return mimicMap(tt, base)
	case vdl.Struct:
		return mimicStruct(tt, base)
	case vdl.Union:
		return mimicUnion(tt, base)
	}
	panic(fmt.Errorf("vdltest: mimicNonNilValue unhandled type %v", tt))
}

func stringOrEnumLabel(vv *vdl.Value) string {
	if vv.Kind() == vdl.String {
		return vv.RawString()
	}
	return vv.EnumLabel()
}

func mimicUint(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	bitlen := uint(tt.Kind().BitLen())
	var x uint64
	switch base.Kind() {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		x = base.Uint()
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return nil
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		ix := base.Int()
		x = uint64(ix)
		if shift := 64 - bitlen; ix < 0 || x != (x<<shift)>>shift {
			return nil
		}
	case vdl.Float32, vdl.Float64:
		fx := base.Float()
		x = uint64(fx)
		if shift := 64 - bitlen; fx != float64(x) || x != (x<<shift)>>shift {
			return nil
		}
	}
	return vdl.UintValue(tt, x)
}

func mimicInt(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	bitlen := uint(tt.Kind().BitLen())
	var x int64
	switch base.Kind() {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		ux := base.Uint()
		x = int64(ux)
		// The shift uses 65 since the topmost bit is the sign bit.  E.g. 32 bit
		// numbers should be shifted by 33 rather than 32.
		if shift := 65 - bitlen; x < 0 || ux != (ux<<shift)>>shift {
			return nil
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		x = base.Int()
		if shift := 64 - bitlen; x != (x<<shift)>>shift {
			return nil
		}
	case vdl.Float32, vdl.Float64:
		fx := base.Float()
		x = int64(fx)
		if shift := 64 - bitlen; fx != float64(x) || x != (x<<shift)>>shift {
			return nil
		}
	}
	return vdl.IntValue(tt, x)
}

func mimicFloat(tt *vdl.Type, base *vdl.Value) *vdl.Value { //nolint:gocyclo
	bitlen := uint(tt.Kind().BitLen())
	var x float64
	switch base.Kind() {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		ux := base.Uint()
		x = float64(ux)
		var max uint64
		if bitlen > 32 {
			max = float64MaxInt
		} else {
			max = float32MaxInt
		}
		if ux > max {
			return nil
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		ix := base.Int()
		x = float64(ix)
		var min, max int64
		if bitlen > 32 {
			min, max = float64MinInt, float64MaxInt
		} else {
			min, max = float32MinInt, float32MaxInt
		}
		if ix < min || ix > max {
			return nil
		}
	case vdl.Float32:
		x = base.Float()
	case vdl.Float64:
		x = base.Float()
		if tt.Kind() == vdl.Float32 {
			// We're trying to mimic a base float64 value with a float32 value.  Make
			// sure we won't lose precision.
			//
			// Float64 has 1 sign bit, 11 exponent bits, and 52 fraction bits.
			// Float32 has 1 sign bit, 8 exponent bits, and 23 fraction bits.
			//
			// The offsetExp is offset by 1023.  Some special values:
			//   offsetExp == 0   && frac == 0 : Negative 0
			//   offsetExp == 0   && frac > 0  : Subnormal number
			//   offsetExp == 7ff && frac == 0 : Inf
			//   offsetExp == 7ff && frac > 0  : NaN
			//
			// https://en.wikipedia.org/wiki/Double-precision_floating-point_format
			// https://en.wikipedia.org/wiki/Single-precision_floating-point_format
			bits := math.Float64bits(x)
			offsetExp := bits >> 52 & ((1 << 11) - 1)
			frac := bits & ((1 << 52) - 1)
			switch exp := int(offsetExp) - 1023; {
			case offsetExp == 0 && frac > 0:
				// Float64 subnormals can't be represented by float32.
				return nil
			case exp < -0xff || exp > 0xff:
				// Float64 with >8 exponent bits can't be represented by float32.
				// TODO(toddw): Handle Inf and Nan in vdl float consts.
				return nil
			case frac&((1<<23)-1) != 0:
				// Float64 with >23 fraction bits can't be represented by float32.
				return nil
			}
		}
	}
	return vdl.FloatValue(tt, x)
}

func mimicArrayList(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	value := vdl.ZeroValue(tt)
	switch tt.Kind() {
	case vdl.Array:
		if tt.Len() != base.Len() {
			return nil
		}
	case vdl.List:
		value.AssignLen(base.Len())
	}
	for ix := 0; ix < base.Len(); ix++ {
		elem := MimicValue(tt.Elem(), base.Index(ix))
		if elem == nil {
			return nil
		}
		value.AssignIndex(ix, elem)
	}
	return value
}

func mimicSet(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for _, baseKey := range base.Keys() {
		key := MimicValue(tt.Key(), baseKey)
		if key == nil {
			return nil
		}
		value.AssignSetKey(key)
	}
	return value
}

func mimicMap(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for _, baseKey := range base.Keys() {
		key := MimicValue(tt.Key(), baseKey)
		if key == nil {
			return nil
		}
		elem := MimicValue(tt.Elem(), base.MapIndex(baseKey))
		if elem == nil {
			return nil
		}
		value.AssignMapIndex(key, elem)
	}
	return value
}

func mimicStruct(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for ix := 0; ix < base.Type().NumField(); ix++ {
		baseField := base.StructField(ix)
		ttField, ttIndex := tt.FieldByName(base.Type().Field(ix).Name)
		if ttIndex == -1 {
			if baseField.IsZero() {
				// Skip zero base fields.  It's fine for the base field to be missing
				// from tt, as long as the base field is zero, since Convert(dst, src)
				// sets all dst (which has type base.Type) fields to zero before setting
				// each matching field in src (which has type tt).
				continue
			}
			// This is a non-zero base field that doesn't exist in tt.  There's no way
			// to create a value of type tt that converts to exactly the base value.
			return nil
		}
		field := MimicValue(ttField.Type, baseField)
		if field == nil {
			return nil
		}
		value.AssignField(ttIndex, field)
	}
	return value
}

func mimicUnion(tt *vdl.Type, base *vdl.Value) *vdl.Value {
	baseIndex, baseField := base.UnionField()
	ttField, ttIndex := tt.FieldByName(base.Type().Field(baseIndex).Name)
	if ttIndex == -1 {
		// The base field doesn't exist in tt.  There's no way to create a value of
		// type tt that converts to exactly the base value.
		return nil
	}
	field := MimicValue(ttField.Type, baseField)
	if field == nil {
		return nil
	}
	return vdl.UnionValue(tt, ttIndex, field)
}
