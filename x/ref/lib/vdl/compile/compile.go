// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package compile implements the VDL compiler, converting a parse tree into
// compiled results.  The CompilePackage function is the main entry point.
package compile

// The job of the compiler is to take parse results as input, and output
// compiled results.  The concepts between the parser and compiler are very
// similar, thus the naming of parse/compile results is also similar.
// E.g. parse.File represents a parsed file, while compile.File represents a
// compiled file.
//
// The flow of the compiler is contained in the Compile function below, and
// basically defines one concept across all files in the package before moving
// onto the next concept.  E.g. we define all types in the package before
// defining all consts in the package.
//
// The logic for simple concepts (e.g. imports) is contained directly in this
// file, while more complicated concepts (types, consts and interfaces) each get
// their own file.

import (
	"path/filepath"
	"sort"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/parse"
)

// CompilePackage compiles a list of parse.Files into a Package.  Updates env
// with the compiled package and returns it on success, or returns nil and
// guarantees !env.Errors.IsEmpty().  All imports that the parsed package depend
// on must already have been compiled and populated into env.
//nolint:golint // API change required.
func CompilePackage(pkgpath, genpath string, pfiles []*parse.File, config vdltool.Config, env *Env) *Package {
	if pkgpath == "" {
		env.Errors.Errorf("Compile called with empty pkgpath")
		return nil
	}
	if env.pkgs[pkgpath] != nil {
		env.Errors.Errorf("%q invalid recompile (already exists in env)", pkgpath)
		return nil
	}
	pkg := compile(pkgpath, genpath, pfiles, config, env)
	if pkg == nil {
		return nil
	}
	if computeDeps(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	env.pkgs[pkg.Path] = pkg
	return pkg
}

// CompileConfig compiles a parse.Config into a value.  Returns the compiled
// value on success, or returns nil and guarantees !env.Errors.IsEmpty().  All
// imports that the parsed config depend on must already have been compiled and
// populated into env.  If t is non-nil, the returned value will be of that
// type.
//nolint:golint // API change required.
func CompileConfig(t *vdl.Type, pconfig *parse.Config, env *Env) *vdl.Value {
	if pconfig == nil || env == nil {
		env.Errors.Errorf("CompileConfig called with nil config or env")
		return nil
	}
	// Since the concepts are so similar between config files and vdl files, we
	// just compile it as a single-file vdl package, and compile the exported
	// config const to retrieve the final exported config value.
	pfile := &parse.File{
		BaseName:   filepath.Base(pconfig.FileName),
		PackageDef: pconfig.ConfigDef,
		Imports:    pconfig.Imports,
		ConstDefs:  pconfig.ConstDefs,
	}
	pkgpath := filepath.ToSlash(filepath.Dir(pconfig.FileName))
	pkg := compile(pkgpath, pkgpath, []*parse.File{pfile}, vdltool.Config{}, env)
	if pkg == nil {
		return nil
	}
	config := compileConst("config", t, pconfig.Config, pkg.Files[0], env)
	// Wait to compute deps after we've compiled the config const expression,
	// since it might include the only usage of some of the imports.
	if computeDeps(pkg, env); !env.Errors.IsEmpty() {
		return nil
	}
	return config
}

// CompileExpr compiles expr into a value.  Returns the compiled value on
// success, or returns nil and guarantees !env.Errors.IsEmpty().  All imports
// that expr depends on must already have been compiled and populated into env.
// If t is non-nil, the returned value will be of that type.
//nolint:golint // API change required.
func CompileExpr(t *vdl.Type, expr parse.ConstExpr, env *Env) *vdl.Value {
	file := &File{
		BaseName: "_expr.vdl",
		Package:  newPackage("_expr", "_expr", "_expr", vdltool.Config{}),
		imports:  make(map[string]*importPath),
	}
	// Add imports to the "File" if the are in env and used in the Expression.
	for _, pkg := range parse.ExtractExprPackagePaths(expr) {
		if env.pkgs[pkg] != nil {
			file.imports[pkg] = &importPath{path: pkg}
		}
	}
	return compileConst("expression", t, expr, file, env)
}

func compile(pkgpath, genpath string, pfiles []*parse.File, config vdltool.Config, env *Env) *Package {
	if len(pfiles) == 0 {
		env.Errors.Errorf("%q compile called with no files", pkgpath)
		return nil
	}
	// Initialize each file and put it in pkg.
	pkgName := parse.InferPackageName(pfiles, env.Errors)
	if _, err := validIdent(pkgName, reservedNormal); err != nil {
		env.Errors.Errorf("package %s invalid name (%s)", pkgName, err.Error())
		return nil
	}
	pkg := newPackage(pkgName, pkgpath, genpath, config)
	for _, pfile := range pfiles {
		pkg.Files = append(pkg.Files, &File{
			BaseName:   pfile.BaseName,
			PackageDef: NamePos(pfile.PackageDef),
			Package:    pkg,
			imports:    make(map[string]*importPath),
		})
	}
	// Compile our various structures.  The order of these operations matters;
	// e.g. we must compile types before consts, since consts may use a type
	// defined in this package.
	if compileFileDoc(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	if compileImports(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	if compileTypeDefs(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	if compileErrorDefs(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	if compileConstDefs(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	if compileInterfaces(pkg, pfiles, env); !env.Errors.IsEmpty() {
		return nil
	}
	return pkg
}

func compileFileDoc(pkg *Package, pfiles []*parse.File, env *Env) {
	for index := range pfiles {
		file, pfile := pkg.Files[index], pfiles[index]
		if index == 0 {
			pkg.FileDoc = pfile.Doc
		} else if pkg.FileDoc != pfile.Doc {
			// We force all file-doc to be the same, since *.vdl files aren't 1-to-1
			// with the generated files in each language, e.g. Java creates one file
			// per class, while Javascript creates a single file for the entire
			// package.  For the common-case where we use file-doc for copyright
			// headers, it also prevents the user from accidentally adding copyright
			// headers to one file but not another, in the same package.
			env.Errorf(file, parse.Pos{Line: 1, Col: 1}, "all files in a package must have the same file doc (the comment on the first line of each *.vdl file that isn't package doc)")
		}
	}
}

func compileImports(pkg *Package, pfiles []*parse.File, env *Env) {
	for index := range pfiles {
		file, pfile := pkg.Files[index], pfiles[index]
		for _, pimp := range pfile.Imports {
			if dep := env.ResolvePackage(pimp.Path); dep == nil {
				env.Errorf(file, pimp.Pos, "import path %q not found", pimp.Path)
			}
			local := pimp.LocalName()
			if dup := file.imports[local]; dup != nil {
				env.Errorf(file, pimp.Pos, "import %s reused (previous at %s)", local, dup.pos)
				continue
			}
			file.imports[local] = &importPath{pimp.Path, pimp.Pos, false}
		}
	}
}

// TODO(toddw): Remove this function and all helpers, after all code generators
// have been updated to compute their own dependencies.  The only code that will
// remain below this point is the loop checking for unused imports.
func computeDeps(pkg *Package, env *Env) { //nolint:gocyclo
	// Check for unused user-supplied imports.
	for _, file := range pkg.Files {
		for _, imp := range file.imports {
			if !imp.used {
				env.Errorf(file, imp.pos, "import path %q unused", imp.path)
			}
		}
	}
	// Compute type and package dependencies per-file, based on the types and
	// interfaces that are actually used.  We ignore const dependencies, since
	// we've already evaluated the const expressions.
	for _, file := range pkg.Files {
		tdeps := make(map[*vdl.Type]bool)
		pdeps := make(map[*Package]bool)
		// TypeDef.Type is always defined in our package; start with sub types.
		for _, def := range file.TypeDefs {
			addSubTypeDeps(def.Type, pkg, env, tdeps, pdeps)
		}
		// Consts contribute their value types.
		for _, def := range file.ConstDefs {
			addValueTypeDeps(def.Value, pkg, env, tdeps, pdeps)
		}
		// Interfaces contribute their arg types and tag values, as well as embedded
		// interfaces.
		for _, iface := range file.Interfaces {
			for _, embed := range iface.TransitiveEmbeds() {
				pdeps[embed.File.Package] = true
			}
			for _, method := range iface.Methods {
				for _, arg := range method.InArgs {
					addTypeDeps(arg.Type, pkg, env, tdeps, pdeps)
				}
				for _, arg := range method.OutArgs {
					addTypeDeps(arg.Type, pkg, env, tdeps, pdeps)
				}
				if stream := method.InStream; stream != nil {
					addTypeDeps(stream, pkg, env, tdeps, pdeps)
				}
				if stream := method.OutStream; stream != nil {
					addTypeDeps(stream, pkg, env, tdeps, pdeps)
				}
				for _, tag := range method.Tags {
					addValueTypeDeps(tag, pkg, env, tdeps, pdeps)
				}
			}
		}
		// Errors contribute their param types.
		for _, def := range file.ErrorDefs {
			for _, param := range def.Params {
				addTypeDeps(param.Type, pkg, env, tdeps, pdeps)
			}
		}
		file.TypeDeps = tdeps
		// Now remove self and built-in package dependencies.  Every package can use
		// itself and the built-in package, so we don't need to record this.
		delete(pdeps, pkg)
		delete(pdeps, BuiltInPackage)
		// Finally populate PackageDeps and sort by package path.
		file.PackageDeps = make([]*Package, 0, len(pdeps))
		for pdep := range pdeps {
			file.PackageDeps = append(file.PackageDeps, pdep)
		}
		sort.Sort(pkgSorter(file.PackageDeps))
	}
}

// Add immediate package deps for t and subtypes of t.
func addTypeDeps(t *vdl.Type, pkg *Package, env *Env, tdeps map[*vdl.Type]bool, pdeps map[*Package]bool) {
	if def := env.typeMap[t]; def != nil {
		// We don't track transitive dependencies, only immediate dependencies.
		tdeps[t] = true
		pdeps[def.File.Package] = true
		if t == vdl.TypeObjectType {
			// Special-case: usage of typeobject implies usage of any, since the zero
			// value for typeobject is any.
			addTypeDeps(vdl.AnyType, pkg, env, tdeps, pdeps)
		}
		return
	}
	// Not all types have TypeDefs; e.g. unnamed lists have no corresponding
	// TypeDef, so we need to traverse those recursively.
	addSubTypeDeps(t, pkg, env, tdeps, pdeps)
}

// Add immediate package deps for subtypes of t.
func addSubTypeDeps(t *vdl.Type, pkg *Package, env *Env, tdeps map[*vdl.Type]bool, pdeps map[*Package]bool) {
	switch t.Kind() {
	case vdl.Array, vdl.List:
		addTypeDeps(t.Elem(), pkg, env, tdeps, pdeps)
	case vdl.Set:
		addTypeDeps(t.Key(), pkg, env, tdeps, pdeps)
	case vdl.Map:
		addTypeDeps(t.Key(), pkg, env, tdeps, pdeps)
		addTypeDeps(t.Elem(), pkg, env, tdeps, pdeps)
	case vdl.Struct, vdl.Union:
		for ix := 0; ix < t.NumField(); ix++ {
			addTypeDeps(t.Field(ix).Type, pkg, env, tdeps, pdeps)
		}
	}
}

// Add immediate package deps for v.Type(), and subvalues.  We must traverse the
// value to know which types are actually used; e.g. an empty struct doesn't
// have a dependency on its field types.
//
// The purpose of this method is to identify the package and type dependencies
// for const or tag values.
func addValueTypeDeps(v *vdl.Value, pkg *Package, env *Env, tdeps map[*vdl.Type]bool, pdeps map[*Package]bool) {
	t := v.Type()
	if def := env.typeMap[t]; def != nil {
		tdeps[t] = true
		pdeps[def.File.Package] = true
		// Fall through to track transitive dependencies, based on the subvalues.
	}
	// Traverse subvalues recursively.
	switch t.Kind() {
	case vdl.Array, vdl.List:
		for ix := 0; ix < v.Len(); ix++ {
			addValueTypeDeps(v.Index(ix), pkg, env, tdeps, pdeps)
		}
	case vdl.Set, vdl.Map:
		for _, key := range v.Keys() {
			addValueTypeDeps(key, pkg, env, tdeps, pdeps)
			if t.Kind() == vdl.Map {
				addValueTypeDeps(v.MapIndex(key), pkg, env, tdeps, pdeps)
			}
		}
	case vdl.Struct:
		// There are no subvalues to track if the value is 0.
		if v.IsZero() {
			return
		}
		for ix := 0; ix < t.NumField(); ix++ {
			addValueTypeDeps(v.StructField(ix), pkg, env, tdeps, pdeps)
		}
	case vdl.Union:
		_, field := v.UnionField()
		addValueTypeDeps(field, pkg, env, tdeps, pdeps)
	case vdl.Any, vdl.Optional:
		if elem := v.Elem(); elem != nil {
			addValueTypeDeps(elem, pkg, env, tdeps, pdeps)
		}
	case vdl.TypeObject:
		// TypeObject has dependencies on everything its zero value depends on.
		addValueTypeDeps(vdl.ZeroValue(v.TypeObject()), pkg, env, tdeps, pdeps)
	}
}

// pkgSorter implements sort.Interface, sorting by package path.
type pkgSorter []*Package

func (s pkgSorter) Len() int           { return len(s) }
func (s pkgSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s pkgSorter) Less(i, j int) bool { return s[i].Path < s[j].Path }
