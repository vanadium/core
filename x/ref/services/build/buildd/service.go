// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/services/binary"
	"v.io/v23/services/build"
	"v.io/v23/verror"
	"v.io/x/lib/host"
)

const pkgPath = "v.io/x/ref/services/build/buildd"

// Errors
var (
	errBuildFailed = verror.Register(pkgPath+".errBuildFailed", verror.NoRetry, "{1:}{2:} build failed{:_}")
)

// builderService implements the Builder server interface.
type builderService struct {
	// Path to the binary and the value of the GOROOT environment variable.
	gobin, goroot string
}

// NewBuilderService returns a new Build service implementation.
func NewBuilderService(gobin, goroot string) build.BuilderServerMethods {
	return &builderService{
		gobin:  gobin,
		goroot: goroot,
	}
}

// TODO(jsimsa): Add support for building for a specific profile
// specified as a suffix the Build().
//
// TODO(jsimsa): Analyze the binary files for shared library
// dependencies and ship these back.
func (i *builderService) Build(ctx *context.T, call build.BuilderBuildServerCall, arch build.Architecture, opsys build.OperatingSystem) ([]byte, error) {
	ctx.VI(1).Infof("Build(%v, %v) called.", arch, opsys)
	dir, prefix := "", ""
	dirPerm, filePerm := os.FileMode(0700), os.FileMode(0600)
	root, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		ctx.Errorf("TempDir(%v, %v) failed: %v", dir, prefix, err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	defer os.RemoveAll(root)
	srcDir := filepath.Join(root, "go", "src")
	if err := os.MkdirAll(srcDir, dirPerm); err != nil {
		ctx.Errorf("MkdirAll(%v, %v) failed: %v", srcDir, dirPerm, err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	iterator := call.RecvStream()
	for iterator.Advance() {
		srcFile := iterator.Value()
		filePath := filepath.Join(srcDir, filepath.FromSlash(srcFile.Name))
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			ctx.Errorf("MkdirAll(%v, %v) failed: %v", dir, dirPerm, err)
			return nil, verror.New(verror.ErrInternal, ctx)
		}
		if err := ioutil.WriteFile(filePath, srcFile.Contents, filePerm); err != nil {
			ctx.Errorf("WriteFile(%v, %v) failed: %v", filePath, filePerm, err)
			return nil, verror.New(verror.ErrInternal, ctx)
		}
	}
	if err := iterator.Err(); err != nil {
		ctx.Errorf("Advance() failed: %v", err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	// NOTE: we actually want run "go install -v {srcDir}/..." here, but
	// the go tool seems to have a bug where it doesn't interpret rooted
	// (absolute) paths with wildcards correctly.  So we run "go install
	// -v all" instead, which has the downside that it might cause some
	// standard packages to be built spuriously.
	cmd := exec.Command(i.gobin, "install", "-v", "all")
	cmd.Env = append(cmd.Env, "GOARCH="+arch.ToGoArch())
	cmd.Env = append(cmd.Env, "GOOS="+opsys.ToGoOS())
	cmd.Env = append(cmd.Env, "GOPATH="+filepath.Dir(srcDir))
	if i.goroot != "" {
		cmd.Env = append(cmd.Env, "GOROOT="+i.goroot)
	}
	if tmpdir, ok := os.LookupEnv("TMPDIR"); ok {
		cmd.Env = append(cmd.Env, "TMPDIR="+tmpdir)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		ctx.Errorf("Run(%q) failed: %v", strings.Join(cmd.Args, " "), err)
		if output.Len() != 0 {
			ctx.Errorf("%v", output.String())
		}
		return output.Bytes(), verror.New(errBuildFailed, ctx)
	}
	binDir := filepath.Join(root, "go", "bin")
	machineArch, err := host.Arch()
	if err != nil {
		ctx.Errorf("Arch() failed: %v", err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	if machineArch != arch.ToGoArch() || runtime.GOOS != opsys.ToGoOS() {
		binDir = filepath.Join(binDir, fmt.Sprintf("%v_%v", opsys.ToGoOS(), arch.ToGoArch()))
	}
	files, err := ioutil.ReadDir(binDir)
	if err != nil && !os.IsNotExist(err) {
		ctx.Errorf("ReadDir(%v) failed: %v", binDir, err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	for _, file := range files {
		binPath := filepath.Join(binDir, file.Name())
		bytes, err := ioutil.ReadFile(binPath)
		if err != nil {
			ctx.Errorf("ReadFile(%v) failed: %v", binPath, err)
			return nil, verror.New(verror.ErrInternal, ctx)
		}
		result := build.File{
			Name:     "bin/" + file.Name(),
			Contents: bytes,
		}
		if err := call.SendStream().Send(result); err != nil {
			ctx.Errorf("Send() failed: %v", err)
			return nil, verror.New(verror.ErrInternal, ctx)
		}
	}
	return output.Bytes(), nil
}

func (i *builderService) Describe(_ *context.T, _ rpc.ServerCall, name string) (binary.Description, error) {
	// TODO(jsimsa): Implement.
	return binary.Description{}, nil
}
