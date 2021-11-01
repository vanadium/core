// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <stdlib.h>
// #include <openssl/crypto.h>
// #include <openssl/err.h>
// #include <openssl/evp.h>
//
// EVP_PKEY *openssl_new_raw_public_key(unsigned char *keyBytes, size_t keyLen, unsigned long *e);
// EVP_PKEY *openssl_new_raw_private_key(unsigned char *keyBytes, size_t keyLen, unsigned long *e);
import "C"

import (
	"crypto/ed25519"
	"runtime"
	"unsafe"
)

type opensslED25519PublicKey struct {
	k         *C.EVP_PKEY
	keyCBytes *C.uchar
	publicKeyCommon
}

func (k *opensslED25519PublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(unsafe.Pointer(k.keyCBytes))
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
		publicKeyCommon: newPublicKeyCommon(SHA512Hash, golang),
		keyCBytes:       (*C.uchar)(C.CBytes(golang)),
	}
	var errno C.ulong
	ret.k = C.openssl_new_raw_public_key(ret.keyCBytes, C.ulong(len(golang)), &errno)
	if ret.k == nil {
		return nil, opensslMakeError(errno)
	}
	runtime.SetFinalizer(ret, func(k *opensslED25519PublicKey) { k.finalize() })
	return ret, nil
}

type opensslED25519Signer struct {
	k         *C.EVP_PKEY
	keyCBytes *C.uchar
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(unsafe.Pointer(k.keyCBytes))
}

func (k *opensslED25519Signer) sign(data []byte) ([]byte, error) {
	return evpSignOneShot(k.k, ed25519.SignatureSize, nil, data)
}

func newOpenSSLED25519Signer(golang ed25519.PrivateKey) (Signer, error) {
	impl := &opensslED25519Signer{}
	// The go ed25119 package stores the private and public keys in the
	// same byte slice as private:public but does not provide a method for
	// obtaining just the private key, so we extract the first 32 bytes
	// here tp get the private key and the trailing 32 for the public key.
	epk := ed25519.PublicKey(golang[ed25519.SeedSize:])
	pubkey, err := newOpenSSLED25519PublicKey(epk)
	if err != nil {
		return nil, err
	}
	privKey := golang[:ed25519.SeedSize]
	impl.keyCBytes = (*C.uchar)(C.CBytes(privKey))
	var errno C.ulong
	impl.k = C.openssl_new_raw_private_key(impl.keyCBytes, C.ulong(len(privKey)), &errno)
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
