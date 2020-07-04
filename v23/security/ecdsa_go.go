// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !openssl

package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"

	"v.io/v23/verror"
)

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey) (Signer, error) {
	sign := func(data []byte) (r, s *big.Int, err error) {
		return ecdsa.Sign(rand.Reader, key, data)
	}
	return &ecdsaSigner{sign: sign, pubkey: &ecdsaPublicKey{&key.PublicKey}}, nil
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey) PublicKey {
	return &ecdsaPublicKey{key}
}

func newInMemoryED25519SignerImpl(key ed25519.PrivateKey) (Signer, error) {
	sign := func(data []byte) []byte {
		return ed25519.Sign(key, data)
	}
	pk := key.Public()
	ed25519PK := pk.(ed25519.PublicKey)
	return &ed25519Signer{sign: sign, pubkey: &ed25519PublicKey{ed25519PK}}, nil
}

func newED25519PublicKeyImpl(key ed25519.PublicKey) PublicKey {
	return &ed25519PublicKey{key}
}

func unmarshalPublicKeyImpl(bytes []byte) (PublicKey, error) {
	key, err := x509.ParsePKIXPublicKey(bytes)
	if err != nil {
		return nil, err
	}
	switch v := key.(type) {
	case *ecdsa.PublicKey:
		return &ecdsaPublicKey{v}, nil
	case ed25519.PublicKey:
		return &ed25519PublicKey{v}, nil
	default:
		return nil, verror.New(errUnrecognizedKey, nil, fmt.Sprintf("%T", key))
	}
}
