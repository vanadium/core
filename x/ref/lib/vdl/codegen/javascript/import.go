// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"sort"
	"strconv"

	"v.io/x/ref/lib/vdl/compile"
)

// TODO(bjornick): Merge with pkg_types.go

// jsImport represents a single package import.
type jsImport struct {
	Path string // Path of the imported package; e.g. "v.io/x/ref/lib/vdl/testdata/arith"

	// Local name that refers to the imported package; either the non-empty import
	// name, or the name of the imported package.
	Local string
}

// jsImports is a collection of package imports.
// REQUIRED: The imports must be sorted by path.
type jsImports []jsImport

// LookupLocal returns the local name that identifies the given pkgPath.
func (x jsImports) LookupLocal(pkgPath string) string {
	ix := sort.Search(len(x), func(i int) bool { return x[i].Path >= pkgPath })
	if ix < len(x) && x[ix].Path == pkgPath {
		return x[ix].Local
	}
	return ""
}

// Each import must end up with a unique local name - when we see a collision we
// simply add a "_N" suffix where N starts at 2 and increments.
func uniqueImport(pkgName, pkgPath string, seen map[string]bool) jsImport {
	iter := 1
	for {
		local := pkgName
		if iter > 1 {
			local += "_" + strconv.Itoa(iter)
		}
		if !seen[local] {
			// Found a unique local name - return the import.
			seen[local] = true
			return jsImport{pkgPath, local}
		}
		iter++
	}
}

type pkgSorter []*compile.Package

func (s pkgSorter) Len() int { return len(s) }

func (s pkgSorter) Less(i, j int) bool { return s[i].Path < s[j].Path }

func (s pkgSorter) Swap(i, j int) { s[j], s[i] = s[i], s[j] }

// jsImportsForFiles returns the imports required for the given files.
func jsImportsForFiles(files ...*compile.File) jsImports {
	seenPath := make(map[string]bool)
	pkgs := pkgSorter{}

	for _, f := range files {
		// TODO(toddw,bjornick): Remove File.PackageDeps and replace by walking through each type.
		for _, dep := range f.PackageDeps {
			if seenPath[dep.Path] {
				continue
			}
			seenPath[dep.Path] = true
			pkgs = append(pkgs, dep)
		}
	}
	sort.Sort(pkgs)

	var ret jsImports
	seenName := make(map[string]bool)
	for _, dep := range pkgs {
		ret = append(ret, uniqueImport(dep.Name, dep.GenPath, seenName))
	}
	return ret
}
