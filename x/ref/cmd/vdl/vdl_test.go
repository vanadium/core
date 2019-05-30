// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"v.io/x/lib/envvar"
	"v.io/x/lib/gosh"
)

const (
	testDir    = "../../lib/vdl/testdata/base"
	outPkgPath = "v.io/x/ref/lib/vdl/testdata/base"
)

func verifyOutput(t *testing.T, outDir string) {
	entries, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Fatalf("ReadDir(%v) failed: %v", testDir, err)
	}
	numEqual := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".vdl.go") {
			continue
		}
		testFile := filepath.Join(testDir, entry.Name())
		testBytes, err := ioutil.ReadFile(testFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", testFile, err)
		}
		outFile := filepath.Join(outDir, outPkgPath, entry.Name())
		outBytes, err := ioutil.ReadFile(outFile)
		if err != nil {
			t.Fatalf("ReadFile(%v) failed: %v", outFile, err)
		}
		if !bytes.Equal(outBytes, testBytes) {
			t.Fatalf("GOT:\n%v\n\nWANT:\n%v\n", string(outBytes), string(testBytes))
		}
		numEqual++
	}
	if numEqual == 0 {
		t.Fatalf("testDir %s has no golden files *.vdl.go", testDir)
	}
}

// Compares generated VDL files against the copy in the repo.
func TestVDLGenerator(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	vdlBin := gosh.BuildGoPkg(sh, sh.MakeTempDir(), "v.io/x/ref/cmd/vdl")

	// Use vdl to generate Go code from input, into a temporary directory.
	outDir := sh.MakeTempDir()
	// TODO(toddw): test the generated java and javascript files too.
	outOpt := fmt.Sprintf("--go-out-dir=%s", outDir)
	sh.Cmd(vdlBin, "generate", "--lang=go", outOpt, testDir).Run()
	// Check that each *.vdl.go file in the testDir matches the generated output.
	verifyOutput(t, outDir)
}

// Asserts that vdl generation works without VDLROOT or JIRI_ROOT being set.
func TestVDLGeneratorBuiltInVDLRoot(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	vdlBin := gosh.BuildGoPkg(sh, sh.MakeTempDir(), "v.io/x/ref/cmd/vdl")

	outDir := sh.MakeTempDir()
	outOpt := fmt.Sprintf("--go-out-dir=%s", outDir)
	env := envvar.SliceToMap(os.Environ())
	delete(env, "JIRI_ROOT")
	delete(env, "VDLROOT")
	cmd := sh.Cmd(vdlBin, "generate", "-v", "--lang=go", outOpt, testDir)
	cmd.Vars = env
	cmd.Run()
	verifyOutput(t, outDir)
}
