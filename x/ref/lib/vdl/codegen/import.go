// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package codegen implements utilities for VDL code generators.  Code
// generators for specific languages live in sub-directories.
package codegen

import (
	"path"
	"sort"
	"strconv"

	"v.io/v23/vdl"
	"v.io/x/lib/set"
	"v.io/x/ref/lib/vdl/compile"
)

// TODO(toddw): Remove this file, after all code generators have been updated to
// compute their own import dependencies.

// Import represents a single package import.
type Import struct {
	Name string // Name of the import; may be empty.
	Path string // Path of the imported package; e.g. "v.io/x/ref/lib/vdl/testdata/arith"

	// Local name that refers to the imported package; either the non-empty import
	// name, or the name of the imported package.
	Local string
}

// Imports is a collection of package imports.
// REQUIRED: The imports must be sorted by path.
type Imports []Import

// LookupLocal returns the local name that identifies the given pkgPath.
func (x Imports) LookupLocal(pkgPath string) string {
	ix := sort.Search(len(x), func(i int) bool { return x[i].Path >= pkgPath })
	if ix < len(x) && x[ix].Path == pkgPath {
		return x[ix].Local
	}
	return ""
}

// Each import must end up with a unique local name - when we see a collision we
// simply add a "_N" suffix where N starts at 2 and increments.
func uniqueImport(pkgName, pkgPath string, seen map[string]bool) Import {
	name := ""
	iter := 1
	for {
		local := pkgName
		if iter > 1 {
			local += "_" + strconv.Itoa(iter)
			name = local
		}
		if !seen[local] {
			// Found a unique local name - return the import.
			seen[local] = true
			return Import{name, pkgPath, local}
		}
		iter++
	}
}

type pkgSorter []*compile.Package

func (s pkgSorter) Len() int { return len(s) }

func (s pkgSorter) Less(i, j int) bool { return s[i].Path < s[j].Path }

func (s pkgSorter) Swap(i, j int) { s[j], s[i] = s[i], s[j] }

// ImportsForFiles returns the imports required for the given files.
func ImportsForFiles(files ...*compile.File) Imports {
	seenPath := make(map[string]bool)
	pkgs := pkgSorter{}

	for _, f := range files {
		for _, dep := range f.PackageDeps {
			if seenPath[dep.Path] {
				continue
			}
			seenPath[dep.Path] = true
			pkgs = append(pkgs, dep)
		}
	}
	sort.Sort(pkgs)

	var ret Imports
	seenName := make(map[string]bool)
	for _, dep := range pkgs {
		ret = append(ret, uniqueImport(dep.Name, dep.Path, seenName))
	}
	return ret
}

// ImportsForValue returns the imports required to represent the given value v,
// from within the given pkgPath.  It requires that type names used in
// v are of the form "PKGPATH.NAME".
func ImportsForValue(v *vdl.Value, pkgPath string) Imports {
	deps := pkgDeps{}
	deps.MergeValue(v)
	var ret Imports
	seen := make(map[string]bool)
	for _, p := range deps.SortedPkgPaths() {
		if p != pkgPath {
			ret = append(ret, uniqueImport(path.Base(p), p, seen))
		}
	}
	return ret
}

// pkgDeps maintains a set of package path dependencies.
type pkgDeps map[string]bool

func (deps pkgDeps) insertIdent(ident string) {
	if pkgPath, _ := vdl.SplitIdent(ident); pkgPath != "" {
		deps[pkgPath] = true
	}
}

// MergeValue merges the package paths required to represent v into deps.
func (deps pkgDeps) MergeValue(v *vdl.Value) {
	deps.insertIdent(v.Type().Name())
	switch v.Kind() {
	case vdl.Any, vdl.Union, vdl.Optional:
		elem := v.Elem()
		if elem != nil {
			deps.MergeValue(elem)
		}
	case vdl.Array, vdl.List:
		deps.insertIdent(v.Type().Elem().Name())
	case vdl.Set:
		deps.insertIdent(v.Type().Key().Name())
	case vdl.Map:
		deps.insertIdent(v.Type().Key().Name())
		deps.insertIdent(v.Type().Elem().Name())
	case vdl.Struct:
		for fx := 0; fx < v.Type().NumField(); fx++ {
			deps.insertIdent(v.Type().Field(fx).Type.Name())
		}
	}
}

// SortedPkgPaths deps as a sorted slice.
func (deps pkgDeps) SortedPkgPaths() []string {
	ret := set.StringBool.ToSlice(deps)
	sort.Strings(ret)
	return ret
}
