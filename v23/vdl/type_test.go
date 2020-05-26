// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"strings"
	"testing"
)

func TestSplitIdent(t *testing.T) {
	tests := []struct {
		ident, pkgpath, name string
	}{
		{".", "", ""},
		{"a", "", "a"},
		{"Foo", "", "Foo"},
		{"a.Foo", "a", "Foo"},
		{"a/Foo", "", "a/Foo"},
		{"a/b.Foo", "a/b", "Foo"},
		{"a.b.Foo", "a.b", "Foo"},
		{"a/b/c.Foo", "a/b/c", "Foo"},
		{"a/b.c.Foo", "a/b.c", "Foo"},
	}
	for _, test := range tests {
		pkgpath, name := SplitIdent(test.ident)
		if got, want := pkgpath, test.pkgpath; got != want {
			t.Errorf("%s got pkgpath %q, want %q", test.ident, got, want)
		}
		if got, want := name, test.name; got != want {
			t.Errorf("%s got name %q, want %q", test.ident, got, want)
		}
	}
}

var singletons = []struct {
	k Kind
	t *Type
	s string
}{
	{Any, AnyType, "any"},
	{Bool, BoolType, "bool"},
	{Byte, ByteType, "byte"},
	{Uint16, Uint16Type, "uint16"},
	{Uint32, Uint32Type, "uint32"},
	{Uint64, Uint64Type, "uint64"},
	{Int8, Int8Type, "int8"},
	{Int16, Int16Type, "int16"},
	{Int32, Int32Type, "int32"},
	{Int64, Int64Type, "int64"},
	{Float32, Float32Type, "float32"},
	{Float64, Float64Type, "float64"},
	{String, StringType, "string"},
	{TypeObject, TypeObjectType, "typeobject"},
}

type l []string

var enums = []struct {
	name   string
	labels l
	str    string
	errstr string
}{
	{"FailNoLabels", l{}, "", "no enum labels"},
	{"FailEmptyLabel", l{""}, "", "empty enum label"},
	{"", l{"A"}, "enum{A}", ""},
	{"A", l{"A"}, "A enum{A}", ""},
	{"AB", l{"A", "B"}, "AB enum{A;B}", ""},
}

type f []Field

var structs = []struct {
	name   string
	fields f
	str    string
	errstr string
}{
	{"FailFieldName", f{{"", BoolType}}, "", "empty field name"},
	{"FailDupFields", f{{"A", BoolType}, {"A", Int32Type}}, "", "duplicate field name"},
	{"FailNilFieldType", f{{"A", nil}}, "", "nil field type"},
	{"", f{}, "struct{}", ""},
	{"Empty", f{}, "Empty struct{}", ""},
	{"A", f{{"A", BoolType}}, "A struct{A bool}", ""},
	{"AB", f{{"A", BoolType}, {"B", Int32Type}}, "AB struct{A bool;B int32}", ""},
	{"ABC", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}}, "ABC struct{A bool;B int32;C uint64}", ""},
	{"ABCD", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}, {"D", StringType}}, "ABCD struct{A bool;B int32;C uint64;D string}", ""},
}

var unions = []struct {
	name   string
	fields f
	str    string
	errstr string
}{
	{"FailNoFields", f{}, "", "no union fields"},
	{"FailFieldName", f{{"", BoolType}}, "", "empty field name"},
	{"FailDupFields", f{{"A", BoolType}, {"A", Int32Type}}, "", "duplicate field name"},
	{"FailNilFieldType", f{{"A", nil}}, "", "nil field type"},
	{"A", f{{"A", BoolType}}, "A union{A bool}", ""},
	{"AB", f{{"A", BoolType}, {"B", Int32Type}}, "AB union{A bool;B int32}", ""},
	{"ABC", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}}, "ABC union{A bool;B int32;C uint64}", ""},
	{"ABCD", f{{"A", BoolType}, {"B", Int32Type}, {"C", Uint64Type}, {"D", StringType}}, "ABCD union{A bool;B int32;C uint64;D string}", ""},
}

func allTypes() (types []*Type) {
	for index, test := range singletons {
		types = append(types, test.t)
		types = append(types, ArrayType(index+1, test.t))
		types = append(types, ListType(test.t))
		if test.t.CanBeKey() {
			types = append(types, SetType(test.t))
			for _, test2 := range singletons {
				types = append(types, MapType(test.t, test2.t))
			}
		}
	}
	for _, test := range enums {
		if test.errstr == "" {
			types = append(types, EnumType(test.labels...))
		}
	}
	for _, test := range structs {
		if test.errstr == "" {
			types = append(types, StructType(test.fields...))
		}
	}
	for _, test := range unions {
		if test.errstr == "" {
			types = append(types, UnionType(test.fields...))
		}
	}
	for ix, t := range types {
		if t.CanBeNamed() {
			types = append(types, NamedType(fmt.Sprintf("Named%d", ix), t))
		}
	}
	for _, t := range types {
		if t.CanBeOptional() {
			types = append(types, OptionalType(t))
		}
	}
	return
}

func TestTypeMismatch(t *testing.T) {
	// Make sure we panic if a method is called for a mismatched kind.
	for _, ty := range allTypes() {
		ty := ty
		k := ty.Kind()
		if k != Enum {
			ExpectMismatchedKind(t, func() { ty.EnumLabel(0) })
			ExpectMismatchedKind(t, func() { ty.EnumIndex("") })
			ExpectMismatchedKind(t, func() { ty.NumEnumLabel() })
		}
		if k != Array {
			ExpectMismatchedKind(t, func() { ty.Len() })
		}
		if k != Optional && k != Array && k != List && k != Map {
			ExpectMismatchedKind(t, func() { ty.Elem() })
		}
		if k != Set && k != Map {
			ExpectMismatchedKind(t, func() { ty.Key() })
		}
		if k != Struct && k != Union {
			ExpectMismatchedKind(t, func() { ty.Field(0) })
			ExpectMismatchedKind(t, func() { ty.FieldByName("") })
			ExpectMismatchedKind(t, func() { ty.NumField() })
		}
	}
}

func testSingleton(t *testing.T, k Kind, ty *Type, s string) {
	if got, want := ty.Kind(), k; got != want {
		t.Errorf(`%s got kind %q, want %q`, k, got, want)
	}
	if got, want := ty.Name(), ""; got != want {
		t.Errorf(`%s got name %q, want %q`, k, got, want)
	}
	if got, want := k.String(), s; got != want {
		t.Errorf(`%s got kind %q, want %q`, k, got, want)
	}
	if got, want := ty.String(), s; got != want {
		t.Errorf(`%s got string %q, want %q`, k, got, want)
	}
	if got, want := ty.Unique(), s; got != want {
		t.Errorf(`%s got unique %q, want %q`, k, got, want)
	}
	if !ty.ContainsKind(WalkAll, k) {
		t.Errorf(`%s !ContainsKind(WalkAll, %v)`, k, k)
	}
	if !ty.ContainsType(WalkAll, ty) {
		t.Errorf(`%s !ContainsType(WalkAll, %v)`, k, ty)
	}
}

func TestSingletonTypes(t *testing.T) {
	for _, test := range singletons {
		testSingleton(t, test.k, test.t, test.s)
	}
}

func TestOptionalTypes(t *testing.T) {
	for _, test := range allTypes() {
		if !test.CanBeOptional() {
			continue
		}
		opt := OptionalType(test)
		if got, want := opt.Kind(), Optional; got != want {
			t.Errorf(`%s got kind %q, want %q`, opt, got, want)
		}
		if got, want := opt.Name(), ""; got != want {
			t.Errorf(`%s got name %q, want %q`, opt, got, want)
		}
		if !test.ContainsKind(WalkAll, Optional, test.Kind()) {
			t.Errorf(`%s !ContainsKind(WalkAll, Optional, %v)`, opt, test.Kind())
		}
		if !test.ContainsType(WalkAll, opt, test) {
			t.Errorf(`%s !ContainsType(WalkAll, %v, %v)`, opt, opt, test)
		}
	}
}

func TestEnumTypes(t *testing.T) {
	for _, test := range enums {
		var x *Type
		name, labels := test.name, test.labels
		create := func() {
			x = EnumType(labels...)
			if name != "" {
				x = NamedType(name, x)
			}
		}
		ExpectPanic(t, create, test.errstr, "%s EnumType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Enum; got != want {
			t.Errorf(`Enum %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "enum"; got != want {
			t.Errorf(`Enum %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`Enum %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`Enum %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.Unique(), test.str; got != want {
			t.Errorf(`Enum %s got unique %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumEnumLabel(), len(test.labels); got != want {
			t.Errorf(`Enum %s got num labels %d, want %d`, test.name, got, want)
		}
		for index, label := range test.labels {
			if got, want := x.EnumLabel(index), label; got != want {
				t.Errorf(`Enum %s got label[%d] %s, want %s`, test.name, index, got, want)
			}
			if got, want := x.EnumIndex(label), index; got != want {
				t.Errorf(`Enum %s got index[%s] %d, want %d`, test.name, label, got, want)
			}
		}
		if !x.ContainsKind(WalkAll, Enum) {
			t.Errorf(`Enum %s !ContainsKind(WalkAll, Enum)`, test.name)
		}
		if !x.ContainsType(WalkAll, x) {
			t.Errorf(`Enum %s !ContainsType(WalkAll, %v)`, test.name, x)
		}
	}
}

func TestArrayTypes(t *testing.T) {
	for index, test := range singletons {
		len := index + 1
		x := ArrayType(len, test.t)
		if got, want := x.Kind(), Array; got != want {
			t.Errorf(`Array %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Kind().String(), "array"; got != want {
			t.Errorf(`Array %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`Array %s got name %q, want %q`, test.k, got, want)
		}
		unique := fmt.Sprintf("[%d]%s", len, test.s)
		if got, want := x.String(), unique; got != want {
			t.Errorf(`Array %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Unique(), unique; got != want {
			t.Errorf(`Array %s got unique %q, want %q`, test.k, got, want)
		}
		if got, want := x.Len(), len; got != want {
			t.Errorf(`Array %s got len %v, want %v`, test.k, got, want)
		}
		if got, want := x.Elem(), test.t; got != want {
			t.Errorf(`Array %s got elem %q, want %q`, test.k, got, want)
		}
		if !x.ContainsKind(WalkAll, Array, test.k) {
			t.Errorf(`Array %s !ContainsKind(WalkAll, Array, %v)`, test.k, test.k)
		}
		if !x.ContainsType(WalkAll, x, test.t) {
			t.Errorf(`Array %s !ContainsType(WalkAll, %v, %v)`, test.k, x, test.t)
		}
	}
}

func TestListTypes(t *testing.T) {
	for _, test := range singletons {
		x := ListType(test.t)
		if got, want := x.Kind(), List; got != want {
			t.Errorf(`List %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Kind().String(), "list"; got != want {
			t.Errorf(`List %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`List %s got name %q, want %q`, test.k, got, want)
		}
		unique := "[]" + test.s
		if got, want := x.String(), unique; got != want {
			t.Errorf(`List %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Unique(), unique; got != want {
			t.Errorf(`List %s got unique %q, want %q`, test.k, got, want)
		}
		if got, want := x.Elem(), test.t; got != want {
			t.Errorf(`List %s got elem %q, want %q`, test.k, got, want)
		}
		if !x.ContainsKind(WalkAll, List, test.k) {
			t.Errorf(`List %s !ContainsKind(WalkAll, List, %v)`, test.k, test.k)
		}
		if !x.ContainsType(WalkAll, x, test.t) {
			t.Errorf(`List %s !ContainsType(WalkAll, %v, %v)`, test.k, x, test.t)
		}
	}
}

func TestSetTypes(t *testing.T) {
	for _, key := range singletons {
		if !key.t.CanBeKey() {
			var builder TypeBuilder
			x := builder.Set().AssignKey(key.t)
			builder.Build()
			_, err := x.Built()
			want := `invalid key "` + key.s + `" in "set[` + key.s + `]"`
			ExpectErr(t, err, want, "building Set[%s]:", key.k)
			continue
		}
		x := SetType(key.t)
		if got, want := x.Kind(), Set; got != want {
			t.Errorf(`Set %s got kind %q, want %q`, key.k, got, want)
		}
		if got, want := x.Kind().String(), "set"; got != want {
			t.Errorf(`Set %s got kind %q, want %q`, key.k, got, want)
		}
		if got, want := x.Name(), ""; got != want {
			t.Errorf(`Set %s got name %q, want %q`, key.k, got, want)
		}
		unique := "set[" + key.s + "]"
		if got, want := x.String(), unique; got != want {
			t.Errorf(`Set %s got string %q, want %q`, key.k, got, want)
		}
		if got, want := x.Unique(), unique; got != want {
			t.Errorf(`Set %s got unique %q, want %q`, key.k, got, want)
		}
		if got, want := x.Key(), key.t; got != want {
			t.Errorf(`Set %s got key %q, want %q`, key.k, got, want)
		}
		if !x.ContainsKind(WalkAll, Set, key.k) {
			t.Errorf(`Set %s !ContainsKind(WalkAll, Set, %v)`, key.k, key.k)
		}
		if !x.ContainsType(WalkAll, x, key.t) {
			t.Errorf(`Set %s !ContainsType(WalkAll, %v, %v)`, key.k, x, key.t)
		}
	}
}

func TestMapTypes(t *testing.T) {
	for _, key := range singletons {
		if !key.t.CanBeKey() {
			var builder TypeBuilder
			x := builder.Map().AssignKey(key.t).AssignElem(key.t)
			builder.Build()
			_, err := x.Built()
			want := `invalid key "` + key.s + `" in "map[` + key.s + `]` + key.s + `"`
			ExpectErr(t, err, want, "building Map[%s]%s", key.k, key.k)
			continue
		}
		for _, elem := range singletons {
			x := MapType(key.t, elem.t)
			if got, want := x.Kind(), Map; got != want {
				t.Errorf(`Map[%s]%s got kind %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Kind().String(), "map"; got != want {
				t.Errorf(`Map[%s]%s got kind %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Name(), ""; got != want {
				t.Errorf(`Map[%s]%s got name %q, want %q`, key.k, elem.k, got, want)
			}
			unique := fmt.Sprintf("map[%s]%s", key.s, elem.s)
			if got, want := x.String(), unique; got != want {
				t.Errorf(`Map[%s]%s got string %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Unique(), unique; got != want {
				t.Errorf(`Map[%s]%s got unique %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Key(), key.t; got != want {
				t.Errorf(`Map[%s]%s got key %q, want %q`, key.k, elem.k, got, want)
			}
			if got, want := x.Elem(), elem.t; got != want {
				t.Errorf(`Map[%s]%s got elem %q, want %q`, key.k, elem.k, got, want)
			}
			if !x.ContainsKind(WalkAll, Map, key.k, elem.k) {
				t.Errorf(`Map[%s]%s !ContainsKind(WalkAll, Map, %v, %v)`, key.k, elem.k, key.k, elem.k)
			}
			if !x.ContainsType(WalkAll, x, key.t, elem.t) {
				t.Errorf(`Map[%s]%s !ContainsType(WalkAll, %v, %v, %v)`, key.k, elem.k, x, key.t, elem.t)
			}
		}
	}
}

var validKeys = []*Type{
	Int32Type,
	ArrayType(3, Int32Type),
	StructType(Field{"A", Int32Type}),
	UnionType(Field{"A", Int32Type}),
}

var invalidKeys = []*Type{
	AnyType,
	ListType(Int32Type),
	MapType(Int32Type, Int32Type),
	OptionalType(NamedType("Foo", StructType(Field{"A", Int32Type}))),
	SetType(Int32Type),
	TypeObjectType,
}

func allInvalidKeyTypes(depth int) []*Type {
	if depth == 0 {
		return invalidKeys
	}
	recursiveTypes := allInvalidKeyTypes(depth - 1)
	var allTypes []*Type
	for _, t := range recursiveTypes {
		allTypes = append(allTypes, ArrayType(3, t))
		for _, v := range validKeys {
			allTypes = append(allTypes, StructType(Field{"T", t}, Field{"V", v}))
		}
	}
	return allTypes
}

func TestInvalidMapTypes(t *testing.T) {
	types := allInvalidKeyTypes(2)
	for _, x := range types {
		var builder TypeBuilder
		m := builder.Map().AssignKey(x).AssignElem(BoolType)
		s := builder.Set().AssignKey(x)
		builder.Build()
		_, errM := m.Built()
		_, errS := s.Built()
		ExpectErr(t, errM, `invalid key`, "building Map[%q]bool", x)
		ExpectErr(t, errS, `invalid key`, "building Set[%q]bool", x)
	}
}

func TestStructTypes(t *testing.T) { //nolint:gocyclo
	for _, test := range structs {
		var x *Type
		name, fields := test.name, test.fields
		create := func() {
			x = StructType(fields...)
			if name != "" {
				x = NamedType(name, x)
			}
		}
		ExpectPanic(t, create, test.errstr, "%s StructType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Struct; got != want {
			t.Errorf(`Struct %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "struct"; got != want {
			t.Errorf(`Struct %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`Struct %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`Struct %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.Unique(), test.str; got != want {
			t.Errorf(`Struct %s got unique %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumField(), len(test.fields); got != want {
			t.Errorf(`Struct %s got num fields %d, want %d`, test.name, got, want)
		}
		if !x.ContainsKind(WalkAll, Struct) {
			t.Errorf(`Struct %s !ContainsKind(WalkAll, Struct)`, test.name)
		}
		if !x.ContainsType(WalkAll, x) {
			t.Errorf(`Struct %s !ContainsType(WalkAll, %v)`, test.name, x)
		}
		for index, field := range test.fields {
			if got, want := x.Field(index), field; got != want {
				t.Errorf(`Struct %s got field[%d] %v, want %v`, test.name, index, got, want)
			}
			gotf, goti := x.FieldByName(field.Name)
			if wantf := field; gotf != wantf {
				t.Errorf(`Struct %s got field[%s] %v, want %v`, test.name, field.Name, gotf, wantf)
			}
			if wanti := index; goti != wanti {
				t.Errorf(`Struct %s got field[%s] index %d, want %d`, test.name, field.Name, goti, wanti)
			}
			if !x.ContainsKind(WalkAll, field.Type.Kind()) {
				t.Errorf(`Struct %s !ContainsKind(WalkAll, field[%d])`, test.name, index)
			}
			if !x.ContainsType(WalkAll, field.Type) {
				t.Errorf(`Struct %s !ContainsType(WalkAll, field[%d])`, test.name, index)
			}
		}
	}
	// Make sure hash consing of struct types respects the ordering of the fields.
	A, B, C := BoolType, Int32Type, Uint64Type
	x := StructType([]Field{{"A", A}, {"B", B}, {"C", C}}...)
	for iter := 0; iter < 10; iter++ {
		abc := StructType([]Field{{"A", A}, {"B", B}, {"C", C}}...)
		acb := StructType([]Field{{"A", A}, {"C", C}, {"B", B}}...)
		bac := StructType([]Field{{"B", B}, {"A", A}, {"C", C}}...)
		bca := StructType([]Field{{"B", B}, {"C", C}, {"A", A}}...)
		cab := StructType([]Field{{"C", C}, {"A", A}, {"B", B}}...)
		cba := StructType([]Field{{"C", C}, {"B", B}, {"A", A}}...)
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`Struct ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := StructType([]Field{{"A", A}, {"C", C}}...)
		ca := StructType([]Field{{"C", C}, {"A", A}}...)
		if x == ac || x == ca {
			t.Errorf(`Struct ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
}

func TestUnionTypes(t *testing.T) { //nolint:gocyclo
	for _, test := range unions {
		var x *Type
		name, fields := test.name, test.fields
		create := func() {
			x = UnionType(fields...)
			if name != "" {
				x = NamedType(name, x)
			}
		}
		ExpectPanic(t, create, test.errstr, "%s UnionType", test.name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), Union; got != want {
			t.Errorf(`Union %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Kind().String(), "union"; got != want {
			t.Errorf(`Union %s got kind %q, want %q`, test.name, got, want)
		}
		if got, want := x.Name(), test.name; got != want {
			t.Errorf(`Union %s got name %q, want %q`, test.name, got, want)
		}
		if got, want := x.String(), test.str; got != want {
			t.Errorf(`Union %s got string %q, want %q`, test.name, got, want)
		}
		if got, want := x.Unique(), test.str; got != want {
			t.Errorf(`Union %s got unique %q, want %q`, test.name, got, want)
		}
		if got, want := x.NumField(), len(test.fields); got != want {
			t.Errorf(`Union %s got num fields %d, want %d`, test.name, got, want)
		}
		if !x.ContainsKind(WalkAll, Union) {
			t.Errorf(`Union %s !ContainsKind(WalkAll, Union)`, test.name)
		}
		if !x.ContainsType(WalkAll, x) {
			t.Errorf(`Union %s !ContainsType(WalkAll, %v)`, test.name, x)
		}
		for index, field := range test.fields {
			if got, want := x.Field(index), field; got != want {
				t.Errorf(`Union %s got field[%d] %v, want %v`, test.name, index, got, want)
			}
			gotf, goti := x.FieldByName(field.Name)
			if wantf := field; gotf != wantf {
				t.Errorf(`Union %s got field[%s] %v, want %v`, test.name, field.Name, gotf, wantf)
			}
			if wanti := index; goti != wanti {
				t.Errorf(`Union %s got field[%s] index %d, want %d`, test.name, field.Name, goti, wanti)
			}
			if !x.ContainsKind(WalkAll, field.Type.Kind()) {
				t.Errorf(`Union %s !ContainsKind(WalkAll, field[%d])`, test.name, index)
			}
			if !x.ContainsType(WalkAll, field.Type) {
				t.Errorf(`Union %s !ContainsType(WalkAll, field[%d])`, test.name, index)
			}
		}
	}
	// Make sure hash consing of struct types respects the ordering of the fields.
	A, B, C := BoolType, Int32Type, Uint64Type
	x := UnionType([]Field{{"A", A}, {"B", B}, {"C", C}}...)
	for iter := 0; iter < 10; iter++ {
		abc := UnionType([]Field{{"A", A}, {"B", B}, {"C", C}}...)
		acb := UnionType([]Field{{"A", A}, {"C", C}, {"B", B}}...)
		bac := UnionType([]Field{{"B", B}, {"A", A}, {"C", C}}...)
		bca := UnionType([]Field{{"B", B}, {"C", C}, {"A", A}}...)
		cab := UnionType([]Field{{"C", C}, {"A", A}, {"B", B}}...)
		cba := UnionType([]Field{{"C", C}, {"B", B}, {"A", A}}...)
		if x != abc || x == acb || x == bac || x == bca || x == cab || x == cba {
			t.Errorf(`Union ABC hash consing broken: %v, %v, %v, %v, %v, %v, %v`, x, abc, acb, bac, bca, cab, cba)
		}
		ac := UnionType([]Field{{"A", A}, {"C", C}}...)
		ca := UnionType([]Field{{"C", C}, {"A", A}}...)
		if x == ac || x == ca {
			t.Errorf(`Union ABC / AC hash consing broken: %v, %v, %v`, x, ac, ca)
		}
	}
}

func TestNamedTypes(t *testing.T) { //nolint:gocyclo
	for _, test := range singletons {
		var errstr string
		switch test.k {
		case Any, TypeObject:
			errstr = "any and typeobject cannot be renamed"
		}
		name := "Named" + test.s
		var x *Type
		typ := test.t
		create := func() { x = NamedType(name, typ) }
		ExpectPanic(t, create, errstr, name)
		if x == nil {
			continue
		}
		if got, want := x.Kind(), test.k; got != want {
			t.Errorf(`Named %s got kind %q, want %q`, test.k, got, want)
		}
		if got, want := x.Name(), name; got != want {
			t.Errorf(`Named %s got name %q, want %q`, test.k, got, want)
		}
		unique := name + " " + test.s
		if got, want := x.String(), unique; got != want {
			t.Errorf(`Named %s got string %q, want %q`, test.k, got, want)
		}
		if got, want := x.Unique(), unique; got != want {
			t.Errorf(`Named %s got unique %q, want %q`, test.k, got, want)
		}
	}
	// Try a chain of named types:
	// type A B
	// type B C
	// type C D
	// type D struct{X []C}
	var builder TypeBuilder
	a, b, c, d := builder.Named("A"), builder.Named("B"), builder.Named("C"), builder.Named("D")
	a.AssignBase(b)
	b.AssignBase(c)
	c.AssignBase(d)
	d.AssignBase(builder.Struct().AppendField("X", builder.List().AssignElem(c)))
	builder.Build()
	bA, errA := a.Built()
	bB, errB := b.Built()
	bC, errC := c.Built()
	bD, errD := d.Built()
	if errA != nil || errB != nil || errC != nil || errD != nil {
		t.Errorf(`Named chain got (%q,%q,%q,%q), want nil`, errA, errB, errC, errD)
	}
	if got, want := bA.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bB.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bC.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bD.Kind(), Struct; got != want {
		t.Errorf(`Named chain got kind %q, want %q`, got, want)
	}
	if got, want := bA.Name(), "A"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bB.Name(), "B"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bC.Name(), "C"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	if got, want := bD.Name(), "D"; got != want {
		t.Errorf(`Named chain got name %q, want %q`, got, want)
	}
	uniqueA := "A struct{X []C struct{X []C}}"
	if got, want := bA.String(), uniqueA; got != want {
		t.Errorf(`Named chain got string %q, want %q`, got, want)
	}
	if got, want := bA.Unique(), uniqueA; got != want {
		t.Errorf(`Named chain got unique %q, want %q`, got, want)
	}
	uniqueB := "B struct{X []C struct{X []C}}"
	if got, want := bB.String(), uniqueB; got != want {
		t.Errorf(`Named chain got string %q, want %q`, got, want)
	}
	if got, want := bB.Unique(), uniqueB; got != want {
		t.Errorf(`Named chain got unique %q, want %q`, got, want)
	}
	uniqueC := "C struct{X []C}"
	if got, want := bC.String(), uniqueC; got != want {
		t.Errorf(`Named chain got string %q, want %q`, got, want)
	}
	if got, want := bC.Unique(), uniqueC; got != want {
		t.Errorf(`Named chain got unique %q, want %q`, got, want)
	}
	uniqueD := "D struct{X []C struct{X []C}}"
	if got, want := bD.String(), uniqueD; got != want {
		t.Errorf(`Named chain got string %q, want %q`, got, want)
	}
	if got, want := bD.Unique(), uniqueD; got != want {
		t.Errorf(`Named chain got unique %q, want %q`, got, want)
	}
	if got, want := bA.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bB.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bC.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bD.NumField(), 1; got != want {
		t.Errorf(`Named chain got NumField %q, want %q`, got, want)
	}
	if got, want := bA.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bB.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bC.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	if got, want := bD.Field(0).Name, "X"; got != want {
		t.Errorf(`Named chain got Field(0).Name %q, want %q`, got, want)
	}
	listC := ListType(bC)
	if got, want := bA.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bB.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bC.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
	if got, want := bD.Field(0).Type, listC; got != want {
		t.Errorf(`Named chain got Field(0).Type %q, want %q`, got, want)
	}
}

func TestHashConsTypes(t *testing.T) {
	// Create a bunch of distinct types multiple times.
	var types [3][]*Type
	for iter := 0; iter < 3; iter++ {
		for _, a := range singletons {
			types[iter] = append(types[iter], a.t)
			if a.t.CanBeNamed() {
				types[iter] = append(types[iter], NamedType("Named"+a.s, a.t))
			}
			if a.t.CanBeOptional() {
				types[iter] = append(types[iter], OptionalType(a.t))
				types[iter] = append(types[iter], NamedType("Optional"+a.s, OptionalType(a.t)))
			}
			if a.t.CanBeKey() {
				types[iter] = append(types[iter], SetType(a.t))
				types[iter] = append(types[iter], NamedType("Set"+a.s, SetType(a.t)))
			}
			types[iter] = append(types[iter], ListType(a.t))
			types[iter] = append(types[iter], NamedType("List"+a.s, ListType(a.t)))
			for _, b := range singletons {
				lA, lB := "A"+a.s, "B"+b.s
				name := lA + lB
				types[iter] = append(types[iter], EnumType(lA, lB))
				types[iter] = append(types[iter], NamedType("Enum"+name, EnumType(lA, lB)))
				if a.t.CanBeKey() {
					types[iter] = append(types[iter], MapType(a.t, b.t))
					types[iter] = append(types[iter], NamedType("Map"+name, MapType(a.t, b.t)))
				}
				fields := []Field{{lA, a.t}, {lB, b.t}}
				types[iter] = append(types[iter], StructType(fields...))
				types[iter] = append(types[iter], NamedType("Struct"+name, StructType(fields...)))
				types[iter] = append(types[iter], UnionType(fields...))
				types[iter] = append(types[iter], NamedType("Union"+name, UnionType(fields...)))
			}
		}
	}

	// Make sure the pointers are the same across iterations, and different within
	// an iteration.
	seen := map[*Type]bool{}
	for ix := 0; ix < len(types[0]); ix++ {
		a, b, c := types[0][ix], types[1][ix], types[2][ix]
		if a != b || a != c {
			t.Errorf(`HashCons mismatched pointer[%d]: %p %p %p`, ix, a, b, c)
		}
		if seen[a] {
			t.Errorf(`HashCons dup pointer[%d]: %v %v`, ix, a, seen)
		}
		seen[a] = true
	}
}

func TestAssignableFrom(t *testing.T) {
	// Systematic testing of AssignableFrom over allTypes will just duplicate the
	// actual logic, so we just spot-check some results manually.
	optType := OptionalType(NamedType("X", StructType(Field{"A", BoolType})))
	optValue := ZeroValue(optType)
	tests := []struct {
		to   *Type
		from *Value
		want bool
	}{
		{BoolType, BoolValue(nil, false), true},
		{optType, optValue, true},
		{optType, ZeroValue(AnyType), true},
		{AnyType, BoolValue(nil, false), true},
		{AnyType, optValue, true},
		{AnyType, ZeroValue(AnyType), true},

		{BoolType, IntValue(Int32Type, 123), false},
		{BoolType, optValue, false},
		{BoolType, AnyValue(optValue), false},
		{optType, BoolValue(nil, false), false},
		{optType, AnyValue(optValue), false},
	}
	for _, test := range tests {
		if test.to.AssignableFrom(test.from) != test.want {
			t.Errorf(`%v.AssignableFrom(%v) want %v`, test.to, test.from, test.want)
		}
	}
}

func TestSelfRecursiveType(t *testing.T) { //nolint:gocyclo
	buildTree := func() (*Type, error, *Type, error) {
		// type Node struct {
		//   Val      string
		//   Children []Node
		// }
		var builder TypeBuilder
		pendN := builder.Named("Node")
		pendC := builder.List().AssignElem(pendN)
		structN := builder.Struct()
		structN.AppendField("Val", StringType)
		structN.AppendField("Children", pendC)
		pendN.AssignBase(structN)
		builder.Build()
		c, cerr := pendC.Built()
		n, nerr := pendN.Built()
		return c, cerr, n, nerr
	}
	c, cerr, n, nerr := buildTree()
	if cerr != nil || nerr != nil {
		t.Errorf(`build got cerr %q nerr %q, want nil`, cerr, nerr)
	}
	// Check node
	if got, want := n.Kind(), Struct; got != want {
		t.Errorf(`node Kind got %s, want %s`, got, want)
	}
	if got, want := n.Name(), "Node"; got != want {
		t.Errorf(`node Name got %q, want %q`, got, want)
	}
	uniqueN := "Node struct{Val string;Children []Node}"
	if got, want := n.String(), uniqueN; got != want {
		t.Errorf(`node String got %q, want %q`, got, want)
	}
	if got, want := n.Unique(), uniqueN; got != want {
		t.Errorf(`node Unique got %q, want %q`, got, want)
	}
	if got, want := n.NumField(), 2; got != want {
		t.Errorf(`node NumField got %q, want %q`, got, want)
	}
	if got, want := n.Field(0).Name, "Val"; got != want {
		t.Errorf(`node Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := n.Field(0).Type, StringType; got != want {
		t.Errorf(`node Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := n.Field(1).Name, "Children"; got != want {
		t.Errorf(`node Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := n.Field(1).Type, c; got != want {
		t.Errorf(`node Field(1).Type got %q, want %q`, got, want)
	}
	if !n.ContainsKind(WalkAll, String, Struct, List) {
		t.Errorf(`node !ContainsKind(WalkAll, String, Struct, List)`)
	}
	if !n.ContainsType(WalkAll, StringType, n, c) {
		t.Errorf(`node !ContainsType(WalkAll, string, %v, %v)`, n, c)
	}
	// Check children
	if got, want := c.Kind(), List; got != want {
		t.Errorf(`children Kind got %s, want %s`, got, want)
	}
	if got, want := c.Name(), ""; got != want {
		t.Errorf(`children Name got %q, want %q`, got, want)
	}
	uniqueC := "[]Node struct{Val string;Children []Node}"
	if got, want := c.String(), uniqueC; got != want {
		t.Errorf(`children String got %q, want %q`, got, want)
	}
	if got, want := c.Unique(), uniqueC; got != want {
		t.Errorf(`children Unique got %q, want %q`, got, want)
	}
	if got, want := c.Elem(), n; got != want {
		t.Errorf(`children Elem got %q, want %q`, got, want)
	}
	if !c.ContainsKind(WalkAll, String, Struct, List) {
		t.Errorf(`children !ContainsKind(WalkAll, String, Struct, List)`)
	}
	if !c.ContainsType(WalkAll, StringType, n, c) {
		t.Errorf(`children !ContainsType(WalkAll, string, %v, %v)`, n, c)
	}
	// Check hash-consing
	for iter := 0; iter < 5; iter++ {
		c2, cerr2, n2, nerr2 := buildTree()
		if cerr2 != nil || nerr2 != nil {
			t.Errorf(`build got cerr %q nerr %q, want nil`, cerr, nerr)
		}
		if got, want := c2, c; got != want {
			t.Errorf(`cons children got %q, want %q`, got, want)
		}
		if got, want := n2, n; got != want {
			t.Errorf(`cons node got %q, want %q`, got, want)
		}
	}
}

func TestStrictCycleType(t *testing.T) {
	var builder TypeBuilder
	pendA, pendB := builder.Named("A"), builder.Named("B")
	stA, stB := builder.Struct(), builder.Struct()
	pendA.AssignBase(stA)
	pendB.AssignBase(stB)
	stA.AppendField("X", Int32Type).AppendField("B", pendB)
	stB.AppendField("X", Int32Type).AppendField("A", pendA)
	builder.Build()
	a, aerr := pendA.Built()
	b, berr := pendB.Built()
	if a != nil || b != nil || aerr == nil || berr == nil {
		t.Errorf(`Type A struct{X int32; B B struct{X int32; A A}} should not be valid (%v, %v, %v, %v)`, a, b, aerr, berr)
	}
}

func TestMutuallyRecursiveType(t *testing.T) { //nolint:gocyclo
	build := func() (*Type, error, *Type, error, *Type, error, *Type, error) {
		// type D A
		// type A struct{X int32;B  B;C C}
		// type B struct{Y int32;A ?A;C C}
		// type C struct{Z string}
		var builder TypeBuilder
		a, b, c, d := builder.Named("A"), builder.Named("B"), builder.Named("C"), builder.Named("D")
		stA, stB, stC := builder.Struct(), builder.Struct(), builder.Struct()
		d.AssignBase(a)
		a.AssignBase(stA)
		b.AssignBase(stB)
		c.AssignBase(stC)
		stA.AppendField("X", Int32Type).AppendField("B", b).AppendField("C", c)
		aOrNil := builder.Optional()
		aOrNil.AssignElem(a)
		stB.AppendField("Y", Int32Type).AppendField("A", aOrNil).AppendField("C", c)
		stC.AppendField("Z", StringType)
		builder.Build()
		builtD, derr := d.Built()
		builtA, aerr := a.Built()
		builtB, berr := b.Built()
		builtC, cerr := c.Built()
		return builtD, derr, builtA, aerr, builtB, berr, builtC, cerr
	}
	d, derr, a, aerr, b, berr, c, cerr := build()
	if derr != nil || aerr != nil || berr != nil || cerr != nil {
		t.Errorf(`build got (%q,%q,%q,%q), want nil`, derr, aerr, berr, cerr)
	}
	// Check D
	if got, want := d.Kind(), Struct; got != want {
		t.Errorf(`D Kind got %s, want %s`, got, want)
	}
	if got, want := d.Name(), "D"; got != want {
		t.Errorf(`D Name got %q, want %q`, got, want)
	}
	uniqueD := "D struct{X int32;B B struct{Y int32;A ?A struct{X int32;B B;C C struct{Z string}};C C};C C}"
	if got, want := d.String(), uniqueD; got != want {
		t.Errorf(`D String got %q, want %q`, got, want)
	}
	if got, want := d.Unique(), uniqueD; got != want {
		t.Errorf(`D Unique got %q, want %q`, got, want)
	}
	if got, want := d.NumField(), 3; got != want {
		t.Errorf(`D NumField got %q, want %q`, got, want)
	}
	if got, want := d.Field(0).Name, "X"; got != want {
		t.Errorf(`D Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(0).Type, Int32Type; got != want {
		t.Errorf(`D Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := d.Field(1).Name, "B"; got != want {
		t.Errorf(`D Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(1).Type, b; got != want {
		t.Errorf(`D Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := d.Field(2).Name, "C"; got != want {
		t.Errorf(`D Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := d.Field(2).Type, c; got != want {
		t.Errorf(`D Field(2).Type got %q, want %q`, got, want)
	}
	if !d.ContainsKind(WalkAll, Int32, String, Struct, Optional) {
		t.Errorf(`D !ContainsKind(WalkAll, Int32, String, Struct, Optional)`)
	}
	if !d.ContainsType(WalkAll, Int32Type, StringType, d, a, b, c) {
		t.Errorf(`D !ContainsType(WalkAll, int32, string, %v, %v, %v, %v)`, d, a, b, c)
	}
	// Check A
	if got, want := a.Kind(), Struct; got != want {
		t.Errorf(`A Kind got %s, want %s`, got, want)
	}
	if got, want := a.Name(), "A"; got != want {
		t.Errorf(`A Name got %q, want %q`, got, want)
	}
	uniqueA := "A struct{X int32;B B struct{Y int32;A ?A;C C struct{Z string}};C C}"
	if got, want := a.String(), uniqueA; got != want {
		t.Errorf(`A String got %q, want %q`, got, want)
	}
	if got, want := a.Unique(), uniqueA; got != want {
		t.Errorf(`A Unique got %q, want %q`, got, want)
	}
	if got, want := a.NumField(), 3; got != want {
		t.Errorf(`A NumField got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Name, "X"; got != want {
		t.Errorf(`A Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(0).Type, Int32Type; got != want {
		t.Errorf(`A Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Name, "B"; got != want {
		t.Errorf(`A Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(1).Type, b; got != want {
		t.Errorf(`A Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := a.Field(2).Name, "C"; got != want {
		t.Errorf(`A Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := a.Field(2).Type, c; got != want {
		t.Errorf(`A Field(2).Type got %q, want %q`, got, want)
	}
	if !a.ContainsKind(WalkAll, Int32, String, Struct, Optional) {
		t.Errorf(`A !ContainsKind(WalkAll, Int32, String, Struct, Optional)`)
	}
	if !a.ContainsType(WalkAll, Int32Type, StringType, a, b, c) {
		t.Errorf(`A !ContainsType(WalkAll, int32, string, %v, %v, %v)`, a, b, c)
	}
	// Check B
	if got, want := b.Kind(), Struct; got != want {
		t.Errorf(`B Kind got %s, want %s`, got, want)
	}
	if got, want := b.Name(), "B"; got != want {
		t.Errorf(`B Name got %q, want %q`, got, want)
	}
	uniqueB := "B struct{Y int32;A ?A struct{X int32;B B;C C struct{Z string}};C C}"
	if got, want := b.String(), uniqueB; got != want {
		t.Errorf(`B String got %q, want %q`, got, want)
	}
	if got, want := b.Unique(), uniqueB; got != want {
		t.Errorf(`B Unique got %q, want %q`, got, want)
	}
	if got, want := b.NumField(), 3; got != want {
		t.Errorf(`B NumField got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Name, "Y"; got != want {
		t.Errorf(`B Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(0).Type, Int32Type; got != want {
		t.Errorf(`B Field(0).Type got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Name, "A"; got != want {
		t.Errorf(`B Field(1).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(1).Type, OptionalType(a); got != want {
		t.Errorf(`B Field(1).Type got %q, want %q`, got, want)
	}
	if got, want := b.Field(2).Name, "C"; got != want {
		t.Errorf(`B Field(2).Name got %q, want %q`, got, want)
	}
	if got, want := b.Field(2).Type, c; got != want {
		t.Errorf(`B Field(2).Type got %q, want %q`, got, want)
	}
	if !b.ContainsKind(WalkAll, Int32, String, Struct, Optional) {
		t.Errorf(`B !ContainsKind(WalkAll, Int32, String, Struct, Optional)`)
	}
	if !b.ContainsType(WalkAll, Int32Type, StringType, a, b, c) {
		t.Errorf(`B !ContainsType(WalkAll, int32, string, %v, %v, %v)`, a, b, c)
	}
	// Check C
	if got, want := c.Kind(), Struct; got != want {
		t.Errorf(`C Kind got %s, want %s`, got, want)
	}
	if got, want := c.Name(), "C"; got != want {
		t.Errorf(`C Name got %q, want %q`, got, want)
	}
	uniqueC := "C struct{Z string}"
	if got, want := c.String(), uniqueC; got != want {
		t.Errorf(`C String got %q, want %q`, got, want)
	}
	if got, want := c.Unique(), uniqueC; got != want {
		t.Errorf(`C Unique got %q, want %q`, got, want)
	}
	if got, want := c.NumField(), 1; got != want {
		t.Errorf(`C NumField got %q, want %q`, got, want)
	}
	if got, want := c.Field(0).Name, "Z"; got != want {
		t.Errorf(`C Field(0).Name got %q, want %q`, got, want)
	}
	if got, want := c.Field(0).Type, StringType; got != want {
		t.Errorf(`C Field(0).Type got %q, want %q`, got, want)
	}
	if !c.ContainsKind(WalkAll, String, Struct) {
		t.Errorf(`C !ContainsKind(WalkAll, String, Struct)`)
	}
	if !c.ContainsType(WalkAll, StringType, c) {
		t.Errorf(`C !ContainsType(WalkAll, string, %v)`, c)
	}
	// Check hash-consing
	for iter := 0; iter < 5; iter++ {
		d2, derr, a2, aerr, b2, berr, c2, cerr := build()
		if derr != nil || aerr != nil || berr != nil || cerr != nil {
			t.Errorf(`build got (%q,%q,%q,%q), want nil`, derr, aerr, berr, cerr)
		}
		if got, want := d2, d; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := a2, a; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := b2, b; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
		if got, want := c2, c; got != want {
			t.Errorf(`build got %q, want %q`, got, want)
		}
	}
}

func TestUniqueTypeNames(t *testing.T) {
	var builder TypeBuilder
	var pending [2][]PendingType
	pending[0] = makeAllPending(&builder)
	pending[1] = makeAllPending(&builder)
	builder.Build()
	// The first pending types have no errors, but have nil types since the other
	// pending types fail to build.
	for _, p := range pending[0] {
		ty, err := p.Built()
		if ty != nil {
			t.Errorf(`built[0] got type %q, want nil`, ty)
		}
		if err != nil {
			t.Errorf(`built[0] got error %q, want nil`, err)
		}
	}
	// The second built types have non-unique name errors, and also nil types.
	for _, p := range pending[1] {
		ty, err := p.Built()
		if ty != nil {
			t.Errorf(`built[0] got type %q, want nil`, ty)
		}
		if got, want := fmt.Sprint(err), "duplicate type names"; !strings.Contains(got, want) {
			t.Errorf(`built[0] got error %s, want %s`, got, want)
		}
	}
}

func makeAllPending(builder *TypeBuilder) []PendingType {
	var ret []PendingType
	for _, test := range singletons {
		if test.t.CanBeNamed() {
			ret = append(ret, builder.Named("Named"+test.s).AssignBase(test.t))
		}
	}
	for _, test := range enums {
		if test.errstr == "" {
			base := builder.Enum()
			for _, l := range test.labels {
				base.AppendLabel(l)
			}
			ret = append(ret, builder.Named("Enum"+test.name).AssignBase(base))
		}
	}
	for _, test := range structs {
		if test.errstr == "" {
			base := builder.Struct()
			for _, f := range test.fields {
				base.AppendField(f.Name, f.Type)
			}
			ret = append(ret, builder.Named("Struct"+test.name).AssignBase(base))
		}
	}
	for _, test := range unions {
		if test.errstr == "" {
			base := builder.Union()
			for _, f := range test.fields {
				base.AppendField(f.Name, f.Type)
			}
			ret = append(ret, builder.Named("Union"+test.name).AssignBase(base))
		}
	}
	return ret
}
