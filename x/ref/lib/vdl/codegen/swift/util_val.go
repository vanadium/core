// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"fmt"
	"strconv"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

// swiftConstVal returns the value string for the provided constant value.
func swiftConstVal(v *vdl.Value, ctx *swiftContext) (ret string) {
	ret = swiftVal(v, ctx)
	if tdef := ctx.env.FindTypeDef(v.Type()); tdef != nil && tdef.File != compile.BuiltInFile { // User-defined type.
		switch tdef.Type.Kind() {
		case vdl.Union:
			ret = fmt.Sprintf("%s.%s", ctx.swiftType(v.Type()), ret)
		case vdl.Struct:
			ret = fmt.Sprintf("%s(%s)", ctx.swiftType(v.Type()), ret)
		case vdl.Enum:
			return
		default:
			ret = fmt.Sprintf("%s(rawValue: %s)", ctx.swiftType(v.Type()), ret)
		}
	}
	return
}

// swiftVal returns the value string for the provided Value.
func swiftVal(v *vdl.Value, ctx *swiftContext) string { //nolint:gocyclo
	switch v.Kind() {
	case vdl.Any:
		// TODO(azinman) Don't exactly know what to do here to generate the inference code... so instead come back to this
		// but it doesn't seem to get triggered in the v23 VDL gen code base anyway?
		panic("Don't know how to handle Any type in a given Value")
	case vdl.Bool:
		switch v.Bool() {
		case true:
			return "true"
		case false:
			return "false"
		}
	case vdl.Byte:
		return "UInt8(0x" + strconv.FormatUint(v.Uint(), 16) + ")"
	case vdl.Int8:
		return "Int8(" + strconv.FormatInt(v.Int(), 10) + ")"
	case vdl.Uint16:
		return "UInt16(" + strconv.FormatUint(v.Uint(), 10) + ")"
	case vdl.Int16:
		return "Int16(" + strconv.FormatInt(v.Int(), 10) + ")"
	case vdl.Uint32:
		return "UInt32(" + strconv.FormatUint(v.Uint(), 10) + ")"
	case vdl.Int32:
		return "Int32(" + strconv.FormatInt(v.Int(), 10) + ")"
	case vdl.Uint64:
		return "UInt64(" + strconv.FormatUint(v.Uint(), 10) + ")"
	case vdl.Int64:
		return "Int64(" + strconv.FormatInt(v.Int(), 10) + ")"
	case vdl.Float32, vdl.Float64:
		c := strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
		if !strings.Contains(c, ".") {
			c += ".0"
		}
		if v.Kind() == vdl.Float32 {
			return "Float(" + c + ")"
		}
		return "Double(" + c + ")"
	case vdl.String:
		in := v.RawString()
		return swiftQuoteString(in)
	case vdl.Enum:
		return fmt.Sprintf("%s.%s", ctx.swiftType(v.Type()), v.EnumLabel())
	case vdl.Array, vdl.List:
		ret := "["
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				ret += ", "
			}
			ret += swiftConstVal(v.Index(i), ctx)
		}
		return ret + "]"
	case vdl.Map:
		ret := "["
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			keyStr := swiftConstVal(key, ctx)
			elemStr := swiftConstVal(v.MapIndex(key), ctx)
			ret = fmt.Sprintf("%s%s:%s, ", ret, keyStr, elemStr)
		}
		return ret + "]"
	case vdl.Union:
		index, value := v.UnionField()
		name := v.Type().Field(index).Name
		elemStr := swiftConstVal(value, ctx)
		return fmt.Sprintf("%s(elem: %s)", name, elemStr)
	case vdl.Set:
		ret := "Set("
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			ret = fmt.Sprintf("%s, %s", ret, swiftConstVal(key, ctx))
		}
		return ret + ")"
	case vdl.Struct:
		tdef := ctx.env.FindTypeDef(v.Type())
		if tdef == nil {
			panic("No plausible way to define a Swift struct constructor without the tdef")
		}
		var ret string
		for i := 0; i < v.Type().NumField(); i++ {
			if i > 0 {
				ret += ", "
			}
			fldValue := v.StructField(i)
			fld := tdef.Type.Field(i)
			ret = fmt.Sprintf("%v%v: %v", ret, swiftVariableName(tdef, fld), swiftConstVal(fldValue, ctx))
		}
		return ret
	case vdl.TypeObject:
		// TODO(zinman) Support TypeObjects once we have vdlType support
		return "VdlTypeObject(type: nil)"
	case vdl.Optional:
		if v.Elem() != nil {
			return swiftConstVal(v.Elem(), ctx)
		}
		return "nil"
	}
	panic(fmt.Errorf("vdl: swiftVal unhandled type %v %v", v.Kind(), v.Type()))
}

func swiftQuoteString(str string) string {
	// This accounts for Swift string interpolation \(likeThis) because
	// vdl will already error if you create a const string that contains
	// \( and not \\(
	// In the future this may change and we might need to explicitly add
	// the escapes.
	return strconv.Quote(str)
}
