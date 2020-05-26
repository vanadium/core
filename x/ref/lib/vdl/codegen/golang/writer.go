// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"strconv"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

// defineWrite returns the VDLWrite method for the def type.
func defineWrite(data *goData, def *compile.TypeDef) string {
	g := genWrite{goData: data}
	return g.Gen(def)
}

type genWrite struct {
	*goData
	// anonWriters holds the anonymous types that we need to generate __VDLWrite
	// functions for.  We only generate the function if we're the first one to add
	// it to goData; otherwise someone else has already generated it.
	anonWriters []*vdl.Type
}

func (g *genWrite) Gen(def *compile.TypeDef) string {
	var s string
	if def.Type.Kind() == vdl.Union {
		s += g.genUnionDef(def)
	} else {
		s += g.genDef(def)
	}
	s += g.genAnonDef()
	return s
}

func (g *genWrite) genDef(def *compile.TypeDef) string {
	body := g.body(def.Type, namedArg{"x", false}, false, true)
	return fmt.Sprintf(`
func (x %[1]s) VDLWrite(enc %[2]sEncoder) error {  //nolint:gocyclo %[3]s
}
`, def.Name, g.Pkg("v.io/v23/vdl"), body)
}

// genUnionDef is a special-case, since we need to generate methods for each
// concrete union struct.
func (g *genWrite) genUnionDef(def *compile.TypeDef) string {
	var s string
	for index := 0; index < def.Type.NumField(); index++ {
		field := def.Type.Field(index)
		body := g.bodyUnion(index, field, namedArg{"x", false})
		s += fmt.Sprintf(`
func (x %[1]s%[2]s) VDLWrite(enc %[3]sEncoder) error { //nolint:gocyclo
	if err := enc.StartValue(%[4]s); err != nil {
		return err
	}%[5]s
	return enc.FinishValue()
}
`, def.Name, field.Name, g.Pkg("v.io/v23/vdl"), g.TypeOf(def.Type), body)
	}
	return s
}

func (g *genWrite) genAnonDef() string {
	var s string
	// Generate the __VDLWrite functions for anonymous types.  Creating the
	// function for one type may cause us to need more, e.g. [][]Certificate.  So
	// we just keep looping until there are no new functions to generate.  There's
	// no danger of infinite looping, since cyclic anonymous types are disallowed
	// in the VDL type system.
	for len(g.anonWriters) > 0 {
		anons := g.anonWriters
		g.anonWriters = nil
		for _, anon := range anons {
			body := g.body(anon, namedArg{"x", false}, false, true)
			s += fmt.Sprintf(`
func %[1]s(enc %[2]sEncoder, x %[3]s) error {%[4]s
}
`, g.anonWriterName(anon), g.Pkg("v.io/v23/vdl"), typeGo(g.goData, anon), body)
		}
	}
	return s
}

func (g *genWrite) body(tt *vdl.Type, arg namedArg, skipNilCheck, topLevel bool) string { //nolint:gocyclo
	kind := tt.Kind()
	sta := fmt.Sprintf(`
	if err := enc.StartValue(%[1]s); err != nil {
		return err
	}`, g.TypeOf(tt))
	fin := `
	if err := enc.FinishValue(); err != nil {
		return err
	}`
	retnil := ""
	if topLevel {
		fin = `
	return enc.FinishValue()`
		retnil = `
	return nil`
	}
	// Handle special cases.  The ordering of the cases is very important.
	switch {
	case tt == vdl.ErrorType:
		// Error types call verror.VDLWrite directly, similar to named types, but
		// even more special-cased.  Appears before optional, since ErrorType is
		// optional.
		return g.bodyError(arg)
	case kind == vdl.Optional:
		// Optional types need special nil handling.  Appears before native types,
		// to allow native types to be optional.
		return g.bodyOptional(tt, arg, skipNilCheck)
	case !topLevel && isNativeType(g.Env, tt):
		// Non-top-level native types need an initial native conversion, while
		// top-level native types use the regular logic to create VDLWrite for the
		// wire type.  Appears as early as possible, so that all subsequent cases
		// have nativity handled correctly.
		return g.bodyNative(tt, arg, skipNilCheck)
	case tt.IsBytes():
		// Bytes call the fast Encoder.WriteValueBytes method.  Appears before named
		// types to avoid an extra VDLWrite method call, and appears before
		// anonymous lists to avoid slow byte-at-a-time encoding.
		return g.bodyFastpath(tt, arg, false) + retnil
	case !topLevel && tt.Name() != "" && !g.hasFastpath(tt, false):
		// Non-top-level named types call the VDLWrite method defined on the arg.
		// The top-level type is always named, and needs a real body generated.
		// We let fastpath types drop through, to avoid the extra method call.
		return g.bodyCallVDLWrite(tt, arg, skipNilCheck)
	case !topLevel && (kind == vdl.List || kind == vdl.Set || kind == vdl.Map):
		// Non-top-level anonymous types call the unexported __VDLWrite* functions
		// generated in g.Gen, after the main VDLWrite method has been generated.
		// Top-level anonymous types use the regular logic, to generate the actual
		// body of the __VDLWrite* functions.
		return g.bodyAnon(tt, arg)
	}
	// Handle each kind of type.
	if g.hasFastpath(tt, false) {
		// Don't perform native conversions, since they were already performed above.
		// All scalar types have a fastpath.
		return g.bodyFastpath(tt, arg, false) + retnil
	}
	switch kind {
	case vdl.Array:
		return sta + g.bodyArray(tt, arg) + fin
	case vdl.List:
		return sta + g.bodyList(tt, arg) + fin
	case vdl.Set:
		return sta + g.bodySet(tt, arg) + fin
	case vdl.Map:
		return sta + g.bodyMap(tt, arg) + fin
	case vdl.Struct:
		return sta + g.bodyStruct(tt, arg) + fin
	case vdl.Any:
		return g.bodyAny(arg, skipNilCheck)
	default:
		panic(fmt.Errorf("VDLWrite unhandled type %s", tt))
	}
}

func (g *genWrite) bodyError(arg namedArg) string {
	return fmt.Sprintf(`
	if err := %[1]sVDLWrite(enc, %[2]s); err != nil {
		return err
	}`, g.Pkg("v.io/v23/verror"), arg.Name)
}

func (g *genWrite) bodyNative(tt *vdl.Type, arg namedArg, skipNilCheck bool) string {
	s := fmt.Sprintf(`
	var wire %[1]s
	if err := %[1]sFromNative(&wire, %[2]s); err != nil {
		return err
	}`, typeGoWire(g.goData, tt), arg.Ref())
	return s + g.bodyCallVDLWrite(tt, typedArg("wire", tt), skipNilCheck)
}

func (g *genWrite) bodyCallVDLWrite(tt *vdl.Type, arg namedArg, skipNilCheck bool) string {
	s := fmt.Sprintf(`
	if err := %[1]s.VDLWrite(enc); err != nil {
		return err
	}`, arg.Name)
	// Handle cases where a nil arg would cause the VDLWrite call to panic.  Here
	// are the potential cases:
	//   Optional:       Never happens; optional types already handled.
	//   TypeObject:     The vdl.Type.VDLWrite method handles nil.
	//   List, Set, Map: VDLWrite uses len(arg) and "range arg", which handle nil.
	//   Union:          Needs handling below.
	//   Any:            Needs handling below.
	if k := tt.Kind(); !skipNilCheck && (k == vdl.Union || k == vdl.Any) {
		s = fmt.Sprintf(`
	switch {
	case %[1]s == nil:
		// Write the zero value of the %[2]s type.
		if err := %[3]sZeroValue(%[4]s).VDLWrite(enc); err != nil {
			return err
		}
	default:%[5]s
	}`, arg.Ref(), k.String(), g.Pkg("v.io/v23/vdl"), g.TypeOf(tt), s)
	}
	return s
}

func (g *genWrite) anonWriterName(tt *vdl.Type) string {
	return fmt.Sprintf("vdlWriteAnon%s%d", tt.Kind().CamelCase(), g.goData.anonWriters[tt])
}

func (g *genWrite) bodyAnon(tt *vdl.Type, arg namedArg) string {
	id := g.goData.anonWriters[tt]
	if id == 0 {
		// This is the first time we've encountered this type, add it.
		id = len(g.goData.anonWriters) + 1
		g.goData.anonWriters[tt] = id
		g.anonWriters = append(g.anonWriters, tt)
	}
	return fmt.Sprintf(`
	if err := %[1]s(enc, %[2]s); err != nil {
		return err
	}`, g.anonWriterName(tt), arg.Ref())
}

func (g *genWrite) hasFastpath(tt *vdl.Type, nativeConv bool) bool {
	method, _, _ := g.fastpathInfo(tt, namedArg{}, nativeConv)
	return method != ""
}

func (g *genWrite) fastpathInfo(tt *vdl.Type, arg namedArg, nativeConv bool) (method string, params []string, init string) { //nolint:gocyclo
	kind := tt.Kind()
	p1, p2 := "", ""
	// When fastpathInfo is called in order to produce NextEntry* or NextField*
	// methods, we must perform the native conversion if tt is a native type.
	if nativeConv && isNativeType(g.Env, tt) {
		init = fmt.Sprintf(`
	var wire %[1]s
	if err := %[1]sFromNative(&wire, %[2]s); err != nil {
		return err
	}`, typeGoWire(g.goData, tt), arg.Ref())
		arg = typedArg("wire", tt)
	}
	// Handle bytes fastpath.  Go doesn't allow type conversions from []MyByte to
	// []byte, but the reflect package does let us perform this conversion.
	if tt.IsBytes() {
		method, p1 = "Bytes", g.TypeOf(tt)
		switch {
		case tt.Elem() != vdl.ByteType:
			slice := arg.Ref()
			if kind == vdl.Array {
				slice = arg.Name + "[:]"
			}
			p2 = fmt.Sprintf(`%sValueOf(%s).Bytes()`, g.Pkg("reflect"), slice)
		case kind == vdl.Array:
			p2 = arg.Name + "[:]"
		default:
			p2 = g.cast(arg.Ref(), tt, vdl.ListType(vdl.ByteType))
		}
	}
	// Handle scalar fastpaths.
	switch kind {
	case vdl.Bool:
		method, p1, p2 = "Bool", g.TypeOf(tt), g.cast(arg.Ref(), tt, vdl.BoolType)
	case vdl.String:
		method, p1, p2 = "String", g.TypeOf(tt), g.cast(arg.Ref(), tt, vdl.StringType)
	case vdl.Enum:
		method, p1, p2 = "String", g.TypeOf(tt), arg.Name+".String()"
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		method, p1, p2 = "Uint", g.TypeOf(tt), g.cast(arg.Ref(), tt, vdl.Uint64Type)
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		method, p1, p2 = "Int", g.TypeOf(tt), g.cast(arg.Ref(), tt, vdl.Int64Type)
	case vdl.Float32, vdl.Float64:
		method, p1, p2 = "Float", g.TypeOf(tt), g.cast(arg.Ref(), tt, vdl.Float64Type)
	case vdl.TypeObject:
		method, p2 = "TypeObject", arg.Ref()
	}
	if method == "" {
		return "", nil, ""
	}
	if p1 != "" {
		params = append(params, p1)
	}
	params = append(params, p2)
	return
}

func (g *genWrite) cast(value string, tt, exact *vdl.Type) string {
	if tt != exact {
		// The types don't have an exact match, so we need a conversion.  This
		// occurs for all named types, as well as numeric types where the bitlen
		// isn't exactly the same.  E.g.
		//
		//   type Foo uint16
		//
		//   x := Foo(123)
		//   enc.WriteValueUint(tt, uint64(x))
		return typeGoWire(g.goData, exact) + "(" + value + ")"
	}
	return value
}

func (g *genWrite) bodyFastpath(tt *vdl.Type, arg namedArg, nativeConv bool) string {
	method, params, init := g.fastpathInfo(tt, arg, nativeConv)
	return fmt.Sprintf(`%[1]s
	if err := enc.WriteValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
}

const (
	encNextEntry = `
	if err := enc.NextEntry(false); err != nil {
		return err
	}`
	encNextEntryDone = `
	if err := enc.NextEntry(true); err != nil {
		return err
	}`
	encNextFieldDone = `
	if err := enc.NextField(-1); err != nil {
		return err
	}`
)

func (g *genWrite) bodyArray(tt *vdl.Type, arg namedArg) string {
	elemArg := typedArg("elem", tt.Elem())
	s := fmt.Sprintf(`
	for _, elem := range %[1]s {`, arg.Ref())
	method, params, init := g.fastpathInfo(tt.Elem(), elemArg, true)
	if method != "" {
		s += fmt.Sprintf(`%[1]s
	if err := enc.NextEntryValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
	} else {
		s += encNextEntry
		s += g.body(tt.Elem(), elemArg, false, false)
	}
	s += `
	}` + encNextEntryDone
	return s
}

func (g *genWrite) bodyList(tt *vdl.Type, arg namedArg) string {
	elemArg := typedArg("elem", tt.Elem())
	s := fmt.Sprintf(`
	if err := enc.SetLenHint(len(%[1]s)); err != nil {
		return err
	}
	for _, elem := range %[1]s {`, arg.Ref())
	method, params, init := g.fastpathInfo(tt.Elem(), elemArg, true)
	if method != "" {
		s += fmt.Sprintf(`%[1]s
	if err := enc.NextEntryValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
	} else {
		s += encNextEntry
		s += g.body(tt.Elem(), elemArg, false, false)
	}
	s += `
	}` + encNextEntryDone
	return s
}

func (g *genWrite) bodySet(tt *vdl.Type, arg namedArg) string {
	keyArg := typedArg("key", tt.Key())
	s := fmt.Sprintf(`
	if err := enc.SetLenHint(len(%[1]s)); err != nil {
		return err
	}
	for key := range %[1]s {`, arg.Ref())
	method, params, init := g.fastpathInfo(tt.Key(), keyArg, true)
	if method != "" {
		s += fmt.Sprintf(`%[1]s
	if err := enc.NextEntryValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
	} else {
		s += encNextEntry
		s += g.body(tt.Key(), keyArg, false, false)
	}
	s += `
	}` + encNextEntryDone
	return s
}

func (g *genWrite) bodyMap(tt *vdl.Type, arg namedArg) string {
	keyArg, elemArg := typedArg("key", tt.Key()), typedArg("elem", tt.Elem())
	s := fmt.Sprintf(`
	if err := enc.SetLenHint(len(%[1]s)); err != nil {
		return err
	}
	for key, elem := range %[1]s {`, arg.Ref())
	method, params, init := g.fastpathInfo(tt.Key(), keyArg, true)
	if method != "" {
		s += fmt.Sprintf(`%[1]s
	if err := enc.NextEntryValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
	} else {
		s += encNextEntry
		s += g.body(tt.Key(), keyArg, false, false)
	}
	s += g.body(tt.Elem(), elemArg, false, false)
	s += `
	}` + encNextEntryDone
	return s
}

func (g *genWrite) bodyStruct(tt *vdl.Type, arg namedArg) string {
	var s string
	for index := 0; index < tt.NumField(); index++ {
		field := tt.Field(index)
		fieldArg := arg.Field(field)
		zero := genIsZero{g.goData}
		expr := zero.Expr(ifNeZero, field.Type, fieldArg, field.Name)
		s += fmt.Sprintf(`
	if %[1]s {`, expr)
		method, params, init := g.fastpathInfo(field.Type, fieldArg, true)
		if method != "" {
			params = append([]string{strconv.Itoa(index)}, params...)
			s += fmt.Sprintf(`%[1]s
		if err := enc.NextFieldValue%[2]s(%[3]s); err != nil {
			return err
		}`, init, method, strings.Join(params, ", "))
		} else {
			// The second-to-last true parameter indicates that nil checks can be
			// skipped, since we've already ensured the field isn't zero here.
			fieldBody := g.body(field.Type, fieldArg, true, false)
			s += fmt.Sprintf(`
		if err := enc.NextField(%[1]d); err != nil {
			return err
		}%[2]s`, index, fieldBody)
		}
		s += `
	}`
	}
	s += encNextFieldDone
	return s
}

func (g *genWrite) bodyUnion(index int, field vdl.Field, arg namedArg) string {
	var s string
	fieldArg := typedArg(arg.Name+".Value", field.Type)
	method, params, init := g.fastpathInfo(field.Type, fieldArg, true)
	if method != "" {
		params = append([]string{strconv.Itoa(index)}, params...)
		s += fmt.Sprintf(`%[1]s
	if err := enc.NextFieldValue%[2]s(%[3]s); err != nil {
		return err
	}`, init, method, strings.Join(params, ", "))
	} else {
		s += fmt.Sprintf(`
	if err := enc.NextField(%[1]d); err != nil {
			return err
	}`, index)
		s += g.body(field.Type, fieldArg, false, false)
	}
	s += encNextFieldDone
	return s
}

func (g *genWrite) bodyOptional(tt *vdl.Type, arg namedArg, skipNilCheck bool) string {
	s := `
	enc.SetNextStartValueIsOptional()` + g.body(tt.Elem(), arg, false, false)
	if !skipNilCheck {
		s = fmt.Sprintf(`
	if %[1]s == nil {
		if err := enc.NilValue(%[2]s); err != nil {
			return err
		}
	} else {%[3]s
	}`, arg.Name, g.TypeOf(tt), s)
	}
	return s
}

func (g *genWrite) bodyAny(arg namedArg, skipNilCheck bool) string {
	mode := goAnyRepMode(g.Package)
	// Handle interface{} special-case.
	if mode == goAnyRepInterface {
		return fmt.Sprintf(`
	if err := %[1]sWrite(enc, %[2]s); err != nil {
		return err
	}`, g.Pkg("v.io/v23/vdl"), arg.Ref())
	}
	// Handle vdl.Value and vom.RawBytes representations.
	s := fmt.Sprintf(`
	if err := %[1]s.VDLWrite(enc); err != nil {
		return err
	}`, arg.Name)
	if !skipNilCheck {
		s = fmt.Sprintf(`
	if %[1]s == nil {
		if err := enc.NilValue(%[2]sAnyType); err != nil {
			return err
		}
	} else {%[3]s
	}`, arg.Ref(), g.Pkg("v.io/v23/vdl"), s)
	}
	return s
}
