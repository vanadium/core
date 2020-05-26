// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"strconv"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

// defineRead returns VDLRead method for the def type.
func defineRead(data *goData, def *compile.TypeDef) string {
	g := genRead{goData: data}
	return g.Gen(def)
}

type genRead struct {
	*goData
	// anonReaders holds the anonymous types that we need to generate __VDLRead
	// functions for.  We only generate the function if we're the first one to add
	// it to goData; otherwise someone else has already generated it.
	anonReaders []*vdl.Type
}

func (g *genRead) Gen(def *compile.TypeDef) string {
	var s string
	if def.Type.Kind() == vdl.Union {
		s += g.genUnionDef(def)
	} else {
		s += g.genDef(def)
	}
	s += g.genAnonDef()
	return s
}

func (g *genRead) genDef(def *compile.TypeDef) string {
	s := fmt.Sprintf(`
func (x *%[1]s) VDLRead(dec %[2]sDecoder) error { //nolint:gocyclo`, def.Name, g.Pkg("v.io/v23/vdl"))
	// Structs need to be zeroed, since some of the fields may be missing.
	if def.Type.Kind() == vdl.Struct {
		s += fmt.Sprintf(`
	*x = %[1]s`, typedConstWire(g.goData, vdl.ZeroValue(def.Type)))
	}
	s += g.body(def.Type, namedArg{"x", true}, true) + `
}
`
	return s
}

// genUnionDef is a special-case, since we can't attach methods to a pointer
// receiver interface.  Instead we create a package-level function.
func (g *genRead) genUnionDef(def *compile.TypeDef) string {
	s := fmt.Sprintf(`
func %[1]s(dec %[2]sDecoder, x *%[3]s) error {  //nolint:gocyclo
	if err := dec.StartValue(%[4]s); err != nil {
		return err
	}`, unionReadFuncName(def), g.Pkg("v.io/v23/vdl"), def.Name, g.TypeOf(def.Type))
	s += g.bodyUnion(def.Type, namedArg{"x", true}) + `
	return dec.FinishValue()
}
`
	return s
}

func unionReadFuncName(def *compile.TypeDef) string {
	if def.Exported {
		return "VDLRead" + def.Name
	}
	return "vdlRead" + def.Name
}

func (g *genRead) genAnonDef() string {
	var s string
	// Generate the vdlRead functions for anonymous types.  Creating the
	// function for one type may cause us to need more, e.g. [][]Certificate.  So
	// we just keep looping until there are no new functions to generate.  There's
	// no danger of infinite looping, since cyclic anonymous types are disallowed
	// in the VDL type system.
	for len(g.anonReaders) > 0 {
		anons := g.anonReaders
		g.anonReaders = nil
		for _, anon := range anons {
			s += fmt.Sprintf(`
func %[1]s(dec %[2]sDecoder, x *%[3]s) error {`, g.anonReaderName(anon), g.Pkg("v.io/v23/vdl"), typeGoWire(g.goData, anon))
			s += g.body(anon, namedArg{"x", true}, true) + `
}
`
		}
	}
	return s
}

func (g *genRead) body(tt *vdl.Type, arg namedArg, topLevel bool) string { //nolint:gocyclo
	kind := tt.Kind()
	sta := fmt.Sprintf(`
	if err := dec.StartValue(%[1]s); err != nil {
		return err
	}`, g.TypeOf(tt))
	fin := `
	if err := dec.FinishValue(); err != nil {
		return err
	}`
	retnil := ""
	if topLevel {
		fin = `
	return dec.FinishValue()`
		retnil = `
	return nil`
	}
	// Handle special cases.  The ordering of the cases is very important.
	switch {
	case tt == vdl.ErrorType:
		// Error types call verror.VDLRead directly, similar to named types, but
		// even more special-cased.  Appears before optional, since ErrorType is
		// optional.
		return g.bodyError(arg)
	case kind == vdl.Optional:
		// Optional types need special nil handling.  Appears as early as possible,
		// so that all subsequent cases have optionality handled correctly.
		return sta + g.bodyOptional(tt, arg)
	case !topLevel && isNativeType(g.Env, tt):
		// Non-top-level native types need a final native conversion, while
		// top-level native types use the regular logic to create VDLRead for the
		// wire type.  Appears as early as possible, so that all subsequent cases
		// have nativity handled correctly.
		return g.bodyNative(tt, arg)
	case tt.IsBytes():
		// Bytes call the fast Decoder.ReadValueBytes method.  Appears before named
		// types to avoid an extra VDLRead method call, and appears before anonymous
		// lists to avoid slow byte-at-a-time decoding.
		return g.bodyBytes(tt, arg) + retnil
	case !topLevel && tt.Name() != "" && !g.hasScalarInfo(tt):
		// Non-top-level named types call the VDLRead method defined on the arg.
		// The top-level type is always named, and needs a real body generated.
		// We let scalars drop through, to avoid the extra method call.
		return g.bodyCallVDLRead(tt, arg)
	case !topLevel && (kind == vdl.List || kind == vdl.Set || kind == vdl.Map):
		// Non-top-level anonymous types call the unexported __VDLRead* functions
		// generated in g.Gen, after the main VDLRead method has been generated.
		// Top-level anonymous types use the regular logic, to generate the actual
		// body of the __VDLRead* functions.
		return g.bodyAnon(tt, arg)
	}
	// Handle each kind of type.
	if g.hasScalarInfo(tt) {
		return g.bodyScalar(tt, arg) + retnil
	}
	switch kind {
	case vdl.Array:
		return sta + g.bodyArray(tt, arg) + fin
	case vdl.List:
		return sta + g.bodyList(tt, arg)
	case vdl.Set, vdl.Map:
		return sta + g.bodySetMap(tt, arg)
	case vdl.Struct:
		return sta + g.bodyStruct(tt, arg)
	case vdl.Any:
		return g.bodyAny(tt, arg)
	default:
		panic(fmt.Errorf("VDLRead unhandled type %s", tt))
	}
}

func (g *genRead) bodyError(arg namedArg) string {
	return fmt.Sprintf(`
	if err := %[1]sVDLRead(dec, &%[2]s); err != nil {
		return err
	}`, g.Pkg("v.io/v23/verror"), arg.Name)
}

func (g *genRead) bodyNative(tt *vdl.Type, arg namedArg) string {
	wireArg := typedArg("wire", tt)
	wireBody := g.bodyCallVDLRead(tt, wireArg)
	return fmt.Sprintf(`
	var %[1]s %[2]s%[3]s
	if err := %[2]sToNative(%[1]s, %[4]s); err != nil {
		return err
	}`, wireArg.Name, typeGoWire(g.goData, tt), wireBody, arg.Ptr())
}

func (g *genRead) bodyCallVDLRead(tt *vdl.Type, arg namedArg) string {
	if tt.Kind() == vdl.Union {
		// Unions are a special-case, since we can't attach methods to a pointer
		// receiver interface.  Instead we call the package-level function.
		def := g.Env.FindTypeDef(tt)
		return fmt.Sprintf(`
	if err := %[1]s%[2]s(dec, %[3]s); err != nil {
		return err
	}`, g.Pkg(def.File.Package.GenPath), unionReadFuncName(def), arg.Ptr())
	}
	return fmt.Sprintf(`
	if err := %[1]s.VDLRead(dec); err != nil {
		return err
	}`, arg.Name)
}

func (g *genRead) anonReaderName(tt *vdl.Type) string {
	return fmt.Sprintf("vdlReadAnon%s%d", tt.Kind().CamelCase(), g.goData.anonReaders[tt])
}

func (g *genRead) bodyAnon(tt *vdl.Type, arg namedArg) string {
	id := g.goData.anonReaders[tt]
	if id == 0 {
		// This is the first time we've encountered this type, add it.
		id = len(g.goData.anonReaders) + 1
		g.goData.anonReaders[tt] = id
		g.anonReaders = append(g.anonReaders, tt)
	}
	return fmt.Sprintf(`
	if err := %[1]s(dec, %[2]s); err != nil {
		return err
	}`, g.anonReaderName(tt), arg.Ptr())
}

func (g *genRead) hasScalarInfo(tt *vdl.Type) bool {
	_, _, exact := g.scalarInfo(tt)
	return exact != nil
}

// TODO(toddw): Change this to fastpathInfo and handle Bytes here as well, just
// like the writer.  Also add Decoder.NextEntryValueBytes for symmetry.
func (g *genRead) scalarInfo(tt *vdl.Type) (method, params string, exact *vdl.Type) {
	bitlen := strconv.Itoa(tt.Kind().BitLen())
	switch tt.Kind() {
	case vdl.Bool:
		return "Bool", "", vdl.BoolType
	case vdl.String, vdl.Enum:
		return "String", "", vdl.StringType
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return "Uint", bitlen, vdl.Uint64Type
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return "Int", bitlen, vdl.Int64Type
	case vdl.Float32, vdl.Float64:
		return "Float", bitlen, vdl.Float64Type
	case vdl.TypeObject:
		return "TypeObject", "", vdl.TypeObjectType
	}
	return "", "", nil
}

func (g *genRead) setEnum(argEnum, argLabel string) string {
	return fmt.Sprintf(`
	if err := %[1]s.Set(%[2]s); err != nil {
		return err
	}`, argEnum, argLabel)
}

func (g *genRead) bodyScalar(tt *vdl.Type, arg namedArg) string {
	method, params, exact := g.scalarInfo(tt)
	valueCast := "value"
	if tt != exact {
		// The types don't have an exact match, so we need a conversion.  This
		// occurs for all named types, as well as numeric types where the bitlen
		// isn't exactly the same.  E.g.
		//
		//   type Foo uint16
		//
		//   switch value, err := dec.ReadValueUint(16); {
		//   case err != nil:
		//     return err
		//   default:
		//     *x = Foo(value)
		//   }
		valueCast = typeGoWire(g.goData, tt) + "(value)"
	}
	s := fmt.Sprintf(`
	switch value, err := dec.ReadValue%[1]s(%[2]s); {
	case err != nil:
		return err
	default:`, method, params)
	if tt.Kind() == vdl.Enum {
		s += g.setEnum(arg.Name, "value")
	} else {
		s += fmt.Sprintf(`
		%[1]s = %[2]s`, arg.Ref(), valueCast)
	}
	return s + `
	}`
}

func (g *genRead) bodyBytes(tt *vdl.Type, arg namedArg) string {
	kind := tt.Kind()
	// Go doesn't allow type conversions from []MyByte to []byte, but the reflect
	// package does let us perform this conversion.  We slice arrays so that we
	// can fill them in directly.
	init, fixedLen, needAssign := "", -1, false
	switch {
	case kind == vdl.Array:
		fixedLen = tt.Len()
		if tt.Elem() == vdl.ByteType {
			init = fmt.Sprintf(`bytes := %s[:]`, arg.Name)
		} else {
			init = fmt.Sprintf(`bytes := %sValueOf(%s[:]).Bytes()`, g.Pkg("reflect"), arg.Name)
		}
	case tt.Name() != "" || tt.Elem() != vdl.ByteType:
		init, needAssign = `var bytes []byte`, true
	}
	fillArg := arg.Ptr()
	if init != "" {
		fillArg = "&bytes"
		init = "\n" + init
	}
	s := fmt.Sprintf(`%[1]s
	if err := dec.ReadValueBytes(%[2]d, %[3]s); err != nil {
		return err
	}`, init, fixedLen, fillArg)
	if needAssign {
		if tt.Elem() == vdl.ByteType {
			s += fmt.Sprintf(`
	%[1]s = bytes`, arg.Ref())
		} else {
			s += fmt.Sprintf(`
	%[1]sValueOf(%[2]s).Elem().SetBytes(bytes)`, g.Pkg("reflect"), arg.Ptr())
		}
	}
	return s
}

func (g *genRead) nextEntrySwitch(varName string, ttEntry *vdl.Type) (string, *vdl.Type) {
	s := `
	switch done, err := dec.NextEntry(); {`
	method, params, exact := g.scalarInfo(ttEntry)
	// TODO(toddw): Add fastpath support for native types with scalar wire types.
	if exact != nil && !isNativeType(g.Env, ttEntry) {
		s = fmt.Sprintf(`
	switch done, %[1]s, err := dec.NextEntryValue%[2]s(%[3]s); {`, varName, method, params)
	}
	return s, exact
}

func (g *genRead) bodyArray(tt *vdl.Type, arg namedArg) string {
	s := fmt.Sprintf(`
	for index := 0; index < %[1]d; index++ {`, tt.Len())
	elem := "elem"
	switchLine, exact := g.nextEntrySwitch(elem, tt.Elem())
	s += switchLine + fmt.Sprintf(`
		case err != nil:
			return err
		case done:
			return %[1]sErrorf("short array, got len %%d < %[2]d %%T)", index, %[3]s)
		default:`, g.Pkg("fmt"), tt.Len(), arg.Ref())
	elemArg := arg.ArrayIndex("index", tt.Elem())
	switch {
	case exact == nil:
		s += g.body(tt.Elem(), elemArg, false)
	case tt.Elem().Kind() == vdl.Enum:
		s += g.setEnum(elemArg.Name, elem)
	default:
		if tt.Elem() != exact {
			elem = typeGoWire(g.goData, tt.Elem()) + "(" + elem + ")"
		}
		s += fmt.Sprintf(`
			%[1]s = %[2]s`, elemArg.Name, elem)
	}
	return s + fmt.Sprintf(`
		}
	}
	switch done, err := dec.NextEntry(); {
	case err != nil:
		return err
	case !done:
		return %[1]sErrorf("long array, got len > %[2]d %%T", %[3]s)
	}`, g.Pkg("fmt"), tt.Len(), arg.Ref())
}

func (g *genRead) bodyList(tt *vdl.Type, arg namedArg) string {
	s := fmt.Sprintf(`
	if len := dec.LenHint(); len > 0 {
		%[1]s = make(%[2]s, 0, len)
	} else {
		%[1]s = nil
	}
	for {`, arg.Ref(), typeGoWire(g.goData, tt))
	elem := "elem"
	switchLine, exact := g.nextEntrySwitch(elem, tt.Elem())
	s += switchLine + `
		case err != nil:
			return err
		case done:
			return dec.FinishValue()
		default:`
	switch {
	case exact == nil:
		elemArg := typedArg(elem, tt.Elem())
		elemBody := g.body(tt.Elem(), elemArg, false)
		s += fmt.Sprintf(`
			var %[1]s %[2]s%[3]s`, elem, typeGo(g.goData, tt.Elem()), elemBody)
	case tt.Elem().Kind() == vdl.Enum:
		s += fmt.Sprintf(`
			var enum %[1]s%[2]s`, typeGo(g.goData, tt.Elem()), g.setEnum("enum", elem))
		elem = "enum"
	case tt.Elem() != exact:
		elem = typeGoWire(g.goData, tt.Elem()) + "(" + elem + ")"
	}
	return s + fmt.Sprintf(`
			%[1]s = append(%[1]s, %[2]s)
		}
	}`, arg.Ref(), elem)
}

func (g *genRead) bodySetMap(tt *vdl.Type, arg namedArg) string {
	s := fmt.Sprintf(`
	var tmpMap %[1]s
	if len := dec.LenHint(); len > 0 {
		tmpMap = make(%[1]s, len)
  }
	for {`, typeGoWire(g.goData, tt))
	key := "key"
	switchLine, exact := g.nextEntrySwitch(key, tt.Key())
	s += switchLine + fmt.Sprintf(`
		case err != nil:
			return err
		case done:
			%[1]s = tmpMap
			return dec.FinishValue()
		default:`, arg.Ref())
	switch {
	case exact == nil:
		keyArg := typedArg(key, tt.Key())
		keyBody := g.body(tt.Key(), keyArg, false)
		s += fmt.Sprintf(`
			var %[1]s %[2]s%[3]s`, key, typeGo(g.goData, tt.Key()), keyBody)
	case tt.Key().Kind() == vdl.Enum:
		s += fmt.Sprintf(`
			var keyEnum %[1]s%[2]s`, typeGo(g.goData, tt.Key()), g.setEnum("keyEnum", key))
		key = "keyEnum"
	case tt.Key() != exact:
		key = typeGoWire(g.goData, tt.Key()) + "(" + key + ")"
	}
	elem := "struct{}{}"
	if tt.Kind() == vdl.Map {
		elem = "elem"
		elemArg := typedArg(elem, tt.Elem())
		elemBody := g.body(tt.Elem(), elemArg, false)
		s += fmt.Sprintf(`
			var %[1]s %[2]s%[3]s`, elem, typeGo(g.goData, tt.Elem()), elemBody)
	}
	return s + fmt.Sprintf(`
			if tmpMap == nil {
				tmpMap = make(%[1]s)
			}
			tmpMap[%[2]s] = %[3]s
		}
	}`, typeGoWire(g.goData, tt), key, elem)
}

func (g *genRead) bodyStruct(tt *vdl.Type, arg namedArg) string {
	s := fmt.Sprintf(`
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != %[1]s {
			index = %[1]s.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}`, g.TypeOf(tt))
	if tt.NumField() == 1 {
		field := tt.Field(0)
		fieldBody := g.body(field.Type, arg.Field(field), false)
		s += fmt.Sprintf(`
		if index == 0 {
			%[1]s
		}
	}`, fieldBody)
		return s
	}

	s += `
		switch index {`
	for index := 0; index < tt.NumField(); index++ {
		field := tt.Field(index)
		fieldBody := g.body(field.Type, arg.Field(field), false)
		s += fmt.Sprintf(`
		case %[1]d:%[2]s`, index, fieldBody)
	}
	return s + `
		}
	}`
}

func (g *genRead) bodyUnion(tt *vdl.Type, arg namedArg) string {
	s := fmt.Sprintf(`
	decType := dec.Type()
	index, err := dec.NextField()
	switch {
	case err != nil:
		return err
	case index == -1:
		return %[1]sErrorf("missing field in union %%T, from %%v", %[2]s, decType)
	}
	if decType != %[3]s {
		name := decType.Field(index).Name
		index = %[3]s.FieldIndexByName(name)
		if index == -1 {
			return %[1]sErrorf("field %%q not in union %%T, from %%v", name, %[2]s, decType)
		}
	}`, g.Pkg("fmt"), arg.Ptr(), g.TypeOf(tt))

	if tt.NumField() == 1 {
		field := tt.Field(0)
		fieldArg := typedArg("field.Value", field.Type)
		fieldBody := g.body(field.Type, fieldArg, false)
		s += fmt.Sprintf(`
	if index == 0 {
		var field %[1]s%[2]s%[3]s
		%[4]s = field
	}`, typeGoWire(g.goData, tt), field.Name, fieldBody, arg.Ref())
		return s
	}
	s += `
	switch index {`
	for index := 0; index < tt.NumField(); index++ {
		// TODO(toddw): Change to using pointers to the union field structs, to
		// resolve https://v.io/i/455
		field := tt.Field(index)
		fieldArg := typedArg("field.Value", field.Type)
		fieldBody := g.body(field.Type, fieldArg, false)
		s += fmt.Sprintf(`
	case %[1]d:
		var field %[2]s%[3]s%[4]s
		%[5]s = field`, index, typeGoWire(g.goData, tt), field.Name, fieldBody, arg.Ref())
	}
	return s + fmt.Sprintf(`
	}
	switch index, err := dec.NextField(); {
	case err != nil:
		return err
	case index != -1:
		return %[1]sErrorf("extra field %%d in union %%T, from %%v", index, %[2]s, dec.Type())
	}`, g.Pkg("fmt"), arg.Ptr())
}

func (g *genRead) bodyOptional(tt *vdl.Type, arg namedArg) string {
	// NOTE: arg.IsPtr is always true here, since tt is optional.
	body := g.body(tt.Elem(), arg, false)
	return fmt.Sprintf(`
	if dec.IsNil() {
		%[1]s = nil
		if err := dec.FinishValue(); err != nil {
			return err
		}
	} else {
		%[1]s = new(%[2]s)
		dec.IgnoreNextStartValue()%[3]s
	}`, arg.Name, typeGo(g.goData, tt.Elem()), body)
}

func (g *genRead) bodyAny(tt *vdl.Type, arg namedArg) string {
	var anyType string
	switch goAnyRepMode(g.Package) {
	case goAnyRepInterface:
		// Handle interface{} special-case.
		return fmt.Sprintf(`
	var readAny interface{}
	if err := %[1]sRead(dec, &readAny); err != nil {
		return err
	}
	%[2]s = readAny`, g.Pkg("v.io/v23/vdl"), arg.Ref())
	case goAnyRepRawBytes:
		anyType = g.Pkg("v.io/v23/vom") + "RawBytes"
	case goAnyRepValue:
		anyType = g.Pkg("v.io/v23/vdl") + "Value"
	}
	// Handle vdl.Value and vom.RawBytes representations, which need to be
	// allocated before we call VDLRead on them.  Even if its already allocated,
	// we need to create a new (invalid) vdl.Value here, to cause VDLRead to fill
	// the value with the exact type read from the decoder.
	return fmt.Sprintf(`
	%[1]s = new(%[2]s)%[3]s`, arg.Ref(), anyType, g.bodyCallVDLRead(tt, arg))
}
