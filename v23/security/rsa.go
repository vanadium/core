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

// NewRSAPublicKey creates a PublicKey object that uses the RSA algorithm and
// the provided RSA public key.
func NewRSAPublicKey(key *rsa.PublicKey) PublicKey {
	if key.Size() < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	return newRSAPublicKeyImpl(key, SHA512Hash)
}

type rsaPublicKey struct {
	key *rsa.PublicKey
	publicKeyCommon
}

func (pk *rsaPublicKey) verify(digest []byte, sig *Signature) bool {
	digest = sum(cryptoHash(sig.Hash), digest)
	err := rsa.VerifyPKCS1v15(pk.key, cryptoHash(sig.Hash), digest, sig.Rsa)
	return err == nil
}

func (pk *rsaPublicKey) messageDigest(hash crypto.Hash, purpose, message []byte) []byte {
	// NOTE: the openssl rsa signer/verifier KCS1v15 implementation always
	// 	     hashes the message it receives, whereas the go implementation
	//       assumes a prehashed version. Consequently this method returns
	//       the results of messageDigestFields and leaves it to the
	//       implementation of the signer to hash that value or not.
	//       For this go implementation, the results returned by this
	//       function are therefore hashed again below (see the sign method
	//       implementation provided when the signer is created).
	return messageDigestFields(hash, pk.keyBytes, purpose, message)
}

// NewInMemoryRSASigner creates a Signer that uses the provided RSA
// private key to sign messages. This private key is kept in the clear in
// the memory of the running process. SHA512 is used for computing the digests
// used for signing.
func NewInMemoryRSASigner(key *rsa.PrivateKey) (Signer, error) {
	signer, err := newInMemoryRSASignerImpl(key, SHA512Hash)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

// NewRSASigner creates a Signer that uses the provided function to sign
// messages. The provided method is invoked to sign messages and may be used
// to access an otherwise protected or encoded key. SHA512 is used for
// computing the digests used for signing.
func NewRSASigner(key *rsa.PublicKey, sign func(data []byte) ([]byte, error)) Signer {
	return &rsaSigner{
		signerCommon: newSignerCommon(NewRSAPublicKey(key), SHA512Hash),
		sign:         sign,
	}
}

type rsaSigner struct {
	signerCommon
	sign func(data []byte) (sig []byte, err error)
	// Object to hold on to for garbage collection
	impl interface{} //nolint:structcheck,unused
}

func (rs *rsaSigner) Sign(purpose, message []byte) (Signature, error) {
	if message = rs.pubkey.messageDigest(rs.chash, purpose, message); message == nil {
		return Signature{}, fmt.Errorf("unable to create bytes to sign from message with hashing function: %v", rs.chash)
	}
	sig, err := rs.sign(message)
	if err != nil {
		return Signature{}, err
	}
	return Signature{
		Purpose: purpose,
		Hash:    rs.vhash,
		Rsa:     sig,
	}, nil
}

func (rs *rsaSigner) PublicKey() PublicKey {
	return rs.pubkey
}

func newGoStdlibRSASigner(key *rsa.PrivateKey, hash Hash) (Signer, error) {
	if key.Size() < (2048 / 8) {
		panic("rsa keys with less than 2048 bits are not supported")
	}
	pk := newGoStdlibRSAPublicKey(&key.PublicKey, hash)
	sc := newSignerCommon(pk, hash)
	chash := sc.chash
	sign := func(data []byte) ([]byte, error) {
		// hash the data since rsa.SignPKCS1v15 assumes prehashed data.
		data = sum(chash, data)
		sig, err := rsa.SignPKCS1v15(rand.Reader, key, chash, data)
		return sig, err
	}
	return &rsaSigner{
		signerCommon: sc,
		sign:         sign,
	}, nil
}

func newGoStdlibRSAPublicKey(key *rsa.PublicKey, hash Hash) PublicKey {
	return &rsaPublicKey{
		key:             key,
		publicKeyCommon: newPublicKeyCommon(key, hash),
	}
}
