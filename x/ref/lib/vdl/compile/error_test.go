// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile_test

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/internal/vdltestutil"
	"v.io/x/ref/lib/vdl/parse"
)

func TestError(t *testing.T) {
	for _, test := range errorTests {
		testError(t, test)
	}
}

func testError(t *testing.T, test errorTest) {
	env := compile.NewEnv(-1)
	for _, epkg := range test.Pkgs {
		// Compile the package with a single file, and adding the "package foo"
		// prefix to the source data automatically.
		files := map[string]string{
			epkg.Name + ".vdl": "package " + epkg.Name + "\n" + epkg.Data,
		}
		buildPkg := vdltestutil.FakeBuildPackage(epkg.Name, epkg.Name, files)
		pkg := build.BuildPackage(buildPkg, env)
		vdltestutil.ExpectResult(t, env.Errors, test.Name, epkg.ErrRE)
		if pkg == nil || epkg.ErrRE != "" {
			continue
		}
		vdltestutil.ExpectResult(t, env.Warnings, test.Name, epkg.WarningRE)
		if pkg == nil || epkg.ErrRE != "" {
			continue
		}
		matchErrorRes(t, test.Name, epkg, pkg.Files[0].ErrorDefs)
	}
}

func matchErrorRes(t *testing.T, tname string, epkg errorPkg, edefs []*compile.ErrorDef) {
	// Look for an ErrorDef called "Res" to compare our expected results.
	for _, edef := range edefs {
		if edef.ID == epkg.Name+".Res" {
			got, want := cleanErrorDef(*edef), cleanErrorDef(epkg.Want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s got %+v, want %+v", tname, got, want)
			}
			return
		}
	}
	t.Errorf("%s couldn't find Res in package %s", tname, epkg.Name)
}

// cleanErrorDef resets fields that we don't care about testing.
func cleanErrorDef(ed compile.ErrorDef) compile.ErrorDef {
	ed.NamePos = compile.NamePos{}
	ed.Exported = false
	ed.ID = ""
	ed.Name = ""
	for _, param := range ed.Params {
		param.Pos = parse.Pos{}
	}
	ed.File = nil
	return ed
}

type errorPkg struct {
	Name      string
	Data      string
	Want      compile.ErrorDef
	ErrRE     string
	WarningRE string
}

type ep []errorPkg

type errorTest struct {
	Name string
	Pkgs ep
}

func arg(name string, t *vdl.Type) *compile.Field {
	arg := new(compile.Field)
	arg.Name = name
	arg.Type = t
	return arg
}

const pre = "{1:}{2:} "

var errorTests = []errorTest{
	{"NoParams1", ep{{"a", `error Res() {}`,
		compile.ErrorDef{},
		"",
		"",
	}}},
	{"NoParamsRetryConnection", ep{{"a", `error Res() {RetryConnection}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryConnection,
		},
		"",
		"",
	}}},
	{"NoParamsRetryRefetch", ep{{"a", `error Res() {RetryRefetch}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryRefetch,
		},
		"",
		"",
	}}},
	{"NoParamsRetryBackoff", ep{{"a", `error Res() {RetryBackoff}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryBackoff,
		},
		"",
		"",
	}}},
	{"NoParamsMulti", ep{{"a", `error Res() {RetryRefetch}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryRefetch,
		},
		"",
		"",
	}}},

	{"WithParams1", ep{{"a", `error Res(x string, y int32) {}`,
		compile.ErrorDef{
			Params: []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
		},
		"",
		"",
	}}},
	{"WithParamsNoRetry", ep{{"a", `error Res(x string, y int32) {NoRetry}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeNoRetry,
		},
		"",
		"",
	}}},
	{"WithParamsRetryConnection", ep{{"a", `error Res(x string, y int32) {RetryConnection}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryConnection,
		},
		"",
		"",
	}}},
	{"WithParamsRetryRefetch", ep{{"a", `error Res(x string, y int32) {RetryRefetch}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryRefetch,
		},
		"",
		"",
	}}},
	{"WithParamsRetryBackoff", ep{{"a", `error Res(x string, y int32) {RetryBackoff}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryBackoff,
		},
		"",
		"",
	}}},
	{"WithSamePackageParam", ep{{"a", `error Res(x Bool) {};type Bool bool`,
		compile.ErrorDef{
			Params: []*compile.Field{arg("x", vdl.NamedType("a.Bool", vdl.BoolType))},
		},
		"",
		"",
	}}},

	// Test multi-package errors.
	{"MultiPkgSameErrorName", ep{
		{
			"a", `error Res() {}`,
			compile.ErrorDef{},
			"",
			"",
		},
	}},
	{"MultiPkgTypeDep", ep{
		{
			"a", `error Res();type Bool bool`,
			compile.ErrorDef{},
			"",
			"",
		},
		{
			"b", `import "a";error Res(x a.Bool) {}`,
			compile.ErrorDef{
				Params: []*compile.Field{arg("x", vdl.NamedType("a.Bool", vdl.BoolType))},
			},
			"",
			"",
		},
	}},
	{"RedefinitionOfImportName", ep{
		{
			"a", `error Res() {}`,
			compile.ErrorDef{},
			"",
			"",
		},
		{
			"b", `import "a";error a() {}`, compile.ErrorDef{},
			"error a name conflict",
			"",
		},
	}},

	// Test errors.
	{"NoParamsNoLangFmt1", ep{{"a", `error Res()`, compile.ErrorDef{}, "", ""}}},
	{"NoParamsNoLangFmt2", ep{{"a", `error Res() {}`, compile.ErrorDef{}, "", ""}}},
	{"NoParamsNoLangFmt3", ep{{"a", `error Res() {NoRetry}`, compile.ErrorDef{}, "", ""}}},

	{"WithParamsNoLangFmt1", ep{{"a", `error Res(x string, y int32)`, compile.ErrorDef{
		Params: []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
	}, "", ""}}},
	{"WithParamsNoLangFmt2", ep{{"a", `error Res(x string, y int32) {}`, compile.ErrorDef{
		Params: []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
	}, "", ""}}},
	{"WithParamsNoLangFmt3", ep{{"a", `error Res(x string, y int32) {NoRetry}`, compile.ErrorDef{
		Params: []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
	}, "", ""}}},

	{"MissingParamName1", ep{{"a", `error Res(bool) {"en":"msg1"}`, compile.ErrorDef{}, "parameters must be named", ""}}},
	{"MissingParamName2", ep{{"a", `error Res(bool, int32) {"en":"msg1"}`, compile.ErrorDef{}, "parameters must be named", ""}}},

	{"UnknownType", ep{{"a", `error Res(x foo) {}`, compile.ErrorDef{}, "type foo undefined", ""}}},
	{"NoTransitiveExportArg", ep{{"a", `type foo bool; error Res(x foo) {}`, compile.ErrorDef{}, "transitively exported", ""}}},
	{"InvalidParam", ep{{"a", `error Res(_x foo) {}`, compile.ErrorDef{}, "param _x invalid", ""}}},
	{"DupParam", ep{{"a", `error Res(x bool, x int32) {}`, compile.ErrorDef{}, "param x duplicate name", ""}}},
	{"UnknownAction", ep{{"a", `error Res() {Foo}`, compile.ErrorDef{}, "unknown action", ""}}},

	{"DupError", ep{{"a", `error Res() {};error Res() {}`, compile.ErrorDef{}, "error Res name conflict", ""}}},
}
