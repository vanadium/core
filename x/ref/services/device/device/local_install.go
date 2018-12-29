// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/binary"
	"v.io/v23/services/device"
	"v.io/v23/services/repository"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/internal/packages"
)

var cmdInstallLocal = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runInstallLocal),
	Name:     "install-local",
	Short:    "Install the given application from the local system.",
	Long:     "Install the given application specified using a local path, and print the name of the new installation.",
	ArgsName: "<device> <title> [ENV=VAL ...] binary [--flag=val ...] [PACKAGES path ...]",
	ArgsLong: `
<device> is the vanadium object name of the device manager's app service.

<title> is the app title.

This is followed by an arbitrary number of environment variable settings, the
local path for the binary to install, and arbitrary flag settings and args.
Optionally, this can be followed by 'PACKAGES' and a list of local files and
directories to be installed as packages for the app`}

func init() {
	cmdInstallLocal.Flags.Var(&configOverride, "config", "JSON-encoded device.Config object, of the form: '{\"flag1\":\"value1\",\"flag2\":\"value2\"}'")
	cmdInstallLocal.Flags.Var(&packagesOverride, "packages", "JSON-encoded application.Packages object, of the form: '{\"pkg1\":{\"File\":\"local file path1\"},\"pkg2\":{\"File\":\"local file path 2\"}}'")
}

type mapDispatcher map[string]interface{}

func (d mapDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	o, ok := d[suffix]
	if !ok {
		return nil, nil, fmt.Errorf("suffix %s not found", suffix)
	}
	// TODO(caprita): Do not allow everyone, even for a short-lived server.
	return o, security.AllowEveryone(), nil
}

type mapServer struct {
	name       string
	dispatcher mapDispatcher
}

func (ms *mapServer) serve(name string, object interface{}) (string, error) {
	if _, ok := ms.dispatcher[name]; ok {
		return "", fmt.Errorf("can't have more than one object with name %v", name)
	}
	ms.dispatcher[name] = object
	return naming.Join(ms.name, name), nil
}

func createServer(ctx *context.T, stderr io.Writer) (*context.T, *mapServer, func(), error) {
	dispatcher := make(mapDispatcher)

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{})
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		return nil, nil, nil, err
	}
	name := server.Status().Endpoints[0].Name()
	cleanup := func() {
		cancel()
		<-server.Closed()
	}
	return ctx, &mapServer{name: name, dispatcher: dispatcher}, cleanup, nil
}

var errNotImplemented = fmt.Errorf("method not implemented")

type binaryInvoker string

func (binaryInvoker) Create(*context.T, rpc.ServerCall, int32, repository.MediaInfo) error {
	return errNotImplemented
}

func (binaryInvoker) Delete(*context.T, rpc.ServerCall) error {
	return errNotImplemented
}

func (i binaryInvoker) Download(ctx *context.T, call repository.BinaryDownloadServerCall, _ int32) error {
	fileName := string(i)
	fStat, err := os.Stat(fileName)
	if err != nil {
		return err
	}
	ctx.VI(1).Infof("Download commenced for %v (%v bytes)", fileName, fStat.Size())
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	bufferLength := 4096
	buffer := make([]byte, bufferLength)
	sender := call.SendStream()
	var sentTotal int64
	const logChunk = 1 << 20
	for {
		n, err := file.Read(buffer)
		switch err {
		case io.EOF:
			ctx.VI(1).Infof("Download complete for %v (%v bytes)", fileName, fStat.Size())
			return nil
		case nil:
			if err := sender.Send(buffer[:n]); err != nil {
				return err
			}
			if sentTotal/logChunk < (sentTotal+int64(n))/logChunk {
				ctx.VI(1).Infof("Download progress for %v: %v/%v", fileName, sentTotal+int64(n), fStat.Size())
			}
			sentTotal += int64(n)
		default:
			return err
		}
	}
}

func (binaryInvoker) DownloadUrl(*context.T, rpc.ServerCall) (string, int64, error) {
	return "", 0, errNotImplemented
}

func (i binaryInvoker) Stat(*context.T, rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	fileName := string(i)
	h := md5.New()
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return []binary.PartInfo{}, repository.MediaInfo{}, err
	}
	h.Write(bytes)
	part := binary.PartInfo{Checksum: hex.EncodeToString(h.Sum(nil)), Size: int64(len(bytes))}
	return []binary.PartInfo{part}, packages.MediaInfoForFileName(fileName), nil
}

func (binaryInvoker) Upload(*context.T, repository.BinaryUploadServerCall, int32) error {
	return errNotImplemented
}

func (binaryInvoker) GetPermissions(*context.T, rpc.ServerCall) (perms access.Permissions, version string, err error) {
	return nil, "", errNotImplemented
}

func (binaryInvoker) SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return errNotImplemented
}

type envelopeInvoker application.Envelope

func (i envelopeInvoker) Match(*context.T, rpc.ServerCall, []string) (application.Envelope, error) {
	return application.Envelope(i), nil
}

func (envelopeInvoker) GetPermissions(*context.T, rpc.ServerCall) (perms access.Permissions, version string, err error) {
	return nil, "", errNotImplemented
}

func (envelopeInvoker) SetPermissions(*context.T, rpc.ServerCall, access.Permissions, string) error {
	return errNotImplemented
}

func (envelopeInvoker) TidyNow(*context.T, rpc.ServerCall) error {
	return errNotImplemented
}

func servePackage(p string, ms *mapServer, tmpZipDir string) (string, string, error) {
	info, err := os.Stat(p)
	if os.IsNotExist(err) {
		return "", "", fmt.Errorf("%v not found: %v", p, err)
	} else if err != nil {
		return "", "", fmt.Errorf("Stat(%v) failed: %v", p, err)
	}
	pkgName := naming.Join("packages", info.Name())
	fileName := p
	// Directory packages first get zip'ped.
	if info.IsDir() {
		fileName = filepath.Join(tmpZipDir, info.Name()+".zip")
		if err := packages.CreateZip(fileName, p); err != nil {
			return "", "", err
		}
	}
	name, err := ms.serve(pkgName, repository.BinaryServer(binaryInvoker(fileName)))
	return info.Name(), name, err
}

// runInstallLocal creates a new envelope on the fly from the provided
// arguments, and then points the device manager back to itself for downloading
// the app envelope and binary.
//
// It sets up an app and binary server that only lives for the duration of the
// command, and listens on the profile's listen spec.
func runInstallLocal(ctx *context.T, env *cmdline.Env, args []string) error {
	if expectedMin, got := 2, len(args); got < expectedMin {
		return env.UsageErrorf("install-local: incorrect number of arguments, expected at least %d, got %d", expectedMin, got)
	}
	deviceName, title := args[0], args[1]
	args = args[2:]
	envelope := application.Envelope{Title: title}
	// Extract the environment settings, binary, and arguments.
	firstNonEnv := len(args)
	for i, arg := range args {
		if strings.Index(arg, "=") <= 0 {
			firstNonEnv = i
			break
		}
	}
	envelope.Env = args[:firstNonEnv]
	args = args[firstNonEnv:]
	if len(args) == 0 {
		return env.UsageErrorf("install-local: missing binary")
	}
	binary := args[0]
	args = args[1:]
	firstNonArg, firstPackage := len(args), len(args)
	for i, arg := range args {
		if arg == "PACKAGES" {
			firstNonArg = i
			firstPackage = i + 1
			break
		}
	}
	envelope.Args = args[:firstNonArg]
	pkgs := args[firstPackage:]
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("binary %v not found: %v", binary, err)
	}
	ctx, server, cancel, err := createServer(ctx, env.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create server: %v", err)
	}
	defer cancel()
	envelope.Binary.File, err = server.serve("binary", repository.BinaryServer(binaryInvoker(binary)))
	if err != nil {
		return err
	}
	ctx.VI(1).Infof("binary %v serving as %v", binary, envelope.Binary.File)

	// For each package dir/file specified in the arguments list, set up an
	// object in the binary service to serve that package, and add the
	// object name to the envelope's Packages map.
	tmpZipDir, err := ioutil.TempDir("", "packages")
	if err != nil {
		return fmt.Errorf("failed to create a temp dir for zip packages: %v", err)
	}
	defer os.RemoveAll(tmpZipDir)
	for _, p := range pkgs {
		if envelope.Packages == nil {
			envelope.Packages = make(application.Packages)
		}
		pname, oname, err := servePackage(p, server, tmpZipDir)
		if err != nil {
			return err
		}
		ctx.VI(1).Infof("package %v serving as %v", pname, oname)
		envelope.Packages[pname] = application.SignedFile{File: oname}
	}
	packagesRewritten := application.Packages{}
	for pname, pspec := range packagesOverride {
		_, oname, err := servePackage(pspec.File, server, tmpZipDir)
		if err != nil {
			return err
		}
		ctx.VI(1).Infof("package %v serving as %v", pname, oname)
		pspec.File = oname
		packagesRewritten[pname] = pspec
	}
	appName, err := server.serve("application", repository.ApplicationServer(envelopeInvoker(envelope)))
	if err != nil {
		return err
	}
	ctx.VI(1).Infof("application serving envelope as %v", appName)
	appID, err := device.ApplicationClient(deviceName).Install(ctx, appName, device.Config(configOverride), packagesRewritten)
	// Reset the value for any future invocations of "install" or
	// "install-local" (we run more than one command per process in unit
	// tests).
	configOverride = configFlag{}
	packagesOverride = packagesFlag{}
	if err != nil {
		return fmt.Errorf("Install failed: %v", err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", naming.Join(deviceName, appID))
	return nil
}
