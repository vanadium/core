// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !openssl

package security

import (
	"crypto/ecdsa"
)

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey) (Signer, error) {
	return newGoStdlibECDSASigner(key)
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey) PublicKey {
	return newGoStdlibECDSAPublicKey(key)
}
