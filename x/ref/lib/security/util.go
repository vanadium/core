// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"

	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/signing/sshagent"
)

// KeyType represents the supported key types.
type KeyType int

// Supported key types.
const (
	UnsupportedKeyType KeyType = iota
	ECDSA256
	ECDSA384
	ECDSA521
	ED25519
	RSA2048
	RSA4096
)

// NewPrivateKey creates a new private key of the requested type.
// keyType must be one of ecdsa256, ecdsa384, ecdsa521, ed25519,
// rsa 2048 or rsa 4096 bit.
func NewPrivateKey(keyType KeyType) (crypto.PrivateKey, error) {
	switch keyType {
	case ECDSA256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case ECDSA384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case ECDSA521:
		return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	case ED25519:
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		return privateKey, err
	case RSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case RSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	default:
		return nil, fmt.Errorf("unsupported key type: %T", keyType)
	}
}

func NewSSHAgentHostedKey(publicKeyFile string) (crypto.PrivateKey, error) {
	keyBytes, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, err
	}
	return sshagent.NewHostedKey(keyBytes)
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
