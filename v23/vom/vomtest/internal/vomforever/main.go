// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command vomforever is a tool that searches for bugs in vom.
// It vom encodes and decodes in a loop and reports the errors that are found.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
	"v.io/v23/vom"
	"v.io/x/lib/textutil"
	"v.io/x/ref/lib/vdl/codegen/vdlgen"
)

const (
	numValuesPerTypeList       = 100
	countBeforeIncreasingDepth = 1000
)

var (
	verboseFlag = flag.Bool("v", false, "verbose output")
	modeFlag    = flag.String("mode", "generated", "test mode")
)

type mode int

const (
	vdlValue mode = iota
	reflect
	generated
)

const vdlPackageName = "v.io/v23/vom"

type entry struct {
	Value *vdl.Value
	Types []*vdl.Type
}

func genEntries() chan entry {
	typegen := vdltest.NewTypeGenerator()
	out := make(chan entry, 1)
	go func() {
		depth := 0
		for i := 0; ; i++ {
			if i%countBeforeIncreasingDepth == 0 {
				depth++
			}
			types := typegen.Gen(depth)
			valgen := vdltest.NewValueGenerator(types)
			modes := []vdltest.GenMode{vdltest.GenFull, vdltest.GenPosMax, vdltest.GenNegMax, vdltest.GenPosMin, vdltest.GenNegMin, vdltest.GenRandom}
			for i := 0; i < numValuesPerTypeList; i++ {
				out <- entry{
					Value: valgen.Gen(types[rand.Intn(len(types))], modes[rand.Intn(len(modes))]),
					Types: types,
				}
			}
		}
	}()
	return out
}

func main() {
	flag.Parse()
	var mode mode
	switch *modeFlag {
	case "reflect":
		mode = reflect
	case "generated":
		mode = generated
	case "vdl-value":
		mode = vdlValue
	default:
		fmt.Printf(`invalid mode %q. expected "reflect", "vdl-value" or "generated"`, *modeFlag)
	}
	for vv := range genEntries() {
		if *verboseFlag {
			fmt.Printf("testing %v\n", vv)
		}
		run(vv, mode)

	}
}

func run(entry entry, mode mode) {
	fmt.Printf("Testing %v\n", entry.Value)
	switch mode {
	case reflect:
		buildAndRunFromVdlFile(entry.Value, entry.Types, []string{"-skip-go-methods"})
	case generated:
		buildAndRunFromVdlFile(entry.Value, entry.Types, nil)
	case vdlValue:
		runVdlValue(entry.Value)
	}
}

func writef(w io.Writer, format string, args ...interface{}) {
	_, err := fmt.Fprintf(w, format, args...)
	panicOnError(err)
}

func createFile(name string) (*os.File, func()) {
	file, err := os.Create(name)
	panicOnError(err)
	return file, func() { panicOnError(file.Close()) }
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

func buildAndRunFromVdlFile(vv *vdl.Value, types []*vdl.Type, vdlFlags []string) {
	dir, err := ioutil.TempDir("", "vomforever")
	defer os.RemoveAll(dir)
	panicOnError(err)

	// Write the type and value vdl files and the go test file.
	gopath := path.Join(dir, "go")
	pkgpath := path.Join("v.io", "v23", "vom", "vomtest", "vomforever", "tmp")
	pkgdir := path.Join(gopath, "src", pkgpath)
	panicOnError(os.MkdirAll(pkgdir, os.ModePerm))
	writeTypeFile(path.Join(pkgdir, "types.vdl"), types, vv)
	writeValueFile(path.Join(pkgdir, "value.vdl"), vv)
	writeTestFile(path.Join(pkgdir, "tmp_test.go"), vv)

	// Run vdl generate.
	vdlFlags = append(vdlFlags, pkgpath)
	vdlFlags = append([]string{"generate"}, vdlFlags...)
	cmd := exec.Command("vdl", vdlFlags...)
	cmd.Env = []string{
		"VDLPATH=" + path.Join(gopath, "src"),
	}
	stderr, err := cmd.StderrPipe()
	panicOnError(err)
	stdout, err := cmd.StdoutPipe()
	panicOnError(err)
	go func() {
		io.Copy(os.Stderr, stderr) //nolint:errcheck
	}()
	go func() {
		io.Copy(os.Stdout, stdout) //nolint:errcheck
	}()
	panicOnError(cmd.Run())

	// Run the test.
	cmd = exec.Command("jiri", "run", "go", "test", pkgpath)
	cmd.Env = []string{
		"GOPATH=" + gopath,
		"PATH=" + os.Getenv("PATH"),
		"JIRI_ROOT=" + os.Getenv("JIRI_ROOT"),
	}
	stderr, err = cmd.StderrPipe()
	panicOnError(err)
	stdout, err = cmd.StdoutPipe()
	panicOnError(err)
	go func() {
		io.Copy(os.Stderr, stderr) //nolint:errcheck
	}()
	go func() {
		io.Copy(os.Stdout, stdout) //nolint:errcheck
	}()
	panicOnError(cmd.Run())
}

func writeValueFile(filename string, vv *vdl.Value) {
	file, cleanup := createFile(filename)
	defer cleanup()
	writef(file, `package tmp

`)
	switch vv.Kind() {
	case vdl.Any, vdl.TypeObject:
		writef(file, "const X = %[1]s", vdlgen.TypedConst(vv, vdlPackageName, nil))
	case vdl.Optional:
		writef(file, "const X = XType{%[1]s}", vdlgen.TypedConst(vv, vdlPackageName, nil))
	default:
		writef(file, "const X = XType(%[1]s)", vdlgen.TypedConst(vv, vdlPackageName, nil))
	}
}

func writeTypeFile(filename string, types []*vdl.Type, vv *vdl.Value) {
	file, cleanup := createFile(filename)
	defer cleanup()
	writef(file, "package tmp\n\n")
	comment := textutil.PrefixLineWriter(file, "// ")
	panicOnError(vdltest.PrintTypeStats(comment, types...))
	writef(comment, "\nOnly named types appear below, by definition.\n")
	panicOnError(comment.Flush())
	writef(file, "\ntype (\n")
	for _, tt := range types {
		if tt.Name() != "" {
			base := vdlgen.BaseType(tt, vdlPackageName, nil)
			base = strings.ReplaceAll(base, "\n", "\n\t")
			writef(file, "\t%[1]s %[2]s\n", tt.Name(), base)
		}
	}
	switch vv.Kind() {
	case vdl.Any, vdl.TypeObject:
	case vdl.Optional:
		base := vdlgen.BaseType(vv.Type(), vdlPackageName, nil)
		base = strings.ReplaceAll(base, "\n", "\n\t")
		writef(file, "\tXType struct{\n\t\tValue %[1]s\n\t}\n", base)
	default:
		base := vdlgen.BaseType(vv.Type(), vdlPackageName, nil)
		base = strings.ReplaceAll(base, "\n", "\n\t")
		writef(file, "\tXType %[1]s\n", base)
	}
	writef(file, ")\n")
}

func writeTestFile(filename string, vv *vdl.Value) {
	file, cleanup := createFile(filename)
	defer cleanup()
	var typename = "XType"
	switch vv.Kind() {
	case vdl.Any:
		typename = "*vom.RawBytes"
	case vdl.TypeObject:
		typename = "*vdl.Type"
	}
	writef(file, `package tmp

import (
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
)

func TestEncodeDecode(t *testing.T) {
	bytes, err := vom.Encode(X)
	if err != nil {
		t.Errorf("error in encode: %%v", err)
	}
	var out %[1]s
	if err := vom.Decode(bytes, &out); err != nil {
		t.Fatalf("error in decode: %%v", err)
	}
	if !vdl.DeepEqual(out, X) {
		t.Errorf("got: %%#v, want: %%#v", out, X)
	}
}`, typename)
}

func runVdlValue(vv *vdl.Value) {
	bytes, err := vom.Encode(vv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error on encode %v\n", vv, err)
		return
	}
	var vvOut *vdl.Value
	if err := vom.Decode(bytes, &vvOut); err != nil {
		fmt.Fprintf(os.Stderr, "%v: error on decode %v\n", vv, err)
		return
	}
	if !vdl.EqualValue(vv, vvOut) {
		fmt.Fprintf(os.Stderr, "%v: decoded value %v did not match input\n", vv, vvOut)
	}
}
