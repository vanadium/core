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

// BuildOption represents an option to Build.
type BuildOption func(o *builder)

type depType int

const (
	getDep depType = iota
)

type dependency struct {
	action       depType
	pkg, version string
}

type builder struct {
	gopath       string
	binary       string
	main         string
	dependencies []dependency
	verbose      bool
}

func (o builder) log(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// GOPATH specifies the GOPATH to use with go mod download, mod edit, go build etc.
// If not specified a new temporary directory is created and used.
func GOPATH(dir string) BuildOption {
	return func(o *builder) {
		o.gopath = dir
	}
}

// Binary specifies the name of the final executable. It may be an absolute
// path or relative to the package's root directory.
func Binary(name string) BuildOption {
	return func(o *builder) {
		o.binary = name
	}
}

// Main specifies the main package or file to be built within this
// package if the package's root directory does not contain that main
// function.
func Main(path string) BuildOption {
	return func(o *builder) {
		o.main = path
	}
}

// Verbose controls printing detailed progress messages to os.Stderr.
func Verbose(v bool) BuildOption {
	return func(o *builder) {
		o.verbose = v
	}
}

// GetPackage specifies a package and version to be used by the package
// being built.
func GetPackage(pkg, version string) BuildOption {
	return func(o *builder) {
		o.dependencies = append(o.dependencies, dependency{getDep, pkg, version})
	}
}

func readFile(dir, file string) string {
	modfile, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return err.Error()
	}
	return string(modfile)
}

func (o builder) handleDownload(ctx context.Context, pkg string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "get", "-d", pkg)
	cmd.Env = append(cmd.Env, "GOPATH="+o.gopath)
	cmd.Dir = o.gopath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s: %s", strings.Join(cmd.Args, " "), err, out)
	}
	pkgDir := filepath.Join(o.gopath, "pkg", "mod", strings.ReplaceAll(pkg, "/", string(filepath.Separator)))
	// determine the full path for the downloaded package, including its version.
	if fi, err := os.Stat(pkgDir); err != nil || !fi.IsDir() {
		gp := pkgDir + "@*"
		o.log("go get of %v did not download to %v, trying %v\n", pkg, pkgDir, gp)
		matches, err := filepath.Glob(gp)
		if err != nil {
			return "", fmt.Errorf("failed to glob %v: %v", gp, err)
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("multiple versions found: %v", strings.Join(matches, ", "))
		}
		pkgDir = matches[0]
		if fi, err := os.Stat(pkgDir); err != nil || !fi.IsDir() {
			return "", fmt.Errorf("go get %v: %v not found or is not a directory", pkg, pkgDir)
		}
	}
	o.log("go get %v -> %v\n", pkg, pkgDir)
	return pkgDir, nil
}

func (o builder) makeWriteable(paths ...string) error {
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if (fi.Mode().Perm() & 0200) == 0200 {
			return nil
		}
		if err := os.Chmod(p, fi.Mode().Perm()|0200); err != nil {
			return err
		}
		o.log("%v is now writeable\n", p)
	}
	return nil
}

func (o builder) run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	cl := strings.Join(cmd.Args, " ")
	if err != nil {
		return fmt.Errorf("%s: %s: %s", cl, err, out)
	}
	o.log("%s\n", cl)
	return nil
}

func (o builder) rewriteGoMod(ctx context.Context, pkgDir string) error {
	o.log("original go.mod:\n %s\n", readFile(pkgDir, "go.mod"))
	if len(o.dependencies) == 0 {
		return nil
	}
	if err := o.makeWriteable(
		pkgDir,
		filepath.Join(pkgDir, "go.mod"),
		filepath.Join(pkgDir, "go.sum"),
	); err != nil {
		return err
	}
	o.log("updating #%v dependencies\n", len(o.dependencies))
	for _, dep := range o.dependencies {
		switch dep.action {
		case getDep:
			if err := o.run(ctx, pkgDir, "go", "get", dep.pkg+"@"+dep.version); err != nil {
				return err
			}
			o.log("after go get %s=%s\n%s\n", dep.pkg, dep.version, readFile(pkgDir, "go.mod"))
		default:
			continue
		}
	}
	return nil
}

// BuildWithDependencies builds a go module with the ability to specify its
// dependencies. It operates by using go get -d and then go mod edit
// on the downloaded package before building it.
// The returned cleanup function must always be called, even when an error is returned.
func BuildWithDependencies(ctx context.Context, pkg string, opts ...BuildOption) (root, binary string, cleanup func(), err error) {
	var o builder
	for _, fn := range opts {
		fn(&o)
	}
	cleanup = func() {}
	if len(o.gopath) == 0 {
		o.gopath, err = os.MkdirTemp("", path.Base(pkg)+"-XXXXXX")
		if err != nil {
			return "", "", nil, err
		}
		cleanup = func() {
			os.RemoveAll(o.gopath)
		}
	}
	o.log("GOPATH: %v\n", o.gopath)

	pkgDir, err := o.handleDownload(ctx, pkg)
	if err != nil {
		return "", "", cleanup, err
	}

	if err := o.rewriteGoMod(ctx, pkgDir); err != nil {
		return "", "", cleanup, err
	}

	mainDir := pkgDir
	buildTarget := "."
	if len(o.main) > 0 {
		mp := strings.ReplaceAll(o.main, "/", string(filepath.Separator))
		fi, err := os.Stat(filepath.Join(pkgDir, mp))
		if err != nil {
			return "", "", cleanup, fmt.Errorf("%v doesn't exist within %v: %v", o.main, pkgDir, err)
		}
		if fi.IsDir() {
			mainDir = filepath.Join(pkgDir, mp)
			binary = filepath.Join(mainDir, path.Base(o.main))
		} else {
			mainDir = filepath.Join(pkgDir, filepath.Dir(mp))
			binary = filepath.Join(pkgDir, strings.TrimSuffix(mp, ".go"))
			buildTarget = filepath.Base(mp)
		}
	}
	if len(o.binary) > 0 {
		if filepath.IsAbs(o.binary) {
			binary = o.binary
		} else {
			binary = filepath.Join(mainDir, o.binary)
		}
	}

	o.log("package directory : %v\n", pkgDir)
	o.log("main directory    : %v\n", mainDir)
	o.log("binary            : %v\n", binary)

	if err := o.makeWriteable(mainDir); err != nil {
		return "", "", cleanup, err
	}

	args := []string{"build"}
	args = append(args, "-o", binary)
	args = append(args, buildTarget)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = mainDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", cleanup, fmt.Errorf("%s: %s: %s", strings.Join(cmd.Args, " "), err, out)
	}
	o.log("%s: %s", strings.Join(cmd.Args, " "), out)
	return pkgDir, binary, cleanup, nil
}
