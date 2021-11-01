// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !openssl
// +build !openssl

package security

import (
	"crypto/rsa"
)

func newInMemoryRSASignerImpl(key *rsa.PrivateKey) (Signer, error) {
	return newGoStdlibRSASigner(key)
}

func newRSAPublicKeyImpl(key *rsa.PublicKey) PublicKey {
	return newGoStdlibRSAPublicKey(key)
}
