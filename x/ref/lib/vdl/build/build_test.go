// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package build_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/internal/vdltestutil"
	"v.io/x/ref/lib/vdl/testdata/base"
	"v.io/x/ref/lib/vdl/vdlutil"
)

func init() {
	// Uncomment this to enable verbose logs for debugging.
	// vdlutil.SetVerbose()
}

// The cwd is set to the directory containing this file.  Currently we have the
// following directory structure:
//   .../release/go/src/v.io/x/ref/lib/vdl/build/build_test.go
// We want to end up with the following:
//   VDLROOT = .../release/go/src/v.io/v23/vdlroot
//   VDLPATH = .../release/go
//
// TODO(toddw): Put a full VDLPATH tree under ../testdata and only use that.
const (
	defaultVDLRoot = "../../../../../v23/vdlroot"
	defaultVDLPath = "../../../../.."
)

func setEnvironment(t *testing.T, vdlroot, vdlpath string) {
	if err := os.Setenv("VDLROOT", vdlroot); err != nil {
		t.Fatalf("Setenv(VDLROOT, %q) failed: %v", vdlroot, err)
	}
	if err := os.Setenv("VDLPATH", vdlpath); err != nil {
		t.Fatalf("Setenv(VDLPATH, %q) failed: %v", vdlpath, err)
	}
}

// Tests the VDLROOT part of SrcDirs().
func TestSrcDirsVDLRoot(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	tests := []struct {
		VDLRoot string
		Want    string
		ErrRE   string
	}{
		{"", "", ""},
		{"/noexist", "", "doesn't exist"},
		{"/a", "/a", ""},
		{"/a/b/c", "/a/b/c", ""},
	}
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	for _, test := range tests {
		// The directory must exist in order to succeed.  Ignore mkdir errors, to
		// allow the same dir to be re-used.
		vdlRoot := test.VDLRoot
		if vdlRoot != "" && vdlRoot != "/noexist" {
			vdlRoot = filepath.Join(tmpDir, vdlRoot)
			os.MkdirAll(vdlRoot, os.ModePerm) //nolint:errcheck
		}
		setEnvironment(t, vdlRoot, defaultVDLPath)
		name := fmt.Sprintf("%+v", test)
		errs := vdlutil.NewErrors(-1)
		got := build.SrcDirs(errs)
		vdltestutil.ExpectResult(t, errs, name, test.ErrRE)
		// Every result will have our valid VDLPATH srcdir.
		var want []string
		if test.Want != "" {
			want = append(want, filepath.Join(tmpDir, test.Want))
		}
		want = append(want, filepath.Join(cwd, defaultVDLPath))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("SrcDirs(%s) got %v, want %v", name, got, want)
		}
	}
}

// Tests the VDLPATH part of SrcDirs().
func TestSrcDirsVDLPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	abs := func(relative string) string {
		return filepath.Join(cwd, relative)
	}
	tests := []struct {
		VDLPath string
		Want    []string
	}{
		{"", nil},
		// Test absolute paths.
		{"/a", []string{"/a"}},
		{"/a/b", []string{"/a/b"}},
		{"/a:/b", []string{"/a", "/b"}},
		{"/a/1:/b/2", []string{"/a/1", "/b/2"}},
		{"/a/1:/b/2:/c/3", []string{"/a/1", "/b/2", "/c/3"}},
		{":::/a/1::::/b/2::::/c/3:::", []string{"/a/1", "/b/2", "/c/3"}},
		// Test relative paths.
		{"a", []string{abs("a")}},
		{"a/b", []string{abs("a/b")}},
		{"a:b", []string{abs("a"), abs("b")}},
		{"a/1:b/2", []string{abs("a/1"), abs("b/2")}},
		{"a/1:b/2:c/3", []string{abs("a/1"), abs("b/2"), abs("c/3")}},
		{":::a/1::::b/2::::c/3:::", []string{abs("a/1"), abs("b/2"), abs("c/3")}},
		// Test mixed absolute / relative paths.
		{"a:/b", []string{abs("a"), "/b"}},
		{"/a/1:b/2", []string{"/a/1", abs("b/2")}},
		{"/a/1:b/2:/c/3", []string{"/a/1", abs("b/2"), "/c/3"}},
		{":::/a/1::::b/2::::/c/3:::", []string{"/a/1", abs("b/2"), "/c/3"}},
	}
	for _, test := range tests {
		setEnvironment(t, defaultVDLRoot, test.VDLPath)
		name := fmt.Sprintf("SrcDirs(%q)", test.VDLPath)
		errs := vdlutil.NewErrors(-1)
		got := build.SrcDirs(errs)
		var errRE string
		if test.Want == nil {
			errRE = "No src dirs; set VDLPATH to a valid value"
		}
		vdltestutil.ExpectResult(t, errs, name, errRE)
		// Every result will have our valid VDLROOT srcdir.
		want := append([]string{abs(defaultVDLRoot)}, test.Want...)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %v, want %v", name, got, want)
		}
	}
}

// Tests Is{Dir,Import}Path.
func TestIsDirImportPath(t *testing.T) {
	tests := []struct {
		Path  string
		IsDir bool
	}{
		// Import paths.
		{"", false},
		{"...", false},
		{".../", false},
		{"all", false},
		{"foo", false},
		{"foo/", false},
		{"foo...", false},
		{"foo/...", false},
		{"a/b/c", false},
		{"a/b/c/", false},
		{"a/b/c...", false},
		{"a/b/c/...", false},
		{"...a/b/c...", false},
		{"...a/b/c/...", false},
		{".../a/b/c/...", false},
		{".../a/b/c...", false},
		// Dir paths.
		{".", true},
		{"..", true},
		{"./", true},
		{"../", true},
		{"./...", true},
		{"../...", true},
		{".././.././...", true},
		{"/", true},
		{"/.", true},
		{"/..", true},
		{"/...", true},
		{"/./...", true},
		{"/foo", true},
		{"/foo/", true},
		{"/foo...", true},
		{"/foo/...", true},
		{"/a/b/c", true},
		{"/a/b/c/", true},
		{"/a/b/c...", true},
		{"/a/b/c/...", true},
		{"/a/b/c/../../...", true},
	}
	for _, test := range tests {
		if got, want := build.IsDirPath(test.Path), test.IsDir; got != want {
			t.Errorf("IsDirPath(%q) want %v", test.Path, want)
		}
		if got, want := build.IsImportPath(test.Path), !test.IsDir; got != want {
			t.Errorf("IsImportPath(%q) want %v", test.Path, want)
		}
	}
}

var allModes = []build.UnknownPathMode{
	build.UnknownPathIsIgnored,
	build.UnknownPathIsError,
}

// Tests TransitivePackages success cases.
func TestTransitivePackages(t *testing.T) {
	// Test with VDLROOT set.
	setEnvironment(t, defaultVDLRoot, defaultVDLPath)
	testTransitivePackages(t)
	// Test with VDLROOT unset.
	setEnvironment(t, "", defaultVDLPath)
	testTransitivePackages(t)
}

func testTransitivePackages(t *testing.T) {
	tests := []struct {
		InPaths  []string // Input paths to TransitivePackages call
		OutPaths []string // Wanted paths from build.Package.Path.
		GenPaths []string // Wanted paths from build.Package.GenPath, same as OutPaths if nil.
	}{
		{nil, nil, nil},
		{[]string{}, nil, nil},
		// Single-package, both import and dir path.
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		{
			[]string{"../testdata/base"},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		// Single-package with wildcard, both import and dir path.
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/base..."},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/base/..."},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		{
			[]string{"../testdata/base..."},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		{
			[]string{"../testdata/base/..."},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		// Redundant specification as both import and dir path.
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/base", "../testdata/base"},
			[]string{"v.io/x/ref/lib/vdl/testdata/base"},
			nil,
		},
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/arith", "../testdata/arith"},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
			},
			nil,
		},
		// Wildcards as both import and dir path.
		{
			[]string{"v.io/x/ref/lib/vdl/testdata..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		{
			[]string{"v.io/x/ref/lib/vdl/testdata/..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		{
			[]string{"../testdata..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		{
			[]string{"../testdata/..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		// Multi-Wildcards as both import and dir path.
		{
			[]string{"v...vdl/testdata/..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		{
			[]string{"../../...vdl/testdata/..."},
			[]string{
				"v.io/x/ref/lib/vdl/testdata/arith/exp",
				"v.io/x/ref/lib/vdl/testdata/base",
				"v.io/x/ref/lib/vdl/testdata/arith",
				"v.io/x/ref/lib/vdl/testdata/nativetest",
				"v.io/x/ref/lib/vdl/testdata/nativedep",
				"v.io/x/ref/lib/vdl/testdata/nativedep2",
				"v.io/x/ref/lib/vdl/testdata/testconfig",
			},
			nil,
		},
		// Multi-Wildcards as both import and dir path.
		{
			[]string{"v...vdl/testdata/...exp"},
			[]string{"v.io/x/ref/lib/vdl/testdata/arith/exp"},
			nil,
		},
		{
			[]string{"../../...vdl/testdata/...exp"},
			[]string{"v.io/x/ref/lib/vdl/testdata/arith/exp"},
			nil,
		},
		// Standard vdl package, as both import and dir path.
		{
			[]string{"vdltool"},
			[]string{"vdltool"},
			[]string{"v.io/v23/vdlroot/vdltool"},
		},
		{
			[]string{"../../../../../v23/vdlroot/vdltool"},
			[]string{"vdltool"},
			[]string{"v.io/v23/vdlroot/vdltool"},
		},
	}
	for _, test := range tests {
		// All modes should result in the same successful output.
		for _, mode := range allModes {
			name := fmt.Sprintf("%v %v", mode, test.InPaths)
			errs := vdlutil.NewErrors(-1)
			warnings := vdlutil.NewErrors(-1)
			pkgs := build.TransitivePackages(test.InPaths, mode, build.Opts{}, errs, warnings)
			vdltestutil.ExpectResult(t, errs, name, "")
			var paths []string
			for _, pkg := range pkgs {
				paths = append(paths, pkg.Path)
			}
			if got, want := paths, test.OutPaths; !reflect.DeepEqual(got, want) {
				t.Errorf("%v got paths:\n%v\nwant:\n%v", name, got, want)
			}
			wantGen := test.GenPaths
			if wantGen == nil {
				wantGen = test.OutPaths
			}
			paths = nil
			for _, pkg := range pkgs {
				paths = append(paths, pkg.GenPath)
			}
			if got, want := paths, wantGen; !reflect.DeepEqual(got, want) {
				t.Errorf("%v got gen paths %v, want %v", name, got, want)
			}
		}
	}
}

// Tests TransitivePackages error cases.
func TestTransitivePackagesUnknownPathError(t *testing.T) {
	// Test with VDLROOT set.
	setEnvironment(t, defaultVDLRoot, defaultVDLPath)
	testTransitivePackagesUnknownPathError(t)
	// Test with VDLROOT unset.
	setEnvironment(t, "", defaultVDLPath)
	testTransitivePackagesUnknownPathError(t)
}

func testTransitivePackagesUnknownPathError(t *testing.T) {
	tests := []struct {
		InPaths []string
		ErrRE   string
	}{
		// Non-existent as both import and dir path.
		{
			[]string{"noexist"},
			`can't resolve "noexist" to any packages`,
		},
		{
			[]string{"./noexist"},
			`can't resolve "./noexist" to any packages`,
		},
		// Invalid package path, as both import and dir path.
		{
			[]string{".foo"},
			`import path ".foo" is invalid`,
		},
		{
			[]string{"foo/.bar"},
			`import path "foo/.bar" is invalid`,
		},
		{
			[]string{"_foo"},
			`import path "_foo" is invalid`,
		},
		{
			[]string{"foo/_bar"},
			`import path "foo/_bar" is invalid`,
		},
		{
			[]string{"../../../../../.foo"},
			`package path "v.io/.foo" is invalid`,
		},
		{
			[]string{"../../../../../foo/.bar"},
			`package path "v.io/foo/.bar" is invalid`,
		},
		{
			[]string{"../../../../../_foo"},
			`package path "v.io/_foo" is invalid`,
		},
		{
			[]string{"../../../../../foo/_bar"},
			`package path "v.io/foo/_bar" is invalid`,
		},
		// Special-case error for packages under vdlroot, which can't be imported
		// using the vdlroot prefix.
		{
			[]string{"v.io/v23/vdlroot/vdltool"},
			`packages under vdlroot must be specified without the vdlroot prefix`,
		},
		{
			[]string{"v.io/v23/vdlroot/..."},
			`can't resolve "v.io/v23/vdlroot/..." to any packages`,
		},
	}
	for _, test := range tests {
		for _, mode := range allModes {
			name := fmt.Sprintf("%v %v", mode, test.InPaths)
			errs := vdlutil.NewErrors(-1)
			warnings := vdlutil.NewErrors(-1)
			pkgs := build.TransitivePackages(test.InPaths, mode, build.Opts{}, errs, warnings)
			errRE := test.ErrRE
			if mode == build.UnknownPathIsIgnored {
				// Ignore mode returns success, while error mode returns error.
				errRE = ""
			}
			vdltestutil.ExpectResult(t, errs, name, errRE)
			if pkgs != nil {
				t.Errorf("%v got unexpected packages %v", name, pkgs)
				return
			}
		}
	}
}

// Tests vdl.config file support.
func TestPackageConfig(t *testing.T) {
	setEnvironment(t, defaultVDLRoot, defaultVDLPath)
	tests := []struct {
		Path   string
		Config vdltool.Config
	}{
		{"v.io/x/ref/lib/vdl/testdata/base", vdltool.Config{}},
		{
			"v.io/x/ref/lib/vdl/testdata/testconfig",
			vdltool.Config{
				GenLanguages: map[vdltool.GenLanguage]struct{}{vdltool.GenLanguageGo: {}},
			},
		},
	}
	for _, test := range tests {
		name := path.Base(test.Path)
		env := compile.NewEnv(-1)
		deps := build.TransitivePackages([]string{test.Path}, build.UnknownPathIsError, build.Opts{}, env.Errors, env.Warnings)
		vdltestutil.ExpectResult(t, env.Errors, name, "")
		if len(deps) != 1 {
			t.Fatalf("TransitivePackages(%q) got %v, want 1 dep", name, deps)
		}
		if got, want := deps[0].Name, name; got != want {
			t.Errorf("TransitivePackages(%q) got Name %q, want %q", name, got, want)
		}
		if got, want := deps[0].Path, test.Path; got != want {
			t.Errorf("TransitivePackages(%q) got Path %q, want %q", name, got, want)
		}
		if got, want := deps[0].Config, test.Config; !reflect.DeepEqual(got, want) {
			t.Errorf("TransitivePackages(%q) got Config %+v, want %+v", name, got, want)
		}
	}
}

// Tests BuildConfig, BuildConfigValue and TransitivePackagesForConfig.
func TestBuildConfig(t *testing.T) {
	setEnvironment(t, defaultVDLRoot, defaultVDLPath)
	tests := []struct {
		Src   string
		Value interface{}
	}{
		{
			`config = x;import "v.io/x/ref/lib/vdl/testdata/base";const x = base.NamedBool(true)`,
			base.NamedBool(true),
		},
		{
			`config = x;import "v.io/x/ref/lib/vdl/testdata/base";const x = base.NamedString("abc")`,
			base.NamedString("abc"),
		},
		{
			`config = x;import "v.io/x/ref/lib/vdl/testdata/base";const x = base.Args{1, 2}`,
			base.Args{A: 1, B: 2},
		},
	}
	for _, test := range tests {
		// Build import package dependencies.
		env := compile.NewEnv(-1)
		deps := build.TransitivePackagesForConfig("file", strings.NewReader(test.Src), build.Opts{}, env.Errors, env.Warnings)
		for _, dep := range deps {
			build.BuildPackage(dep, env)
		}
		vdltestutil.ExpectResult(t, env.Errors, test.Src, "")
		// Test BuildConfig
		wantV := vdl.ZeroValue(vdl.TypeOf(test.Value))
		if err := vdl.Convert(wantV, test.Value); err != nil {
			t.Errorf("Convert(%v) got error %v, want nil", test.Value, err)
		}
		gotV := build.BuildConfig("file", strings.NewReader(test.Src), nil, nil, env)
		if !vdl.EqualValue(gotV, wantV) {
			t.Errorf("BuildConfig(%v) got %v, want %v", test.Src, gotV, wantV)
		}
		vdltestutil.ExpectResult(t, env.Errors, test.Src, "")
		// TestBuildConfigValue
		gotRV := reflect.New(reflect.TypeOf(test.Value))
		build.BuildConfigValue("file", strings.NewReader(test.Src), nil, env, gotRV.Interface())
		if got, want := gotRV.Elem().Interface(), test.Value; !reflect.DeepEqual(got, want) {
			t.Errorf("BuildConfigValue(%v) got %v, want %v", test.Src, got, want)
		}
		vdltestutil.ExpectResult(t, env.Errors, test.Src, "")
	}
}

type ts []*vdl.Type
type vs []*vdl.Value

func TestBuildExprs(t *testing.T) {
	ttArray := vdl.ArrayType(2, vdl.Int32Type)
	ttStruct := vdl.StructType(
		vdl.Field{
			Name: "A",
			Type: vdl.Int32Type,
		}, vdl.Field{
			Name: "B",
			Type: vdl.StringType,
		},
	)
	vvArray := vdl.ZeroValue(ttArray)
	vvArray.Index(0).AssignInt(1)
	vvArray.Index(1).AssignInt(-2)
	vvStruct := vdl.ZeroValue(ttStruct)
	vvStruct.StructField(0).AssignInt(1)
	vvStruct.StructField(1).AssignString("abc")
	tests := []struct {
		Data  string
		Types ts
		Want  vs
		Err   string
	}{
		{``, nil, nil, "syntax error"},
		{`true`, nil, vs{vdl.BoolValue(nil, true)}, ""},
		{`false`, nil, vs{vdl.BoolValue(nil, false)}, ""},
		{`"abc"`, nil, vs{vdl.StringValue(nil, "abc")}, ""},
		{`1`, nil, vs{nil}, "1 must be assigned a type"},
		{`1`, ts{vdl.Int64Type}, vs{vdl.IntValue(vdl.Int64Type, 1)}, ""},
		{`1.0`, ts{vdl.Int64Type}, vs{vdl.IntValue(vdl.Int64Type, 1)}, ""},
		{`1.5`, ts{vdl.Int64Type}, vs{nil}, "loses precision"},
		{`1.0`, ts{vdl.Float64Type}, vs{vdl.FloatValue(vdl.Float64Type, 1.0)}, ""},
		{`1.5`, ts{vdl.Float64Type}, vs{vdl.FloatValue(vdl.Float64Type, 1.5)}, ""},
		{`1+2`, ts{vdl.Int64Type}, vs{vdl.IntValue(vdl.Int64Type, 3)}, ""},
		{`1+2,"abc"`, ts{vdl.Int64Type, nil}, vs{vdl.IntValue(vdl.Int64Type, 3), vdl.StringValue(nil, "abc")}, ""},
		{`1,2,3`, ts{vdl.Int64Type}, vs{vdl.IntValue(vdl.Int64Type, 1), vdl.IntValue(vdl.Int64Type, 2), vdl.IntValue(vdl.Int64Type, 3)}, ""},
		{`{1,-2}`, ts{ttArray}, vs{vvArray}, ""},
		{`{0+1,1-3}`, ts{ttArray}, vs{vvArray}, ""},
		{`{1,"abc"}`, ts{ttStruct}, vs{vvStruct}, ""},
		{`{A:1,B:"abc"}`, ts{ttStruct}, vs{vvStruct}, ""},
		{`{B:"abc",A:1}`, ts{ttStruct}, vs{vvStruct}, ""},
		{`{B:"a"+"bc",A:1*1}`, ts{ttStruct}, vs{vvStruct}, ""},
	}
	for _, test := range tests {
		env := compile.NewEnv(-1)
		values := build.BuildExprs(test.Data, test.Types, env)
		vdltestutil.ExpectResult(t, env.Errors, test.Data, test.Err)
		if got, want := len(values), len(test.Want); got != want {
			t.Errorf("%s got len %d, want %d", test.Data, got, want)
		}
		for ix, want := range test.Want {
			var got *vdl.Value
			if ix < len(values) {
				got = values[ix]
			}
			if !vdl.EqualValue(got, want) {
				t.Errorf("%s got value #%d %v, want %v", test.Data, ix, got, want)
			}
		}
	}
}

func TestPackageSplit(t *testing.T) {
	defer build.SetFilePathSeparator(string(filepath.Separator))
	for i, test := range []struct {
		dir, path            string
		prefix, body, suffix string
		windows              bool
	}{
		{"", "", "", "", "", false},
		{"a", "a", "", "", "a", false},
		{"ab", "b", "a", "", "b", false},
		{"a/b", "a/b", "", "", "a/b", false},
		{`a\b`, "a/b", "", "", "a/b", true},
		{`c:a\b`, "a/b", "c:", "", "a/b", true},
		{"a/b/", "a/b/", "", "", "a/b/", false},
		{`a\b/`, "a/b/", "", "", "a/b/", true},
		{"a/b/c/d/e", "c/d/e", "a/b", "", "c/d/e", false},
		{`a\b\c\d\e`, "c/d/e", `a\b`, "", "c/d/e", true},
		{"a/b/c/d/e", "z/c/d/e", "a/b", "z", "c/d/e", false},
		{`a\b\c\d\e`, "z/c/d/e", `a\b`, "z", "c/d/e", true},
	} {
		build.SetFilePathSeparator("/")
		if test.windows {
			build.SetFilePathSeparator(`\`)
		}
		prefix, body, suffix := build.PackagePathSplit(test.dir, test.path)
		if got, want := prefix, test.prefix; got != want {
			t.Errorf("(%v): got %q, want %q", i, got, want)
		}
		if got, want := body, test.body; got != want {
			t.Errorf("(%v): got %q, want %q", i, got, want)
		}
		if got, want := suffix, test.suffix; got != want {
			t.Errorf("(%v): got %q, want %q", i, got, want)
		}
	}
}

func here() string {
	_, file, _, _ := runtime.Caller(1)
	return file
}

func TestGoMod(t *testing.T) {
	file := here()
	root := filepath.Clean(strings.TrimSuffix(file, "x/ref/lib/vdl/build/build_test.go"))
	gomod, err := build.GoModuleName(root)
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}
	if got, want := gomod, "v.io"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	pkg := "x/ref/lib/vdl/build"
	prefix, module, suffix := build.PackagePathSplit(filepath.Dir(file), "v.io/"+pkg)

	expect := func(p, m, s string) {
		if got, want := prefix, p; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := module, m; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if got, want := suffix, s; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	switch len(module) {
	case 0:
		// v.io is in the directory structure.
		expect(strings.TrimSuffix(root, "/"+gomod), "", path.Join(gomod, pkg))
	default:
		// v.io is not in the directory structure.
		expect(prefix, gomod, pkg)
	}
}
