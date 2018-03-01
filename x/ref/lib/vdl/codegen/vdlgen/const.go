// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strconv"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/codegen"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// TypedConst returns the explicitly-typed vdl const corresponding to v, in the
// given pkgPath, with the given imports.
func TypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) string {
	if v == nil {
		return "nil"
	}
	k, t := v.Kind(), v.Type()
	typestr := Type(t, pkgPath, imports)
	if k == vdl.Optional {
		// TODO(toddw): This only works if the optional elem is a composite literal.
		if elem := v.Elem(); elem != nil {
			return typestr + UntypedConst(elem, pkgPath, imports)
		}
		return typestr + "(nil)"
	}
	valstr := UntypedConst(v, pkgPath, imports)
	if k == vdl.TypeObject || t == vdl.BoolType || t == vdl.StringType {
		// TypeObject already includes the type in its value.
		// Built-in bool and string are implicitly convertible from literals.
		return valstr
	}
	switch k {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Union:
		// { } are used instead of ( ) for composites, except for []byte and [N]byte
		if !t.IsBytes() {
			return typestr + valstr
		}
	case vdl.Enum:
		if t.Name() != "" {
			return typestr + "." + valstr
		}
		return valstr
	}
	return typestr + "(" + valstr + ")"
}

// UntypedConst returns the untyped vdl const corresponding to v, in the given
// pkgPath, with the given imports.
func UntypedConst(v *vdl.Value, pkgPath string, imports codegen.Imports) string {
	k, t := v.Kind(), v.Type()
	if t.IsBytes() {
		return strconv.Quote(string(v.Bytes()))
	}
	switch k {
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return TypedConst(elem, pkgPath, imports)
		}
		return "nil"
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return UntypedConst(elem, pkgPath, imports)
		}
		return "nil"
	case vdl.Bool:
		return strconv.FormatBool(v.Bool())
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Float32, vdl.Float64:
		return formatFloat(v.Float(), k)
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Enum:
		return v.EnumLabel()
	case vdl.TypeObject:
		return "typeobject(" + Type(v.TypeObject(), pkgPath, imports) + ")"
	case vdl.Array, vdl.List:
		if v.IsZero() {
			return "{}"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			if ix > 0 {
				s += ", "
			}
			s += UntypedConst(v.Index(ix), pkgPath, imports)
		}
		return s + "}"
	case vdl.Set, vdl.Map:
		s := "{"
		for ix, key := range vdl.SortValuesAsString(v.Keys()) {
			if ix > 0 {
				s += ", "
			}
			s += UntypedConst(key, pkgPath, imports)
			if k == vdl.Map {
				s += ": " + UntypedConst(v.MapIndex(key), pkgPath, imports)
			}
		}
		return s + "}"
	case vdl.Struct:
		s := "{"
		hasFields := false
		for ix := 0; ix < t.NumField(); ix++ {
			vf := v.StructField(ix)
			if vf.IsZero() {
				continue
			}
			if hasFields {
				s += ", "
			}
			s += t.Field(ix).Name + ": " + UntypedConst(vf, pkgPath, imports)
			hasFields = true
		}
		return s + "}"
	case vdl.Union:
		index, value := v.UnionField()
		return "{" + t.Field(index).Name + ": " + UntypedConst(value, pkgPath, imports) + "}"
	default:
		panic(fmt.Errorf("vdlgen.Const unhandled type: %v %v", k, t))
	}
}

// JSON returns a JSON representation of a value. This was adapted from
// untypedConst function from v.io/x/ref/lib/vdl/codegen/javascript.
func JSON(v *vdl.Value, pkgPath string, imports codegen.Imports) string {
	switch v.Kind() {
	case vdl.Bool:
		if v.Bool() {
			return "true"
		} else {
			return "false"
		}
	case vdl.Byte, vdl.Uint16, vdl.Uint32:
		return strconv.FormatUint(v.Uint(), 10)
	case vdl.Int8, vdl.Int16, vdl.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Uint64:
		return fmt.Sprintf(`"%d"`, v.Uint())
	case vdl.Int64:
		return fmt.Sprintf(`"%d"`, v.Int())
	case vdl.Float32, vdl.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
	case vdl.String:
		return strconv.Quote(v.RawString())
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			return JSON(elem, pkgPath, imports)
		}
		return "null"
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return JSON(elem, pkgPath, imports)
		}
		return "null"
	case vdl.Enum:
		return fmt.Sprintf(`"%s"`, v.EnumLabel())
	case vdl.Array, vdl.List:
		result := "["
		for ix := 0; ix < v.Len(); ix++ {
			if ix > 0 {
				result += ","
			}
			val := JSON(v.Index(ix), pkgPath, imports)
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
			result += JSON(key, pkgPath, imports)
		}
		result += "]"
		return result
	case vdl.Map:
		result := "{"
		for i, key := range vdl.SortValuesAsString(v.Keys()) {
			if i > 0 {
				result += ","
			}
			result += fmt.Sprintf(`%s: %s`, quote(JSON(key, pkgPath, imports)), JSON(v.MapIndex(key), pkgPath, imports))
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
			result += fmt.Sprintf(`"%s": %s`, vdlutil.FirstRuneToLower(t.Field(ix).Name), JSON(v.StructField(ix), pkgPath, imports))
		}
		return result + "}"
	case vdl.Union:
		ix, innerVal := v.UnionField()
		return fmt.Sprintf("{ %q: %v }", vdlutil.FirstRuneToLower(v.Type().Field(ix).Name), JSON(innerVal, pkgPath, imports))
	case vdl.TypeObject:
		// TODO(razvanm): check it this is correct.
		return Type(v.TypeObject(), pkgPath, imports)
	default:
		panic(fmt.Errorf("vdl: untypedConst unhandled type %v %v", v.Kind(), v.Type()))
	}
}

func formatFloat(x float64, kind vdl.Kind) string {
	var bitSize int
	switch kind {
	case vdl.Float32:
		bitSize = 32
	case vdl.Float64:
		bitSize = 64
	default:
		panic(fmt.Errorf("formatFloat unhandled kind: %v", kind))
	}
	return strconv.FormatFloat(x, 'g', -1, bitSize)
}

func bitlen(kind vdl.Kind) int {
	switch kind {
	case vdl.Float32:
		return 32
	case vdl.Float64:
		return 64
	}
	panic(fmt.Errorf("vdl: bitLen unhandled kind %v", kind))
}

func quote(s string) string {
	if len(s) > 2 && s[0] != '"' && s[len(s)-1] != '"' {
		s = fmt.Sprintf(`"%s"`, s)
	}
	return s
}
