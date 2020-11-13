// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !vdlbootstrapping

package main

import (
	"path/filepath"
	"strings"

	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/codegen/java"
	"v.io/x/ref/lib/vdl/codegen/javascript"
	"v.io/x/ref/lib/vdl/codegen/swift"
	"v.io/x/ref/lib/vdl/compile"
)

var (
	genLangAll = genLangs(vdltool.GenLanguageAll[:])

	optGenJavaOutDir = genOutDir{
		rules: xlateRules{
			{"release/go/src", "release/java/lib/generated-src/vdl"},
			{"roadmap/go/src", "release/java/lib/generated-src/vdl"},
		},
	}

	optGenJavascriptOutDir = genOutDir{
		rules: xlateRules{
			{"release/go/src", "release/javascript/core/src"},
			{"roadmap/go/src", "release/javascript/core/src"},
			{"third_party/go/src", "SKIP"},
			{"tools/go/src", "SKIP"},
			// TODO(toddw): Skip vdlroot javascript generation for now.
			{"release/go/src/v.io/v23/vdlroot", "SKIP"},
		},
	}
	optGenJavaOutPkg = xlateRules{
		{"v.io", "io/v"},
	}
	optGenSwiftOutDir = genOutDir{
		rules: xlateRules{
			{"release/go/src", "release/swift/lib/generated-src/vdl"},
			{"roadmap/go/src", "release/swift/lib/generated-src/vdl"},
		},
	}
	optGenSwiftOutPkg = xlateRules{
		{"v.io/x", "SKIP"},
		{"/internal", "SKIP"},
		{"/testdata", "SKIP"},
		{"v.io/v23", ""},
	}
	optPathToJSCore string
)

func init() {
	cmdGenerate.Flags.Var(&optGenJavaOutDir, "java-out-dir",
		"Same semantics as --go-out-dir but applies to java code generation.")
	cmdGenerate.Flags.Var(&optGenJavaOutPkg, "java-out-pkg", `
Java output package translation rules.  Must be of the form:
"src->dst[,s2->d2...]"
If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
commas to separate multiple rules; the first rule matching src is used, and if
there are no matching rules, the package remains unchanged.  The special dst
SKIP indicates those packages containing the string are skipped. Note this skip
behavior is slightly different than the -out-dir semantics which is prefix-based.`)
	cmdGenerate.Flags.Var(&optGenJavascriptOutDir, "js-out-dir",
		"Same semantics as --go-out-dir but applies to js code generation.")
	cmdGenerate.Flags.StringVar(&optPathToJSCore, "js-relative-path-to-core", "",
		"If set, this is the relative path from js-out-dir to the root of the JS core")
	cmdGenerate.Flags.Var(&optGenSwiftOutDir, "swift-out-dir",
		"Same semantics as --go-out-dir but applies to Swift code generation.")
	cmdGenerate.Flags.Var(&optGenSwiftOutPkg, "swift-out-pkg", `
Swift output package translation rules.  Must be of the form:
"src->dst[,s2->d2...]"
If a VDL package has a prefix src, the prefix will be replaced with dst.  Use
commas to separate multiple rules; the first rule matching src is used, and if
there are no matching rules, the package remains unchanged.  The special dst
SKIP indicates those packages containing the string are skipped. Note this skip
behavior is slightly different than the -out-dir semantics which is prefix-based.`)
	// Options for audit are identical to generate.
	cmdAudit.Flags = cmdGenerate.Flags
}

func genLanguageFromString(lang string) (vdltool.GenLanguage, error) {
	return vdltool.GenLanguageFromString(lang)
}

func shouldGenerate(config vdltool.Config, lang vdltool.GenLanguage) bool {
	// If config.GenLanguages is empty, all languages are allowed to be generated.
	_, ok := config.GenLanguages[lang]
	return len(config.GenLanguages) == 0 || ok
}

func handleLanguages(gl vdltool.GenLanguage, target *build.Package, audit bool, pkg *compile.Package, env *compile.Env, pathToDir map[string]string) bool {
	switch gl {
	case vdltool.GenLanguageGo:
		return handleGo(audit, pkg, env, target)
	case vdltool.GenLanguageJava:
		return handleJava(audit, pkg, env, target)
	case vdltool.GenLanguageJavascript:
		return handleJavaScript(audit, pkg, env, target)
	case vdltool.GenLanguageSwift:
		return handleSwift(audit, pkg, env, target, pathToDir)
	default:
		env.Errors.Errorf("Generating code for language %v isn't supported", gl)
	}
	return false
}

func handleJava(audit bool, pkg *compile.Package, env *compile.Env, target *build.Package) bool {
	if !shouldGenerate(pkg.Config, vdltool.GenLanguageJava) {
		return false
	}
	pkgPath, err := xlatePkgPath(pkg.GenPath, optGenJavaOutPkg)
	if handleErrorOrSkip("--java-out-pkg", err, env) {
		return false
	}
	dir, err := xlateOutDir(target.Dir, target.GenPath, optGenJavaOutDir, pkgPath)
	if handleErrorOrSkip("--java-out-dir", err, env) {
		return false
	}
	java.SetPkgPathXlator(func(pkgPath string) string {
		result, _ := xlatePkgPath(pkgPath, optGenJavaOutPkg)
		return result
	})
	changed := false
	for _, file := range java.Generate(pkg, env) {
		fileDir := filepath.Join(dir, file.Dir)
		if writeFile(audit, file.Data, fileDir, file.Name, env, nil) {
			changed = true
		}
	}
	return changed
}

func handleJavaScript(audit bool, pkg *compile.Package, env *compile.Env, target *build.Package) bool {
	if !shouldGenerate(pkg.Config, vdltool.GenLanguageJavascript) {
		return false
	}
	dir, err := xlateOutDir(target.Dir, target.GenPath, optGenJavascriptOutDir, pkg.GenPath)
	if handleErrorOrSkip("--js-out-dir", err, env) {
		return false
	}
	prefix := filepath.Clean(target.Dir[0 : len(target.Dir)-len(target.GenPath)])
	path := func(importPath string) string {
		pkgDir := filepath.Join(prefix, filepath.FromSlash(importPath))
		fullDir, err := xlateOutDir(pkgDir, importPath, optGenJavascriptOutDir, importPath)
		if err != nil {
			panic(err)
		}
		cleanPath, err := filepath.Rel(dir, fullDir)
		if err != nil {
			panic(err)
		}
		return cleanPath
	}
	data := javascript.Generate(pkg, env, path, optPathToJSCore)
	return writeFile(audit, data, dir, "index.js", env, nil)
}

func handleSwift(audit bool, pkg *compile.Package, env *compile.Env, target *build.Package, pathToDir map[string]string) bool {
	if !shouldGenerate(pkg.Config, vdltool.GenLanguageSwift) {
		return false
	}
	pkgPath, err := xlatePkgPath(pkg.GenPath, optGenSwiftOutPkg)
	if handleErrorOrSkip("--swift-out-pkg", err, env) {
		return false
	}
	dir, err := xlateOutDir(target.Dir, target.GenPath, optGenSwiftOutDir, pkgPath)
	if handleErrorOrSkip("--swift-out-dir", err, env) {
		return false
	}
	swift.SetPkgPathXlator(func(pkgPath string) string {
		result, _ := xlatePkgPath(pkgPath, optGenSwiftOutPkg)
		return result
	})
	changed := false
	for _, file := range swift.Generate(pkg, env, pathToDir) {
		if optGenSwiftOutDir.dir == "" {
			panic("optGenSwiftOurDir.Dir must be defined")
		}
		relativeDir := strings.TrimPrefix(dir, optGenSwiftOutDir.dir)
		fileDir := filepath.Join(optGenSwiftOutDir.dir, file.Module, relativeDir, file.Dir)
		if writeFile(audit, file.Data, fileDir, file.Name, env, nil) {
			changed = true
		}
	}
	return changed
}
