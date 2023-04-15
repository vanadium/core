// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !openssl

package security

import (
	"crypto/ecdsa"
)

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey, hash Hash) (Signer, error) {
	return newGoStdlibECDSASigner(key, hash)
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey, hash Hash) PublicKey {
	return newGoStdlibECDSAPublicKey(key, hash)
}
