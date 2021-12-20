// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security

// #cgo CFLAGS: -DOPENSSL_API_COMPAT=30000 -DOPENSSL_NO_DEPRECATED
// #cgo pkg-config: libcrypto
// #include <openssl/evp.h>
//
// EVP_PKEY *openssl_evp_private_key(int keyType, const unsigned char* data, long len, unsigned long* e);
import "C"

import (
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"runtime"
)

type opensslED25519PublicKey struct {
	opensslPublicKeyCommon
}

func (k *opensslED25519PublicKey) finalize() {
	C.EVP_PKEY_free(k.osslKey)
}

func (k *opensslED25519PublicKey) messageDigest(hash crypto.Hash, purpose, message []byte) []byte {
	return sum(hash, messageDigestFields(hash, k.keyBytes, purpose, message))
}

func (k *opensslED25519PublicKey) verify(digest []byte, signature *Signature) bool {
	ok, _ := evpVerify(k.osslKey, nil, digest, signature.Ed25519)
	return ok
}

func newOpenSSLED25519PublicKey(golang ed25519.PublicKey, hash Hash) (PublicKey, error) {
	pc, err := newOpensslPublicKeyCommon(golang, hash)
	if err != nil {
		return nil, err
	}
	ret := &opensslED25519PublicKey{pc}
	runtime.SetFinalizer(ret, func(k *opensslED25519PublicKey) { k.finalize() })
	return ret, nil
}

type opensslED25519Signer struct {
	osslKey *C.EVP_PKEY
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.osslKey)
}

func (k *opensslED25519Signer) sign(data []byte) ([]byte, error) {
	return evpSignOneShot(k.osslKey, ed25519.SignatureSize, nil, data)
}

func newOpenSSLED25519Signer(golang ed25519.PrivateKey, hash Hash) (Signer, error) {
	impl := &opensslED25519Signer{}
	epk := ed25519.PublicKey(golang[ed25519.SeedSize:])
	pubkey, err := newOpenSSLED25519PublicKey(epk, hash)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	impl.osslKey = C.openssl_evp_private_key(C.EVP_PKEY_ED25519, uchar(der), C.long(len(der)), &errno)
	if impl.osslKey == nil {
		return nil, opensslMakeError(errno)
	}
	runtime.SetFinalizer(impl, func(k *opensslED25519Signer) { k.finalize() })
	return &ed25519Signer{
		signerCommon: newSignerCommon(pubkey, hash),
		sign:         impl.sign,
		impl:         impl,
	}, nil
}

func newInMemoryED25519SignerImpl(key ed25519.PrivateKey, hash Hash) (Signer, error) {
	return newOpenSSLED25519Signer(key, hash)
}

func newED25519PublicKeyImpl(key ed25519.PublicKey, hash Hash) PublicKey {
	if key, err := newOpenSSLED25519PublicKey(key, hash); err == nil {
		return key
	}
	return newGoStdlibED25519PublicKey(key, hash)
}
