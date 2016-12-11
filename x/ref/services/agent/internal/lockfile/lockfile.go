// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lockfile provides methods to associate process ids (PIDs) with a file.
package lockfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"v.io/x/lib/vlog"
	"v.io/x/ref/services/agent/internal/lockutil"
)

const (
	lockSuffix = "lock"
	tempSuffix = "templock"
)

// CreateLockfile associates the provided file with the process ID of the
// caller.
//
// file should not contain any useful content because CreateLockfile may
// delete and recreate it.
//
// Only one active process can be associated with a file at a time. Thus, if
// another active process is currently associated with file, then
// CreateLockfile will return an error.
func CreateLockfile(file string) error {
	tmpFile, err := lockutil.CreateLockFile(filepath.Dir(file), filepath.Base(file)+"-"+tempSuffix)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)
	lockfile := file + "-" + lockSuffix
	if err = os.Link(tmpFile, lockfile); err == nil {
		return nil
	}

	// Check for a stale lock
	lockProcessInfo, err := ioutil.ReadFile(lockfile)
	if err != nil {
		return err
	}
	if running, err := lockutil.StillHeld(lockProcessInfo); running {
		return fmt.Errorf("process is already running:\n%s", lockProcessInfo)
	} else if err != nil {
		return err
	}

	// Delete the file if the old process left one behind.
	if err = os.Remove(file); !os.IsNotExist(err) {
		return err
	}

	// Note(ribrdb): There's a race here between checking the file contents
	// and deleting the file, but I don't think it will be an issue in normal
	// usage.
	return os.Rename(tmpFile, lockfile)
}

// RemoveLockfile removes file and the corresponding lock on it.
func RemoveLockfile(file string) {
	path := file + "-" + lockSuffix
	if err := os.Remove(path); err != nil {
		vlog.Infof("Unable to remove lockfile %q: %v", path, err)
	}
	if err := os.Remove(file); err != nil {
		vlog.Infof("Unable to remove %q: %v", path, err)
	}
}
