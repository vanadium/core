// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
)

// NewECDSAPublicKey creates a PublicKey object that uses the ECDSA algorithm and the provided ECDSA public key.
func NewECDSAPublicKey(key *ecdsa.PublicKey) PublicKey {
	return newECDSAPublicKeyImpl(key)
}

type ecdsaPublicKey struct {
	key *ecdsa.PublicKey
	publicKeyCommon
}

func (pk *ecdsaPublicKey) verify(digest []byte, sig *Signature) bool {
	var r, s big.Int
	return ecdsa.Verify(pk.key, digest, r.SetBytes(sig.R), s.SetBytes(sig.S))
}

func (pk *ecdsaPublicKey) messageDigest(purpose, message []byte) []byte {
	return pk.h.sum(messageDigestFields(pk.h, pk.keyBytes, purpose, message))
}

func ecdsaHash(key *ecdsa.PublicKey) Hash {
	nbits := key.Curve.Params().BitSize
	switch {
	case nbits <= 160:
		return SHA1Hash
	case nbits <= 256:
		return SHA256Hash
	case nbits <= 384:
		return SHA384Hash
	default:
		return SHA512Hash
	}
}

// NewInMemoryECDSASigner creates a Signer that uses the provided ECDSA private
// key to sign messages.  This private key is kept in the clear in the memory
// of the running process.
func NewInMemoryECDSASigner(key *ecdsa.PrivateKey) (Signer, error) {
	signer, err := newInMemoryECDSASignerImpl(key)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

// NewECDSASigner creates a Signer that uses the provided function to sign
// messages. The provided method is invoked to sign messages and may be used
// to access an otherwise protected or encoded key.
func NewECDSASigner(key *ecdsa.PublicKey, sign func(data []byte) (r, s *big.Int, err error)) Signer {
	return &ecdsaSigner{sign: sign, pubkey: NewECDSAPublicKey(key)}
}

type ecdsaSigner struct {
	sign   func(data []byte) (r, s *big.Int, err error)
	pubkey PublicKey
	// Object to hold on to for garbage collection
	impl interface{} //nolint:structcheck,unused
}

func (c *ecdsaSigner) Sign(purpose, message []byte) (Signature, error) {
	hash := c.pubkey.hash()
	if message = c.pubkey.messageDigest(purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", hash)
	}
	r, s, err := c.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    hash,
		R:       r.Bytes(),
		S:       s.Bytes(),
	}, nil
}

func (c *ecdsaSigner) PublicKey() PublicKey {
	return c.pubkey
}

func newGoStdlibECDSASigner(key *ecdsa.PrivateKey) (Signer, error) {
	sign := func(data []byte) (r, s *big.Int, err error) {
		return ecdsa.Sign(rand.Reader, key, data)
	}
	return &ecdsaSigner{sign: sign, pubkey: newGoStdlibECDSAPublicKey(&key.PublicKey)}, nil
}

func newGoStdlibECDSAPublicKey(key *ecdsa.PublicKey) PublicKey {
	return &ecdsaPublicKey{
		key:             key,
		publicKeyCommon: newPublicKeyCommon(ecdsaHash(key), key),
	}
}
