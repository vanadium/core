// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package swift implements Swift code generation from compiled VDL packages.
package swift

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/internal/vdltestutil"
)

const (
	pkgAFile1 = `package a

const ABool = true

type StructA struct {
	X bool
}`

	pkgBFile1 = `package b

const BBool = true

type StructB struct {
	X bool
}`

	pkgCFile1 = `package c

import "v.io/v23/a"

const CBool = a.ABool

type (
	StructC struct {
		X bool
		Y a.StructA
	}

	SelfContainedStructC struct {
		X bool
	}

	UnionC union {
		X bool
		Y a.StructA
	}

	MapCKey map[a.StructA]string
	MapCValue map[string]a.StructA
	ArrayC [5]a.StructA
	ListC []a.StructA
	SetC set[a.StructA]
)
`
)

type moduleConfig struct {
	Name string
	Path string
}

type pkgConfig struct {
	Name  string
	Path  string
	Files map[string]string
}

func createTmpVdlPath(t *testing.T, modules []moduleConfig, pkgs []pkgConfig) (string, func()) {
	oldVdlPath := os.Getenv("VDLPATH")
	sh := gosh.NewShell(nil)
	tempDir := sh.MakeTempDir()
	os.Setenv("VDLPATH", tempDir)
	for _, module := range modules {
		if strings.HasPrefix(module.Path, "/") {
			sh.Cleanup()
			t.Fatalf("module.Path must be relative")
		}
		sh.Cmd("mkdir", "-p", filepath.Join(tempDir, module.Path)).Run()
		moduleConfigPath := filepath.Join(tempDir, module.Path, "swiftmodule")
		err := ioutil.WriteFile(moduleConfigPath, []byte(module.Name), 0644)
		if err != nil {
			sh.Cleanup()
			t.Fatalf("Unable to create temp vdl.config file: %v", err)
		}
	}
	for _, pkg := range pkgs {
		if strings.HasPrefix(pkg.Path, "/") {
			sh.Cleanup()
			t.Errorf("pkg.Path must be relative")
		}
		sh.Cmd("mkdir", "-p", filepath.Join(tempDir, pkg.Path)).Run()
		for file, contents := range pkg.Files {
			vdlPath := filepath.Join(tempDir, pkg.Path, file)
			err := ioutil.WriteFile(vdlPath, []byte(contents), 0644)
			if err != nil {
				sh.Cleanup()
				t.Fatalf("Unable to create temp vdl file at %v: %v", vdlPath, err)
			}
		}
	}
	return tempDir, func() {
		sh.Cleanup()
		os.Setenv("VDLPATH", oldVdlPath)
	}
}

func createPackagesAndSwiftContext(t *testing.T, modules []moduleConfig, pkgs []pkgConfig, ctxPath string) ([]*compile.Package, *swiftContext, func()) {
	vdlPath, cleanup := createTmpVdlPath(t, modules, pkgs)
	built := make([]*compile.Package, len(pkgs))
	i := 0
	env := compile.NewEnv(-1)
	genPathToDir := map[string]string{}
	for _, pkg := range pkgs {
		buildPkg := vdltestutil.FakeBuildPackage(pkg.Name, pkg.Path, pkg.Files)
		built[i] = build.BuildPackage(buildPkg, env)
		built[i].GenPath = built[i].Path
		genPathToDir[built[i].GenPath] = filepath.Join(vdlPath, pkg.Path)
		i++
	}
	// Find ctxPkgPath
	var ctxPkg *compile.Package = nil
	for _, pkg := range built {
		if pkg.Path == ctxPath {
			ctxPkg = pkg
			break
		}
	}
	if ctxPkg == nil {
		cleanup()
		t.Fatalf("Could not find test context pkg %v", ctxPath)
	}
	ctx := swiftContext{
		env,
		ctxPkg,
		genPathToDir,
		map[string]bool{vdlPath: true},
		map[*compile.TypeDef]string{},
	}
	return built, &ctx, cleanup
}

func TestPkgIsSameModule(t *testing.T) {
	tests := []struct {
		name    string
		modules []moduleConfig
		pkgs    []pkgConfig
		pkgPath string
		ctxPath string
		expect  bool
	}{
		{"only one module",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/b",
			true},
		{"two modules, in same module",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/b",
			true},
		{"in diff module",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/b",
			"v.io/v23/services/c",
			false},
	}
	for _, test := range tests {
		pkgs, ctx, cleanup := createPackagesAndSwiftContext(t, test.modules, test.pkgs, test.ctxPath)
		// Find test pkgPath
		var testPkg *compile.Package = nil
		for _, pkg := range pkgs {
			if pkg.Path == test.pkgPath {
				testPkg = pkg
				break
			}
		}
		if testPkg == nil {
			cleanup()
			t.Errorf("Could not find test pkg")
			return
		}
		if ctxModule, pkgModule := ctx.swiftModule(ctx.pkg), ctx.swiftModule(testPkg); ctxModule == "" || pkgModule == "" {
			cleanup()
			t.Errorf("%s\n ctx or pkg module is nil", test.name)
			return
		}
		if got, want := ctx.pkgIsSameModule(testPkg), test.expect; got != want {
			t.Errorf("%s\n GOT %v\nWANT %v", test.name, got, want)
		}
		cleanup()
	}
}

func TestSwiftModule(t *testing.T) {
	tests := []struct {
		name    string
		modules []moduleConfig
		pkgs    []pkgConfig
		ctxPath string
		expect  string
	}{
		{"no modules",
			[]moduleConfig{},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			""},
		{"only one module",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			V23SwiftFrameworkName},
		{"two modules (VanadiumCore)",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/a",
			V23SwiftFrameworkName},
		{"two modules (VanadiumServices)",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"VanadiumServices"},
		{"not in path",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23/a"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/b", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/b",
			""},
	}
	for _, test := range tests {
		_, ctx, cleanup := createPackagesAndSwiftContext(t, test.modules, test.pkgs, test.ctxPath)
		if got, want := ctx.swiftModule(ctx.pkg), test.expect; got != want {
			t.Errorf("%s\n GOT %v\nWANT %v", test.name, got, want)
		}
		cleanup()
	}
}

func TestPackageName(t *testing.T) {
	tests := []struct {
		name    string
		modules []moduleConfig
		pkgs    []pkgConfig
		ctxPath string
		pkgPath string
		expect  string
	}{
		{"no modules",
			[]moduleConfig{},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/a",
			"VioV23A"},
		{"removes module path",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/a",
			"A"},
		{"root is empty name",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23/a"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/a",
			""},
		{"path not in module",
			[]moduleConfig{{"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/a",
			"v.io/v23/a",
			"VioV23A"},
		{"path in module",
			[]moduleConfig{{"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/a",
			"v.io/v23/services/c",
			"C"},
		{"two modules (VanadiumServices)",
			[]moduleConfig{{V23SwiftFrameworkName, "v.io/v23"}, {"VanadiumServices", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/a",
			"v.io/v23/services/c",
			"C"},
	}
	for _, test := range tests {
		pkgs, ctx, cleanup := createPackagesAndSwiftContext(t, test.modules, test.pkgs, test.ctxPath)
		// Find test pkgPath
		var testPkg *compile.Package = nil
		for _, pkg := range pkgs {
			if pkg.Path == test.pkgPath {
				testPkg = pkg
				break
			}
		}
		if testPkg == nil {
			cleanup()
			t.Errorf("Could not find test pkg")
			return
		}
		if got, want := ctx.swiftPackageName(testPkg), test.expect; got != want {
			t.Errorf("%s\n GOT %v\nWANT %v", test.name, got, want)
		}
		cleanup()
	}
}

func TestImportedModules(t *testing.T) {
	tests := []struct {
		name              string
		modules           []moduleConfig
		pkgs              []pkgConfig
		ctxPath           string
		tdefQualifiedPath string
		expect            string
	}{
		{"no modules",
			[]moduleConfig{},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/a.StructA",
			""},
		{"one module",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"b", "v.io/v23/b", map[string]string{"file1.vdl": pkgBFile1}}},
			"v.io/v23/a",
			"v.io/v23/a.StructA",
			""},
		{"path not in module",
			[]moduleConfig{{"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/a",
			"v.io/v23/a.StructA",
			""},
		{"path in module",
			[]moduleConfig{{"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.SelfContainedStructC",
			""},
		{"cross module union field",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.UnionC",
			"VanadiumC"},
		{"cross module struct field",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.StructC",
			"VanadiumC"},
		{"cross module map key",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.MapCKey",
			"VanadiumC"},
		{"cross module map value",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.MapCValue",
			"VanadiumC"},
		{"cross module array",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.ArrayC",
			"VanadiumC"},
		{"cross module list",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.ListC",
			"VanadiumC"},
		{"cross module set",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.SetC",
			"VanadiumC"},
		{"non-cross module",
			[]moduleConfig{{"VanadiumC", "v.io/v23"}, {"VanadiumS", "v.io/v23/services"}},
			[]pkgConfig{{"a", "v.io/v23/a", map[string]string{"file1.vdl": pkgAFile1}},
				{"c", "v.io/v23/services/c", map[string]string{"file1.vdl": pkgCFile1}}},
			"v.io/v23/services/c",
			"v.io/v23/services/c.SelfContainedStructC",
			"VanadiumC"},
	}
	for _, test := range tests {
		pkgs, ctx, cleanup := createPackagesAndSwiftContext(t, test.modules, test.pkgs, test.ctxPath)
		// Find test tdef
		var testTdef *compile.TypeDef = nil
		for _, pkg := range pkgs {
			if strings.HasPrefix(test.tdefQualifiedPath, pkg.Path) {
				for _, tdef := range pkg.TypeDefs() {
					if strings.HasSuffix(test.tdefQualifiedPath, tdef.Name) {
						testTdef = tdef
						break
					}
				}
				for _, cdef := range pkg.ConstDefs() {
					tdef := ctx.env.FindTypeDef(cdef.Value.Type())
					if strings.HasSuffix(test.tdefQualifiedPath, cdef.Name) {
						testTdef = tdef
						break
					}
				}

				break
			}
		}
		if testTdef == nil {
			cleanup()
			t.Errorf("Could not find tdef %v", test.tdefQualifiedPath)
			return
		}
		got := ctx.importedModules([]*compile.TypeDef{testTdef})
		// Remove VanadiumCore if we aren't expecting it as it's always imported
		foundCore := false
		gotModule := ""
		for _, module := range got {
			if module == V23SwiftFrameworkName {
				foundCore = true
			} else {
				gotModule = module
			}
		}
		if !foundCore {
			t.Errorf("Couldn't find expected core import %v", V23SwiftFrameworkName)
		}
		if gotModule != test.expect {
			t.Errorf("%s\n GOT %v\nWANT %v", test.name, gotModule, test.expect)
		}
		cleanup()
	}
}
