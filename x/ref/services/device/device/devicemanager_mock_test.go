// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/binary"
	"v.io/v23/services/device"
	"v.io/v23/services/repository"

	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/internal/packages"
	"v.io/x/ref/services/internal/servicetest"
)

type mockDeviceInvoker struct {
	tape *servicetest.Tape
	t    *testing.T
}

// Mock ListAssociations
type ListAssociationResponse struct {
	na  []device.Association
	err error
}

func (mdi *mockDeviceInvoker) ListAssociations(ctx *context.T, _ rpc.ServerCall) (associations []device.Association, err error) {
	ctx.VI(2).Infof("ListAssociations() was called")

	ir := mdi.tape.Record("ListAssociations")
	r := ir.(ListAssociationResponse)
	return r.na, r.err
}

// Mock AssociateAccount
type AddAssociationStimulus struct {
	fun           string
	identityNames []string
	accountName   string
}

// simpleCore implements the core of all mock methods that take
// arguments and return error.
func (mdi *mockDeviceInvoker) simpleCore(callRecord interface{}, name string) error {
	ri := mdi.tape.Record(callRecord)
	switch r := ri.(type) {
	case nil:
		return nil
	case error:
		return r
	}
	log.Fatalf("%s (mock) response %v is of bad type", name, ri)
	return nil
}

func (mdi *mockDeviceInvoker) AssociateAccount(_ *context.T, _ rpc.ServerCall, identityNames []string, accountName string) error {
	return mdi.simpleCore(AddAssociationStimulus{"AssociateAccount", identityNames, accountName}, "AssociateAccount")
}

func (mdi *mockDeviceInvoker) Claim(_ *context.T, _ rpc.ServerCall, pairingToken string) error {
	return mdi.simpleCore("Claim", "Claim")
}

func (*mockDeviceInvoker) Describe(*context.T, rpc.ServerCall) (device.Description, error) {
	return device.Description{}, nil
}

func (*mockDeviceInvoker) IsRunnable(_ *context.T, _ rpc.ServerCall, description binary.Description) (bool, error) {
	return false, nil
}

func (*mockDeviceInvoker) Reset(_ *context.T, _ rpc.ServerCall, deadline uint64) error { return nil }

// Mock Install
type InstallStimulus struct {
	fun      string
	appName  string
	config   device.Config
	packages application.Packages
	envelope application.Envelope
	// files holds a map from file  or package name to file or package size.
	// The app binary  has the key "binary". Each of  the packages will have
	// the key "package/<package name>". The override packages will have the
	// key "overridepackage/<package name>".
	files map[string]int64
}

type InstallResponse struct {
	appID string
	err   error
}

const (
	// If provided with this app name, the mock device manager skips trying
	// to fetch the envelope from the name.
	appNameNoFetch = "skip-envelope-fetching"
	// If provided with a fetcheable app name, the mock device manager sets
	// the app name in the stimulus to this constant.
	appNameAfterFetch = "envelope-fetched"
	// The mock device manager sets the binary name in the envelope in the
	// stimulus to this constant.
	binaryNameAfterFetch = "binary-fetched"
)

func packageSize(pkgPath string) int64 {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return -1
	}
	if info.IsDir() {
		infos, err := ioutil.ReadDir(pkgPath)
		if err != nil {
			return -1
		}
		var size int64
		for _, i := range infos {
			size += i.Size()
		}
		return size
	}
	return info.Size()
}

func fetchPackageSize(ctx *context.T, pkgVON string) (int64, error) {
	dir, err := ioutil.TempDir("", "package")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp package dir: %v", err)
	}
	defer os.RemoveAll(dir)
	tmpFile := filepath.Join(dir, "downloaded")
	if err := binarylib.DownloadToFile(ctx, pkgVON, tmpFile); err != nil {
		return 0, fmt.Errorf("DownloadToFile failed: %v", err)
	}
	dst := filepath.Join(dir, "install")
	if err := packages.Install(tmpFile, dst); err != nil {
		return 0, fmt.Errorf("packages.Install failed: %v", err)
	}
	return packageSize(dst), nil
}

func (mdi *mockDeviceInvoker) Install(ctx *context.T, _ rpc.ServerCall, appName string, config device.Config, packages application.Packages) (string, error) {
	is := InstallStimulus{"Install", appName, config, packages, application.Envelope{}, nil}
	if appName != appNameNoFetch {
		// Fetch the envelope and record it in the stimulus.
		envelope, err := repository.ApplicationClient(appName).Match(ctx, []string{"test"})
		if err != nil {
			return "", err
		}
		binaryName := envelope.Binary.File
		envelope.Binary.File = binaryNameAfterFetch
		is.appName = appNameAfterFetch
		is.files = make(map[string]int64)
		// Fetch the binary and record its size in the stimulus.
		data, mediaInfo, err := binarylib.Download(ctx, binaryName)
		if err != nil {
			return "", err
		}
		is.files["binary"] = int64(len(data))
		if mediaInfo.Type != "application/octet-stream" {
			return "", fmt.Errorf("unexpected media type: %v", mediaInfo)
		}
		// Iterate over the packages, download them, compute the size of
		// the file(s) that make up each package, and record that in the
		// stimulus.
		for pkgLocalName, pkgVON := range envelope.Packages {
			size, err := fetchPackageSize(ctx, pkgVON.File)
			if err != nil {
				return "", err
			}
			is.files[naming.Join("packages", pkgLocalName)] = size
		}
		envelope.Packages = nil
		for pkgLocalName, pkg := range packages {
			size, err := fetchPackageSize(ctx, pkg.File)
			if err != nil {
				return "", err
			}
			is.files[naming.Join("overridepackages", pkgLocalName)] = size
		}
		is.packages = nil
		is.envelope = envelope
	}
	r := mdi.tape.Record(is).(InstallResponse)
	return r.appID, r.err
}

func (mdi *mockDeviceInvoker) Run(*context.T, rpc.ServerCall) error {
	return mdi.simpleCore("Run", "Run")
}

func (mdi *mockDeviceInvoker) Revert(*context.T, rpc.ServerCall) error {
	return mdi.simpleCore("Revert", "Revert")
}

type InstantiateResponse struct {
	err        error
	instanceID string
}

func (mdi *mockDeviceInvoker) Instantiate(*context.T, rpc.StreamServerCall) (string, error) {
	ir := mdi.tape.Record("Instantiate")
	r := ir.(InstantiateResponse)
	return r.instanceID, r.err
}

type KillStimulus struct {
	fun   string
	delta time.Duration
}

func (mdi *mockDeviceInvoker) Kill(_ *context.T, _ rpc.ServerCall, delta time.Duration) error {
	return mdi.simpleCore(KillStimulus{"Kill", delta}, "Kill")
}

func (mdi *mockDeviceInvoker) Delete(*context.T, rpc.ServerCall) error {
	return mdi.simpleCore("Delete", "Delete")
}

func (*mockDeviceInvoker) Uninstall(*context.T, rpc.ServerCall) error { return nil }

func (mdi *mockDeviceInvoker) Update(*context.T, rpc.ServerCall) error {
	return mdi.simpleCore("Update", "Update")
}

func (*mockDeviceInvoker) UpdateTo(*context.T, rpc.ServerCall, string) error { return nil }

// Mock Permissions getting and setting
type GetPermissionsResponse struct {
	perms   access.Permissions
	version string
	err     error
}

type SetPermissionsStimulus struct {
	fun     string
	perms   access.Permissions
	version string
}

func (mdi *mockDeviceInvoker) SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return mdi.simpleCore(SetPermissionsStimulus{"SetPermissions", perms, version}, "SetPermissions")
}

func (mdi *mockDeviceInvoker) GetPermissions(*context.T, rpc.ServerCall) (access.Permissions, string, error) {
	ir := mdi.tape.Record("GetPermissions")
	r := ir.(GetPermissionsResponse)
	return r.perms, r.version, r.err
}

func (mdi *mockDeviceInvoker) Debug(*context.T, rpc.ServerCall) (string, error) {
	ir := mdi.tape.Record("Debug")
	r := ir.(string)
	return r, nil
}

func (mdi *mockDeviceInvoker) Status(*context.T, rpc.ServerCall) (device.Status, error) {
	ir := mdi.tape.Record("Status")
	switch r := ir.(type) {
	case device.Status:
		return r, nil
	case error:
		return nil, r
	default:
		log.Fatalf("Status (mock) response %v is of bad type", ir)
		return nil, nil
	}
}

type GlobStimulus struct {
	pattern string
}

type GlobResponse struct {
	results []string
	err     error
}

func (mdi *mockDeviceInvoker) Glob__(_ *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	gs := GlobStimulus{g.String()}
	gr := mdi.tape.Record(gs).(GlobResponse)
	for _, r := range gr.results {
		//nolint:errcheck
		call.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: r}})
	}
	return gr.err
}

type dispatcher struct {
	tapes *servicetest.TapeMap
	t     *testing.T
}

func newDispatcher(t *testing.T, tapes *servicetest.TapeMap) rpc.Dispatcher {
	return &dispatcher{tapes: tapes, t: t}
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return &mockDeviceInvoker{tape: d.tapes.ForSuffix(suffix), t: d.t}, nil, nil
}
