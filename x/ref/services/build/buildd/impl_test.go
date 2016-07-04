// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/services/build"
	"v.io/x/ref/test"
)

// findGoBinary returns the path to the given Go binary and
// the GOROOT environment variable to use.
func findGoBinary(t *testing.T, name string) (bin, goroot string) {
	root := os.Getenv("JIRI_ROOT")
	if root == "" {
		t.Fatalf("JIRI_ROOT is not set")
	}
	envroot := filepath.Join(root, "environment", "go", runtime.GOOS, runtime.GOARCH, "go")
	envbin := filepath.Join(envroot, "bin", name)
	if _, err := os.Stat(envbin); err == nil {
		return envbin, envroot
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%v) failed: %v", envbin, err)
	}
	pathbin, err := exec.LookPath(name)
	if err != nil {
		if err == exec.ErrNotFound {
			t.Fatalf("%q does not exist and %q not found in PATH", envbin, name)
		} else {
			t.Fatalf("LookPath(%q) failed: %v", name, err)
		}
	}
	return pathbin, os.Getenv("GOROOT")
}

// getArch returns an Architecture representing the host architecture.
func getArch(t *testing.T) (arch build.Architecture) {
	if err := arch.SetFromGoArch(runtime.GOARCH); err != nil {
		t.Fatalf("%v", err)
	}
	return
}

// getOS returns an OperatingSystem representing the host operating
// system.
func getOS(t *testing.T) (os build.OperatingSystem) {
	if err := os.SetFromGoOS(runtime.GOOS); err != nil {
		t.Fatalf("%v", err)
	}
	return
}

// startServer starts the build server.
func startServer(t *testing.T, ctx *context.T) build.BuilderClientMethods {
	gobin, goroot := findGoBinary(t, "go")
	service := build.BuilderServer(NewBuilderService(gobin, goroot))
	unpublished := ""
	_, server, err := v23.WithNewServer(ctx, unpublished, service, nil)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	return build.BuilderClient(server.Status().Endpoints[0].Name())
}

func invokeBuild(t *testing.T, ctx *context.T, client build.BuilderClientMethods, files []build.File) ([]byte, []build.File, error) {
	arch, os := getArch(t), getOS(t)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.Build(ctx, arch, os)
	if err != nil {
		t.Errorf("Build(%v, %v) failed: %v", err, arch, os)
		return nil, nil, err
	}
	sender := stream.SendStream()
	for _, file := range files {
		if err := sender.Send(file); err != nil {
			t.Logf("Send() failed: %v", err)
			return nil, nil, err
		}
	}
	if err := sender.Close(); err != nil {
		t.Logf("Close() failed: %v", err)
		return nil, nil, err
	}
	bins := make([]build.File, 0)
	rStream := stream.RecvStream()
	for rStream.Advance() {
		bins = append(bins, rStream.Value())
	}
	if err := rStream.Err(); err != nil {
		t.Logf("Advance() failed: %v", err)
		return nil, nil, err
	}
	output, err := stream.Finish()
	if err != nil {
		t.Logf("Finish() failed: %v", err)
		return nil, nil, err
	}
	return output, bins, nil
}

const mainSrc = `package main

import "fmt"

func main() {
	fmt.Println("Hello World!")
}
`

func containsPkg(pkgs, target string) bool {
	pkgs = strings.TrimSpace(strings.Replace(pkgs, "\n", " ", -1))
	for _, str := range strings.Split(pkgs, " ") {
		if str == target {
			return true
		}
	}
	return false
}

// TestSuccess checks that the build server successfully builds a
// package that depends on the standard Go library.
func TestSuccess(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	client := startServer(t, ctx)

	files := []build.File{
		build.File{
			Name:     "testfoopkg/main.go",
			Contents: []byte(mainSrc),
		},
	}
	output, bins, err := invokeBuild(t, ctx, client, files)
	if err != nil {
		t.FailNow()
	}
	if got, want := string(output), "testfoopkg"; !containsPkg(got, want) {
		t.Fatalf("Unexpected output: got %v, want %v", got, want)
	}
	if got, want := len(bins), 1; got != want {
		t.Fatalf("Unexpected number of binaries: got %v, want %v", got, want)
	}
}

const fooSrc = `package foo

import "fmt"

func foo() {
	fmt.Println("Hello World!")
}
`

// TestEmpty checks that the build server successfully builds a
// package that does not produce a binary.
func TestEmpty(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	client := startServer(t, ctx)

	files := []build.File{
		build.File{
			Name:     "test/foo.go",
			Contents: []byte(fooSrc),
		},
	}
	output, bins, err := invokeBuild(t, ctx, client, files)
	if err != nil {
		t.FailNow()
	}
	if got, expected := strings.TrimSpace(string(output)), "test"; got != expected {
		t.Fatalf("Unexpected output: got %v, expected %v", got, expected)
	}
	if got, expected := len(bins), 0; got != expected {
		t.Fatalf("Unexpected number of binaries: got %v, expected %v", got, expected)
	}
}

const failSrc = `package main

import "fmt"

func main() {
        ...
}
`

// TestFailure checks that the build server fails to build a package
// consisting of an empty file.
func TestFailure(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	client := startServer(t, ctx)

	files := []build.File{
		build.File{
			Name:     "test/main.go",
			Contents: []byte(failSrc),
		},
	}
	if output, _, err := invokeBuild(t, ctx, client, files); err == nil {
		t.Logf("%v", string(output))
		t.FailNow()
	}
}
