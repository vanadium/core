// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile_test

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/internal/vdltestutil"
)

const qual = "package path qualified identifier"

func testType(t *testing.T, test typeTest, qualifiedPaths bool) {
	env := compile.NewEnv(-1)
	if !qualifiedPaths {
		env.DisallowPathQualifiers()
		test.Name = "NoQual" + test.Name
	}
	for _, tpkg := range test.Pkgs {
		// Compile the package with a single file, and adding the "package foo"
		// prefix to the source data automatically.
		files := map[string]string{
			tpkg.Name + ".vdl": "package " + tpkg.Name + "\n" + tpkg.Data,
		}
		pkgPath := "p.kg/" + tpkg.Name // use dots in pkgpath to test tricky cases
		buildPkg := vdltestutil.FakeBuildPackage(tpkg.Name, pkgPath, files)
		pkg := build.BuildPackage(buildPkg, env)
		if tpkg.ErrRE == qual {
			if qualifiedPaths {
				tpkg.ErrRE = "" // the test should pass if running with qualified paths.
			} else {
				tpkg.ExpectBase = nil // otherwise the test should fail
			}
		}
		vdltestutil.ExpectResult(t, env.Errors, test.Name, tpkg.ErrRE)
		if pkg == nil || tpkg.ErrRE != "" {
			continue
		}
		matchTypeRes(t, test.Name, tpkg, pkg.Files[0].TypeDefs)
	}
}

func TestType(t *testing.T) {
	// Run all tests in both regular and qualfiedPaths mode
	for _, test := range typeTests {
		testType(t, test, false)
	}
	for _, test := range typeTests {
		testType(t, test, true)
	}
}

func matchTypeRes(t *testing.T, tname string, tpkg typePkg, tdefs []*compile.TypeDef) {
	if tpkg.ExpectBase == nil {
		return
	}
	// Look for a TypeDef called "Res" to compare our expected results.
	for _, tdef := range tdefs {
		if tdef.Name == "Res" {
			base := tpkg.ExpectBase
			resname := "p.kg/" + tpkg.Name + ".Res"
			res := vdl.NamedType(resname, base)
			if got, want := tdef.Type, res; got != want {
				t.Errorf("%s type got %s, want %s", tname, got, want)
			}
			if got, want := tdef.BaseType, base; got != want {
				t.Errorf("%s base type got %s, want %s", tname, got, want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, tpkg.Name)
}

func namedX(base *vdl.Type) *vdl.Type   { return vdl.NamedType("p.kg/a.X", base) }
func namedRes(base *vdl.Type) *vdl.Type { return vdl.NamedType("p.kg/a.Res", base) }

var byteListType = vdl.ListType(vdl.ByteType)
var byteArrayType = vdl.ArrayType(4, vdl.ByteType)

type typePkg struct {
	Name       string
	Data       string
	ExpectBase *vdl.Type
	ErrRE      string
}

type tp []typePkg

type typeTest struct {
	Name string
	Pkgs tp
}

var typeTests = []typeTest{
	// Test named built-ins.
	{"Bool", tp{{"a", `type Res bool`, vdl.BoolType, ""}}},
	{"Byte", tp{{"a", `type Res byte`, vdl.ByteType, ""}}},
	{"Uint16", tp{{"a", `type Res uint16`, vdl.Uint16Type, ""}}},
	{"Uint32", tp{{"a", `type Res uint32`, vdl.Uint32Type, ""}}},
	{"Uint64", tp{{"a", `type Res uint64`, vdl.Uint64Type, ""}}},
	{"Int16", tp{{"a", `type Res int16`, vdl.Int16Type, ""}}},
	{"Int32", tp{{"a", `type Res int32`, vdl.Int32Type, ""}}},
	{"Int64", tp{{"a", `type Res int64`, vdl.Int64Type, ""}}},
	{"Float32", tp{{"a", `type Res float32`, vdl.Float32Type, ""}}},
	{"Float64", tp{{"a", `type Res float64`, vdl.Float64Type, ""}}},
	{"String", tp{{"a", `type Res string`, vdl.StringType, ""}}},
	{"ByteList", tp{{"a", `type Res []byte`, byteListType, ""}}},
	{"ByteArray", tp{{"a", `type Res [4]byte`, byteArrayType, ""}}},
	{"Typeobject", tp{{"a", `type Res typeobject`, nil, "any and typeobject cannot be renamed"}}},
	{"Any", tp{{"a", `type Res any`, nil, "any and typeobject cannot be renamed"}}},
	{"Error", tp{{"a", `type Res error`, nil, "error cannot be renamed"}}},

	// Test composite vdl.
	{"Enum", tp{{"a", `type Res enum{A;B;C}`, vdl.EnumType("A", "B", "C"), ""}}},
	{"Array", tp{{"a", `type Res [2]bool`, vdl.ArrayType(2, vdl.BoolType), ""}}},
	{"List", tp{{"a", `type Res []int32`, vdl.ListType(vdl.Int32Type), ""}}},
	{"Set", tp{{"a", `type Res set[int32]`, vdl.SetType(vdl.Int32Type), ""}}},
	{"Map", tp{{"a", `type Res map[int32]string`, vdl.MapType(vdl.Int32Type, vdl.StringType), ""}}},
	{"Struct", tp{{"a", `type Res struct{A int32;B string}`, vdl.StructType([]vdl.Field{{Name: "A", Type: vdl.Int32Type}, {Name: "B", Type: vdl.StringType}}...), ""}}},
	{"Union", tp{{"a", `type Res union{A bool;B int32;C string}`, vdl.UnionType([]vdl.Field{{Name: "A", Type: vdl.BoolType}, {Name: "B", Type: vdl.Int32Type}, {Name: "C", Type: vdl.StringType}}...), ""}}},
	{"Optional", tp{{"a", `type Res []?X;type X struct{A bool}`, vdl.ListType(vdl.OptionalType(namedX(vdl.StructType(vdl.Field{
		Name: "A",
		Type: vdl.BoolType,
	})))), ""}}},

	// Test named types based on named types.
	{"NBool", tp{{"a", `type Res X;type X bool`, namedX(vdl.BoolType), ""}}},
	{"NByte", tp{{"a", `type Res X;type X byte`, namedX(vdl.ByteType), ""}}},
	{"NUint16", tp{{"a", `type Res X;type X uint16`, namedX(vdl.Uint16Type), ""}}},
	{"NUint32", tp{{"a", `type Res X;type X uint32`, namedX(vdl.Uint32Type), ""}}},
	{"NUint64", tp{{"a", `type Res X;type X uint64`, namedX(vdl.Uint64Type), ""}}},
	{"NInt16", tp{{"a", `type Res X;type X int16`, namedX(vdl.Int16Type), ""}}},
	{"NInt32", tp{{"a", `type Res X;type X int32`, namedX(vdl.Int32Type), ""}}},
	{"NInt64", tp{{"a", `type Res X;type X int64`, namedX(vdl.Int64Type), ""}}},
	{"NFloat32", tp{{"a", `type Res X;type X float32`, namedX(vdl.Float32Type), ""}}},
	{"NFloat64", tp{{"a", `type Res X;type X float64`, namedX(vdl.Float64Type), ""}}},
	{"NString", tp{{"a", `type Res X;type X string`, namedX(vdl.StringType), ""}}},
	{"NByteList", tp{{"a", `type Res X;type X []byte`, namedX(byteListType), ""}}},
	{"NByteArray", tp{{"a", `type Res X;type X [4]byte`, namedX(byteArrayType), ""}}},
	{"NEnum", tp{{"a", `type Res X;type X enum{A;B;C}`, namedX(vdl.EnumType("A", "B", "C")), ""}}},
	{"NArray", tp{{"a", `type Res X;type X [2]bool`, namedX(vdl.ArrayType(2, vdl.BoolType)), ""}}},
	{"NList", tp{{"a", `type Res X;type X []int32`, namedX(vdl.ListType(vdl.Int32Type)), ""}}},
	{"NSet", tp{{"a", `type Res X;type X set[int32]`, namedX(vdl.SetType(vdl.Int32Type)), ""}}},
	{"NMap", tp{{"a", `type Res X;type X map[int32]string`, namedX(vdl.MapType(vdl.Int32Type, vdl.StringType)), ""}}},
	{"NStruct", tp{{"a", `type Res X;type X struct{A int32;B string}`, namedX(vdl.StructType([]vdl.Field{{Name: "A", Type: vdl.Int32Type}, {Name: "B", Type: vdl.StringType}}...)), ""}}},
	{"NUnion", tp{{"a", `type Res X;type X union{A bool;B int32;C string}`, namedX(vdl.UnionType([]vdl.Field{{Name: "A", Type: vdl.BoolType}, {Name: "B", Type: vdl.Int32Type}, {Name: "C", Type: vdl.StringType}}...)), ""}}},

	// Test multi-package types
	{"MultiPkgSameTypeName", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `type Res bool`, vdl.BoolType, ""}}},
	{"MultiPkgDep", tp{
		{"a", `type Res X;type X bool`, namedX(vdl.BoolType), ""},
		{"b", `import "p.kg/a";type Res []a.Res`, vdl.ListType(namedRes(vdl.BoolType)), ""}}},
	{"MultiPkgDepQualifiedPath", tp{
		{"a", `type Res X;type X bool`, namedX(vdl.BoolType), ""},
		{"b", `import "p.kg/a";type Res []"p.kg/a".Res`, vdl.ListType(namedRes(vdl.BoolType)), qual}}},
	{"MultiPkgUnexportedType", tp{
		{"a", `type Res X;type X bool`, namedX(vdl.BoolType), ""},
		{"b", `import "p.kg/a";type Res []a.x`, nil, "type a.x undefined"}}},
	{"MultiPkgSamePkgName", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"a", `type Res bool`, nil, "invalid recompile"}}},
	{"MultiPkgUnimportedPkg", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `type Res []a.Res`, nil, "type a.Res undefined"}}},
	{"RedefinitionOfImportedName", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `import "p.kg/a"; type a string; type Res a`, nil, "type a name conflict"}}},

	// Test errors.
	{"InvalidName", tp{{"a", `type _Res bool`, nil, "type _Res invalid name"}}},
	{"Undefined", tp{{"a", `type Res foo`, nil, "type foo undefined"}}},
	{"UnnamedArray", tp{{"a", `type Res [][3]int64`, nil, "unnamed array type invalid"}}},
	{"UnnamedEnum", tp{{"a", `type Res []enum{A;B;C}`, nil, "unnamed enum type invalid"}}},
	{"UnnamedStruct", tp{{"a", `type Res []struct{A int32}`, nil, "unnamed struct type invalid"}}},
	{"UnnamedUnion", tp{{"a", `type Res []union{A bool;B int32;C string}`, nil, "unnamed union type invalid"}}},
	{"TopLevelOptional", tp{{"a", `type Res ?bool`, nil, "can't define type based on top-level optional"}}},
	{"MultiPkgUnmatchedType", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `import "p.kg/a";type Res a.Res.foobar`, nil, `\(\.foobar unmatched\)`}}},
	{"UnterminatedPath1", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `import "p.kg/a";type Res "a.Res`, nil, "syntax error"}}},
	{"UnterminatedPath2", tp{
		{"a", `type Res bool`, vdl.BoolType, ""},
		{"b", `import "p.kg/a";type Res a".Res`, nil, "syntax error"}}},
	{"ZeroLengthArray", tp{{"a", `type Res [0]int32`, nil, "negative or zero array length"}}},
	{"InvalidOptionalFollowedByValidType", tp{{"a", `type Res struct{X ?int32};type x string`, nil, "invalid optional type"}}},
	{"NoTransitiveExport", tp{{"a", `type t bool; type Res t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportList", tp{{"a", `type t bool; type Res []t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportSet", tp{{"a", `type t bool; type Res set[t]`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportMapKey", tp{{"a", `type t bool; type Res map[t]string`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportMapElem", tp{{"a", `type t bool; type Res map[string]t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportArray", tp{{"a", `type t [2]bool; type Res t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportArrayElem", tp{{"a", `type t bool; type X [2]t; type Res X`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportStruct", tp{{"a", `type t struct{}; type Res t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportStructField", tp{{"a", `type t bool; type X struct{F t}; type Res X`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportUnion", tp{{"a", `type t union{F bool}; type Res t`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportUnionField", tp{{"a", `type t bool; type X union{F t}; type Res X`, nil, "must be transitively exported"}}},
	{"NoTransitiveExportOptional", tp{{"a", `type t bool; type X struct{F ?t}; type Res X`, nil, "must be transitively exported"}}},
}
