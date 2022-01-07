// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"embed"
	_ "embed"
	"strings"
)

//go:embed testdata/ssh-*
var sshKeys embed.FS

// SSHKeydir creates a pre-populated directory of ssh keys to use in
// tests. The following keys are installed:
//	 ssh-ecdsa-256, ssh-ecdsa-256.pub
//	 ssh-ecdsa-384, ssh-ecdsa-384.pub
//	 ssh-ecdsa-521, ssh-ecdsa-521.pub
//	 ssh-ed25519, ssh-ed25519.pub
//	 ssh-rsa-2048, ssh-rsa-2048.pub
//	 ssh-rsa-3072, ssh-rsa-3072.pub
func SSHKeydir() (string, []string, error) {
	entries, err := sshKeys.ReadDir("testdata")
	if err != nil {
		return "", nil, err
	}
	dir, err := prepopulatedDir("ssh-keys", "testdata", sshKeys)
	if err != nil {
		return "", nil, err
	}
	names := []string{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pub") {
			names = append(names, e.Name())
		}
	}
	return dir, names, nil
}
