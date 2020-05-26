// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
)

var (
	keyType = StructType([]Field{{"I", Int64Type}, {"S", StringType}}...)

	strA, strB, strC = StringValue(nil, "A"), StringValue(nil, "B"), StringValue(nil, "C")
	int1, int2       = IntValue(Int64Type, 1), IntValue(Int64Type, 2)
	key1, key2, key3 = makeKey(1, "A"), makeKey(2, "B"), makeKey(3, "C")
)

func makeKey(a int64, b string) *Value {
	key := ZeroValue(keyType)
	key.StructField(0).AssignInt(a)
	key.StructField(1).AssignString(b)
	return key
}

func TestValueInvalid(t *testing.T) {
	tests := []*Value{nil, new(Value)}
	for ix, test := range tests {
		if test.IsValid() {
			t.Errorf(`%d IsValid()`, ix)
		}
		if got, want := test.String(), "INVALID"; got != want {
			t.Errorf(`%d String() got %v, want %v`, ix, got, want)
		}
	}
}

func TestValue(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		k Kind
		t *Type
		s string
	}{
		{Bool, BoolType, "false"},
		{Byte, ByteType, "byte(0)"},
		{Uint16, Uint16Type, "uint16(0)"},
		{Uint32, Uint32Type, "uint32(0)"},
		{Uint64, Uint64Type, "uint64(0)"},
		{Int8, Int8Type, "int8(0)"},
		{Int16, Int16Type, "int16(0)"},
		{Int32, Int32Type, "int32(0)"},
		{Int64, Int64Type, "int64(0)"},
		{Float32, Float32Type, "float32(0)"},
		{Float64, Float64Type, "float64(0)"},
		{String, StringType, `""`},
		{List, ListType(ByteType), `[]byte("")`},
		{Array, ArrayType(3, ByteType), `[3]byte("\x00\x00\x00")`},
		{Enum, EnumType("A", "B", "C"), "enum{A;B;C}(A)"},
		{TypeObject, TypeObjectType, "typeobject(any)"},
		{Array, ArrayType(2, StringType), `[2]string{"", ""}`},
		{List, ListType(StringType), "[]string{}"},
		{Set, SetType(StringType), "set[string]{}"},
		{Set, SetType(keyType), "set[struct{I int64;S string}]{}"},
		{Map, MapType(StringType, Int64Type), "map[string]int64{}"},
		{Map, MapType(keyType, Int64Type), "map[struct{I int64;S string}]int64{}"},
		{Struct, StructType([]Field{{"A", Int64Type}, {"B", StringType}, {"C", BoolType}}...), `struct{A int64;B string;C bool}{A: 0, B: "", C: false}`},
		{Union, UnionType([]Field{{"A", Int64Type}, {"B", StringType}, {"C", BoolType}}...), `union{A int64;B string;C bool}{A: 0}`},
		{Any, AnyType, "any(nil)"},
	}
	for _, test := range tests {
		x := ZeroValue(test.t)
		if !x.IsValid() {
			t.Errorf(`!ZeroValue(%s).IsValid`, test.k)
		}
		if !x.IsZero() {
			t.Errorf(`ZeroValue(%s) isn't zero`, test.k)
		}
		if test.k == Any && !x.IsNil() {
			t.Errorf(`ZeroValue(Any) isn't nil`)
		}
		if got, want := x.Kind(), test.k; got != want {
			t.Errorf(`ZeroValue(%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := x.Type(), test.t; got != want {
			t.Errorf(`ZeroValue(%s) got type %v, want %v`, test.k, got, want)
		}
		if got, want := x.String(), test.s; got != want {
			t.Errorf(`ZeroValue(%s) got string %q, want %q`, test.k, got, want)
		}
		y := CopyValue(x)
		if !y.IsValid() {
			t.Errorf(`!CopyValue(ZeroValue(%s)).IsValid`, test.k)
		}
		if !y.IsZero() {
			t.Errorf(`ZeroValue(%s) of copy isn't zero, y: %v`, test.k, y)
		}
		if test.k == Any && !y.IsNil() {
			t.Errorf(`ZeroValue(Any) of copy isn't nil`)
		}
		if !EqualValue(x, y) {
			t.Errorf(`ZeroValue(%s) !Equal after copy, x: %v, y: %v`, test.k, x, y)
		}

		// Invariant here: x == y == 0
		// The assign[KIND] functions assign a nonzero value.
		assignBool(t, x)
		assignUint(t, x)
		assignInt(t, x)
		assignFloat(t, x)
		assignString(t, x)
		assignEnum(t, x)
		assignTypeObject(t, x)
		assignArray(t, x)
		assignList(t, x)
		assignSet(t, x)
		assignMap(t, x)
		assignStruct(t, x)
		assignUnion(t, x)
		assignAny(t, x)

		// Invariant here: x != 0 && y == 0
		if EqualValue(x, y) {
			t.Errorf(`ZeroValue(%s) Equal after assign, x: %v, y: %v`, test.k, x, y)
		}
		z := CopyValue(x) // x == z && x != 0 && y == 0
		if !EqualValue(x, z) {
			t.Errorf(`ZeroValue(%s) !Equal after copy, x: %v, z: %v`, test.k, x, z)
		}
		if EqualValue(y, z) {
			t.Errorf(`ZeroValue(%s) Equal after copy, y: %v, z: %v`, test.k, y, z)
		}

		z.Assign(nil) // x != 0 && y == z == 0
		if !z.IsValid() {
			t.Errorf(`%s z after Assign(nil) isn't valid`, test.k)
		}
		if !z.IsZero() {
			t.Errorf(`%s z after Assign(nil) isn't zero`, test.k)
		}
		if test.k == Any && !z.IsNil() {
			t.Errorf(`Any z after Assign(nil) isn't nil`)
		}
		if EqualValue(x, z) {
			t.Errorf(`%s Equal after Assign(nil), x: %v, z: %v`, test.k, x, z)
		}
		if !EqualValue(y, z) {
			t.Errorf(`%s !Equal after Assign(nil), y: %v, z: %v`, test.k, y, z)
		}

		y.Assign(x) // x == y && x != 0 && z == 0
		if !y.IsValid() {
			t.Errorf(`%s y after Assign(x) isn't valid`, test.k)
		}
		if y.IsZero() {
			t.Errorf(`%s y after Assign(x) is zero, y: %v`, test.k, y)
		}
		if y.IsNil() {
			t.Errorf(`%s y after Assign(x) is nil, y: %v`, test.k, y)
		}
		if !EqualValue(x, y) {
			t.Errorf(`%s Equal after Assign(x), x: %v, y: %v`, test.k, x, y)
		}
		if EqualValue(y, z) {
			t.Errorf(`%s Equal after Assign(x), y: %v, z: %v`, test.k, y, z)
		}

		// Test optional types
		if !test.t.CanBeOptional() {
			continue
		}
		ntype := OptionalType(test.t)

		// nx == nil
		nx := ZeroValue(ntype)
		if !nx.IsValid() {
			t.Errorf(`!ZeroValue(?%s).IsValid`, test.k)
		}
		if !nx.IsZero() {
			t.Errorf(`ZeroValue(?%s) isn't zero`, test.k)
		}
		if !nx.IsNil() {
			t.Errorf(`ZeroValue(?%s) isn't nil`, test.k)
		}
		if got, want := nx.Kind(), Optional; got != want {
			t.Errorf(`ZeroValue(?%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := nx.Type(), ntype; got != want {
			t.Errorf(`ZeroValue(?%s) got type %v, want %v`, test.k, got, want)
		}
		if got, want := nx.String(), ntype.String()+"(nil)"; got != want {
			t.Errorf(`ZeroValue(?%s) got string %q, want %q`, test.k, got, want)
		}
		// ny != nil
		ny := NonNilZeroValue(ntype)
		if !ny.IsValid() {
			t.Errorf(`!NonNilZeroValue(?%s).IsValid`, test.k)
		}
		if ny.IsZero() {
			t.Errorf(`NonNilZeroValue(?%s) is zero`, test.k)
		}
		if ny.IsNil() {
			t.Errorf(`NonNilZeroValue(?%s) is nil`, test.k)
		}
		if got, want := ny.Kind(), Optional; got != want {
			t.Errorf(`NonNilZeroValue(?%s) got kind %v, want %v`, test.k, got, want)
		}
		if got, want := ny.Type(), ntype; got != want {
			t.Errorf(`NonNilZeroValue(?%s) got type %v, want %v`, test.k, got, want)
		}
	}
}

// Each of the below assign{KIND} functions assigns a nonzero value to x for
// matching kinds, otherwise it expects a mismatched-kind panic when trying the
// kind-specific methods.  The point is to ensure we've tried all combinations
// of kinds and methods.

func assignBool(t *testing.T, x *Value) {
	newval, newstr := true, "true"
	if x.Kind() == Bool {
		if got, want := x.Bool(), false; got != want {
			t.Errorf(`Bool zero value got %v, want %v`, got, want)
		}
		x.AssignBool(newval)
		if got, want := x.Bool(), newval; got != want {
			t.Errorf(`Bool assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`Bool string got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.Bool() })
		ExpectMismatchedKind(t, func() { x.AssignBool(newval) })
	}
}

func assignUint(t *testing.T, x *Value) {
	newval := uint64(123)
	switch x.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		if got, want := x.Uint(), uint64(0); got != want {
			t.Errorf(`Uint zero value got %v, want %v`, got, want)
		}
		x.AssignUint(newval)
		if got, want := x.Uint(), newval; got != want {
			t.Errorf(`Uint assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(123)"; got != want {
			t.Errorf(`Uint string got %v, want %v`, got, want)
		}
	default:
		ExpectMismatchedKind(t, func() { x.Uint() })
		ExpectMismatchedKind(t, func() { x.AssignUint(newval) })
	}
}

func assignInt(t *testing.T, x *Value) {
	newval := int64(123)
	switch x.Kind() {
	case Int8, Int16, Int32, Int64:
		if got, want := x.Int(), int64(0); got != want {
			t.Errorf(`Int zero value got %v, want %v`, got, want)
		}
		x.AssignInt(newval)
		if got, want := x.Int(), newval; got != want {
			t.Errorf(`Int assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(123)"; got != want {
			t.Errorf(`Int string got %v, want %v`, got, want)
		}
	default:
		ExpectMismatchedKind(t, func() { x.Int() })
		ExpectMismatchedKind(t, func() { x.AssignInt(newval) })
	}
}

func assignFloat(t *testing.T, x *Value) {
	newval := float64(1.23)
	switch x.Kind() {
	case Float32, Float64:
		if got, want := x.Float(), float64(0); got != want {
			t.Errorf(`Float zero value got %v, want %v`, got, want)
		}
		x.AssignFloat(newval)
		if got, want := x.Float(), newval; got != want {
			t.Errorf(`Float assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), x.Kind().String()+"(1.23)"; got != want {
			t.Errorf(`Float string got %v, want %v`, got, want)
		}
	default:
		ExpectMismatchedKind(t, func() { x.Float() })
		ExpectMismatchedKind(t, func() { x.AssignFloat(newval) })
	}
}

func assignString(t *testing.T, x *Value) {
	zerostr, newval, newstr := `""`, "abc", `"abc"`
	if x.Kind() == String {
		if got, want := x.String(), zerostr; got != want {
			t.Errorf(`String zero value got %v, want %v`, got, want)
		}
		x.AssignString(newval)
		if got, want := x.RawString(), newval; got != want {
			t.Errorf(`String assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`String assign string rep got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.RawString() })
		ExpectMismatchedKind(t, func() { x.AssignString(newval) })
	}
}

func assignEnum(t *testing.T, x *Value) {
	if x.Kind() == Enum {
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 0, "A"; gi != wi || gl != wl {
			t.Errorf(`Enum zero value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		x.AssignEnumIndex(1)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 1, "B"; gi != wi || gl != wl {
			t.Errorf(`Enum assign index value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "enum{A;B;C}(B)"; got != want {
			t.Errorf(`Enum string got %v, want %v`, got, want)
		}
		x.AssignEnumIndex(2)
		if gi, gl, wi, wl := x.EnumIndex(), x.EnumLabel(), 2, "C"; gi != wi || gl != wl {
			t.Errorf(`Enum assign label value got [%d]%v, want [%d]%v`, gi, gl, wi, wl)
		}
		if got, want := x.String(), "enum{A;B;C}(C)"; got != want {
			t.Errorf(`Enum string got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.EnumIndex() })
		ExpectMismatchedKind(t, func() { x.EnumLabel() })
		ExpectMismatchedKind(t, func() { x.AssignEnumIndex(0) })
		ExpectMismatchedKind(t, func() { x.AssignEnumLabel("A") })
	}
}

func assignTypeObject(t *testing.T, x *Value) {
	newval, newstr := BoolType, "typeobject(bool)"
	if x.Kind() == TypeObject {
		if got, want := x.TypeObject(), zeroTypeObject; got != want {
			t.Errorf(`TypeObject zero value got %v, want %v`, got, want)
		}
		x.AssignTypeObject(newval)
		if got, want := x.TypeObject(), newval; got != want {
			t.Errorf(`TypeObject assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), newstr; got != want {
			t.Errorf(`TypeObject string got %v, want %v`, got, want)
		}
		x.AssignTypeObject(nil) // assigning nil sets the zero typeobject
		if got, want := x.TypeObject(), zeroTypeObject; got != want {
			t.Errorf(`TypeObject assign value got %v, want %v`, got, want)
		}
		x.AssignTypeObject(newval)
	} else {
		ExpectMismatchedKind(t, func() { x.TypeObject() })
		ExpectMismatchedKind(t, func() { x.AssignTypeObject(newval) })
	}
}

func assignArray(t *testing.T, x *Value) {
	if x.Kind() == Array {
		if x.Type().IsBytes() {
			assignBytes(t, x)
			return
		}
		if got, want := x.Len(), 2; got != want {
			t.Errorf(`Array zero len got %v, want %v`, got, want)
		}
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `""`, `""`; g0 != w0 || g1 != w1 {
			t.Errorf(`Array assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		x.Index(0).AssignString("A")
		x.Index(1).AssignString("B")
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `"A"`, `"B"`; g0 != w0 || g1 != w1 {
			t.Errorf(`Array assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[2]string{"A", "B"}`; got != want {
			t.Errorf(`Array string got %v, want %v`, got, want)
		}
	} else {
		if x.Kind() != List {
			// Index is allowed for Array and List
			ExpectMismatchedKind(t, func() { x.Index(0) })
		}
		if x.Kind() != List && x.Kind() != Set && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			ExpectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignList(t *testing.T, x *Value) { //nolint:gocyclo
	if x.Kind() == List {
		if x.Type().IsBytes() {
			assignBytes(t, x)
			return
		}
		if got, want := x.Len(), 0; got != want {
			t.Errorf(`List zero len got %v, want %v`, got, want)
		}
		x.AssignLen(2)
		if got, want := x.Len(), 2; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `""`, `""`; g0 != w0 || g1 != w1 {
			t.Errorf(`List assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[]string{"", ""}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.Index(0).AssignString("A")
		x.Index(1).AssignString("B")
		if g0, g1, w0, w1 := x.Index(0).String(), x.Index(1).String(), `"A"`, `"B"`; g0 != w0 || g1 != w1 {
			t.Errorf(`List assign values got %v %v, want %v %v`, g0, g1, w0, w1)
		}
		if got, want := x.String(), `[]string{"A", "B"}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.AssignLen(1)
		if got, want := x.Len(), 1; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, w0 := x.Index(0).String(), `"A"`; g0 != w0 {
			t.Errorf(`List assign values got %v, want %v`, g0, w0)
		}
		if got, want := x.String(), `[]string{"A"}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
		x.AssignLen(3)
		if got, want := x.Len(), 3; got != want {
			t.Errorf(`List assign len got %v, want %v`, got, want)
		}
		if g0, g1, g2, w0, w1, w2 := x.Index(0).String(), x.Index(1).String(), x.Index(2).String(), `"A"`, `""`, `""`; g0 != w0 || g1 != w1 || g2 != w2 {
			t.Errorf(`List assign values got %v %v %v, want %v %v %v`, g0, g1, g2, w0, w1, w2)
		}
		if got, want := x.String(), `[]string{"A", "", ""}`; got != want {
			t.Errorf(`List string got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.AssignLen(0) })
		if x.Kind() != Array {
			// Index is allowed for Array and List
			ExpectMismatchedKind(t, func() { x.Index(0) })
		}
		if x.Kind() != Array && x.Kind() != Set && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			ExpectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignBytes(t *testing.T, x *Value) { //nolint:gocyclo
	abval, abcval, abcdval := []byte("ab"), []byte("abc"), []byte("abcd")
	zeroval, typestr := []byte{}, "[]byte"
	if x.Kind() == Array {
		zeroval, typestr = []byte{0, 0, 0}, "[3]byte"
	}

	if got, want := x.Bytes(), zeroval; !bytes.Equal(got, want) {
		t.Errorf(`Bytes zero value got %v, want %v`, got, want)
	}
	// AssignBytes fills all bytes of the array if the array len is equal to the
	// bytes len, and automatically assigns the len for lists.
	x.AssignBytes(abcval)
	if got, want := x.Bytes(), abcval; !bytes.Equal(got, want) {
		t.Errorf(`Bytes AssignBytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("abc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	abcval[1] = 'Z' // doesn't affect x
	if got, want := x.Bytes(), []byte("abc"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("abc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	// AssignBytes panics for arrays if the array len is not equal to the bytes
	// len, and automatically assigns the len for lists.
	if x.Kind() == Array {
		ExpectPanic(t, func() { x.AssignBytes(abval) }, "AssignBytes", "[3]byte AssignBytes(%v)", abval)
		ExpectPanic(t, func() { x.AssignBytes(abcdval) }, "AssignBytes", "[3]byte AssignBytes(%v)", abcdval)
	} else {
		x.AssignBytes(abval)
		if got, want := x.Bytes(), abval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes AssignBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("ab")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignBytes(abcdval)
		if got, want := x.Bytes(), abcdval; !bytes.Equal(got, want) {
			t.Errorf(`Bytes AssignBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("abcd")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
		x.AssignLen(3)
		if got, want := x.Bytes(), []byte("abc"); !bytes.Equal(got, want) {
			t.Errorf(`Bytes AssignBytes got %v, want %v`, got, want)
		}
		if got, want := x.String(), typestr+`("abc")`; got != want {
			t.Errorf(`Bytes string got %v, want %v`, got, want)
		}
	}
	// Bytes gives the underlying byteslice, which may be mutated.
	x.Bytes()[1] = 'Z'
	if got, want := x.Bytes(), []byte("aZc"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("aZc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
	// Indexing also works, just like for any other list.
	index := x.Index(1)
	if got, want := index.Type(), ByteType; got != want {
		t.Errorf(`Bytes index type got %v, want %v`, got, want)
	}
	if got, want := index.String(), fmt.Sprintf("byte(%d)", 'Z'); got != want {
		t.Errorf(`Bytes index string got %v, want %v`, got, want)
	}
	if got, want := index.Uint(), uint64('Z'); got != want {
		t.Errorf(`Bytes index Byte got %v, want %v`, got, want)
	}
	if got, want := index, UintValue(ByteType, 'Z'); !EqualValue(got, want) {
		t.Errorf(`Bytes index value got %v, want %v`, got, want)
	}
	index.AssignUint(uint64('Y'))
	if got, want := index.String(), fmt.Sprintf("byte(%d)", 'Y'); got != want {
		t.Errorf(`Bytes index string got %v, want %v`, got, want)
	}
	if got, want := index.Uint(), uint64('Y'); got != want {
		t.Errorf(`Bytes index Byte got %v, want %v`, got, want)
	}
	if got, want := index, UintValue(ByteType, 'Y'); !EqualValue(got, want) {
		t.Errorf(`Bytes index value got %v, want %v`, got, want)
	}
	// Make sure the original bytes were mutated.
	if got, want := x.Bytes(), []byte("aYc"); !bytes.Equal(got, want) {
		t.Errorf(`Bytes got %v, want %v`, got, want)
	}
	if got, want := x.String(), typestr+`("aYc")`; got != want {
		t.Errorf(`Bytes string got %v, want %v`, got, want)
	}
}

func toStringSlice(a []*Value) []string {
	ret := make([]string, len(a))
	for ix := 0; ix < len(a); ix++ {
		ret[ix] = a[ix].String()
	}
	return ret
}

// matchKeys returns true iff a and b hold the same values, in any order.
func matchKeys(a, b []*Value) bool {
	ass := toStringSlice(a)
	bss := toStringSlice(a)
	sort.Strings(ass)
	sort.Strings(bss)
	return strings.Join(ass, "") == strings.Join(bss, "")
}

// matchMapString returns true iff a and b are equivalent string representations
// of a map, dealing with entries in different orders.
func matchMapString(a, b string) bool {
	atypeval := strings.SplitN(a, "{", 2)
	btypeval := strings.SplitN(b, "{", 2)
	if len(atypeval) != 2 || len(btypeval) != 2 || atypeval[0] != btypeval[0] {
		return false
	}

	a = atypeval[1]
	b = btypeval[1]

	n := len(a)
	if n != len(b) || n < 1 || a[n-1] != '}' || b[n-1] != '}' {
		return false
	}
	asplit := strings.Split(a[:n-1], ", ")
	bsplit := strings.Split(b[:n-1], ", ")
	sort.Strings(asplit)
	sort.Strings(bsplit)
	return strings.Join(asplit, "") == strings.Join(bsplit, "")
}

func assignSet(t *testing.T, x *Value) { //nolint:gocyclo
	if x.Kind() == Set {
		k1, k2, k3 := key1, key2, key3
		setstr1 := `set[struct{I int64;S string}]{{I: 1, S: "A"}, {I: 2, S: "B"}}`
		setstr2 := `set[struct{I int64;S string}]{{I: 2, S: "B"}}`
		if x.Type().Key() == StringType {
			k1, k2, k3 = strA, strB, strC
			setstr1, setstr2 = `set[string]{"A", "B"}`, `set[string]{"B"}`
		}

		if got, want := x.Keys(), []*Value{}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), false; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		x.AssignSetKey(k1)
		x.AssignSetKey(k2)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), true; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), setstr1; !matchMapString(got, want) {
			t.Errorf(`Set String got %v, want %v`, got, want)
		}
		x.DeleteSetKey(k1)
		if got, want := x.Keys(), []*Value{k2}; !matchKeys(got, want) {
			t.Errorf(`Set Keys got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Set ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Set ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Set ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), setstr2; !matchMapString(got, want) {
			t.Errorf(`Set String got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.AssignSetKey(nil) })
		ExpectMismatchedKind(t, func() { x.DeleteSetKey(nil) })
		if x.Kind() != Map {
			// Keys and ContainsKey are allowed for Set and Map
			ExpectMismatchedKind(t, func() { x.Keys() })
			ExpectMismatchedKind(t, func() { x.ContainsKey(nil) })
		}
		if x.Kind() != Array && x.Kind() != List && x.Kind() != Map {
			// Len is allowed for Array, List, Set and Map
			ExpectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignMap(t *testing.T, x *Value) { //nolint:gocyclo
	if x.Kind() == Map {
		k1, k2, k3 := key1, key2, key3
		v1, v2 := int1, int2
		mapstr1 := `map[struct{I int64;S string}]int64{{I: 1, S: "A"}: 1, {I: 2, S: "B"}: 2}`
		mapstr2 := `map[struct{I int64;S string}]int64{{I: 2, S: "B"}: 2}`
		if x.Type().Key() == StringType {
			k1, k2, k3 = strA, strB, strC
			mapstr1 = `map[string]int64{"A": 1, "B": 2}`
			mapstr2 = `map[string]int64{"B": 2}`
		}

		if got, want := x.Keys(), []*Value{}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map MapIndex k1 got %v, want nil`, got)
		}
		if got := x.MapIndex(k2); got != nil {
			t.Errorf(`Map MapIndex k2 got %v, want nil`, got)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), false; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		x.AssignMapIndex(k1, v1)
		x.AssignMapIndex(k2, v2)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k1), v1; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k1 got %v, want %v`, got, want)
		}
		if got, want := x.MapIndex(k2), v2; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k2 got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), true; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), mapstr1; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
		x.DeleteMapIndex(k1)
		if got, want := x.Keys(), []*Value{k1, k2}; !matchKeys(got, want) {
			t.Errorf(`Map Keys got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k1); got != nil {
			t.Errorf(`Map MapIndex k1 got %v, want nil`, got)
		}
		if got, want := x.MapIndex(k2), v2; !EqualValue(got, want) {
			t.Errorf(`Map MapIndex k2 got %v, want %v`, got, want)
		}
		if got := x.MapIndex(k3); got != nil {
			t.Errorf(`Map MapIndex k3 got %v, want nil`, got)
		}
		if got, want := x.ContainsKey(k1), false; got != want {
			t.Errorf(`Map ContainsKey k1 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k2), true; got != want {
			t.Errorf(`Map ContainsKey k2 got %v, want %v`, got, want)
		}
		if got, want := x.ContainsKey(k3), false; got != want {
			t.Errorf(`Map ContainsKey k3 got %v, want %v`, got, want)
		}
		if got, want := x.String(), mapstr2; !matchMapString(got, want) {
			t.Errorf(`Map string got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.MapIndex(nil) })
		ExpectMismatchedKind(t, func() { x.AssignMapIndex(nil, nil) })
		ExpectMismatchedKind(t, func() { x.DeleteMapIndex(nil) })
		if x.Kind() != Set {
			// Keys and ContainsKey are allowed for Set and Map
			ExpectMismatchedKind(t, func() { x.Keys() })
			ExpectMismatchedKind(t, func() { x.ContainsKey(nil) })
		}
		if x.Kind() != Array && x.Kind() != List && x.Kind() != Set {
			// Len is allowed for Array, List, Set and Map
			ExpectMismatchedKind(t, func() { x.Len() })
		}
	}
}

func assignStruct(t *testing.T, x *Value) {
	if x.Kind() == Struct {
		x.StructField(0).AssignInt(1)
		if x.IsZero() {
			t.Errorf(`Struct assign index 0 is zero`)
		}
		if x.StructField(0) != x.StructFieldByName("A") {
			t.Errorf(`struct{A int64;B string;C bool} does not have matching values at index 0 and field name A`)
		}
		if got, want := x.String(), `struct{A int64;B string;C bool}{A: 1, B: "", C: false}`; got != want {
			t.Errorf(`Struct assign index 0 got %v, want %v`, got, want)
		}
		x.StructField(1).AssignString("a")
		if x.IsZero() {
			t.Errorf(`Struct assign index 1 is zero`)
		}
		if x.StructField(1) != x.StructFieldByName("B") {
			t.Errorf(`struct{A int64;B string;C bool} does not have matching values at index 1 and field name B`)
		}
		if got, want := x.String(), `struct{A int64;B string;C bool}{A: 1, B: "a", C: false}`; got != want {
			t.Errorf(`Struct assign index 1 got %v, want %v`, got, want)
		}
		if value := x.StructFieldByName("NotAStructField"); value != nil {
			t.Errorf(`struct{A int64;B string;C bool} had value %v for field name NotAStructField`, value)
		}
		y := CopyValue(x)
		y.StructField(2).AssignBool(false)
		if !EqualValue(x, y) {
			t.Errorf(`Struct !equal %v and %v`, x, y)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.StructField(0) })
		ExpectMismatchedKind(t, func() { x.StructFieldByName("A") })
		if x.Kind() != Union {
			// AssignField is also allowed for Union
			ExpectMismatchedKind(t, func() { x.AssignField(0, nil) })
		}
	}
}

func assignUnion(t *testing.T, x *Value) {
	if x.Kind() == Union {
		goti, gotv := x.UnionField()
		if got, want := goti, 0; got != want {
			t.Errorf(`Union zero value got index %v, want %v`, got, want)
		}
		if got, want := gotv, IntValue(Int64Type, 0); !EqualValue(got, want) {
			t.Errorf(`Union zero value got value %v, want %v`, got, want)
		}
		x.AssignField(1, strA)
		goti, gotv = x.UnionField()
		if got, want := goti, 1; got != want {
			t.Errorf(`Union assign B value got index %v, want %v`, got, want)
		}
		if got, want := gotv, strA; !EqualValue(got, want) {
			t.Errorf(`Union assign B value got value %v, want %v`, got, want)
		}
		if x.IsZero() {
			t.Errorf(`Union assign B value is zero`)
		}
		if got, want := x.String(), `union{A int64;B string;C bool}{B: "A"}`; got != want {
			t.Errorf(`Union assign B value got %v, want %v`, got, want)
		}
	} else {
		ExpectMismatchedKind(t, func() { x.UnionField() })
		if x.Kind() != Struct {
			// AssignField is also allowed for Struct
			ExpectMismatchedKind(t, func() { x.AssignField(0, nil) })
		}
	}
}

func assignAny(t *testing.T, x *Value) {
	if x.Kind() == Any {
		if got := x.Elem(); got != nil {
			t.Errorf(`Any zero value got %v, want nil`, got)
		}
		x.Assign(int1)
		if got, want := x.Elem(), int1; !EqualValue(got, want) {
			t.Errorf(`Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), "any(int64(1))"; got != want {
			t.Errorf(`Any assign string got %v, want %v`, got, want)
		}
		x.Assign(strA)
		if got, want := x.Elem(), strA; !EqualValue(got, want) {
			t.Errorf(`Any assign value got %v, want %v`, got, want)
		}
		if got, want := x.String(), `any("A")`; got != want {
			t.Errorf(`Any assign string got %v, want %v`, got, want)
		}
	} else if x.Kind() != Optional {
		ExpectMismatchedKind(t, func() { x.Elem() })
	}
}
