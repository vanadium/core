// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/textutil"
	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/codegen/vdlgen"
	"v.io/x/ref/lib/vdl/compile"
)

var cmdGen = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runGen),
	Name:   "vdltestgen",
	Short:  "generates test data for the vdltest package",
	Long: `
Command vdltestgen generates types and values for the vdltest package.  The
following files are generated:

   vtype_gen.vdl       - Named "V" types, regular VDL types.
   ventry_pass_gen.vdl - Entries that pass conversion from source to target.
   ventry_fail_gen.vdl - Entries that fail conversion from source to target.

   xtype_gen.vdl       - Named "X" types, no VDL{IsZero,Read,Write} methods.
   xentry_pass_gen.vdl - Entries that pass conversion from source to target.
   xentry_fail_gen.vdl - Entries that fail conversion from source to target.

Do not run this tool manually.  Instead invoke it via:
   $ go generate v.io/v23/vdl/vdltest
`,
}

func main() {
	cmdline.Main(cmdGen)
}

const (
	vdltestPkgName     = "v.io/v23/vdl/vdltest"
	typeGenFileName    = "type_gen.vdl"
	typeManualFileName = "type_manual.vdl"
	passGenFileName    = "entry_pass_gen.vdl"
	failGenFileName    = "entry_fail_gen.vdl"
)

func runGen(_ *cmdline.Env, _ []string) error {
	// Build the vdltest package, to pick up manually-generated types and entries.
	vdltestPkg, err := buildVDLTestPackage()
	if err != nil {
		return err
	}
	// Generate "v" types and entries.
	vGenTypes := genAndWriteTypes("v")
	vManTypes := collectManualTypes("v", vdltestPkg)
	vPass, vFail := genEntries(vGenTypes, vGenTypes)
	pass, fail := genEntries(vManTypes, append(vManTypes, vGenTypes...))
	writeEntries("v", append(vPass, pass...), append(vFail, fail...))
	// Generate "x" types and entries, skipping entries already covered by "v".
	xGenTypes := genAndWriteTypes("x")
	xManTypes := collectManualTypes("x", vdltestPkg)
	xTargetTypes := subtractTypes(xGenTypes, vGenTypes)
	xPass, xFail := genEntries(xTargetTypes, xGenTypes)
	pass, fail = genEntries(xManTypes, append(xManTypes, xGenTypes...))
	writeEntries("x", append(xPass, pass...), append(xFail, fail...))
	return nil
}

func buildVDLTestPackage() (*compile.Package, error) {
	env := compile.NewEnv(-1)
	pkgs := build.TransitivePackages([]string{vdltestPkgName}, build.UnknownPathIsError, build.Opts{}, env.Errors, env.Warnings)
	if !env.Errors.IsEmpty() {
		return nil, env.Errors.ToError()
	}
	for _, pkg := range pkgs {
		build.BuildPackage(pkg, env)
	}
	return env.ResolvePackage(vdltestPkgName), env.Errors.ToError()
}

// genAndWriteTypes generates types and writes the type file.
func genAndWriteTypes(prefix string) []*vdl.Type {
	const maxTypeDepth = 3
	gen := vdltest.NewTypeGenerator()
	gen.RandSeed(1)
	gen.NamePrefix = strings.ToUpper(prefix)
	genTypes := gen.Gen(maxTypeDepth)
	writeTypeFile(prefix+typeGenFileName, genTypes)
	return genTypes
}

// collectManualTypes collects manually-defined types from pkg.
func collectManualTypes(prefix string, pkg *compile.Package) []*vdl.Type {
	var manTypes []*vdl.Type
	for _, file := range pkg.Files {
		if file.BaseName != prefix+typeManualFileName {
			continue
		}
		for _, def := range file.TypeDefs {
			manTypes = append(manTypes, def.Type)
		}
	}
	return manTypes
}

// subtractTypes returns all types that don't appear in sub.
func subtractTypes(types, sub []*vdl.Type) []*vdl.Type {
	subMap := make(map[*vdl.Type]bool)
	for _, tt := range sub {
		subMap[tt] = true
	}
	var result []*vdl.Type
	for _, tt := range types {
		if !subMap[tt] {
			result = append(result, tt)
		}
	}
	return result
}

// genEntries generates entries for all target types, using source types as the
// candidates to mimic conversion values.
func genEntries(target, source []*vdl.Type) (pass, fail []vdltest.EntryValue) {
	gen := vdltest.NewEntryGenerator(source)
	gen.RandSeed(1)
	pass = gen.GenAllPass(target)
	gen.RandSeed(1)
	fail = gen.GenAllFail(target)
	return
}

func writeEntries(prefix string, pass, fail []vdltest.EntryValue) {
	writeEntryFile(prefix+passGenFileName, prefix+"AllPass", pass)
	writeEntryFile(prefix+failGenFileName, prefix+"AllFail", fail)
}

// This tool is only used to generate test cases for the vdltest package, so the
// strategy is to panic on any error, to make the code simpler.
func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func createFile(name string) (*os.File, func()) {
	file, err := os.Create(name)
	panicOnError(err)
	return file, func() { panicOnError(file.Close()) }
}

func writef(w io.Writer, format string, args ...interface{}) {
	_, err := fmt.Fprintf(w, format, args...)
	panicOnError(err)
}

func writeHeader(w io.Writer) {
	writef(w, `// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by v.io/v23/vdl/vdltest/internal/vdltestgen
// Run the following to re-generate:
//   $ go generate v.io/v23/vdl/vdltest

//nolint:golint
package vdltest

`)
}

func writeTypeFile(fileName string, types []*vdl.Type) {
	fmt.Printf("Writing %[1]s:\t%[2]d types\n", fileName, len(types))
	file, cleanup := createFile(fileName)
	defer cleanup()
	writeHeader(file)
	comment := textutil.PrefixLineWriter(file, "// ")
	panicOnError(vdltest.PrintTypeStats(comment, types...))
	writef(comment, "\nOnly named types appear below, by definition.\n")
	panicOnError(comment.Flush())
	writef(file, "\ntype (\n")
	for _, tt := range types {
		if tt.Name() != "" {
			base := vdlgen.BaseType(tt, vdltestPkgName, nil)
			base = strings.ReplaceAll(base, "\n", "\n\t")
			writef(file, "\t%[1]s %[2]s\n", tt.Name(), base)
		}
	}
	writef(file, ")\n")
}

func writeEntryFile(fileName, constName string, entries []vdltest.EntryValue) {
	fmt.Printf("Writing %[1]s:\t%[2]d entries\n", fileName, len(entries))
	file, cleanup := createFile(fileName)
	defer cleanup()
	writeHeader(file)
	comment := textutil.PrefixLineWriter(file, "// ")
	panicOnError(vdltest.PrintEntryStats(comment, entries...))
	panicOnError(comment.Flush())
	writef(file, "\nconst %[1]s = []vdlEntry{\n", constName)
	for _, e := range entries {
		if e.IsCanonical() {
			writef(file, "\t// Canonical\n")
		}
		target := vdlgen.TypedConst(e.Target, vdltestPkgName, nil)
		source := vdlgen.TypedConst(e.Source, vdltestPkgName, nil)
		if len(target)*2+len(source)*2 < 100 {
			// Write a pretty one-liner, if it's short enough.
			writef(file, "\t{ %[1]v, %#[2]q, %#[3]q, %[3]s, %#[4]q, %[4]s },\n", e.IsCanonical(), e.Label, target, source)
		} else {
			writef(file, "\t{ %[1]v, %#[2]q,\n\t  %#[3]q,\n\t  %[3]s,\n\t  %#[4]q,\n\t  %[4]s,\n\t},\n", e.IsCanonical(), e.Label, target, source)
		}
	}
	writef(file, "}\n")
}
