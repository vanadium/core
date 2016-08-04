// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lockutil contains utilities for building file locks.
package lockutil

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
)

const (
	// Should be incremented each time there is a change to the contents of
	// the lock file.
	currentVersion uint32 = 1
	// Should not change across versions.
	versionLabel = "VERSION"
)

// CreateLockFile writes information about the process that holds the lock to a
// temporary file having the specified name prefix in the specified directory.
// It returns the name of the temporary file.
func CreateLockFile(dir, file string) (string, error) {
	f, err := ioutil.TempFile(dir, file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s:%d\n", versionLabel, currentVersion); err != nil {
		return "", err
	}
	switch currentVersion {
	case 0:
		err = createV0(f)
	case 1:
		err = createV1(f)
	default:
		return "", fmt.Errorf("unknown version: %d", currentVersion)
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func parseValue(data []byte, key string) (string, []byte, error) {
	eol := bytes.IndexByte(data, '\n')
	if eol == -1 {
		return "", nil, fmt.Errorf("couldn't parse %s from %s", key, string(data))
	}
	prefix := []byte(key + ":")
	if !bytes.HasPrefix(data, prefix) {
		return "", nil, fmt.Errorf("couldn't parse %s from %s", key, string(data))
	}
	return string(bytes.TrimPrefix(data[:eol], prefix)), data[eol+1:], nil
}

// StillHeld verifies if the given lock information corresponds to a running
// process.  False positives are allowed (returning true when in fact the lock
// holder process no longer runs), but false negatives should be disallowed
// (returning false when in fact the lock holder still runs -- since that leads
// to poaching active locks).
func StillHeld(info []byte) (bool, error) {
	versionStr, infoLeft, err := parseValue(info, versionLabel)
	if err != nil {
		return false, err
	}
	version, err := strconv.ParseUint(versionStr, 10, 32)
	if err != nil {
		return false, fmt.Errorf("couldn't parse version from %s", versionStr)
	}
	switch version {
	case 0:
		return stillHeldV0(infoLeft)
	case 1:
		return stillHeldV1(infoLeft)
	default:
		return false, fmt.Errorf("unknown version: %d", version)
	}
}
