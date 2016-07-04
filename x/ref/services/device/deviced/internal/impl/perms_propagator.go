// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"path/filepath"

	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/ref/services/internal/pathperms"
)

// computePath builds the desired path for the debug perms.
func computePath(path string) string {
	return filepath.Join(path, "debugacls")
}

// setPermsForDebugging constructs a Permissions file for use by applications
// that permits principals with a Debug right on an application instance to
// access names in the app's __debug space.
func setPermsForDebugging(blessings []string, perms access.Permissions, instancePath string, permsStore *pathperms.PathStore) error {
	path := computePath(instancePath)
	newPerms := make(access.Permissions)

	// Add blessings for the DM so that it can access the app too.

	set := func(bl security.BlessingPattern) {
		for _, tag := range []access.Tag{access.Resolve, access.Debug} {
			newPerms.Add(bl, string(tag))
		}
	}

	for _, b := range blessings {
		set(security.BlessingPattern(b))
	}

	// add Resolve for every blessing that has debug
	for _, v := range perms["Debug"].In {
		set(v)
	}
	_, err := permsStore.SetShareable(path, newPerms, "", true, true)
	return err
}
