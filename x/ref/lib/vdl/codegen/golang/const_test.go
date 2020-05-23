// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

func TestConst(t *testing.T) {
	testingMode = true
	tests := []struct {
		Name string
		V    *vdl.Value
		Want string
	}{
		{"True", vdl.BoolValue(nil, true), `true`},
		{"False", vdl.BoolValue(nil, false), `false`},
		{"String", vdl.StringValue(nil, "abc"), `"abc"`},
		{"Bytes", vdl.BytesValue(nil, []byte("abc")), `[]byte("abc")`},
		{"EmptyBytes", vdl.BytesValue(nil, nil), `[]byte(nil)`},
		{"Byte", vdl.UintValue(vdl.ByteType, 111), `byte(111)`},
		{"Uint16", vdl.UintValue(vdl.Uint16Type, 222), `uint16(222)`},
		{"Uint32", vdl.UintValue(vdl.Uint32Type, 333), `uint32(333)`},
		{"Uint64", vdl.UintValue(vdl.Uint64Type, 444), `uint64(444)`},
		{"Int8", vdl.IntValue(vdl.Int8Type, -111), `int8(-111)`},
		{"Int16", vdl.IntValue(vdl.Int16Type, -555), `int16(-555)`},
		{"Int32", vdl.IntValue(vdl.Int32Type, -666), `int32(-666)`},
		{"Int64", vdl.IntValue(vdl.Int64Type, -777), `int64(-777)`},
		{"Float32", vdl.FloatValue(vdl.Float32Type, 1.5), `float32(1.5)`},
		{"Float64", vdl.FloatValue(vdl.Float64Type, 2.5), `float64(2.5)`},
		{"Enum", vdl.EnumValue(tEnum, 1), `TestEnumB`},
		{"EmptyArray", vEmptyArray, "[3]string{}"},
		{"EmptyList", vEmptyList, "[]string(nil)"},
		{"EmptySet", vEmptySet, "map[string]struct{}(nil)"},
		{"EmptyMap", vEmptyMap, "map[string]int64(nil)"},
		{"EmptyStruct", vEmptyStruct, "TestStruct{}"},
		{"NilOptionalStruct", vNilOptional, "(*TestStruct)(nil)"},
		{"NonNilOptionalStruct", vNonNilOptional, "&TestStruct{}"},
		{"Array", vArray, `[3]string{
"A",
"B",
"C",
}`},
		{"List", vList, `[]string{
"A",
"B",
"C",
}`},
		{"List of ByteList", vListOfByteList, `[][]byte{
[]byte("abc"),
nil,
}`},
		{"Set", vSet, `map[string]struct{}{
"A": {},
}`},
		{"Map", vMap, `map[string]int64{
"A": 1,
}`},
		{"Struct", vStruct, `TestStruct{
A: "foo",
B: 123,
}`},
		{"UnionABC", vUnionABC, `TestUnion(TestUnionA{Value: "abc"})`},
		{"Union123", vUnion123, `TestUnion(TestUnionB{Value: 123})`},
		{"AnyABC", vAnyABC, `vom.RawBytesOf("abc")`},
		{"Any123", vAny123, `vom.RawBytesOf(int64(123))`},
		// TODO(toddw): Add tests for optional types.
	}
	data := &goData{Env: compile.NewEnv(-1)}
	for _, test := range tests {
		data.Package = &compile.Package{}
		if got, want := typedConst(data, test.V), test.Want; got != want {
			t.Errorf("%s\n GOT %s\nWANT %s", test.Name, got, want)
		}
	}
}

var (
	vEmptyArray  = vdl.ZeroValue(tArray)
	vEmptyList   = vdl.ZeroValue(tList)
	vEmptySet    = vdl.ZeroValue(tSet)
	vEmptyMap    = vdl.ZeroValue(tMap)
	vEmptyStruct = vdl.ZeroValue(tStruct)

	vArray          = vdl.ZeroValue(tArray)
	vList           = vdl.ZeroValue(tList)
	vListOfByteList = vdl.ZeroValue(tListOfByteList)
	vSet            = vdl.ZeroValue(tSet)
	vMap            = vdl.ZeroValue(tMap)
	vStruct         = vdl.ZeroValue(tStruct)
	vNilOptional    = vdl.ZeroValue(vdl.OptionalType(tStruct))
	vNonNilOptional = vdl.NonNilZeroValue(vdl.OptionalType(tStruct))
	vUnionABC       = vdl.ZeroValue(tUnion)
	vUnion123       = vdl.ZeroValue(tUnion)
	vAnyABC         = vdl.ZeroValue(vdl.AnyType)
	vAny123         = vdl.ZeroValue(vdl.AnyType)
)

func init() {
	vArray.Index(0).AssignString("A")
	vArray.Index(1).AssignString("B")
	vArray.Index(2).AssignString("C")
	vList.AssignLen(3)
	vList.Index(0).AssignString("A")
	vList.Index(1).AssignString("B")
	vList.Index(2).AssignString("C")
	vListOfByteList.AssignLen(2)
	vListOfByteList.Index(0).Assign(vdl.BytesValue(nil, []byte("abc")))
	vListOfByteList.Index(1).Assign(vdl.BytesValue(nil, nil))
	// TODO(toddw): Assign more items once the ordering is fixed.
	vSet.AssignSetKey(vdl.StringValue(nil, "A"))
	vMap.AssignMapIndex(vdl.StringValue(nil, "A"), vdl.IntValue(vdl.Int64Type, 1))

	vStruct.StructField(0).AssignString("foo")
	vStruct.StructField(1).AssignInt(123)

	vUnionABC.AssignField(0, vdl.StringValue(nil, "abc"))
	vUnion123.AssignField(1, vdl.IntValue(vdl.Int64Type, 123))

	vAnyABC.Assign(vdl.StringValue(nil, "abc"))
	vAny123.Assign(vdl.IntValue(vdl.Int64Type, 123))
}
