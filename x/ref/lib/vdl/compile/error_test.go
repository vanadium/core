// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile_test

import (
	"reflect"
	"testing"

	"v.io/v23/i18n"
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
	for _, param := range ed.Params {
		param.Pos = parse.Pos{}
	}
	ed.File = nil
	return ed
}

type errorPkg struct {
	Name  string
	Data  string
	Want  compile.ErrorDef
	ErrRE string
}

type ep []errorPkg

type errorTest struct {
	Name string
	Pkgs ep
}

const (
	en i18n.LangID = "en"
	zh             = "zh"
)

func arg(name string, t *vdl.Type) *compile.Field {
	arg := new(compile.Field)
	arg.Name = name
	arg.Type = t
	return arg
}

const pre = "{1:}{2:} "

var errorTests = []errorTest{
	{"NoParams1", ep{{"a", `error Res() {"en":"msg1"}`,
		compile.ErrorDef{
			Formats: []compile.LangFmt{{en, pre + "msg1"}},
			English: pre + "msg1",
		},
		"",
	}}},
	{"NoParams2", ep{{"a", `error Res() {"en":"msg1","zh":"msg2"}`,
		compile.ErrorDef{
			Formats: []compile.LangFmt{{en, pre + "msg1"}, {zh, pre + "msg2"}},
			English: pre + "msg1",
		},
		"",
	}}},
	{"NoParamsNoRetry", ep{{"a", `error Res() {NoRetry,"en":"msg1"}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeNoRetry,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"NoParamsRetryConnection", ep{{"a", `error Res() {RetryConnection,"en":"msg1"}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryConnection,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"NoParamsRetryRefetch", ep{{"a", `error Res() {RetryRefetch,"en":"msg1"}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryRefetch,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"NoParamsRetryBackoff", ep{{"a", `error Res() {RetryBackoff,"en":"msg1"}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryBackoff,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"NoParamsMulti", ep{{"a", `error Res() {RetryRefetch,"en":"msg1","zh":"msg2"}`,
		compile.ErrorDef{
			RetryCode: vdl.WireRetryCodeRetryRefetch,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}, {zh, pre + "msg2"}},
			English:   pre + "msg1",
		},
		"",
	}}},

	{"WithParams1", ep{{"a", `error Res(x string, y int32) {"en":"msg1"}`,
		compile.ErrorDef{
			Params:  []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			Formats: []compile.LangFmt{{en, pre + "msg1"}},
			English: pre + "msg1",
		},
		"",
	}}},
	{"WithParams2", ep{{"a", `error Res(x string, y int32) {"en":"msg1","zh":"msg2"}`,
		compile.ErrorDef{
			Params:  []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			Formats: []compile.LangFmt{{en, pre + "msg1"}, {zh, pre + "msg2"}},
			English: pre + "msg1",
		},
		"",
	}}},
	{"WithParamsNoRetry", ep{{"a", `error Res(x string, y int32) {NoRetry,"en":"msg1"}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeNoRetry,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"WithParamsRetryConnection", ep{{"a", `error Res(x string, y int32) {RetryConnection,"en":"msg1"}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryConnection,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"WithParamsRetryRefetch", ep{{"a", `error Res(x string, y int32) {RetryRefetch,"en":"msg1"}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryRefetch,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"WithParamsRetryBackoff", ep{{"a", `error Res(x string, y int32) {RetryBackoff,"en":"msg1"}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryBackoff,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"WithParamsMulti", ep{{"a", `error Res(x string, y int32) {RetryRefetch,"en":"msg1","zh":"msg2"}`,
		compile.ErrorDef{
			Params:    []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			RetryCode: vdl.WireRetryCodeRetryRefetch,
			Formats:   []compile.LangFmt{{en, pre + "msg1"}, {zh, pre + "msg2"}},
			English:   pre + "msg1",
		},
		"",
	}}},
	{"WithParamsFormat", ep{{"a", `error Res(x string, y int32) {"en":"en {x} {y}","zh":"zh {y} {x}"}`,
		compile.ErrorDef{
			Params:  []*compile.Field{arg("x", vdl.StringType), arg("y", vdl.Int32Type)},
			Formats: []compile.LangFmt{{en, pre + "en {3} {4}"}, {zh, pre + "zh {4} {3}"}},
			English: pre + "en {3} {4}",
		},
		"",
	}}},
	{"WithSamePackageParam", ep{{"a", `error Res(x Bool) {"en":"en {x}"};type Bool bool`,
		compile.ErrorDef{
			Params:  []*compile.Field{arg("x", vdl.NamedType("a.Bool", vdl.BoolType))},
			Formats: []compile.LangFmt{{en, pre + "en {3}"}},
			English: pre + "en {3}",
		},
		"",
	}}},

	// Test multi-package errors.
	{"MultiPkgSameErrorName", ep{
		{
			"a", `error Res() {"en":"msg1"}`,
			compile.ErrorDef{
				Formats: []compile.LangFmt{{en, pre + "msg1"}},
				English: pre + "msg1",
			},
			"",
		},
		{
			"b", `error Res() {"en":"msg2"}`,
			compile.ErrorDef{
				Formats: []compile.LangFmt{{en, pre + "msg2"}},
				English: pre + "msg2",
			},
			"",
		},
	}},
	{"MultiPkgTypeDep", ep{
		{
			"a", `error Res() {"en":"msg1"};type Bool bool`,
			compile.ErrorDef{
				Formats: []compile.LangFmt{{en, pre + "msg1"}},
				English: pre + "msg1",
			},
			"",
		},
		{
			"b", `import "a";error Res(x a.Bool) {"en":"en {x}"}`,
			compile.ErrorDef{
				Params:  []*compile.Field{arg("x", vdl.NamedType("a.Bool", vdl.BoolType))},
				Formats: []compile.LangFmt{{en, pre + "en {3}"}},
				English: pre + "en {3}",
			},
			"",
		},
	}},
	{"RedefinitionOfImportName", ep{
		{
			"a", `error Res() {"en":"msg1"}`,
			compile.ErrorDef{
				Formats: []compile.LangFmt{{en, pre + "msg1"}},
				English: pre + "msg1",
			},
			"",
		},
		{
			"b", `import "a";error a() {"en":"en {}"}`, compile.ErrorDef{},
			"error a name conflict",
		},
	}},

	// Test errors.
	{"NoParamsNoLangFmt1", ep{{"a", `error Res()`, compile.ErrorDef{}, englishFormat}}},
	{"NoParamsNoLangFmt2", ep{{"a", `error Res() {}`, compile.ErrorDef{}, englishFormat}}},
	{"NoParamsNoLangFmt3", ep{{"a", `error Res() {NoRetry}`, compile.ErrorDef{}, englishFormat}}},

	{"WithParamsNoLangFmt1", ep{{"a", `error Res(x string, y int32)`, compile.ErrorDef{}, englishFormat}}},
	{"WithParamsNoLangFmt2", ep{{"a", `error Res(x string, y int32) {}`, compile.ErrorDef{}, englishFormat}}},
	{"WithParamsNoLangFmt3", ep{{"a", `error Res(x string, y int32) {NoRetry}`, compile.ErrorDef{}, englishFormat}}},

	{"MissingParamName1", ep{{"a", `error Res(bool) {"en":"msg1"}`, compile.ErrorDef{}, "parameters must be named"}}},
	{"MissingParamName2", ep{{"a", `error Res(bool, int32) {"en":"msg1"}`, compile.ErrorDef{}, "parameters must be named"}}},

	{"UnknownType", ep{{"a", `error Res(x foo) {"en":"msg1"}`, compile.ErrorDef{}, "type foo undefined"}}},
	{"NoTransitiveExportArg", ep{{"a", `type foo bool; error Res(x foo) {"en":"msg1"}`, compile.ErrorDef{}, "transitively exported"}}},
	{"InvalidParam", ep{{"a", `error Res(_x foo) {"en":"msg1"}`, compile.ErrorDef{}, "param _x invalid"}}},
	{"DupParam", ep{{"a", `error Res(x bool, x int32) {"en":"msg1"}`, compile.ErrorDef{}, "param x duplicate name"}}},
	{"UnknownAction", ep{{"a", `error Res() {Foo,"en":"msg1"}`, compile.ErrorDef{}, "unknown action"}}},
	{"EmptyLanguage", ep{{"a", `error Res() {"":"msg"}`, compile.ErrorDef{}, "empty language"}}},
	{"DupLanguage", ep{{"a", `error Res() {"en":"msg1","en":"msg2"}`, compile.ErrorDef{}, "duplicate language en"}}},
	{"UnknownParam", ep{{"a", `error Res() {"en":"{foo}"}`, compile.ErrorDef{}, `unknown param "foo"`}}},
	{"DupError", ep{{"a", `error Res() {"en":"msg1"};error Res() {"en":"msg1"}`, compile.ErrorDef{}, "error Res name conflict"}}},
}

const englishFormat = "must define at least one English format"
