// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package keys provides support for working with an extensible set of
// cryptographic keys. In particular, marshaling and parsing of key data
// is supported.
package keys

// CryptoAlgo represents the supported cryptographic algorithms.
type CryptoAlgo int

// Supported key types.
const (
	UnsupportedAlgoType CryptoAlgo = iota
	ECDSA256
	ECDSA384
	ECDSA521
	ED25519
	RSA2048
	RSA4096
)

func (algo CryptoAlgo) String() string {
	switch algo {
	case ECDSA256:
		return "ecdsa-256"
	case ECDSA384:
		return "ecdsa-384"
	case ECDSA521:
		return "ecdsa-521"
	case ED25519:
		return "ed25519"
	case RSA2048:
		return "rsa-2048"
	case RSA4096:
		return "rsa-4096"
	}
	return "unknown"
}
