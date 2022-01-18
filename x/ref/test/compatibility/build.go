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

type actionType int

const (
	goGet actionType = iota
	goModEdit
)

type action struct {
	action actionType
	args   []string
}

type buildSpec struct {
	path   string
	binary string
}

type builder struct {
	gopath  string
	gocmd   string
	build   []buildSpec
	edit    []action
	verbose bool
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

// Build specifies the main package or file to be built within this
// package if the package's root directory does not contain that main
// function.
func Build(path, binary string) BuildOption {
	return func(o *builder) {
		o.build = append(o.build, buildSpec{path, binary})
	}
}

// Verbose controls printing detailed progress messages to os.Stderr.
func Verbose(v bool) BuildOption {
	return func(o *builder) {
		o.verbose = v
	}
}

// GoMod adds a 'go mod args...' invocation to the set of commands to be run
// to modify the go.mod file.
func GoMod(args ...string) BuildOption {
	return func(o *builder) {
		o.edit = append(o.edit, action{goModEdit, args})
	}
}

// GoGet adds a 'go get args...' invocation to the set of commands to be run
// to modify the go.mod file.
func GoGet(args ...string) BuildOption {
	return func(o *builder) {
		o.edit = append(o.edit, action{goGet, args})
	}
}

// GoCmd specifies a specific instance of the 'go' command to use in place
// of the system available version of 'go'.
func GoCmd(cmd string) BuildOption {
	return func(o *builder) {
		o.gocmd = cmd
	}
}

func readFile(dir, file string) string {
	modfile, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return err.Error()
	}
	return string(modfile)
}

func (o builder) run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(cmd.Env,
		"GOPATH="+o.gopath,
		"HOME="+os.Getenv("HOME"),
		"PATH="+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	cl := strings.Join(cmd.Args, " ")
	if err != nil {
		return fmt.Errorf("%s: %s: %s", cl, err, out)
	}
	o.log("%s\n", cl)
	return nil
}

func (o builder) handleDownload(ctx context.Context, pkg string) (string, error) {
	err := o.run(ctx, o.gopath, o.gocmd, "get", "-d", pkg)
	if err != nil {
		return "", err
	}
	pkgDir := filepath.Join(o.gopath, "pkg", "mod", strings.ReplaceAll(pkg, "/", string(filepath.Separator)))
	// determine the full path for the downloaded package, including its version.
	if fi, err := os.Stat(pkgDir); err != nil || !fi.IsDir() {
		gp := pkgDir
		idx := strings.Index(gp, "@")
		if idx < 0 {
			gp += "@*"
		} else {
			gp = gp[:idx] + "@*" + gp[idx+1:] + "*"
		}
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

func (o builder) rewriteGoMod(ctx context.Context, pkgDir string) error {
	o.log("original go.mod:\n%s\n", readFile(pkgDir, "go.mod"))
	if len(o.edit) == 0 {
		return nil
	}
	if err := o.makeWriteable(
		pkgDir,
		filepath.Join(pkgDir, "go.mod"),
		filepath.Join(pkgDir, "go.sum"),
	); err != nil {
		return err
	}
	o.log("updating #%v dependencies\n", len(o.edit))

	for _, cmd := range o.edit {
		var cl []string
		switch cmd.action {
		case goModEdit:
			cl = append([]string{o.gocmd, "mod"}, cmd.args...)
		case goGet:
			cl = append([]string{o.gocmd, "get"}, cmd.args...)
		}
		if err := o.run(ctx, pkgDir, cl...); err != nil {
			return err
		}
		o.log("after %s\n%s\n", strings.Join(cl, " "), readFile(pkgDir, "go.mod"))
	}
	return nil
}

func (o builder) goBuild(ctx context.Context, pkgDir string, spec buildSpec) (string, error) {
	mainDir := pkgDir
	buildTarget := "."
	binary := ""
	if len(spec.path) > 0 {
		mp := strings.ReplaceAll(spec.path, "/", string(filepath.Separator))
		fi, err := os.Stat(filepath.Join(pkgDir, mp))
		if err != nil {
			return "", fmt.Errorf("%v doesn't exist within %v: %v", spec.path, pkgDir, err)
		}
		if fi.IsDir() {
			mainDir = filepath.Join(pkgDir, mp)
			binary = filepath.Join(mainDir, path.Base(spec.path))
		} else {
			mainDir = filepath.Join(pkgDir, filepath.Dir(mp))
			binary = filepath.Join(pkgDir, strings.TrimSuffix(mp, ".go"))
			buildTarget = filepath.Base(mp)
		}
	}
	if len(spec.binary) > 0 {
		if filepath.IsAbs(spec.binary) {
			binary = spec.binary
		} else {
			binary = filepath.Join(mainDir, spec.binary)
		}
	}

	o.log("build spec path   : %v\n", spec.path)
	o.log("build spec binary : %v\n", spec.binary)
	o.log("package directory : %v\n", pkgDir)
	o.log("main directory    : %v\n", mainDir)
	o.log("binary            : %v\n", binary)

	if err := o.makeWriteable(mainDir); err != nil {
		return "", err
	}

	err := o.run(ctx, mainDir, o.gocmd, "build", "-o", binary, buildTarget)
	if err != nil {
		return "", err
	}
	return binary, nil
}

// BuildWithDependencies builds a go module with the ability to specify its
// dependencies. It operates by using go get -d and then go mod edit
// on the downloaded package before building it.
// The returned cleanup function must always be called, even when an error is returned.
func BuildWithDependencies(ctx context.Context, pkg string, opts ...BuildOption) (root string, binaries []string, cleanup func(), err error) {
	var o builder
	for _, fn := range opts {
		fn(&o)
	}
	if len(o.gocmd) == 0 {
		o.gocmd = "go"
	}
	cleanup = func() {}
	if len(o.gopath) == 0 {
		o.gopath, err = os.MkdirTemp("", path.Base(pkg)+"-XXXXXX")
		if err != nil {
			return "", nil, nil, err
		}
		cleanup = func() {
			os.RemoveAll(o.gopath)
		}
	}
	o.log("GOPATH: %v\n", o.gopath)

	pkgDir, err := o.handleDownload(ctx, pkg)
	if err != nil {
		return "", nil, cleanup, err
	}

	if err := o.rewriteGoMod(ctx, pkgDir); err != nil {
		return "", nil, cleanup, err
	}

	for _, spec := range o.build {
		binary, err := o.goBuild(ctx, pkgDir, spec)
		if err != nil {
			return "", nil, cleanup, err
		}
		binaries = append(binaries, binary)
	}

	return pkgDir, binaries, cleanup, nil
}
