// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package json implements JSON generation for VDL const values.
package json

// TODO(razvanm): Add more tests.

import (
	"fmt"
	"strconv"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/codegen"
	"v.io/x/ref/lib/vdl/codegen/vdlgen"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// Const returns a JSON representation of a value.
//
// TODO(razvanm): use this function to replace most (all?) of the untypedConst
// function from v.io/x/ref/lib/vdl/codegen/javascript.
func Const(v *vdl.Value, pkgPath string, imports codegen.Imports) string { //nolint:gocyclo
	switch v.Kind() {
	case vdl.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case vdl.Byte, vdl.Uint16, vdl.Uint32:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int8, vdl.Int16, vdl.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Uint64:
		// We use strings to avoid loss of precision for languages that use 64-bit
		// float for handling numbers.
		return fmt.Sprintf(`"%d"`, v.Uint())
	case vdl.Int64:
		// We use strings for the same reasons as for vdl.Uint64.
		return fmt.Sprintf(`"%d"`, v.Int())
	case vdl.Float32, vdl.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return Const(elem, pkgPath, imports)
		}
		return "null"
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return Const(elem, pkgPath, imports)
		}
		return "null"
	case vdl.Enum:
		return strconv.Quote(v.EnumLabel())
	case vdl.Array, vdl.List:
		result := "["
		for ix := 0; ix < v.Len(); ix++ {
			if ix > 0 {
				result += ","
			}
			val := Const(v.Index(ix), pkgPath, imports)
			result += val
		}
		result += "]"
		return result
	case vdl.Set:
		result := "["
		for ix, key := range vdl.SortValuesAsString(v.Keys()) {
			if ix > 0 {
				result += ","
			}
			result += Const(key, pkgPath, imports)
		}
		result += "]"
		return result
	case vdl.Map:
		result := "{"
		for i, key := range vdl.SortValuesAsString(v.Keys()) {
			if i > 0 {
				result += ","
			}
			// TODO(razvanm): figure out what to do if the key is not a scalar type.
			result += fmt.Sprintf(`%s: %s`, quote(Const(key, pkgPath, imports)), Const(v.MapIndex(key), pkgPath, imports))
		}
		result += "}"
		return result
	case vdl.Struct:
		result := "{"
		t := v.Type()
		for ix := 0; ix < t.NumField(); ix++ {
			if ix > 0 {
				result += ","
			}
			result += fmt.Sprintf(`"%s": %s`, vdlutil.FirstRuneToLower(t.Field(ix).Name), Const(v.StructField(ix), pkgPath, imports))
		}
		return result + "}"
	case vdl.Union:
		ix, innerVal := v.UnionField()
		return fmt.Sprintf("{ %q: %v }", vdlutil.FirstRuneToLower(v.Type().Field(ix).Name), Const(innerVal, pkgPath, imports))
	case vdl.TypeObject:
		// TODO(razvanm): check it this is correct.
		return vdlgen.Type(v.TypeObject(), pkgPath, imports)
	default:
		panic(fmt.Errorf("vdl: Const unhandled type %v %v", v.Kind(), v.Type()))
	}
}

func bitlen(kind vdl.Kind) int {
	switch kind {
	case vdl.Float32:
		return 32
	case vdl.Float64:
		return 64
	}
	panic(fmt.Errorf("vdl: bitlen unhandled kind %v", kind))
}

// quote surrounds a string argument with double quotes is the string doesn't
// already have quotes around. This is useful for JSON names.
func quote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s
	}
	return fmt.Sprintf(`"%s"`, s)
}
