// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package versioning handles device manager versioning.  Device manager
// binaries need to be compatible with the existing device manager installation.
package versioning

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/x/lib/metadata"
	"v.io/x/ref/services/device/internal/errors"
)

// Version info for this device manager binary. Increment as appropriate when the binary changes.
// The major version number should be incremented whenever a change to the binary makes it incompatible
// with on-disk state created by a binary from a different major version.
type Version struct{ Major, Minor int }

var CurrentVersion = Version{1, 0}

// CreationInfo holds data about the binary that originally created the device manager on-disk state
type CreatorInfo struct {
	Version  Version
	MetaData string
}

func SaveCreatorInfo(ctx *context.T, dir string) error {
	info := CreatorInfo{
		Version:  CurrentVersion,
		MetaData: metadata.ToXML(),
	}
	jsonInfo, err := json.Marshal(info)
	if err != nil {
		ctx.Errorf("Marshal(%v) failed: %v", info, err)
		return verror.New(errors.ErrOperationFailed, nil)
	}
	if err := os.MkdirAll(dir, os.FileMode(0700)); err != nil {
		ctx.Errorf("MkdirAll(%v) failed: %v", dir, err)
		return verror.New(errors.ErrOperationFailed, nil)
	}
	infoPath := filepath.Join(dir, "creation_info")
	if err := ioutil.WriteFile(infoPath, jsonInfo, 0600); err != nil {
		ctx.Errorf("WriteFile(%v) failed: %v", infoPath, err)
		return verror.New(errors.ErrOperationFailed, nil)
	}
	// Make the file read-only as we don't want anyone changing it
	if err := os.Chmod(infoPath, 0400); err != nil {
		ctx.Errorf("Chmod(0400, %v) failed: %v", infoPath, err)
		return verror.New(errors.ErrOperationFailed, nil)
	}
	return nil
}

func loadCreatorInfo(ctx *context.T, dir string) (*CreatorInfo, error) {
	infoPath := filepath.Join(dir, "creation_info")
	info := new(CreatorInfo)
	if infoBytes, err := ioutil.ReadFile(infoPath); err != nil {
		ctx.Errorf("ReadFile(%v) failed: %v", infoPath, err)
		return nil, verror.New(errors.ErrOperationFailed, nil)
	} else if err := json.Unmarshal(infoBytes, info); err != nil {
		ctx.Errorf("Unmarshal(%v) failed: %v", infoBytes, err)
		return nil, verror.New(errors.ErrOperationFailed, nil)
	}
	return info, nil
}

// Checks the compatibilty of the running binary against the device manager directory on disk
func CheckCompatibility(ctx *context.T, dir string) error {
	if infoOnDisk, err := loadCreatorInfo(ctx, dir); err != nil {
		ctx.Errorf("Failed to load creator info from %s", dir)
		return verror.New(errors.ErrOperationFailed, nil)
	} else if CurrentVersion.Major != infoOnDisk.Version.Major {
		ctx.Errorf("Device Manager binary vs disk major version mismatch (%+v vs %+v)",
			CurrentVersion, infoOnDisk.Version)
		return verror.New(errors.ErrOperationFailed, nil)
	}
	return nil
}
