// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parse_test

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"v.io/x/ref/lib/vdl/internal/vdltestutil"
	"v.io/x/ref/lib/vdl/parse"
	"v.io/x/ref/lib/vdl/vdlutil"
)

func pos(line, col int) parse.Pos {
	return parse.Pos{line, col}
}

func sp(str string, line, col int) parse.StringPos {
	return parse.StringPos{String: str, Pos: pos(line, col)}
}

func lf(l, f parse.StringPos) parse.LangFmt {
	return parse.LangFmt{Lang: l, Fmt: f}
}

func np(name string, line, col int) parse.NamePos {
	return parse.NamePos{Name: name, Pos: pos(line, col)}
}

func npptr(name string, line, col int) *parse.NamePos {
	ret := np(name, line, col)
	return &ret
}

func tn(name string, line, col int) *parse.TypeNamed {
	return &parse.TypeNamed{Name: name, P: pos(line, col)}
}

func cn(name string, line, col int) *parse.ConstNamed {
	return &parse.ConstNamed{Name: name, P: pos(line, col)}
}

func cl(lit interface{}, line, col int) *parse.ConstLit {
	return &parse.ConstLit{Lit: lit, P: pos(line, col)}
}

// Tests of vdl imports and file parsing.
type vdlParseTest struct {
	name   string
	src    string
	expect *parse.File
	errors []string
}

func testParseVDL(t *testing.T, test vdlParseTest, opts parse.Opts) {
	errs := vdlutil.NewErrors(-1)
	actual := parse.ParseFile("testfile", strings.NewReader(test.src), opts, errs)
	vdltestutil.ExpectResult(t, errs, test.name, test.errors...)
	if !reflect.DeepEqual(test.expect, actual) {
		t.Errorf("%v\nEXPECT %+v\nACTUAL %+v", test.name, test.expect, actual)
	}
}

func TestParseVDLImports(t *testing.T) {
	for _, test := range vdlImportsTests {
		testParseVDL(t, test, parse.Opts{ImportsOnly: true})
	}
	for _, test := range vdlFileTests {
		// We only run the success tests from vdlFileTests on the imports only
		// parser, since the failure tests are testing failures in stuff after the
		// imports, which won't cause failures in the imports only parser.
		//
		// The imports-only parser isn't supposed to fill in fields after the
		// imports, so we clear them from the expected result.  We must copy the
		// file to ensure the actual vdlFileTests isn't overwritten since the
		// full-parser tests needs the full expectations.  The test itself doesn't
		// need to be copied, since it's already copied in the range-for.
		if test.expect != nil {
			copyFile := *test.expect
			test.expect = &copyFile
			test.expect.TypeDefs = nil
			test.expect.ConstDefs = nil
			test.expect.ErrorDefs = nil
			test.expect.Interfaces = nil
			testParseVDL(t, test, parse.Opts{ImportsOnly: true})
		}
	}
}

func TestParseVDLFile(t *testing.T) {
	for _, test := range append(vdlImportsTests, vdlFileTests...) {
		testParseVDL(t, test, parse.Opts{ImportsOnly: false})
	}
}

// Tests of config imports and file parsing.
type configTest struct {
	name   string
	src    string
	expect *parse.Config
	errors []string
}

func testParseConfig(t *testing.T, test configTest, opts parse.Opts) {
	errs := vdlutil.NewErrors(-1)
	actual := parse.ParseConfig("testfile", strings.NewReader(test.src), opts, errs)
	vdltestutil.ExpectResult(t, errs, test.name, test.errors...)
	if !reflect.DeepEqual(test.expect, actual) {
		t.Errorf("%v\nEXPECT %+v\nACTUAL %+v", test.name, test.expect, actual)
	}
}

func TestParseConfigImports(t *testing.T) {
	for _, test := range configTests {
		// We only run the success tests from configTests on the imports only
		// parser, since the failure tests are testing failures in stuff after the
		// imports, which won't cause failures in the imports only parser.
		//
		// The imports-only parser isn't supposed to fill in fields after the
		// imports, so we clear them from the expected result.  We must copy the
		// file to ensure the actual configTests isn't overwritten since the
		// full-parser tests needs the full expectations.  The test itself doesn't
		// need to be copied, since it's already copied in the range-for.
		if test.expect != nil {
			copyConfig := *test.expect
			test.expect = &copyConfig
			test.expect.Config = nil
			test.expect.ConstDefs = nil
			testParseConfig(t, test, parse.Opts{ImportsOnly: true})
		}
	}
}

func TestParseConfig(t *testing.T) {
	for _, test := range configTests {
		testParseConfig(t, test, parse.Opts{ImportsOnly: false})
	}
}

// vdlImportsTests contains tests of stuff up to and including the imports.
var vdlImportsTests = []vdlParseTest{
	// Empty file isn't allowed (need at least a package clause).
	{
		"FAILEmptyFile",
		"",
		nil,
		[]string{"vdl file must start with package clause"}},

	// Comment tests.
	{
		"PackageDocOneLiner",
		`// One liner
// Another line
package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: parse.NamePos{Name: "testpkg", Pos: pos(3, 9), Doc: `// One liner
// Another line
`}},
		nil},
	{
		"PackageDocMultiLiner",
		`/* Multi liner
Another line
*/
package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: parse.NamePos{Name: "testpkg", Pos: pos(4, 9), Doc: `/* Multi liner
Another line
*/
`}},
		nil},
	{
		"FileDocNoPackageDoc",
		`// File doc, has extra newline so not package doc

package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 3, 9), Doc: "// File doc, has extra newline so not package doc\n"},
		nil},
	{
		"FileDocAndPackageDoc",
		`// File doc

// Package doc
package testpkg`,
		&parse.File{BaseName: "testfile", PackageDef: parse.NamePos{Name: "testpkg", Pos: pos(4, 9), Doc: "// Package doc\n"}, Doc: "// File doc\n"},
		nil},
	{
		"FAILUnterminatedComment",
		`/* Unterminated
Another line
package testpkg`,
		nil,
		[]string{"comment not terminated"}},

	// Package tests.
	{
		"Package",
		"package testpkg;",
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"PackageNoSemi",
		"package testpkg",
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"FAILBadPackageName",
		"package foo.bar",
		nil,
		[]string{"testfile:1:12 syntax error"}},

	// Import tests.
	{
		"EmptyImport",
		`package testpkg;
import (
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"OneImport",
		`package testpkg;
import "foo/bar";`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 2, 8)}}},
		nil},
	{
		"OneImportLocalNameNoSemi",
		`package testpkg
import baz "foo/bar"`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("baz", 2, 8)}}},
		nil},
	{
		"OneImportParens",
		`package testpkg
import (
  "foo/bar";
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}}},
		nil},
	{
		"OneImportParensNoSemi",
		`package testpkg
import (
  "foo/bar"
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}}},
		nil},
	{
		"MixedImports",
		`package testpkg
import "foo/bar"
import (
  "baz";"a/b"
  "c/d"
)
import "z"`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Imports: []*parse.Import{
				{Path: "foo/bar", NamePos: np("", 2, 8)},
				{Path: "baz", NamePos: np("", 4, 3)},
				{Path: "a/b", NamePos: np("", 4, 9)},
				{Path: "c/d", NamePos: np("", 5, 3)},
				{Path: "z", NamePos: np("", 7, 8)}}},
		nil},
	{
		"FAILImportParensNotClosed",
		`package testpkg
import (
  "foo/bar"`,
		nil,
		[]string{"testfile:3:12 syntax error"}},
}

// vdlFileTests contains tests of stuff after the imports.
var vdlFileTests = []vdlParseTest{
	// Data type tests.
	{
		"TypeNamed",
		`package testpkg
type foo bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: tn("bar", 2, 10)}}},
		nil},
	{
		"TypeNamedQualified",
		`package testpkg
type foo bar.baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: tn("bar.baz", 2, 10)}}},
		nil},
	{
		"TypeNamedQualifiedPath",
		`package testpkg
type foo "a/b/c/bar".baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: tn(`"a/b/c/bar".baz`, 2, 10)}}},
		nil},
	{
		"TypeEnum",
		`package testpkg
type foo enum{A;B;C}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeEnum{
					Labels: []parse.NamePos{np("A", 2, 15), np("B", 2, 17), np("C", 2, 19)},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeEnumNewlines",
		`package testpkg
type foo enum {
  A
  B
  C
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeEnum{
					Labels: []parse.NamePos{np("A", 3, 3), np("B", 4, 3), np("C", 5, 3)},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeArray",
		`package testpkg
type foo [2]bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeArray{
					Len: 2, Elem: tn("bar", 2, 13), P: pos(2, 10)}}}},
		nil},
	{
		"TypeList",
		`package testpkg
type foo []bar`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeList{
					Elem: tn("bar", 2, 12), P: pos(2, 10)}}}},
		nil},
	{
		"TypeSet",
		`package testpkg
type foo set[bar]`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeSet{
					Key: tn("bar", 2, 14), P: pos(2, 10)}}}},
		nil},
	{
		"TypeMap",
		`package testpkg
type foo map[bar]baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeMap{
					Key: tn("bar", 2, 14), Elem: tn("baz", 2, 18), P: pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneField",
		`package testpkg
type foo struct{a b;}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 2, 17), Type: tn("b", 2, 19)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNoSemi",
		`package testpkg
type foo struct{a b}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 2, 17), Type: tn("b", 2, 19)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNewline",
		`package testpkg
type foo struct{
  a b;
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldNewlineNoSemi",
		`package testpkg
type foo struct{
  a b
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)}},
					P:      pos(2, 10)}}}},
		nil},
	{
		"TypeStructOneFieldList",
		`package testpkg
type foo struct{a,b,c d}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{
						{NamePos: np("a", 2, 17), Type: tn("d", 2, 23)},
						{NamePos: np("b", 2, 19), Type: tn("d", 2, 23)},
						{NamePos: np("c", 2, 21), Type: tn("d", 2, 23)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeStructMixed",
		`package testpkg
type foo struct{
  a b;c,d e
  f,g h
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeStruct{
					Fields: []*parse.Field{
						{NamePos: np("a", 3, 3), Type: tn("b", 3, 5)},
						{NamePos: np("c", 3, 7), Type: tn("e", 3, 11)},
						{NamePos: np("d", 3, 9), Type: tn("e", 3, 11)},
						{NamePos: np("f", 4, 3), Type: tn("h", 4, 7)},
						{NamePos: np("g", 4, 5), Type: tn("h", 4, 7)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeUnion",
		`package testpkg
type foo union{A a;B b;C c}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeUnion{
					Fields: []*parse.Field{
						{NamePos: np("A", 2, 16), Type: tn("a", 2, 18)},
						{NamePos: np("B", 2, 20), Type: tn("b", 2, 22)},
						{NamePos: np("C", 2, 24), Type: tn("c", 2, 26)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeUnionNewlines",
		`package testpkg
type foo union{
  A a
  B b
  C c
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeUnion{
					Fields: []*parse.Field{
						{NamePos: np("A", 3, 3), Type: tn("a", 3, 5)},
						{NamePos: np("B", 4, 3), Type: tn("b", 4, 5)},
						{NamePos: np("C", 5, 3), Type: tn("c", 5, 5)}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeOptional",
		`package testpkg
type foo union{A a;B ?b;C ?c}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeUnion{
					Fields: []*parse.Field{
						{NamePos: np("A", 2, 16), Type: tn("a", 2, 18)},
						{NamePos: np("B", 2, 20),
							Type: &parse.TypeOptional{Base: tn("b", 2, 23), P: pos(2, 22)}},
						{NamePos: np("C", 2, 25),
							Type: &parse.TypeOptional{Base: tn("c", 2, 28), P: pos(2, 27)}}},
					P: pos(2, 10)}}}},
		nil},
	{
		"TypeOptionalNewlines",
		`package testpkg
type foo union{
  A a
  B ?b
  C ?c
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			TypeDefs: []*parse.TypeDef{
				{NamePos: np("foo", 2, 6), Type: &parse.TypeUnion{
					Fields: []*parse.Field{
						{NamePos: np("A", 3, 3), Type: tn("a", 3, 5)},
						{NamePos: np("B", 4, 3),
							Type: &parse.TypeOptional{Base: tn("b", 4, 6), P: pos(4, 5)}},
						{NamePos: np("C", 5, 3),
							Type: &parse.TypeOptional{Base: tn("c", 5, 6), P: pos(5, 5)}}},
					P: pos(2, 10)}}}},
		nil},
	{
		"FAILTypeStructNotClosed",
		`package testpkg
type foo struct{
  a b`,
		nil,
		[]string{"testfile:3:6 syntax error"}},
	{
		"FAILTypeStructUnnamedField",
		`package testpkg
type foo struct{a}`,
		nil,
		[]string{"testfile:2:18 syntax error"}},
	{
		"FAILTypeStructUnnamedFieldList",
		`package testpkg
type foo struct{a, b}`,
		nil,
		[]string{"testfile:2:21 syntax error"}},

	// Const definition tests.
	{
		"BoolConst",
		`package testpkg
const foo = true
const bar = false`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cn("true", 2, 13)},
				{NamePos: np("bar", 3, 7), Expr: cn("false", 3, 13)}}},
		nil},
	{
		"StringConst",
		"package testpkg\nconst foo = \"abc\"\nconst bar = `def`",
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cl("abc", 2, 13)},
				{NamePos: np("bar", 3, 7), Expr: cl("def", 3, 13)}}},
		nil},
	{
		"IntegerConst",
		`package testpkg
const foo = 123`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cl(big.NewInt(123), 2, 13)}}},
		nil},
	{
		"FloatConst",
		`package testpkg
const foo = 1.5`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cl(big.NewRat(3, 2), 2, 13)}}},
		nil},
	{
		"NamedConst",
		`package testpkg
const foo = baz
const bar = pkg.box`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cn("baz", 2, 13)},
				{NamePos: np("bar", 3, 7), Expr: cn("pkg.box", 3, 13)}}},
		nil},
	{
		"NamedConstQualified",
		`package testpkg
const foo = bar.baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cn("bar.baz", 2, 13)}}},
		nil},
	{
		"NamedConstQualifiedPath",
		`package testpkg
const foo = "a/b/c/bar".baz`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: cn(`"a/b/c/bar".baz`, 2, 13)}}},
		nil},
	{
		"CompLitConst",
		`package testpkg
const foo = {"a","b"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstCompositeLit{
					KVList: []parse.KVLit{
						{Value: cl("a", 2, 14)},
						{Value: cl("b", 2, 18)}},
					P: pos(2, 13)}}}},
		nil},
	{
		"CompLitKVConst",
		`package testpkg
const foo = {"a":1,"b":2}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstCompositeLit{
					KVList: []parse.KVLit{
						{cl("a", 2, 14), cl(big.NewInt(1), 2, 18)},
						{cl("b", 2, 20), cl(big.NewInt(2), 2, 24)}},
					P: pos(2, 13)}}}},
		nil},
	{
		"CompLitTypedConst",
		`package testpkg
const foo = bar{"a","b"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstCompositeLit{
					Type: tn("bar", 2, 13),
					KVList: []parse.KVLit{
						{Value: cl("a", 2, 17)},
						{Value: cl("b", 2, 21)}},
					P: pos(2, 16)}}}},
		nil},
	{
		"CompLitKVTypedConst",
		`package testpkg
const foo = bar{"a":1,"b":2}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstCompositeLit{
					Type: tn("bar", 2, 13),
					KVList: []parse.KVLit{
						{cl("a", 2, 17), cl(big.NewInt(1), 2, 21)},
						{cl("b", 2, 23), cl(big.NewInt(2), 2, 27)}},
					P: pos(2, 16)}}}},
		nil},
	{
		"UnaryOpConst",
		`package testpkg
const foo = !false
const bar = +1
const baz = -2
const box = ^3`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstUnaryOp{
					Op:   "!",
					Expr: cn("false", 2, 14),
					P:    pos(2, 13)},
				},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstUnaryOp{
					Op:   "+",
					Expr: cl(big.NewInt(1), 3, 14),
					P:    pos(3, 13)},
				},
				{NamePos: np("baz", 4, 7), Expr: &parse.ConstUnaryOp{
					Op:   "-",
					Expr: cl(big.NewInt(2), 4, 14),
					P:    pos(4, 13)},
				},
				{NamePos: np("box", 5, 7), Expr: &parse.ConstUnaryOp{
					Op:   "^",
					Expr: cl(big.NewInt(3), 5, 14),
					P:    pos(5, 13),
				}}}},
		nil},
	{
		"TypeConvConst",
		`package testpkg
const foo = baz(true)
const bar = pkg.box(false)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstTypeConv{
					Type: tn("baz", 2, 13),
					Expr: cn("true", 2, 17),
					P:    pos(2, 13),
				}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstTypeConv{
					Type: tn("pkg.box", 3, 13),
					Expr: cn("false", 3, 21),
					P:    pos(3, 13),
				}}}},
		nil},
	{
		"TypeObjectConst",
		`package testpkg
const foo = typeobject(bool)
const bar = typeobject(pkg.box)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("foo", 2, 7), Expr: &parse.ConstTypeObject{
					Type: tn("bool", 2, 24),
					P:    pos(2, 13),
				}},
				{NamePos: np("bar", 3, 7), Expr: &parse.ConstTypeObject{
					Type: tn("pkg.box", 3, 24),
					P:    pos(3, 13),
				}}}},
		nil},
	{
		"BinaryOpConst",
		`package testpkg
const a = true || false
const b = true && false
const c = 1 < 2
const d = 3 > 4
const e = 5 <= 6
const f = 7 >= 8
const g = 9 != 8
const h = 7 == 6
const i = 5 + 4
const j = 3 - 2
const k = 1 * 2
const l = 3 / 4
const m = 5 % 6
const n = 7 | 8
const o = 9 & 8
const p = 7 ^ 6
const q = 5 << 4
const r = 3 >> 2`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("a", 2, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "||",
						Lexpr: cn("true", 2, 11),
						Rexpr: cn("false", 2, 19),
						P:     pos(2, 16),
					}},
				{NamePos: np("b", 3, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "&&",
						Lexpr: cn("true", 3, 11),
						Rexpr: cn("false", 3, 19),
						P:     pos(3, 16),
					}},
				{NamePos: np("c", 4, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "<",
						Lexpr: cl(big.NewInt(1), 4, 11),
						Rexpr: cl(big.NewInt(2), 4, 15),
						P:     pos(4, 13),
					}},
				{NamePos: np("d", 5, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    ">",
						Lexpr: cl(big.NewInt(3), 5, 11),
						Rexpr: cl(big.NewInt(4), 5, 15),
						P:     pos(5, 13),
					}},
				{NamePos: np("e", 6, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "<=",
						Lexpr: cl(big.NewInt(5), 6, 11),
						Rexpr: cl(big.NewInt(6), 6, 16),
						P:     pos(6, 13),
					}},
				{NamePos: np("f", 7, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    ">=",
						Lexpr: cl(big.NewInt(7), 7, 11),
						Rexpr: cl(big.NewInt(8), 7, 16),
						P:     pos(7, 13),
					}},
				{NamePos: np("g", 8, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "!=",
						Lexpr: cl(big.NewInt(9), 8, 11),
						Rexpr: cl(big.NewInt(8), 8, 16),
						P:     pos(8, 13),
					}},
				{NamePos: np("h", 9, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "==",
						Lexpr: cl(big.NewInt(7), 9, 11),
						Rexpr: cl(big.NewInt(6), 9, 16),
						P:     pos(9, 13),
					}},
				{NamePos: np("i", 10, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "+",
						Lexpr: cl(big.NewInt(5), 10, 11),
						Rexpr: cl(big.NewInt(4), 10, 15),
						P:     pos(10, 13),
					}},
				{NamePos: np("j", 11, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "-",
						Lexpr: cl(big.NewInt(3), 11, 11),
						Rexpr: cl(big.NewInt(2), 11, 15),
						P:     pos(11, 13),
					}},
				{NamePos: np("k", 12, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "*",
						Lexpr: cl(big.NewInt(1), 12, 11),
						Rexpr: cl(big.NewInt(2), 12, 15),
						P:     pos(12, 13),
					}},
				{NamePos: np("l", 13, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "/",
						Lexpr: cl(big.NewInt(3), 13, 11),
						Rexpr: cl(big.NewInt(4), 13, 15),
						P:     pos(13, 13),
					}},
				{NamePos: np("m", 14, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "%",
						Lexpr: cl(big.NewInt(5), 14, 11),
						Rexpr: cl(big.NewInt(6), 14, 15),
						P:     pos(14, 13),
					}},
				{NamePos: np("n", 15, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "|",
						Lexpr: cl(big.NewInt(7), 15, 11),
						Rexpr: cl(big.NewInt(8), 15, 15),
						P:     pos(15, 13),
					}},
				{NamePos: np("o", 16, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "&",
						Lexpr: cl(big.NewInt(9), 16, 11),
						Rexpr: cl(big.NewInt(8), 16, 15),
						P:     pos(16, 13),
					}},
				{NamePos: np("p", 17, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "^",
						Lexpr: cl(big.NewInt(7), 17, 11),
						Rexpr: cl(big.NewInt(6), 17, 15),
						P:     pos(17, 13),
					}},
				{NamePos: np("q", 18, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    "<<",
						Lexpr: cl(big.NewInt(5), 18, 11),
						Rexpr: cl(big.NewInt(4), 18, 16),
						P:     pos(18, 13),
					}},
				{NamePos: np("r", 19, 7),
					Expr: &parse.ConstBinaryOp{
						Op:    ">>",
						Lexpr: cl(big.NewInt(3), 19, 11),
						Rexpr: cl(big.NewInt(2), 19, 16),
						P:     pos(19, 13),
					}}}},
		nil},
	{
		"FAILConstOnlyName",
		`package testpkg
const foo`,
		nil,
		[]string{"testfile:2:10 syntax error"}},
	{
		"FAILConstNoEquals",
		`package testpkg
const foo bar`,
		nil,
		[]string{"testfile:2:11 syntax error"}},
	{
		"FAILConstNoValue",
		`package testpkg
const foo =`,
		nil,
		[]string{"testfile:2:12 syntax error"}},

	// Error definition tests.
	{
		"ErrorEmpty",
		`package testpkg
error()`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9)},
		nil},
	{
		"ErrorDefNoParamsNoDetails1",
		`package testpkg
error ErrFoo()`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{NamePos: np("ErrFoo", 2, 7)}}},
		nil},
	{
		"ErrorDefNoParamsNoDetails2",
		`package testpkg
error ErrFoo() {}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{NamePos: np("ErrFoo", 2, 7)}}},
		nil},
	{
		"ErrorDefNoParamsWithDetails1",
		`package testpkg
error ErrFoo() {NoRetry}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Actions: []parse.StringPos{sp("NoRetry", 2, 17)}}}},
		nil},
	{
		"ErrorDefNoParamsWithDetails2",
		`package testpkg
error ErrFoo() {"en":"a"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Formats: []parse.LangFmt{lf(sp("en", 2, 17), sp("a", 2, 22))}}}},
		nil},
	{
		"ErrorDefNoParamsWithDetails3",
		`package testpkg
error ErrFoo() {NoRetry, "en":"a", "zh":"b"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Actions: []parse.StringPos{sp("NoRetry", 2, 17)},
				Formats: []parse.LangFmt{
					lf(sp("en", 2, 26), sp("a", 2, 31)),
					lf(sp("zh", 2, 36), sp("b", 2, 41)),
				}}}},
		nil},
	{
		"ErrorDefWithParamsNoDetails1",
		`package testpkg
error ErrFoo(x int, y bool)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Params: []*parse.Field{
					{NamePos: np("x", 2, 14), Type: tn("int", 2, 16)},
					{NamePos: np("y", 2, 21), Type: tn("bool", 2, 23)}}}}},
		nil},
	{
		"ErrorDefWithParamsNoDetails2",
		`package testpkg
error ErrFoo(x int, y bool) {}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Params: []*parse.Field{
					{NamePos: np("x", 2, 14), Type: tn("int", 2, 16)},
					{NamePos: np("y", 2, 21), Type: tn("bool", 2, 23)}}}}},
		nil},
	{
		"ErrorDefWithParamsWithDetails1",
		`package testpkg
error ErrFoo(x int, y bool) {NoRetry}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Params: []*parse.Field{
					{NamePos: np("x", 2, 14), Type: tn("int", 2, 16)},
					{NamePos: np("y", 2, 21), Type: tn("bool", 2, 23)}},
				Actions: []parse.StringPos{sp("NoRetry", 2, 30)}}}},
		nil},
	{
		"ErrorDefWithParamsWithDetails2",
		`package testpkg
error ErrFoo(x int, y bool) {"en":"a"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Params: []*parse.Field{
					{NamePos: np("x", 2, 14), Type: tn("int", 2, 16)},
					{NamePos: np("y", 2, 21), Type: tn("bool", 2, 23)}},
				Formats: []parse.LangFmt{lf(sp("en", 2, 30), sp("a", 2, 35))}}}},
		nil},
	{
		"ErrorDefWithParamsWithDetails3",
		`package testpkg
error ErrFoo(x int, y bool) {NoRetry, "en":"a", "zh":"b"}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{{
				NamePos: np("ErrFoo", 2, 7),
				Params: []*parse.Field{
					{NamePos: np("x", 2, 14), Type: tn("int", 2, 16)},
					{NamePos: np("y", 2, 21), Type: tn("bool", 2, 23)}},
				Actions: []parse.StringPos{sp("NoRetry", 2, 30)},
				Formats: []parse.LangFmt{
					lf(sp("en", 2, 39), sp("a", 2, 44)),
					lf(sp("zh", 2, 49), sp("b", 2, 54)),
				}}}},
		nil},
	{
		"ErrorDefMulti",
		`package testpkg
error (
  ErrFoo()
  ErrBar() {NoRetry, "en":"a", "zh":"b"}
  ErrBaz(x int, y bool) {NoRetry, "en":"a", "zh":"b"}
)`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			ErrorDefs: []*parse.ErrorDef{
				{
					NamePos: np("ErrFoo", 3, 3),
				},
				{
					NamePos: np("ErrBar", 4, 3),
					Actions: []parse.StringPos{sp("NoRetry", 4, 13)},
					Formats: []parse.LangFmt{
						lf(sp("en", 4, 22), sp("a", 4, 27)),
						lf(sp("zh", 4, 32), sp("b", 4, 37)),
					},
				},
				{
					NamePos: np("ErrBaz", 5, 3),
					Params: []*parse.Field{
						{NamePos: np("x", 5, 10), Type: tn("int", 5, 12)},
						{NamePos: np("y", 5, 17), Type: tn("bool", 5, 19)}},
					Actions: []parse.StringPos{sp("NoRetry", 5, 26)},
					Formats: []parse.LangFmt{
						lf(sp("en", 5, 35), sp("a", 5, 40)),
						lf(sp("zh", 5, 45), sp("b", 5, 50)),
					},
				},
			}},
		nil},

	// Interface tests.
	{
		"InterfaceEmpty",
		`package testpkg
type foo interface{}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6)}}},
		nil},
	{
		"InterfaceOneMethodOneInUnnamedOut",
		`package testpkg
type foo interface{meth1(a b) (c | error)}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20),
					InArgs:  []*parse.Field{{NamePos: np("a", 2, 26), Type: tn("b", 2, 28)}},
					OutArgs: []*parse.Field{{NamePos: np("", 2, 32), Type: tn("c", 2, 32)}}}}}}},
		nil},
	{
		"InterfaceErrors",
		`package testpkg
type foo interface{meth1(err error) error}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{{NamePos: np("meth1", 2, 20),
					InArgs: []*parse.Field{{NamePos: np("err", 2, 26), Type: tn("error", 2, 30)}}}}}}},
		nil},
	{
		"InterfaceMixedMethods",
		`package testpkg
type foo interface{
  meth1(a b) (c | error);meth2() error
  meth3(e f, g, h i) (j k, l, m n | error)
}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Methods: []*parse.Method{
					{NamePos: np("meth1", 3, 3),
						InArgs:  []*parse.Field{{NamePos: np("a", 3, 9), Type: tn("b", 3, 11)}},
						OutArgs: []*parse.Field{{NamePos: np("", 3, 15), Type: tn("c", 3, 15)}}},
					{NamePos: np("meth2", 3, 26)},
					{NamePos: np("meth3", 4, 3),
						InArgs: []*parse.Field{
							{NamePos: np("e", 4, 9), Type: tn("f", 4, 11)},
							{NamePos: np("g", 4, 14), Type: tn("i", 4, 19)},
							{NamePos: np("h", 4, 17), Type: tn("i", 4, 19)}},
						OutArgs: []*parse.Field{
							{NamePos: np("j", 4, 23), Type: tn("k", 4, 25)},
							{NamePos: np("l", 4, 28), Type: tn("n", 4, 33)},
							{NamePos: np("m", 4, 31), Type: tn("n", 4, 33)}}}}}}},
		nil},
	{
		"InterfaceEmbed",
		`package testpkg
type foo interface{bar}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Embeds: []*parse.NamePos{npptr("bar", 2, 20)}}}},
		nil},
	{
		"InterfaceEmbedQualified",
		`package testpkg
type foo interface{bar.baz}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Embeds: []*parse.NamePos{npptr("bar.baz", 2, 20)}}}},
		nil},
	{
		"InterfaceEmbedQualifiedPath",
		`package testpkg
type foo interface{"a/b/c/bar".baz}`,
		&parse.File{BaseName: "testfile", PackageDef: np("testpkg", 1, 9),
			Interfaces: []*parse.Interface{{NamePos: np("foo", 2, 6),
				Embeds: []*parse.NamePos{npptr(`"a/b/c/bar".baz`, 2, 20)}}}},
		nil},
	{
		"FAILInterfaceUnclosedInterface",
		`package testpkg
type foo interface{
  meth1()`,
		nil,
		[]string{"testfile:3:10 syntax error"}},
	{
		"FAILInterfaceUnclosedArgs",
		`package testpkg
type foo interface{
  meth1(
}`,
		nil,
		[]string{"testfile:4:1 syntax error"}},
	{
		"FAILInterfaceVariableNames",
		`package testpkg
type foo interface{
  meth1([]a, []b []c)
}`,
		nil,
		[]string{"expected one or more variable names",
			"testfile:3:18 perhaps you forgot a comma"}},
}

// configTests contains tests of config files.
var configTests = []configTest{
	// Empty file isn't allowed (need at least a package clause).
	{
		"FAILEmptyFile",
		"",
		nil,
		[]string{"config file must start with config clause"}},

	// Comment tests.
	{
		"ConfigDocOneLiner",
		`// One liner
// Another line
config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: parse.NamePos{Name: "config", Pos: pos(3, 1), Doc: `// One liner
// Another line
`},
			Config: cn("true", 3, 10)},
		nil},
	{
		"ConfigDocMultiLiner",
		`/* Multi liner
Another line
*/
config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: parse.NamePos{Name: "config", Pos: pos(4, 1), Doc: `/* Multi liner
Another line
*/
`},
			Config: cn("true", 4, 10)},
		nil},
	{
		"FileDocNoConfigDoc",
		`// File doc, has extra newline so not config doc

config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 3, 1),
			Doc:    "// File doc, has extra newline so not config doc\n",
			Config: cn("true", 3, 10)},
		nil},
	{
		"FileDocAndConfigDoc",
		`// File doc

// Config doc
config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: parse.NamePos{Name: "config", Pos: pos(4, 1), Doc: "// Config doc\n"},
			Doc:    "// File doc\n",
			Config: cn("true", 4, 10)},
		nil},
	{
		"FAILUnterminatedComment",
		`/* Unterminated
Another line
config = true`,
		nil,
		[]string{"comment not terminated"}},

	// Config tests.
	{
		"Config",
		"config = true;",
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("true", 1, 10)},
		nil},
	{
		"ConfigNoSemi",
		"config = true",
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("true", 1, 10)},
		nil},
	{
		"ConfigNamedConfig",
		"config = config",
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("config", 1, 10)},
		nil},
	{
		"FAILConfigNoEqual",
		"config true",
		nil,
		[]string{"testfile:1:8 syntax error"}},

	// Import tests.
	{
		"EmptyImport",
		`config = foo
import (
)`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("foo", 1, 10)},
		nil},
	{
		"OneImport",
		`config = foo
import "foo/bar";`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 2, 8)}},
			Config:  cn("foo", 1, 10)},
		nil},
	{
		"OneImportLocalNameNoSemi",
		`config = foo
import baz "foo/bar"`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("baz", 2, 8)}},
			Config:  cn("foo", 1, 10)},
		nil},
	{
		"OneImportParens",
		`config = foo
import (
  "foo/bar";
)`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}},
			Config:  cn("foo", 1, 10)},
		nil},
	{
		"OneImportParensNoSemi",
		`config = foo
import (
  "foo/bar"
)`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("", 3, 3)}},
			Config:  cn("foo", 1, 10)},
		nil},
	{
		"OneImportParensNamed",
		`config = foo
import (
  baz "foo/bar"
)`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo/bar", NamePos: np("baz", 3, 3)}},
			Config:  cn("foo", 1, 10)},
		nil},
	{
		"MixedImports",
		`config = foo
import "foo/bar"
import (
  "baz";"a/b"
  "c/d"
)
import "z"`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{
				{Path: "foo/bar", NamePos: np("", 2, 8)},
				{Path: "baz", NamePos: np("", 4, 3)},
				{Path: "a/b", NamePos: np("", 4, 9)},
				{Path: "c/d", NamePos: np("", 5, 3)},
				{Path: "z", NamePos: np("", 7, 8)}},
			Config: cn("foo", 1, 10)},
		nil},
	{
		"FAILImportParensNotClosed",
		`config = foo
import (
  "foo/bar"`,
		nil,
		[]string{"testfile:3:12 syntax error"}},

	// Inline config tests.
	{
		"BoolConst",
		`config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("true", 1, 10)},
		nil},
	{
		"StringConst",
		`config = "abc"`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cl("abc", 1, 10)},
		nil},
	{
		"IntegerConst",
		`config = 123`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cl(big.NewInt(123), 1, 10)},
		nil},
	{
		"FloatConst",
		`config = 1.5`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cl(big.NewRat(3, 2), 1, 10)},
		nil},
	{
		"NamedConst",
		`config = pkg.foo`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: cn("pkg.foo", 1, 10)},
		nil},
	{
		"CompLitConst",
		`config = {"a","b"}`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: &parse.ConstCompositeLit{
				KVList: []parse.KVLit{
					{Value: cl("a", 1, 11)},
					{Value: cl("b", 1, 15)}},
				P: pos(1, 10)}},
		nil},
	{
		"CompLitKVConst",
		`config = {"a":1,"b":2}`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: &parse.ConstCompositeLit{
				KVList: []parse.KVLit{
					{cl("a", 1, 11), cl(big.NewInt(1), 1, 15)},
					{cl("b", 1, 17), cl(big.NewInt(2), 1, 21)}},
				P: pos(1, 10)}},
		nil},
	{
		"CompLitTypedConst",
		`config = foo{"a","b"}`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: &parse.ConstCompositeLit{
				Type: tn("foo", 1, 10),
				KVList: []parse.KVLit{
					{Value: cl("a", 1, 14)},
					{Value: cl("b", 1, 18)}},
				P: pos(1, 13)}},
		nil},
	{
		"CompLitKVTypedConst",
		`config = foo{"a":1,"b":2}`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Config: &parse.ConstCompositeLit{
				Type: tn("foo", 1, 10),
				KVList: []parse.KVLit{
					{cl("a", 1, 14), cl(big.NewInt(1), 1, 18)},
					{cl("b", 1, 20), cl(big.NewInt(2), 1, 24)}},
				P: pos(1, 13)}},
		nil},
	{
		"FAILConstNoEquals",
		`config 123`,
		nil,
		[]string{"testfile:1:8 syntax error"}},
	{
		"FAILConstNoValue",
		`config =`,
		nil,
		[]string{"testfile:1:9 syntax error"}},

	// Out-of-line config tests.
	{
		"BoolOutOfLineConfig",
		`config = config
import "foo"
const config = true`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo", NamePos: np("", 2, 8)}},
			Config:  cn("config", 1, 10),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("config", 3, 7), Expr: cn("true", 3, 16)}}},
		nil},
	{
		"BoolOutOfLineBar",
		`config = bar
import "foo"
const bar = true`,
		&parse.Config{FileName: "testfile", ConfigDef: np("config", 1, 1),
			Imports: []*parse.Import{{Path: "foo", NamePos: np("", 2, 8)}},
			Config:  cn("bar", 1, 10),
			ConstDefs: []*parse.ConstDef{
				{NamePos: np("bar", 3, 7), Expr: cn("true", 3, 13)}}},
		nil},

	// Errors, types and interfaces return error
	{
		"FAILError",
		`config = true
error foo()`,
		nil,
		[]string{"config files may not contain error, type or interface definitions"}},
	{
		"FAILType",
		`config = true
type foo bool`,
		nil,
		[]string{"config files may not contain error, type or interface definitions"}},
	{
		"FAILInterface",
		`config = true
type foo interface{}`,
		nil,
		[]string{"config files may not contain error, type or interface definitions"}},
}

func configImports(imports ...string) *parse.Config {
	config := new(parse.Config)
	for _, i := range imports {
		config.Imports = append(config.Imports, &parse.Import{Path: i})
	}
	return config
}

func TestConfigHasImport(t *testing.T) {
	config := configImports("a", "b/c")
	tests := []struct {
		Path string
		Want bool
	}{
		{"a", true},
		{"b/c", true},
		{"b", false},
		{"c", false},
		{"d", false},
	}
	for _, test := range tests {
		if got, want := config.HasImport(test.Path), test.Want; got != want {
			t.Errorf("HasImport(%q) got %v, want %v", test.Path, got, want)
		}
	}
}

func TestConfigAddImports(t *testing.T) {
	tests := []struct {
		Base    *parse.Config
		Imports []string
		Want    *parse.Config
	}{
		{configImports(), []string{"a", "b/c"}, configImports("a", "b/c")},
		{configImports("a"), []string{"a", "b/c"}, configImports("a", "b/c")},
		{configImports("a", "b/c"), []string{"a", "b/c"}, configImports("a", "b/c")},
		{configImports("a", "b/c"), []string{"a", "b/c", "d"}, configImports("a", "b/c", "d")},
	}
	for _, test := range tests {
		test.Base.AddImports(test.Imports...)
		if got, want := test.Base, test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("AddImports(%q) got %v, want %v", test.Imports, got, want)
		}
	}
}

func TestParseExprs(t *testing.T) {
	tests := []struct {
		Data  string
		Exprs []parse.ConstExpr
		Err   string
	}{
		{``, nil, "syntax error"},
		{`true`, []parse.ConstExpr{cn("true", 1, 1)}, ""},
		{`false`, []parse.ConstExpr{cn("false", 1, 1)}, ""},
		{`abc`, []parse.ConstExpr{cn("abc", 1, 1)}, ""},
		{`"a/b/c".abc`, []parse.ConstExpr{cn(`"a/b/c".abc`, 1, 1)}, ""},
		{`"abc"`, []parse.ConstExpr{cl("abc", 1, 1)}, ""},
		{`1`, []parse.ConstExpr{cl(big.NewInt(1), 1, 1)}, ""},
		{`123`, []parse.ConstExpr{cl(big.NewInt(123), 1, 1)}, ""},
		{`1.0`, []parse.ConstExpr{cl(big.NewRat(1, 1), 1, 1)}, ""},
		{`1.5`, []parse.ConstExpr{cl(big.NewRat(3, 2), 1, 1)}, ""},
		{`{1,2}`, []parse.ConstExpr{
			&parse.ConstCompositeLit{
				KVList: []parse.KVLit{
					{Value: cl(big.NewInt(1), 1, 2)},
					{Value: cl(big.NewInt(2), 1, 4)},
				},
				P: pos(1, 1),
			},
		}, ""},
		{`1+2`, []parse.ConstExpr{
			&parse.ConstBinaryOp{"+",
				cl(big.NewInt(1), 1, 1),
				cl(big.NewInt(2), 1, 3),
				pos(1, 2),
			},
		}, ""},
		{`1,"abc"`, []parse.ConstExpr{
			cl(big.NewInt(1), 1, 1),
			cl("abc", 1, 3),
		}, ""},
	}
	for _, test := range tests {
		errs := vdlutil.NewErrors(-1)
		exprs := parse.ParseExprs(test.Data, errs)
		vdltestutil.ExpectResult(t, errs, test.Data, test.Err)
		if got, want := exprs, test.Exprs; !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %v, want %v", test.Data, got, want)
		}
	}
}
