// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"strconv"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

func defineConst(data *goData, def *compile.ConstDef) string {
	v := def.Value
	return fmt.Sprintf("%s%s %s = %s%s", def.Doc, constOrVar(v.Kind()), def.Name, typedConst(data, v), def.DocSuffix)
}

func constOrVar(k vdl.Kind) string {
	switch k {
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.String, vdl.Enum:
		return "const"
	}
	return "var"
}

func isByteList(t *vdl.Type) bool {
	return t.Kind() == vdl.List && t.Elem().Kind() == vdl.Byte
}

func genValueOf(data *goData, v *vdl.Value) string {
	// There's no need to convert the value to its native representation, since
	// it'll just be converted back in vdl.ValueOf.
	return data.Pkg("v.io/v23/vdl") + "ValueOf(" + typedConstWire(data, v) + ")"
}

func typedConst(data *goData, v *vdl.Value) string {
	if native := typedConstNative(data, v); native != "" {
		return native
	}
	return typedConstWire(data, v)
}

// TODO(bprosnitz): Generate the full tag name e.g. security.Read instead of
// security.Label(1)
//
// TODO(toddw): This is broken for optional types that can't be represented
// using a composite literal (e.g. optional primitives).
//
// https://github.com/vanadium/issues/issues/213
func typedConstWire(data *goData, v *vdl.Value) string {
	k, t := v.Kind(), v.Type()
	typestr := typeGoWire(data, t)
	if k == vdl.Optional {
		if elem := v.Elem(); elem != nil {
			return "&" + typedConst(data, elem)
		}
		return "(" + typestr + ")(nil)" // results in (*Foo)(nil)
	}
	valstr := untypedConstWire(data, v)
	// Enum, TypeObject, Union, Any and non-zero []byte already include the type
	// in their "untyped" values.  Built-in bool and string are implicitly
	// convertible from their literals.
	if k == vdl.Enum || k == vdl.TypeObject || k == vdl.Any || (isByteList(t) && !v.IsZero()) || t == vdl.BoolType || t == vdl.StringType {
		return valstr
	}
	// Everything else requires an explicit type.
	// { } are used instead of ( ) for composites
	switch k {
	case vdl.Array, vdl.Struct:
		return typestr + valstr
	case vdl.List, vdl.Set, vdl.Map:
		// Special-case empty variable-length collections, which we generate as a type
		// conversion from nil
		if !v.IsZero() {
			return typestr + valstr
		}
	}
	return typestr + "(" + valstr + ")"
}

func untypedConst(data *goData, v *vdl.Value) string {
	if native := untypedConstNative(data, v); native != "" {
		return native
	}
	return untypedConstWire(data, v)
}

func untypedConstWire(data *goData, v *vdl.Value) string { //nolint:gocyclo
	k, t := v.Kind(), v.Type()
	typestr := typeGoWire(data, t)
	if isByteList(t) {
		if v.IsZero() {
			return "nil"
		}
		return typestr + "(" + strconv.Quote(string(v.Bytes())) + ")"
	}
	switch k {
	case vdl.Any:
		if elem := v.Elem(); elem != nil {
			// For the interface{} case, we just return a Go expression representing
			// the typed elem value.  If the elem type is native, we need to return a
			// value of the native type.
			//
			// Otherwise we need to generate a Go expression of type *vom.RawBytes or
			// *vdl.Value.  Since the rest of our logic can already generate the Go
			// code for any value, we just wrap it in vom.RawBytesOf / vdl.ValueOf to
			// produce the final result.  We don't need to generate the native
			// representation, since it'll just be converted back to a wire
			// representation in RawBytesOf / ValueOf anyways.
			//
			// This may seem like a strange roundtrip, but results in less generator
			// and generated code.
			switch goAnyRepMode(data.Package) {
			case goAnyRepRawBytes:
				return data.Pkg("v.io/v23/vom") + "RawBytesOf(" + typedConstWire(data, elem) + ")"
			case goAnyRepValue:
				return data.Pkg("v.io/v23/vdl") + "ValueOf(" + typedConstWire(data, elem) + ")"
			default:
				// Special-case for error(nil) contained within an any, to avoid losing
				// type information.  Note that both of the Go expressions error(nil)
				// and (*verror.E)(nil) are valid representations of the VDL nil error.
				// But since error is a Go interface, we lose the type information from
				// error(nil) when it's contained in another interface (i.e. any).  So
				// we pick a representation that doesn't lose type information.
				if elem.Type() == vdl.ErrorType && elem.IsNil() {
					return "(*" + data.Pkg("v.io/v23/verror") + "E)(nil)"
				}
				// We need the final result to be the native type.
				return typedConst(data, elem)
			}
		}
		switch goAnyRepMode(data.Package) {
		case goAnyRepRawBytes:
			// TODO(bprosnitz) Can this just be vom.RawBytesOf(nil)
			return data.Pkg("v.io/v23/vom") + "RawBytesOf(" + data.Pkg("v.io/v23/vdl") + "ZeroValue(vdl.AnyType)" + ")"
		case goAnyRepValue:
			return data.Pkg("v.io/v23/vdl") + "ZeroValue(vdl.AnyType)"
		default:
			return "nil"
		}
	case vdl.Optional:
		if elem := v.Elem(); elem != nil {
			return untypedConst(data, elem)
		}
		return "nil"
	case vdl.TypeObject:
		return data.InitializationExpression(v.TypeObject())
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
		return typestr + v.EnumLabel()
	case vdl.Array:
		if v.IsZero() && isGoZeroValueCanonical(data, t.Elem()) {
			// If the array is zero and the element type represents zero using the go
			// zero value, we can special-case using the go zero value.
			return "{}"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			s += "\n" + untypedConst(data, v.Index(ix)) + ","
		}
		return s + "\n}"
	case vdl.List:
		if v.IsZero() {
			return "nil"
		}
		s := "{"
		for ix := 0; ix < v.Len(); ix++ {
			s += "\n" + untypedConst(data, v.Index(ix)) + ","
		}
		return s + "\n}"
	case vdl.Set, vdl.Map:
		if v.IsZero() {
			return "nil"
		}
		s := "{"
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			s += "\n" + untypedConst(data, key)
			if k == vdl.Set {
				s += ": {},"
			} else {
				s += ": " + untypedConst(data, v.MapIndex(key)) + ","
			}
		}
		return s + "\n}"
	case vdl.Struct:
		s := "{"
		hasFields := false
		for ix := 0; ix < t.NumField(); ix++ {
			vf := v.StructField(ix)
			if !vf.IsZero() || !isGoZeroValueCanonical(data, vf.Type()) {
				// Only set the field if the field isn't zero or the field type doesn't
				// represent zero using the go zero value.  Otherwise simply skip the
				// field, letting the default go zero value occur.
				s += "\n" + t.Field(ix).Name + ": " + fieldConst(data, vf) + ","
				hasFields = true
			}
		}
		if hasFields {
			s += "\n"
		}
		return s + "}"
	case vdl.Union:
		ix, vf := v.UnionField()
		var inner string
		if !vf.IsZero() || !isGoZeroValueCanonical(data, vf.Type()) {
			// Only set the field if the field isn't zero or the field type doesn't
			// represent zero using the go zero value.  Otherwise simply skip the
			// field, letting the default go zero value occur.
			inner = "Value: " + fieldConst(data, vf)
		}
		return typestr + t.Field(ix).Name + "{" + inner + "}"
	default:
		data.Env.Errors.Errorf("%s: %v untypedConstWire not implemented for %v", data.Package.Name, t, k)
		return "INVALID"
	}
}

// fieldConst deals with a quirk regarding Go composite literals.  Go allows us
// to elide the type from composite literal Y when the type is implied;
// basically when Y is contained in another composite literal X.  However it
// requires the type for Y when X is a struct and we're filling in its fields.
//
// Thus fieldConst is called when filling in struct and union fields, and
// ensures typedConst is only called when the field itself will result in
// another Go composite literal.
func fieldConst(data *goData, v *vdl.Value) string {
	if native, _, ok := findNativeType(data.Env, v.Type()); ok {
		switch native.Kind {
		case vdltool.GoKindArray, vdltool.GoKindSlice, vdltool.GoKindMap, vdltool.GoKindStruct:
			return typedConst(data, v)
		}
	}
	switch v.Kind() {
	case vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.Struct, vdl.Optional:
		return typedConst(data, v)
	}
	// Union is not in the list above.  It *is* generated as a Go composite
	// literal, but untypedConst already has the type of the concrete union struct
	// attached to it, and we don't need to convert this to the union interface
	// type, since the concrete union struct is assignable to the union interface.
	return untypedConst(data, v)
}

func formatFloat(x float64, kind vdl.Kind) string {
	var bitSize int
	switch kind {
	case vdl.Float32:
		bitSize = 32
	case vdl.Float64:
		bitSize = 64
	default:
		panic(fmt.Errorf("vdl: formatFloat unhandled kind: %v", kind))
	}
	return strconv.FormatFloat(x, 'g', -1, bitSize)
}

// typedConstNative returns a typed native constant, or returns the empty string
// if v isn't a native type.
func typedConstNative(data *goData, v *vdl.Value) string {
	return constNative(data, v, true)
}

// untypedConstNative returns an untyped native constant, or returns the empty
// string if v isn't a native type.
func untypedConstNative(data *goData, v *vdl.Value) string {
	return constNative(data, v, false)
}

func constNative(data *goData, v *vdl.Value, typed bool) string {
	if v.Type() == vdl.ErrorType {
		return constNativeError(data, v)
	}
	if native, wirePkg, ok := findNativeType(data.Env, v.Type()); ok {
		nType := nativeType(data, native, wirePkg)
		if native.Zero.Mode != vdltool.GoZeroModeUnknown && v.IsZero() {
			// This is the case where the value is zero, and the zero mode is either
			// Canonical or Unique, which means that the Go zero value of the native
			// type is sufficient to represent the value.
			if typed {
				return typedConstNativeZero(native.Kind, nType)
			}
			return untypedConstNativeZero(native.Kind, nType)
		}
		return constNativeConversion(data, v, nType, toNative(data, native, v.Type()))
	}
	return ""
}

func constNativeError(data *goData, v *vdl.Value) string {
	if elem := v.Elem(); elem != nil {
		wireError := typedConstWire(data, elem)
		return data.Pkg("v.io/v23/verror") + "FromWire(&" + wireError + ")"
	}
	return "nil"
}

func constNativeConversion(data *goData, v *vdl.Value, nType, toNative string) string {
	// TODO(toddw): Change const generation to use the same style as
	// genIsZero, which creates separate setup and expr code, so that we can
	// handle errors without panicing.
	return fmt.Sprintf(`func() %[1]s {
	var native %[1]s
	wire := %[2]s
	if err := %[3]s(wire, &native); err != nil {
		panic(err)
	}
	return native
}()`, nType, typedConstWire(data, v), toNative)
}

func typedConstNativeZero(kind vdltool.GoKind, nType string) string {
	zero := untypedConstNativeZero(kind, nType)
	switch kind {
	case vdltool.GoKindStruct, vdltool.GoKindArray:
		return zero // untyped const is already typed, e.g. NativeType{}
	default:
		return nType + "(" + zero + ")" // e.g. NativeType(0)
	}
}

func untypedConstNativeZero(kind vdltool.GoKind, nType string) string {
	switch kind {
	case vdltool.GoKindStruct, vdltool.GoKindArray:
		return nType + "{}" // No way to create an untyped zero struct or array.
	case vdltool.GoKindBool:
		return "false"
	case vdltool.GoKindNumber:
		return "0"
	case vdltool.GoKindString:
		return `""`
	default:
		return "nil"
	}
}
