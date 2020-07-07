// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !openssl

package security

import "crypto/ed25519"

func newInMemoryED25519SignerImpl(key ed25519.PrivateKey) (Signer, error) {
	sign := func(data []byte) ([]byte, error) {
		return ed25519.Sign(key, data), nil
	}
	pk := key.Public()
	ed25519PK := pk.(ed25519.PublicKey)
	return &ed25519Signer{sign: sign, pubkey: &ed25519PublicKey{ed25519PK}}, nil
}

func newED25519PublicKeyImpl(key ed25519.PublicKey) PublicKey {
	return &ed25519PublicKey{key}
}
