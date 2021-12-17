// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

// NewRSAPublicKey creates a PublicKey object that uses the RSA algorithm and the provided RSA public key.
func NewRSAPublicKey(key *rsa.PublicKey) PublicKey {
	if key.Size() < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	return newRSAPublicKeyImpl(key)
}

type rsaPublicKey struct {
	key *rsa.PublicKey
	publicKeyCommon
}

func (pk *rsaPublicKey) verify(digest []byte, sig *Signature) bool {
	digest = sig.Hash.sum(digest)
	err := rsa.VerifyPKCS1v15(pk.key, cryptoHash(sig.Hash), digest, sig.Rsa)
	return err == nil
}

func (pk *rsaPublicKey) messageDigest(purpose, message []byte) []byte {
	// NOTE: the openssl rsa signer/verifier KCS1v15 implementation always
	// 	     hashes the message it receives, whereas the go implementation
	//       assumes a prehashed version. Consequently this method returns
	//       the results of messageDigestFields and leaves it to the
	//       implementation of the signer to hash that value or not.
	//       For this go implementation, the results returned by this
	//       function are therefore hashed again below (see the sign method
	//       implementation provided when the signer is created).
	return messageDigestFields(pk.h, pk.keyBytes, purpose, message)
}

// NewInMemoryRSASigner creates a Signer that uses the provided RSA
// private key to sign messages. This private key is kept in the clear in
// the memory of the running process.
func NewInMemoryRSASigner(key *rsa.PrivateKey) (Signer, error) {
	signer, err := newInMemoryRSASignerImpl(key)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

// NewRSASigner creates a Signer that uses the provided function to sign
// messages. The provided method is invoked to sign messages and may be used
// to access an otherwise protected or encoded key.
func NewRSASigner(key *rsa.PublicKey, sign func(data []byte) ([]byte, error)) Signer {
	return &rsaSigner{sign: sign, pubkey: NewRSAPublicKey(key)}
}

type rsaSigner struct {
	sign   func(data []byte) (sig []byte, err error)
	pubkey PublicKey
	// Object to hold on to for garbage collection
	impl interface{} //nolint:structcheck,unused
}

func (c *rsaSigner) Sign(purpose, message []byte) (Signature, error) {
	hash := c.pubkey.hash()
	if message = c.pubkey.messageDigest(purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", hash)
	}
	sig, err := c.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    hash,
		Rsa:     sig,
	}, nil
}

func (c *rsaSigner) PublicKey() PublicKey {
	return c.pubkey
}

func newGoStdlibRSASigner(key *rsa.PrivateKey) (Signer, error) {
	if key.Size() < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	pk := newGoStdlibRSAPublicKey(&key.PublicKey)
	vhash := pk.hash()
	chash := crypto.SHA512
	sign := func(data []byte) ([]byte, error) {
		// hash the data since rsa.SignPKCS1v15 assumes prehashed data.
		data = vhash.sum(data)
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, chash, data)
		return sig, err
	}
	return &rsaSigner{sign: sign, pubkey: pk}, nil
}

func newGoStdlibRSAPublicKey(key *rsa.PublicKey) PublicKey {
	return &rsaPublicKey{
		key:             key,
		publicKeyCommon: newPublicKeyCommon(SHA512Hash, key),
	}
}

func cryptoHash(h Hash) crypto.Hash {
	switch h {
	case SHA1Hash:
		return crypto.SHA1
	case SHA256Hash:
		return crypto.SHA256
	case SHA384Hash:
		return crypto.SHA384
	case SHA512Hash:
		return crypto.SHA512
	}
	return crypto.SHA512
}
