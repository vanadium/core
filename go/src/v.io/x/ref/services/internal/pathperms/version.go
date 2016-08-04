// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathperms

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"

	"v.io/v23/security/access"
)

// ComputeVersion produces the tag value returned by access.GetPermissions()
// (per v23/services/permissions/service.vdl) that GetPermissions/SetPermissions
// use to determine if the Permissions have been asynchronously modified.
func ComputeVersion(perms access.Permissions) (string, error) {
	b := new(bytes.Buffer)
	if err := access.WritePermissions(b, perms); err != nil {
		return "", err
	}

	md5hash := md5.Sum(b.Bytes())
	version := hex.EncodeToString(md5hash[:])
	return version, nil
}
