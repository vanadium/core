// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
)

// NewECDSAPublicKey creates a PublicKey object that uses the ECDSA algorithm and the provided ECDSA public key.
func NewECDSAPublicKey(key *ecdsa.PublicKey) PublicKey {
	return newECDSAPublicKeyImpl(key, ecdsaHash(key))
}

type ecdsaPublicKey struct {
	publicKeyCommon
	key *ecdsa.PublicKey
}

func (pk *ecdsaPublicKey) equal(key crypto.PublicKey) bool {
	return pk.key.Equal(key)
}

func (pk *ecdsaPublicKey) verify(digest []byte, sig *Signature) bool {
	var r, s big.Int
	return ecdsa.Verify(pk.key, digest, r.SetBytes(sig.R), s.SetBytes(sig.S))
}

func (pk *ecdsaPublicKey) messageDigest(h crypto.Hash, purpose, message []byte) []byte {
	return cryptoSum(h, messageDigestFields(h, pk.keyBytes, purpose, message))
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
// key to sign messages. This private key is kept in the clear in the memory
// of the running process. The hash function used for computing digests used
// for signing is based on the curve used by the public key.
func NewInMemoryECDSASigner(key *ecdsa.PrivateKey) (Signer, error) {
	signer, err := newInMemoryECDSASignerImpl(key, ecdsaHash(&key.PublicKey))
	if err != nil {
		return nil, err
	}
	return signer, nil
}

type ecdsaSigner struct {
	signerCommon
	sign func(data []byte) (r, s *big.Int, err error)
	// Object to hold on to for garbage collection
	impl interface{} //nolint:unused
}

// NewECDSASigner creates a Signer that uses the provided function to sign
// messages. The provided method is invoked to sign messages and may be used
// to access an otherwise protected or encoded key. The hash function used
// for computing digests used for signing is based on the curve used by the
// public key.
func NewECDSASigner(key *ecdsa.PublicKey, sign func(data []byte) (r, s *big.Int, err error)) Signer {
	hash := ecdsaHash(key)
	return &ecdsaSigner{
		signerCommon: newSignerCommon(NewECDSAPublicKey(key), hash),
		sign:         sign,
	}
}

func (es *ecdsaSigner) Sign(purpose, message []byte) (Signature, error) {
	if message = es.pubkey.messageDigest(es.chash, purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", es.chash)
	}
	r, s, err := es.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    es.vhash,
		R:       r.Bytes(),
		S:       s.Bytes(),
	}, nil
}

func (es *ecdsaSigner) PublicKey() PublicKey {
	return es.pubkey
}

func newGoStdlibECDSASigner(key *ecdsa.PrivateKey, hash Hash) (Signer, error) {
	sign := func(data []byte) (r, s *big.Int, err error) {
		return ecdsa.Sign(rand.Reader, key, data)
	}
	return &ecdsaSigner{
		signerCommon: newSignerCommon(newGoStdlibECDSAPublicKey(&key.PublicKey, hash), hash),
		sign:         sign}, nil
}

func newGoStdlibECDSAPublicKey(key *ecdsa.PublicKey, hash Hash) PublicKey {
	return &ecdsaPublicKey{
		key:             key,
		publicKeyCommon: newPublicKeyCommon(key, hash),
	}
}
