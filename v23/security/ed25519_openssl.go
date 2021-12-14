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
// EVP_PKEY *openssl_evp_public_key(const unsigned char *data, long len, unsigned long *e);
import "C"

import (
	"crypto/ed25519"
	"crypto/x509"
	"runtime"
)

type opensslED25519PublicKey struct {
	opensslPublicKeyCommon
}

func (k *opensslED25519PublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslED25519PublicKey) messageDigest(purpose, message []byte) []byte {
	return k.h.sum(messageDigestFields(k.h, k.keyBytes, purpose, message))
}

func (k *opensslED25519PublicKey) verify(digest []byte, signature *Signature) bool {
	ok, _ := evpVerify(k.k, nil, digest, signature.Ed25519)
	return ok
}

func newOpenSSLED25519PublicKey(golang ed25519.PublicKey) (PublicKey, error) {
	ret := &opensslED25519PublicKey{
		opensslPublicKeyCommon: newOpensslPublicKeyCommon(SHA512Hash, golang),
	}
	var errno C.ulong
	ret.k = C.openssl_evp_public_key(uchar(ret.keyBytes), C.long(len(ret.keyBytes)), &errno)
	if ret.k == nil {
		return nil, opensslMakeError(errno)
	}
	runtime.SetFinalizer(ret, func(k *opensslED25519PublicKey) { k.finalize() })
	return ret, nil
}

type opensslED25519Signer struct {
	k *C.EVP_PKEY
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslED25519Signer) sign(data []byte) ([]byte, error) {
	return evpSignOneShot(k.k, ed25519.SignatureSize, nil, data)
}

func newOpenSSLED25519Signer(golang ed25519.PrivateKey) (Signer, error) {
	impl := &opensslED25519Signer{}
	epk := ed25519.PublicKey(golang[ed25519.SeedSize:])
	pubkey, err := newOpenSSLED25519PublicKey(epk)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	impl.k = C.openssl_evp_private_key(C.EVP_PKEY_ED25519, uchar(der), C.long(len(der)), &errno)
	if impl.k == nil {
		return nil, opensslMakeError(errno)
	}
	runtime.SetFinalizer(impl, func(k *opensslED25519Signer) { k.finalize() })
	return &ed25519Signer{
		sign:   impl.sign,
		pubkey: pubkey,
		impl:   impl,
	}, nil
}

func newInMemoryED25519SignerImpl(key ed25519.PrivateKey) (Signer, error) {
	return newOpenSSLED25519Signer(key)
}

func newED25519PublicKeyImpl(key ed25519.PublicKey) PublicKey {
	if key, err := newOpenSSLED25519PublicKey(key); err == nil {
		return key
	}
	return newGoStdlibED25519PublicKey(key)
}
