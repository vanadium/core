// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

// TODO(toddw): Add tests for this logic.

import (
	"sort"
	"strconv"

	"v.io/x/ref/lib/vdl/compile"
)

// goImport represents a single import in the generated Go file.
//   Example A: import     "v.io/v23/abc"
//   Example B: import foo "v.io/v23/abc"
type goImport struct {
	// Name of the import.
	//   Example A: ""
	//   Example B: "foo"
	Name string
	// Path of the import.
	//   Example A: "v.io/v23/abc"
	//   Example B: "v.io/v23/abc"
	Path string
	// Local identifier within the generated go file to reference the imported
	// package.
	//   Example A: "abc"
	//   Example B: "foo"
	Local string
}

// goImports holds all imports for a generated Go file.
type goImports []goImport

// importMap maps from package path to package name.  It's used to collect
// package import information.
type importMap map[string]string

// AddPackage adds a regular dependency on pkg; some block of generated code
// will reference the pkg.
func (im importMap) AddPackage(genPath, pkgName string) {
	im[genPath] = pkgName
}

// AddForcedPackage adds a "forced" dependency on a pkg.  This means that we need
// to import pkg even if no other block of generated code references the pkg.
func (im importMap) AddForcedPackage(genPath string) {
	if im[genPath] == "" {
		im[genPath] = "_"
	}
}

func (im importMap) DeletePackage(pkg *compile.Package) {
	delete(im, pkg.GenPath)
}

func (im importMap) Sort() []goImport {
	var sortedPaths []string
	for path := range im {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)
	seen := make(map[string]bool)
	var ret []goImport
	for _, path := range sortedPaths {
		ret = append(ret, uniqueImport(im[path], path, seen))
	}
	return ret
}

// Each import must end up with a unique local name.  Here's some examples.
//   uniqueImport("a", "v.io/a", {})           -> goImport{"", "v.io/a", "a"}
//   uniqueImport("z", "v.io/a", {})           -> goImport{"", "v.io/a", "z"}
//   uniqueImport("a", "v.io/a", {"a"})        -> goImport{"a_2", "v.io/a", "a_2"}
//   uniqueImport("a", "v.io/a", {"a", "a_2"}) -> goImport{"a_3", "v.io/a", "a_3"}
//   uniqueImport("_", "v.io/a", {})           -> goImport{"_", "v.io/a", ""}
//   uniqueImport("_", "v.io/a", {"a"})        -> goImport{"_", "v.io/a", ""}
//   uniqueImport("_", "v.io/a", {"a", "a_2"}) -> goImport{"_", "v.io/a", ""}
func uniqueImport(pkgName, pkgPath string, seen map[string]bool) goImport {
	if pkgName == "_" {
		// This is a forced import that isn't otherwise used.
		return goImport{"_", pkgPath, ""}
	}
	name, iter := "", 1
	// Add special-cases to always use named imports for time and math, since they
	// are easily mistaken for the go standard time and math packages. Also,
	// the introduction of go modules confuses v23 as a package version vs
	// a package name, so it needs to be imported as named import to avoid
	// confusing lint tools etc.
	switch pkgPath {
	case "v.io/v23/vdlroot/time":
		name, pkgName = "vdltime", "vdltime"
	case "v.io/v23/vdlroot/math":
		name, pkgName = "vdlmath", "vdlmath"
	case "v.io/v23":
		name, pkgName = "v23", "v23"
	}
	for {
		local := pkgName
		if iter > 1 {
			local += "_" + strconv.Itoa(iter)
			name = local
		}
		if !seen[local] {
			// Found a unique local name - return the import.
			seen[local] = true
			return goImport{name, pkgPath, local}
		}
		iter++
	}
}

// LookupLocal returns the local identifier within the generated go file that
// identifies the given pkgPath.
func (imports goImports) LookupLocal(pkgPath string) string {
	ix := sort.Search(
		len(imports),
		func(i int) bool { return imports[i].Path >= pkgPath },
	)
	if ix < len(imports) && imports[ix].Path == pkgPath {
		return imports[ix].Local
	}
	return ""
}
