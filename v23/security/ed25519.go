// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ed25519"
	"fmt"
)

// NewED25519PublicKey creates a PublicKey object that uses the ED25519
// algorithm and the provided ED25519 public key.
func NewED25519PublicKey(key ed25519.PublicKey) PublicKey {
	return newED25519PublicKeyImpl(key, SHA512Hash)
}

type ed25519PublicKey struct {
	publicKeyCommon
	key ed25519.PublicKey
}

func (pk *ed25519PublicKey) equal(key crypto.PublicKey) bool {
	return pk.key.Equal(key)
}

func (pk *ed25519PublicKey) verify(digest []byte, sig *Signature) bool {
	return ed25519.Verify(pk.key, digest, sig.Ed25519)
}

func (pk *ed25519PublicKey) messageDigest(hash crypto.Hash, purpose, message []byte) []byte {
	return cryptoSum(hash, messageDigestFields(hash, pk.keyBytes, purpose, message))
}

// NewInMemoryED25519Signer creates a Signer that uses the provided ED25519
// private  key to sign messages.  This private key is kept in the clear in
// the memory of the running process. SHA512 is used for computing the digests
// used for signing.
func NewInMemoryED25519Signer(key ed25519.PrivateKey) (Signer, error) {
	signer, err := newInMemoryED25519SignerImpl(key, SHA512Hash)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

type ed25519Signer struct {
	signerCommon
	sign func(data []byte) ([]byte, error)
	// Object to hold on to for garbage collection
	impl interface{} //nolint:unused
}

// NewED25519Signer creates a Signer that uses the provided function to sign
// messages. The provided method is invoked to sign messages and may be used
// to access an otherwise protected or encoded key. SHA512 is used for
// computing the digests used for signing.
func NewED25519Signer(key ed25519.PublicKey, sign func(data []byte) ([]byte, error)) Signer {
	return &ed25519Signer{
		signerCommon: newSignerCommon(NewED25519PublicKey(key), SHA512Hash),
		sign:         sign,
	}
}

func (es *ed25519Signer) Sign(purpose, message []byte) (Signature, error) {
	if message = es.pubkey.messageDigest(es.chash, purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", es.chash)
	}
	sig, err := es.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    es.vhash,
		Ed25519: sig,
	}, nil
}

func (es *ed25519Signer) PublicKey() PublicKey {
	return es.pubkey
}

func newGoStdlibED25519Signer(key ed25519.PrivateKey, hash Hash) (Signer, error) {
	sign := func(data []byte) ([]byte, error) {
		return ed25519.Sign(key, data), nil
	}
	pk := key.Public().(ed25519.PublicKey)
	return &ed25519Signer{
			signerCommon: newSignerCommon(newGoStdlibED25519PublicKey(pk, hash), hash),
			sign:         sign,
		},
		nil
}

func newGoStdlibED25519PublicKey(key ed25519.PublicKey, hash Hash) PublicKey {
	return &ed25519PublicKey{
		key:             key,
		publicKeyCommon: newPublicKeyCommon(key, hash),
	}
}
