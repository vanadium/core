// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

func newInMemoryRSASignerImpl(key *rsa.PrivateKey) (Signer, error) {
	// TODO(cnicolaou): openssl support for RSA.
	return newGoStdlibRSASigner(key)
}

func newRSAPublicKeyImpl(key *rsa.PublicKey) PublicKey {
	// TODO(cnicolaou): openssl support for RSA.
	return newGoStdlibRSAPublicKey(key)
}
