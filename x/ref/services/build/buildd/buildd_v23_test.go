// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"v.io/x/lib/envvar"
	"v.io/x/ref/test/v23test"
)

var testProgram = `package main

import "fmt"

func main() { fmt.Println("Hello World!") }
`

func TestV23BuildServerIntegration(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("%v", err)
	}
	goRoot := runtime.GOROOT()

	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Build binaries for the client and server.
	// Since Permissions are not setup on the server, the client must pass the
	// default authorization policy, i.e., must be a "delegate" of the server.
	var (
		buildServerBin   = v23test.BuildGoPkg(sh, "v.io/x/ref/services/build/buildd")
		buildBin         = v23test.BuildGoPkg(sh, "v.io/x/ref/services/build/build")
		buildServerCreds = sh.ForkCredentials("buildd")
		buildCreds       = sh.ForkCredentials("buildd:client")
	)

	// Start the build server.
	buildServerName := "test-build-server"
	sh.Cmd(buildServerBin,
		"-name="+buildServerName,
		"-gobin="+goBin,
		"-goroot="+goRoot,
		"-v23.tcp.address=127.0.0.1:0").WithCredentials(buildServerCreds).Start()

	// Create and build a test source file.
	testGoPath := sh.MakeTempDir()
	testBinDir := filepath.Join(testGoPath, "bin")
	if err := os.MkdirAll(testBinDir, os.FileMode(0700)); err != nil {
		t.Fatalf("MkdirAll(%v) failed: %v", testBinDir, err)
	}
	testBinFile := filepath.Join(testBinDir, "test")
	testSrcDir := filepath.Join(testGoPath, "src", "test")
	if err := os.MkdirAll(testSrcDir, os.FileMode(0700)); err != nil {
		t.Fatalf("MkdirAll(%v) failed: %v", testSrcDir, err)
	}
	testSrcFile := filepath.Join(testSrcDir, "test.go")
	if err := ioutil.WriteFile(testSrcFile, []byte(testProgram), os.FileMode(0600)); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", testSrcFile, err)
	}
	buildCmd := sh.Cmd(buildBin,
		"build",
		buildServerName,
		"test").WithCredentials(buildCreds)
	buildCmd.Vars = envvar.MergeMaps(buildCmd.Vars, map[string]string{
		"GOPATH": testGoPath,
		"GOROOT": goRoot,
		"TMPDIR": testBinDir,
	})
	buildCmd.Run()
	var testOut bytes.Buffer
	testCmd := exec.Command(testBinFile)
	testCmd.Stdout = &testOut
	testCmd.Stderr = &testOut
	if err := testCmd.Run(); err != nil {
		t.Fatalf("%q failed: %v\n%v", strings.Join(testCmd.Args, " "), err, testOut.String())
	}
	if got, want := strings.TrimSpace(testOut.String()), "Hello World!"; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
