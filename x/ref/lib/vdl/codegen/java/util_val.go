// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"fmt"
	"strconv"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const javaMaxStringLen = 2 << 16 // 64k

// javaConstVal returns the value string for the provided constant value.
func javaConstVal(v *vdl.Value, env *compile.Env) (ret string) {
	if v == nil {
		return "null"
	}
	if v.IsZero() {
		return javaZeroValue(v.Type(), env)
	}

	ret = javaVal(v, env)
	switch v.Type().Kind() {
	case vdl.Enum, vdl.Union, vdl.Int8, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return
	}
	if def := env.FindTypeDef(v.Type()); def != nil && def.File != compile.BuiltInFile { // User-defined type.
		ret = fmt.Sprintf("new %s(%s)", javaType(v.Type(), false, env), ret)
	}
	return
}

// javaVal returns the value string for the provided Value.
func javaVal(v *vdl.Value, env *compile.Env) string { //nolint:gocyclo
	const longSuffix = "L"
	const floatSuffix = "f"

	if v.Kind() == vdl.Array || (v.Kind() == vdl.List && v.Type().Elem().Kind() == vdl.Byte && v.Type().Name() == "") {
		ret := fmt.Sprintf("new %s[] {", javaType(v.Type().Elem(), false, env))
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				ret += ", "
			}
			ret += javaConstVal(v.Index(i), env)
		}
		return ret + "}"
	}

	switch v.Kind() {
	case vdl.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case vdl.Byte:
		return "(byte)0x" + strconv.FormatUint(v.Uint(), 16)
	case vdl.Int8:
		return fmt.Sprintf("new %s((byte) %s)", javaType(v.Type(), true, env), strconv.FormatInt(v.Int(), 10))
	case vdl.Uint16:
		return fmt.Sprintf("new %s((short) %s)", javaType(v.Type(), true, env), strconv.FormatUint(v.Uint(), 10))
	case vdl.Int16:
		return "(short)" + strconv.FormatInt(v.Int(), 10)
	case vdl.Uint32:
		return fmt.Sprintf("new %s((int) %s)", javaType(v.Type(), true, env), strconv.FormatUint(v.Uint(), 10)+longSuffix)
	case vdl.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case vdl.Uint64:
		// Note: this is formatting the number as an int because Java will error on unsigned contants that use the
		// full 64-bits.
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), true, env), strconv.FormatInt(int64(v.Uint()), 10)+longSuffix)
	case vdl.Int64:
		return strconv.FormatInt(v.Int(), 10) + longSuffix
	case vdl.Float32, vdl.Float64:
		c := strconv.FormatFloat(v.Float(), 'g', -1, bitlen(v.Kind()))
		if !strings.Contains(c, ".") && !strings.Contains(c, "e") {
			c += ".0"
		}
		if v.Kind() == vdl.Float32 {
			return c + floatSuffix
		}
		return c
	case vdl.String:
		in := v.RawString()
		if len(in) < javaMaxStringLen {
			return strconv.Quote(in)
		}

		// Java is unable to handle constant strings larger than 64k so split them into multiple strings.
		out := "String.join(\"\", new java.util.ArrayList<CharSequence>("
		for len(in) > javaMaxStringLen {
			out += strconv.Quote(in[:javaMaxStringLen]) + ","
			in = in[javaMaxStringLen:]
		}
		return out + strconv.Quote(in) + "))"
	case vdl.Any:
		if v.Elem() == nil {
			return fmt.Sprintf("new %s()", javaType(v.Type(), false, env))
		}
		elemReflectTypeStr := javaReflectType(v.Elem().Type(), env)
		elemStr := javaConstVal(v.Elem(), env)
		return fmt.Sprintf("new %s(%s, %s)", javaType(v.Type(), false, env), elemReflectTypeStr, elemStr)
	case vdl.Enum:
		return fmt.Sprintf("%s.%s", javaType(v.Type(), false, env), v.EnumLabel())
	case vdl.List:
		elemTypeStr := javaType(v.Type().Elem(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableList.Builder<%s>()", elemTypeStr)
		for i := 0; i < v.Len(); i++ {
			ret = fmt.Sprintf("%s\n.add(%s)", ret, javaConstVal(v.Index(i), env))
		}
		return ret + ".build()"
	case vdl.Map:
		keyTypeStr := javaType(v.Type().Key(), true, env)
		elemTypeStr := javaType(v.Type().Elem(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableMap.Builder<%s, %s>()", keyTypeStr, elemTypeStr)
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			keyStr := javaConstVal(key, env)
			elemStr := javaConstVal(v.MapIndex(key), env)
			ret = fmt.Sprintf("%s\n.put(%s, %s)", ret, keyStr, elemStr)
		}
		return ret + ".build()"
	case vdl.Union:
		index, value := v.UnionField()
		name := v.Type().Field(index).Name
		elemStr := javaConstVal(value, env)
		return fmt.Sprintf("new %s.%s(%s)", javaType(v.Type(), false, env), name, elemStr)
	case vdl.Set:
		keyTypeStr := javaType(v.Type().Key(), true, env)
		ret := fmt.Sprintf("new com.google.common.collect.ImmutableSet.Builder<%s>()", keyTypeStr)
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			ret = fmt.Sprintf("%s\n.add(%s)", ret, javaConstVal(key, env))
		}
		return ret + ".build()"
	case vdl.Struct:
		var ret string
		for i := 0; i < v.Type().NumField(); i++ {
			if i > 0 {
				ret += ", "
			}
			ret += javaConstVal(v.StructField(i), env)
		}
		return ret
	case vdl.TypeObject:
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), false, env), javaReflectType(v.TypeObject(), env))
	case vdl.Optional:
		if v.Elem() != nil {
			return fmt.Sprintf("io.v.v23.vdl.VdlOptional.of(%s)", javaConstVal(v.Elem(), env))
		}
		return fmt.Sprintf("new %s(%s)", javaType(v.Type(), false, env), javaReflectType(v.Type(), env))
	}
	panic(fmt.Errorf("vdl: javaVal unhandled type %v %v", v.Kind(), v.Type()))
}

// javaZeroValue returns the zero value string for the provided VDL value.
// We assume that default constructor of user-defined types returns a zero value.
func javaZeroValue(t *vdl.Type, env *compile.Env) string { //nolint:gocyclo
	if _, ok := javaNativeType(t, env); ok {
		return "null"
	}

	// First process user-defined types.
	switch t.Kind() {
	case vdl.Enum:
		return fmt.Sprintf("%s.%s", javaType(t, false, env), t.EnumLabel(0))
	case vdl.Union:
		return fmt.Sprintf("new %s.%s()", javaType(t, false, env), t.Field(0).Name)
	}
	if def := env.FindTypeDef(t); def != nil && def.File != compile.BuiltInFile {
		return fmt.Sprintf("new %s()", javaType(t, false, env))
	}

	// Arrays, enums, structs and unions can be user-defined only.
	if t.Kind() == vdl.List && t.Elem().Kind() == vdl.Byte {
		return fmt.Sprintf("new %s[]{}", javaType(t.Elem(), false, env))
	}
	switch t.Kind() {
	case vdl.Bool:
		return "false"
	case vdl.Byte:
		return "(byte) 0"
	case vdl.Int16:
		return "(short) 0"
	case vdl.Int32:
		return "0"
	case vdl.Int64:
		return "0L"
	case vdl.Float32:
		return "0.0f"
	case vdl.Float64:
		return "0.0"
	case vdl.Any, vdl.TypeObject, vdl.Int8, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return fmt.Sprintf("new %s()", javaType(t, false, env))
	case vdl.String:
		return "\"\""
	case vdl.List:
		return fmt.Sprintf("new java.util.ArrayList<%s>()", javaType(t.Elem(), true, env))
	case vdl.Map:
		keyTypeStr := javaType(t.Key(), true, env)
		elemTypeStr := javaType(t.Elem(), true, env)
		return fmt.Sprintf("new java.util.HashMap<%s, %s>()", keyTypeStr, elemTypeStr)
	case vdl.Set:
		return fmt.Sprintf("new java.util.HashSet<%s>()", javaType(t.Key(), true, env))
	case vdl.Optional:
		return fmt.Sprintf("new %s(%s)", javaType(t, false, env), javaReflectType(t, env))
	}
	panic(fmt.Errorf("vdl: javaZeroValue unhandled type %v", t))
}
