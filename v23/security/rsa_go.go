// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !openssl

package security

import (
	"crypto/rsa"
)

func newInMemoryRSASignerImpl(key *rsa.PrivateKey, hash Hash) (Signer, error) {
	return newGoStdlibRSASigner(key, hash)
}

func newRSAPublicKeyImpl(key *rsa.PublicKey, hash Hash) PublicKey {
	return newGoStdlibRSAPublicKey(key, hash)
}
