// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

func TestType(t *testing.T) {
	testingMode = true
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{vdl.AnyType, `*vom.RawBytes`},
		{vdl.TypeObjectType, `*vdl.Type`},
		{vdl.BoolType, `bool`},
		{vdl.StringType, `string`},
		{vdl.ListType(vdl.ByteType), `[]byte`},
		{vdl.ByteType, `byte`},
		{vdl.Uint16Type, `uint16`},
		{vdl.Uint32Type, `uint32`},
		{vdl.Uint64Type, `uint64`},
		{vdl.Int16Type, `int16`},
		{vdl.Int32Type, `int32`},
		{vdl.Int64Type, `int64`},
		{vdl.Float32Type, `float32`},
		{vdl.Float64Type, `float64`},
		{tArray, `[3]string`},
		{tList, `[]string`},
		{tSet, `map[string]struct{}`},
		{tMap, `map[string]int64`},
	}
	data := &goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		data.Package = &compile.Package{}
		if got, want := typeGo(data, test.T), test.Want; got != want {
			t.Errorf("%s\nGOT %s\nWANT %s", test.T, got, want)
		}
	}
}

// TODO(bprosnitz) Disabled because frustrating while making changes. Either re-enable and fix output or re-write this test.
func disabledTestTypeDef(t *testing.T) {
	testingMode = true
	tests := []struct {
		T    *vdl.Type
		Want string
	}{
		{tEnum, `type TestEnum int
const (
	TestEnumA TestEnum = iota
	TestEnumB
	TestEnumC
)

// TestEnumAll holds all labels for TestEnum.
var TestEnumAll = [...]TestEnum{TestEnumA, TestEnumB, TestEnumC}

// TestEnumFromString creates a TestEnum from a string label.
func TestEnumFromString(label string) (x TestEnum, err error) {
	err = x.Set(label)
	return
}

// Set assigns label to x.
func (x *TestEnum) Set(label string) error {
	switch label {
	case "A", "a":
		*x = TestEnumA
		return nil
	case "B", "b":
		*x = TestEnumB
		return nil
	case "C", "c":
		*x = TestEnumC
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in TestEnum", label)
}

// String returns the string label of x.
func (x TestEnum) String() string {
	switch x {
	case TestEnumA:
		return "A"
	case TestEnumB:
		return "B"
	case TestEnumC:
		return "C"
	}
	return ""
}

func (TestEnum) __VDLReflect(struct{
	Name string ` + "`" + `vdl:"TestEnum"` + "`" + `
	Enum struct{ A, B, C string }
}) {
}

func (m TestEnum) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
	if err := t.FromEnumLabel(m.String(), __VDLType__TestEnum); err != nil {
	return err
	}
	return nil
}

func (m TestEnum) MakeVDLTarget() vdl.Target {
	return nil
}`},
		{tUnion, `type (
	// TestUnion represents any single field of the TestUnion union type.
	TestUnion interface {
		// Index returns the field index.
		Index() int
		// Interface returns the field value as an interface.
		Interface() interface{}
		// Name returns the field name.
		Name() string
		// __VDLReflect describes the TestUnion union type.
		__VDLReflect(__TestUnionReflect)
		FillVDLTarget(vdl.Target, *vdl.Type) error
	}
	// TestUnionA represents field A of the TestUnion union type.
	TestUnionA struct{ Value string }
	// TestUnionB represents field B of the TestUnion union type.
	TestUnionB struct{ Value int64 }
	// __TestUnionReflect describes the TestUnion union type.
	__TestUnionReflect struct {
		Name string ` + "`" + `vdl:"TestUnion"` + "`" + `
		Type TestUnion
		Union struct {
			A TestUnionA
			B TestUnionB
		}
	}
)

func (x TestUnionA) Index() int { return 0 }
func (x TestUnionA) Interface() interface{} { return x.Value }
func (x TestUnionA) Name() string { return "A" }
func (x TestUnionA) __VDLReflect(__TestUnionReflect) {}

func (m TestUnionA) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
` + "\t" + `
	fieldsTarget1, err := t.StartFields(__VDLType__TestUnion)
	if err != nil {
		return err
	}
	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("A")
	if err != nil {
		return err
	}
	if err := fieldTarget3.FromString(string(m.Value), vdl.StringType); err != nil {
	return err
}
	if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
		return err
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
` + "\t" + `
	return nil
}

func (m TestUnionA) MakeVDLTarget() vdl.Target {
	return nil
}

func (x TestUnionB) Index() int { return 1 }
func (x TestUnionB) Interface() interface{} { return x.Value }
func (x TestUnionB) Name() string { return "B" }
func (x TestUnionB) __VDLReflect(__TestUnionReflect) {}

func (m TestUnionB) FillVDLTarget(t vdl.Target, tt *vdl.Type) error {
` + "\t" + `
	fieldsTarget1, err := t.StartFields(__VDLType__TestUnion)
	if err != nil {
		return err
	}
	keyTarget2, fieldTarget3, err := fieldsTarget1.StartField("B")
	if err != nil {
		return err
	}
	if err := fieldTarget3.FromInt(int64(m.Value), vdl.Int64Type); err != nil {
	return err
}
	if err := fieldsTarget1.FinishField(keyTarget2, fieldTarget3); err != nil {
		return err
	}
	if err := t.FinishFields(fieldsTarget1); err != nil {
		return err
	}
` + "\t" + `
	return nil
}

func (m TestUnionB) MakeVDLTarget() vdl.Target {
	return nil
}`},
	}
	data := &goData{
		Package:        &compile.Package{},
		Env:            compile.NewEnv(-1),
		createdTargets: make(map[*vdl.Type]bool),
	}
	for _, test := range tests {
		firstLetter, _ := utf8.DecodeRuneInString(test.T.Name())
		def := &compile.TypeDef{
			NamePos:  compile.NamePos{Name: test.T.Name()},
			Type:     test.T,
			Exported: unicode.IsUpper(firstLetter),
		}
		switch test.T.Kind() {
		case vdl.Enum:
			def.LabelDoc = make([]string, test.T.NumEnumLabel())
			def.LabelDocSuffix = make([]string, test.T.NumEnumLabel())
		case vdl.Struct, vdl.Union:
			def.FieldDoc = make([]string, test.T.NumField())
			def.FieldDocSuffix = make([]string, test.T.NumField())
		}
		if got, want := defineType(data, def), test.Want; got != want {
			// TIP: Try https://gist.github.com/bprosnitz/60d93bdcc53d8148b8f9 for debugging
			// invisible differences.
			t.Errorf("%s\n GOT %s\nWANT %s", test.T, got, want)
		}
	}
}

var (
	tEnum           = vdl.NamedType("TestEnum", vdl.EnumType("A", "B", "C"))
	tArray          = vdl.ArrayType(3, vdl.StringType)
	tList           = vdl.ListType(vdl.StringType)
	tListOfByteList = vdl.ListType(vdl.ListType(vdl.ByteType))
	tSet            = vdl.SetType(vdl.StringType)
	tMap            = vdl.MapType(vdl.StringType, vdl.Int64Type)
	tStruct         = vdl.NamedType("TestStruct", vdl.StructType(
		vdl.Field{
			Name: "A",
			Type: vdl.StringType,
		},
		vdl.Field{
			Name: "B",
			Type: vdl.Int64Type,
		},
	))
	tUnion = vdl.NamedType("TestUnion", vdl.UnionType(
		vdl.Field{
			Name: "A",
			Type: vdl.StringType,
		},
		vdl.Field{
			Name: "B",
			Type: vdl.Int64Type,
		},
	))
)
