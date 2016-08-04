// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile_test

import (
	"fmt"
	"path"
	"testing"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/internal/vdltestutil"
)

type f map[string]string

func TestParseAndCompile(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string
		errRE  string
		expect func(t *testing.T, name string, pkg *compile.Package)
	}{
		{"test1", f{"1.vdl": pkg1file1, "2.vdl": pkg1file2}, "", expectPkg1},
		{"test2", f{"1.vdl": "package native"}, `reserved word in a generated language`, nil},
	}
	for _, test := range tests {
		path := path.Join("a/b", test.name)
		buildPkg := vdltestutil.FakeBuildPackage(test.name, path, test.files)
		env := compile.NewEnv(-1)
		pkg := build.BuildPackage(buildPkg, env)
		vdltestutil.ExpectResult(t, env.Errors, test.name, test.errRE)
		if pkg == nil {
			continue
		}
		if got, want := pkg.Name, test.name; got != want {
			t.Errorf("%v got package name %s, want %s", buildPkg, got, want)
		}
		if got, want := pkg.Path, path; got != want {
			t.Errorf("%v got package path %s, want %s", buildPkg, got, want)
		}
		test.expect(t, test.name, pkg)
	}
}

func TestParseAndCompileExprs(t *testing.T) {
	env := compile.NewEnv(-1)
	path := path.Join("a/b/test1")
	buildPkg := vdltestutil.FakeBuildPackage("test1", path, f{"1.vdl": pkg1file1, "2.vdl": pkg1file2})
	pkg := build.BuildPackage(buildPkg, env)
	if pkg == nil {
		t.Fatal("failed to build package")
	}
	// Test that expressions from the built packages compile correctly.
	scalarsType := env.ResolvePackage("a/b/test1").ResolveType("Scalars").Type
	exprTests := []struct {
		data  string
		vtype *vdl.Type
	}{
		{`"a/b/test1".Cint64`, vdl.Int64Type},
		{`"a/b/test1".FiveSquared + "a/b/test1".Cint32`, vdl.Int32Type},
		{`"a/b/test1".Scalars{A:true,C:"a/b/test1".FiveSquared+"a/b/test1".Cint32}`, scalarsType},
	}
	for _, test := range exprTests {
		vals := build.BuildExprs(test.data, []*vdl.Type{test.vtype}, env)
		if !env.Errors.IsEmpty() || len(vals) != 1 {
			t.Errorf("failed to build %v: %v", test.data, env.Errors)
		}
	}
}

const pkg1file1 = `package test1

type Scalars struct {
	A bool
	B byte
	C int32
	D int64
	E uint32
	F uint64
	G float32
	H float64
	I string
	J error
	K any
}

type KeyScalars struct {
	A bool
	B byte
	C int32
	D int64
	E uint32
	F uint64
	G float32
	H float64
	I string
}

type CompComp struct {
	A Composites
	B []Composites
	C map[string]Composites
}

const (
	Cbool = true
	Cbyte = byte(1)
	Cint32 = int32(2)
	Cint64 = int64(3)
	Cuint32 = uint32(4)
	Cuint64 = uint64(5)
	Cfloat32 = float32(6)
	Cfloat64 = float64(7)
	Cstring = "foo"
	Cany = Cbool

	True = true
	Foo = "foo"
	Five = int32(5)
	SixSquared = Six*Six
)

type ServiceA interface {
	MethodA1() error
	MethodA2(a int32, b string) (s string | error)
	MethodA3(a int32) stream<_, Scalars> (s string | error) {"tag", Six}
	MethodA4(a int32) stream<int32, string> error
}
`

const pkg1file2 = `package test1
type Composites struct {
	A Scalars
	B []Scalars
	C map[string]Scalars
	D map[KeyScalars][]map[string]int32
}

const (
	FiveSquared = Five*Five
	Six = uint64(6)
)

type ServiceB interface {
	ServiceA
	MethodB1(a Scalars, b Composites) (c CompComp | error)
}
`

func expectPkg1(t *testing.T, name string, pkg *compile.Package) {
	// TODO(toddw): verify real expectations, and add more tests.
	fmt.Println(pkg)
}
