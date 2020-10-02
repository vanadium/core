// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package swift implements Swift code generation from compiled VDL packages.
package swift

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const V23SwiftFrameworkName = "VanadiumCore"

var (
	memoizedSwiftModule = map[string]string{}
	memoizedPackageName = map[*compile.Package]string{}
)

func (ctx *swiftContext) pkgIsSameModule(pkg *compile.Package) bool {
	pkgModule := ctx.swiftModule(pkg)
	ctxModule := ctx.swiftModule(ctx.pkg)
	return pkgModule == ctxModule
}

func (ctx *swiftContext) swiftModule(pkg *compile.Package) string {
	fileDir := ctx.genPathToDir[pkg.GenPath]
	if fileDir == "" {
		if pkg.GenPath == "_builtin" {
			return ""
		}
		panic(fmt.Sprintf("Unable to find source directory for genPath %q", pkg.GenPath))
	}
	if !strings.HasPrefix(fileDir, "/") {
		if strings.HasPrefix(pkg.GenPath, "v.io/v23/vdlroot") {
			panic("You must have the VDLROOT env var to generate VanadiumCore swift files")
		}
		panic(fmt.Sprintf("Unsupported file system for dir path: %v", fileDir))
	}
	if swiftModule, ok := memoizedSwiftModule[fileDir]; ok {
		return swiftModule
	}
	_, module := ctx.findSwiftModule(pkg)
	memoizedSwiftModule[fileDir] = module
	return module
}

func (ctx *swiftContext) findSwiftModule(pkg *compile.Package) (path string, module string) {
	fileDir := ctx.genPathToDir[pkg.GenPath]
	for {
		configPath := filepath.Join(fileDir, "swiftmodule")
		module, err := ioutil.ReadFile(configPath)
		if err == nil && module != nil {
			return fileDir, strings.TrimSpace(string(module))
		}
		foundInSrcDirs := ctx.srcDirs[fileDir]
		if foundInSrcDirs && strings.Contains(fileDir, "v.io/v23/vdlroot") {
			// Special case VDLROOT which will be in a srcDir (so normally
			// it would stop here), but the swiftmodule file is defined below it.
			foundInSrcDirs = false
		}
		if fileDir == "/" || fileDir == "" || foundInSrcDirs {
			// We're at the root, nothing else to check.
			return "", ""
		}
		// Go up a level
		s := strings.Split(fileDir, "/")
		s = s[:len(s)-1]
		fileDir = "/" + filepath.Join(s...)
	}
}

// swiftPackageName returns a Swift package name that is a mashup of this
// package name and its path ancestors. It traverses towards the root package
// until it finds the Swift Module, and concatenates the child paths.
// For example, v.io/v23/syncbase/nosql => SyncbaseNosql where v23 forms the root.
// If no Swift Module is found, we presume the entire path is valid and the
// entirety becomes concatenated using the pkg.Path. This exception allows
// for third-party code to conservatively work (since we can assume nothing
// about the root package we don't want to throw away information) and
// vdlroot's packages to be correctly named (signature => signature).
func (ctx *swiftContext) swiftPackageName(pkg *compile.Package) string {
	if name, ok := memoizedPackageName[pkg]; ok {
		return name
	}
	// Remove the package path of the swift module from our pkg.Path
	// First find that package path
	moduleDir, _ := ctx.findSwiftModule(pkg)
	modulePkgPath := "__INVALID__"
	if moduleDir != "" {
		// moduleDir is the full path to our swiftmodule file
		// find the VDLROOT/VDLPATH that contains this
		srcDirs := build.SrcDirs(ctx.env.Errors)
		for _, dir := range srcDirs {
			if strings.HasPrefix(moduleDir, dir) {
				// We found the VDLPATH/VDLROOT that corresponds to this SwiftModule.
				// So lop off the VDLPATH. For example:
				// Before moduleDir = /Users/aaron/v23/release/go/src/v.io/v23/services
				// Before dir (srcDir) = /Users/aaron/v23/release/go/src
				modulePkgPath = strings.TrimPrefix(moduleDir, dir)
				modulePkgPath = strings.TrimPrefix(modulePkgPath, "/")
				// After: modulePkgPath = v.io/v23/services
				break
			}
		}
		if modulePkgPath == "__INVALID__" {
			log.Fatalf("Could not find pkg path for module in dir %v", moduleDir)
		}
	}
	// Remove module package path, for example:
	// Before pkg.Path = v.io/v23/services/syncbase
	// Before modulePkgPath = v.io/v23/services
	modulePkgPath = strings.TrimPrefix(pkg.Path, modulePkgPath)
	modulePkgPath = strings.TrimPrefix(modulePkgPath, "/")
	// After modulePkgPath = syncbase
	// CamelCaseThePath
	name := ""
	for _, pkg := range strings.Split(modulePkgPath, "/") {
		// Remove any periods (e.g. v.io/ -> Vio)
		pkg = strings.ReplaceAll(pkg, ".", "")
		name += vdlutil.FirstRuneToUpper(pkg)
	}
	memoizedPackageName[pkg] = name
	return name
}

func (ctx *swiftContext) importedModules(tdefs []*compile.TypeDef) []string {
	// Find the transitive closure of tdefs' dependencies.
	walkedTdefs := map[*compile.TypeDef]bool{}
	modules := map[string]bool{}
	for _, tdef := range tdefs {
		walkTdef(tdef, ctx.env, walkedTdefs)
	}
	// Extract the modules that aren't the same as the context's
	for tdef := range walkedTdefs {
		module := ctx.swiftModule(tdef.File.Package)
		if module != "" && !ctx.pkgIsSameModule(tdef.File.Package) {
			modules[module] = true
			continue
		}
	}
	if ctx.swiftModule(ctx.pkg) != V23SwiftFrameworkName {
		// Make sure VanadiumCore is imported as that's where we define VError, VdlPrimitive, etc
		modules[V23SwiftFrameworkName] = true
	}
	return flattenImportedModuleSet(modules)
}

func walkTdef(tdef *compile.TypeDef, env *compile.Env, seen map[*compile.TypeDef]bool) {
	// If this typedef has a native type, then we can ignore the import because
	// that information is replaced anyway.
	pkg := tdef.File.Package
	if _, ok := pkg.Config.Swift.WireToNativeTypes[tdef.Name]; ok {
		return
	}
	seen[tdef] = true
	switch tdef.Type.Kind() {
	case vdl.Struct, vdl.Union:
		for i := 0; i < tdef.Type.NumField(); i++ {
			fld := tdef.Type.Field(i)
			if innerTdef := env.FindTypeDef(fld.Type); innerTdef != nil {
				walkTdef(innerTdef, env, seen)
			}
		}

	case vdl.List, vdl.Array, vdl.Map, vdl.Optional:
		if innerTdef := env.FindTypeDef(tdef.Type.Elem()); innerTdef != nil {
			walkTdef(innerTdef, env, seen)
		}
	}
	switch tdef.Type.Kind() {
	case vdl.Map, vdl.Set:
		if innerTdef := env.FindTypeDef(tdef.Type.Key()); innerTdef != nil {
			walkTdef(innerTdef, env, seen)
		}
	}
}

func flattenImportedModuleSet(modules map[string]bool) []string {
	imports := make([]string, len(modules))
	i := 0
	for moduleName := range modules {
		if moduleName != "" {
			imports[i] = moduleName
			i++
		}
	}
	// Make it deterministic
	sort.Strings(imports)
	return imports
}
