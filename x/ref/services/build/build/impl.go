// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"fmt"
	"go/build"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"v.io/v23/context"
	vbuild "v.io/v23/services/build"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

func main() {
	cmdline.Main(cmdRoot)
}

var (
	flagArch string
	flagOS   string
)

func init() {
	cmdline.HideGlobalFlagsExcept()
	cmdBuild.Flags.StringVar(&flagArch, "arch", runtime.GOARCH, "Target architecture.  The default is the value of runtime.GOARCH.")
	cmdBuild.Flags.Lookup("arch").DefValue = "<runtime.GOARCH>"
	cmdBuild.Flags.StringVar(&flagOS, "os", runtime.GOOS, "Target operating system.  The default is the value of runtime.GOOS.")
	cmdBuild.Flags.Lookup("os").DefValue = "<runtime.GOOS>"
}

var cmdRoot = &cmdline.Command{
	Name:  "build",
	Short: "sends commands to a Vanadium build server",
	Long: `
Command build sends commands to a Vanadium build server.
`,
	Children: []*cmdline.Command{cmdBuild},
}

var cmdBuild = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBuild),
	Name:   "build",
	Short:  "Build vanadium Go packages",
	Long: `
Build vanadium Go packages using a remote build server. The command collects all
source code files that are not part of the Go standard library that the target
packages depend on, sends them to a build server, and receives the built
binaries.
`,
	ArgsName: "<name> <packages>",
	ArgsLong: `
<name> is a vanadium object name of a build server <packages> is a list of
packages to build, specified as arguments for each command. The format is
similar to the go tool.  In its simplest form each package is an import path;
e.g. "v.io/x/ref/services/build/build". A package that ends with "..." does a
wildcard match against all packages with that prefix.
`,
}

// TODO(jsimsa): Add support for importing (and remotely building)
// packages from multiple package source root GOPATH directories with
// identical names.
func importPackages(paths []string, pkgMap map[string]*build.Package) error {
	for _, path := range paths {
		recurse := false
		if strings.HasSuffix(path, "...") {
			recurse = true
			path = strings.TrimSuffix(path, "...")
		}
		if _, exists := pkgMap[path]; !exists {
			srcDir, mode := "", build.ImportMode(0)
			pkg, err := build.Import(path, srcDir, mode)
			if err != nil {
				// "C" is a pseudo-package for cgo: http://golang.org/cmd/cgo/
				// Do not attempt recursive imports.
				if pkg.ImportPath == "C" {
					continue
				}
				return fmt.Errorf("Import(%q,%q,%v) failed: %v", path, srcDir, mode, err)
			}
			if pkg.Goroot {
				continue
			}
			pkgMap[path] = pkg
			if err := importPackages(pkg.Imports, pkgMap); err != nil {
				return err
			}
		}
		if recurse {
			pkg := pkgMap[path]
			fis, err := ioutil.ReadDir(pkg.Dir)
			if err != nil {
				return fmt.Errorf("ReadDir(%v) failed: %v", pkg.Dir, err)
			}
			for _, fi := range fis {
				if fi.IsDir() {
					subPath := filepath.Join(path, fi.Name(), "...")
					if err := importPackages([]string{subPath}, pkgMap); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func getSources(ctx *context.T, pkgMap map[string]*build.Package, errchan chan<- error) <-chan vbuild.File {
	sources := make(chan vbuild.File)
	go func() {
		defer close(sources)
		for _, pkg := range pkgMap {
			for _, files := range [][]string{pkg.CFiles, pkg.CgoFiles, pkg.GoFiles, pkg.SFiles} {
				for _, file := range files {
					path := filepath.Join(pkg.Dir, file)
					bytes, err := ioutil.ReadFile(path)
					if err != nil {
						errchan <- fmt.Errorf("ReadFile(%v) failed: %v", path, err)
						return
					}
					select {
					case sources <- vbuild.File{Contents: bytes, Name: filepath.Join(pkg.ImportPath, file)}:
					case <-ctx.Done():
						errchan <- fmt.Errorf("Get sources failed: %v", ctx.Err())
						return
					}
				}
			}
		}
		errchan <- nil
	}()
	return sources
}

func invokeBuild(ctx *context.T, name string, sources <-chan vbuild.File, errchan chan<- error) <-chan vbuild.File {
	binaries := make(chan vbuild.File)
	go func() {
		defer close(binaries)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		client := vbuild.BuilderClient(name)
		var arch vbuild.Architecture
		if err := arch.SetFromGoArch(flagArch); err != nil {
			errchan <- err
			return
		}
		var os vbuild.OperatingSystem
		if err := os.SetFromGoOS(flagOS); err != nil {
			errchan <- err
			return
		}
		stream, err := client.Build(ctx, arch, os)
		if err != nil {
			errchan <- fmt.Errorf("Build() failed: %v", err)
			return
		}
		sender := stream.SendStream()
		for source := range sources {
			if err := sender.Send(source); err != nil {
				errchan <- fmt.Errorf("Send() failed: %v", err)
				return
			}
		}
		if err := sender.Close(); err != nil {
			errchan <- fmt.Errorf("Close() failed: %v", err)
			return
		}
		iterator := stream.RecvStream()
		for iterator.Advance() {
			select {
			case binaries <- iterator.Value():
			case <-ctx.Done():
				errchan <- fmt.Errorf("Invoke build failed: %v", ctx.Err())
				return
			}
		}
		if err := iterator.Err(); err != nil {
			errchan <- fmt.Errorf("Advance() failed: %v", err)
			return
		}
		if out, err := stream.Finish(); err != nil {
			errchan <- fmt.Errorf("Finish() failed: (%v, %v)", string(out), err)
			return
		}
		errchan <- nil
	}()
	return binaries
}

func saveBinaries(ctx *context.T, prefix string, binaries <-chan vbuild.File, errchan chan<- error) {
	go func() {
		for binary := range binaries {
			select {
			case <-ctx.Done():
				errchan <- fmt.Errorf("Save binaries failed: %v", ctx.Err())
				return
			default:
			}
			path, perm := filepath.Join(prefix, filepath.Base(binary.Name)), os.FileMode(0755)
			if err := ioutil.WriteFile(path, binary.Contents, perm); err != nil {
				errchan <- fmt.Errorf("WriteFile(%v, %v) failed: %v", path, perm, err)
				return
			}
			fmt.Printf("Generated binary %v\n", path)
		}
		errchan <- nil
	}()
}

// runBuild identifies the source files needed to build the packages
// specified on command-line and then creates a pipeline that
// concurrently 1) reads the source files, 2) sends them to the build
// server and receives binaries from the build server, and 3) writes
// the binaries out to the disk.
func runBuild(ctx *context.T, env *cmdline.Env, args []string) error {
	name, paths := args[0], args[1:]
	pkgMap := map[string]*build.Package{}
	if err := importPackages(paths, pkgMap); err != nil {
		return err
	}
	errchan := make(chan error)
	defer close(errchan)

	ctx, ctxCancel := context.WithTimeout(ctx, time.Minute)
	defer ctxCancel()

	// Start all stages of the pipeline.
	sources := getSources(ctx, pkgMap, errchan)
	binaries := invokeBuild(ctx, name, sources, errchan)
	saveBinaries(ctx, os.TempDir(), binaries, errchan)
	// Wait for all stages of the pipeline to terminate.
	errors, numStages := []error{}, 3
	for i := 0; i < numStages; i++ {
		if err := <-errchan; err != nil {
			errors = append(errors, err)
			ctxCancel()
		}
	}
	if len(errors) != 0 {
		return fmt.Errorf("build failed(%v)", errors)
	}
	return nil
}
