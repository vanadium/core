// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"context"

	"v.io/x/ref/lib/security/keys"
)

// Make the key functions local to this package available to this package.
var keyRegistrar = keys.NewRegistrar()

func init() {
	MustRegister(keyRegistrar)
}

func importPrivateKeyBytes(ctx context.Context, keyBytes, origPassphrase, newPassphrase []byte) ([]byte, error) {
	if len(newPassphrase) == 0 {
		// downstream code can cope with cleartext or encrypted keys.
		return keyBytes, nil
	}
	// We need to encrypt the imported key bytes, so obtain the original
	// key (which may also be encrypted) and then reencrypt it.
	privKey, err := keyRegistrar.ParsePrivateKey(ctx, keyBytes, origPassphrase)
	if err != nil {
		return nil, err
	}
	// Note that the encrypted key will always be in PCKS8 format.
	return keys.MarshalPKCS8PrivateKey(privKey, newPassphrase)
}
