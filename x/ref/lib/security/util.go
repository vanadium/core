// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/keys/sshkeys"
)

// NewSSHAgentHostedKey returns a *sshkeys.HostedKey for the supplied ssh
// public key file assuming that the private key is stored in an accessible
// ssh agent.
func NewSSHAgentHostedKey(publicKeyFile string) (*sshkeys.HostedKey, error) {
	keyBytes, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	key, comment, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
	if err != nil {
		return nil, err
	}
	return sshkeys.NewHostedKey(key, comment), nil
}

// lockAndLoad only needs to read the credentials information.
func readLockAndLoad(flock *lockedfile.Mutex, loader func() error) (func(), error) {
	if flock == nil {
		// in-memory store
		return func() {}, loader()
	}
	if _, ok := ref.ReadonlyCredentialsDir(); ok {
		// running on a read-only filesystem.
		return func() {}, loader()
	}
	unlock, err := flock.RLock()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		unlock, err = flock.RLock()
		if err != nil {
			return nil, fmt.Errorf("failed to create new read lock: %v", err)
		}
	}
	return unlock, loader()
}

func writeLockAndLoad(flock *lockedfile.Mutex, loader func() error) (func(), error) {
	if flock == nil {
		// in-memory store
		return func() {}, loader()
	}
	if reason, ok := ref.ReadonlyCredentialsDir(); ok {
		// running on a read-only filesystem.
		return func() {}, fmt.Errorf("the credentials directory is considered read-only and hence writes are disallowed (%v)", reason)
	}
	unlock, err := flock.Lock()
	if err != nil {
		return nil, err
	}
	return unlock, loader()
}
