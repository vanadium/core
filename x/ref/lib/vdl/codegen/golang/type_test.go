// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"bufio"
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
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

func TestTypeDef(t *testing.T) {
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
//nolint:deadcode,unused
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

func (TestEnum) VDLReflect(struct{
	Name string ` + "`" + `vdl:"TestEnum"` + "`" + `
	Enum struct{ A, B, C string }
}) {
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
		// VDLReflect describes the TestUnion union type.
		VDLReflect(vdlTestUnionReflect)
		VDLIsZero() bool
		VDLWrite(vdl.Encoder) error
	}
	// TestUnionA represents field A of the TestUnion union type.
	TestUnionA struct{ Value string }
	// TestUnionB represents field B of the TestUnion union type.
	TestUnionB struct{ Value int64 }
	// vdlTestUnionReflect describes the TestUnion union type.
	vdlTestUnionReflect struct {
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
func (x TestUnionA) VDLReflect(vdlTestUnionReflect) {}

func (x TestUnionB) Index() int { return 1 }
func (x TestUnionB) Interface() interface{} { return x.Value }
func (x TestUnionB) Name() string { return "B" }
func (x TestUnionB) VDLReflect(vdlTestUnionReflect) {}`},
		{tStruct, `type TestStruct struct {
	A string
	B int64
	C int64
}

func (TestStruct) VDLReflect(struct{
	Name string ` + "`" + `vdl:"TestStruct"` + "`" + `
}) {
}`},
	}
	data := &goData{
		Package:        &compile.Package{},
		Env:            compile.NewEnv(-1),
		createdTargets: make(map[*vdl.Type]bool),
	}
	for _, test := range tests {
		if got, want := defineTypeForTest(data, test.T), test.Want; got != want {
			diffLines(got, want)
			t.Errorf("%s\n GOT %s\nWANT %s", test.T, got, want)
		}
	}
}

func defineTypeForTest(data *goData, vdlType *vdl.Type) string {
	firstLetter, _ := utf8.DecodeRuneInString(vdlType.Name())
	def := &compile.TypeDef{
		NamePos:  compile.NamePos{Name: vdlType.Name()},
		Type:     vdlType,
		Exported: unicode.IsUpper(firstLetter),
	}
	switch vdlType.Kind() {
	case vdl.Enum:
		def.LabelDoc = make([]string, vdlType.NumEnumLabel())
		def.LabelDocSuffix = make([]string, vdlType.NumEnumLabel())
	case vdl.Struct, vdl.Union:
		def.FieldDoc = make([]string, vdlType.NumField())
		def.FieldDocSuffix = make([]string, vdlType.NumField())
	}
	return defineType(data, def)
}

func diffLines(a, b string) {
	fmt.Printf("len(a) %d len(b) %d\n", len(a), len(b))
	sa := bufio.NewScanner(strings.NewReader(a))
	sb := bufio.NewScanner(strings.NewReader(b))

	index := 0
	for {
		index++
		var readSomething bool
		var aline string
		var bline string
		if sa.Scan() {
			aline = sa.Text()
			readSomething = true
		}
		if sb.Scan() {
			bline = sb.Text()
			readSomething = true
		}
		if !readSomething {
			break
		}
		if aline != bline {
			fmt.Printf("%4d:\ngot:\n%s\nwant:\n%s\n", index, aline, bline)
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
		vdl.Field{
			Name: "C",
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

func TestStructTags(t *testing.T) {
	testingMode = true

	tagA := `json:"a,omitempty" other:"a,,'a quoted'"`
	tagB := `json:"b" other:"b,,'b quoted'"`

	withTags := &goData{
		Env: compile.NewEnv(-1),
		Package: &compile.Package{
			Config: vdltool.Config{
				Go: vdltool.GoConfig{
					StructTags: map[string][]vdltool.GoStructTag{
						"TestStruct": {
							{Field: "A", Tag: tagA},
							{Field: "B", Tag: tagB},
						},
					},
				},
			},
		},
	}
	noTags := &goData{Env: compile.NewEnv(-1)}

	typeDefWithTags := defineTypeForTest(withTags, tStruct)
	typeDefNoTags := defineTypeForTest(noTags, tStruct)
	for _, tag := range []string{
		"A string `" + tagA + "`\n",
		"B int64 `" + tagB + "`\n",
	} {
		if !strings.Contains(typeDefWithTags, tag) {
			t.Errorf("missing tag: %v", tag)
		}
		if strings.Contains(typeDefNoTags, tag) {
			t.Errorf("contains tag: %v", tag)
		}
	}
	for _, def := range []string{typeDefWithTags, typeDefNoTags} {
		if !strings.Contains(def, "C int64\n") {
			t.Errorf("missing field")
		}
	}
}
