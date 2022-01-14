// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package compatibility provides support for compatibility testing between
// different versions of vanadium.
package compatibility

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// Option represents an option to Build.
type Option func(o *options)

type dependency struct {
	pkg, version string
}

type options struct {
	gopath       string
	binary       string
	main         string
	dependencies []dependency
	verbose      bool
}

// GOPATH specifies the GOPATH to use with go mod download, mod edit, go build etc.
// If not specified a new temporary directory is created and used.
func GOPATH(dir string) Option {
	return func(o *options) {
		o.gopath = dir
	}
}

// Binary specifies the name of the final executable. It may be an absolute
// path or relative to the package's root directory.
func Binary(name string) Option {
	return func(o *options) {
		o.binary = name
	}
}

// Main specifies the main package or file to be built within this
// package if the package's root directory does not contain that main
// function.
func Main(path string) Option {
	return func(o *options) {
		o.main = path
	}
}

// Verbose controls printing detailed progress messages to os.Stderr.
func Verbose(v bool) Option {
	return func(o *options) {
		o.verbose = v
	}
}

// Dependency specifies a package and version to be used by the package
// being built. go mod edit require is used to set this dependency.
func Dependency(pkg, version string) Option {
	return func(o *options) {
		o.dependencies = append(o.dependencies, dependency{pkg, version})
	}
}

func readModFile(dir, msg string) string {
	modfile, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return err.Error()
	}
	return msg + "\n" + string(modfile)
}

// BuildWithDependencies builds a go module with the ability to specify its
// dependencies. It operates by using go mod download and then go mod edit
// on the downloaded package before building it.
// The returned cleanup must always be called, even when an error is returned.
func BuildWithDependencies(ctx context.Context, pkg string, opts ...Option) (binary string, cleanup func(), err error) {
	var o options
	for _, fn := range opts {
		fn(&o)
	}
	cleanup = func() {}
	if len(o.gopath) == 0 {
		o.gopath, err = os.MkdirTemp("", path.Base(pkg)+"-XXXXXX")
		if err != nil {
			return "", nil, err
		}
		cleanup = func() {
			os.RemoveAll(o.gopath)
		}
	}

	if o.verbose {
		fmt.Fprintf(os.Stderr, "workdir/GOPATH: %s", o.gopath)
	}

	cmd := exec.CommandContext(ctx, "go", "mod", "download", pkg)
	cmd.Env = append(cmd.Env, "GOPATH="+o.gopath)
	cmd.Dir = o.gopath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", cleanup, fmt.Errorf("%s: %s: %s", strings.Join(cmd.Args, " "), err, out)
	}

	pkgDir := filepath.Join(o.gopath, "pkg", "mod", strings.ReplaceAll(pkg, "/", string(filepath.Separator)))

	if o.verbose {
		fmt.Fprintln(os.Stderr, readModFile("original go.mod", pkgDir))
	}

	for _, dep := range o.dependencies {
		cmd := exec.CommandContext(ctx, "go", "mod", "edit", "require", dep.pkg+"="+dep.version)
		cmd.Dir = pkgDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", cleanup, fmt.Errorf("%s: %s: %s", strings.Join(cmd.Args, " "), err, out)
		}
		if o.verbose {
			fmt.Fprintln(os.Stderr, readModFile(fmt.Sprintf("%s", strings.Join(cmd.Args, " ")), pkgDir))
		}
	}

	mainDir := pkgDir
	if len(o.main) > 0 {
		mp := strings.ReplaceAll(o.main, "/", string(filepath.Separator))
		fi, err := os.Stat(filepath.Join(pkgDir, mp))
		if err != nil {
			return "", cleanup, fmt.Errorf("%v doesn't exist within %v: %v", o.main, pkgDir, err)
		}
		if fi.IsDir() {
			mainDir = filepath.Join(pkgDir, mp)
			binary = filepath.Join(mainDir, path.Base(o.main))
		} else {
			mainDir = filepath.Join(pkgDir, filepath.Dir(mp))
			binary = filepath.Join(mainDir, strings.TrimSuffix(o.main, ".go"))
		}
	}
	if len(o.binary) > 0 {
		if filepath.IsAbs(o.binary) {
			binary = o.binary
		} else {
			binary = filepath.Join(mainDir, o.binary)
		}
	}

	args := []string{"build"}
	if len(o.binary) > 0 {
		args = append(args, "-o", o.binary)
	}
	args = append(args, ".")
	cmd = exec.CommandContext(ctx, "go", args...)
	cmd.Dir = mainDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", cleanup, fmt.Errorf("%s: %s: %s", strings.Join(cmd.Args, " "), err, out)
	}
	if o.verbose {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("%s: %s", strings.Join(cmd.Args, " "), out))
	}
	return binary, cleanup, nil
}
