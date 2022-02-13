// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"

	"v.io/v23/security"
)

// CreatePersistentPrincipal wraps CreatePersistentPrincipalUsingKey to
// creates a new Principal using a newly generated ECSDA key.
func CreatePersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	store, err := CreateFilesystemStore(dir)
	if err != nil {
		return nil, err
	}
	return CreatePrincipalOpts(context.TODO(), UseStore(store))
}

// CreatePersistentPrincipalUsingKey creates a new Principal using the supplied
// key and commits all state changes to the provided directory.
//
// The private key is serialized and saved encrypted if the
// 'passphrase' is non-nil, and unencrypted otherwise.
//
// If the directory has any preexisting principal data, an error is returned.
//
// The specified directory may not exist, in which case it will be created.
// The follow key types are supported:
// *ecdsa.PrivateKey, ed25519.PrivateKey, *rsa.PrivateKey and *sshkeys.HostedKey.
func CreatePersistentPrincipalUsingKey(ctx context.Context, key crypto.PrivateKey, dir string, passphrase []byte) (security.Principal, error) {
	store, err := CreateFilesystemStore(dir)
	if err != nil {
		return nil, err
	}
	return CreatePrincipalOpts(ctx, UseStore(store), UsePrivateKey(key, passphrase))
}
