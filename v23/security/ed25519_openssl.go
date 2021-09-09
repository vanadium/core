// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <stdlib.h>
// #include <openssl/bn.h>
// #include <openssl/crypto.h>
// #include <openssl/err.h>
// #include <openssl/evp.h>
//
import "C"

import (
	"crypto/ed25519"
	"fmt"
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
	return evpVerify(k.k, "ED25519", nil, digest, signature.Ed25519)
}

func newOpenSSLED25519PublicKey(golang ed25519.PublicKey) (PublicKey, error) {
	ret := &opensslED25519PublicKey{
		publicKeyCommon: newPublicKeyCommon(SHA512Hash, golang),
	}
	pkb := (*C.uchar)(C.CBytes(golang))
	k := C.EVP_PKEY_new_raw_public_key(
		C.EVP_PKEY_ED25519,
		nil,
		pkb,
		C.ulong(len(golang)),
	)
	if err := opensslGetErrors(); err != nil {
		return nil, fmt.Errorf("newOpenSSLED25519PublicKey: %v", err)
	}
	ret.k = k
	ret.keyCBytes = pkb
	runtime.SetFinalizer(ret, func(k *opensslED25519PublicKey) { k.finalize() })
	return ret, nil
}

type opensslED25519Signer struct {
	k         *C.EVP_PKEY
	keyCBytes unsafe.Pointer
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(k.keyCBytes)
}

func (k *opensslED25519Signer) sign(data []byte) ([]byte, error) {
	return evpSignOneShot(k.k, "ED25519", ed25519.SignatureSize, nil, data)
}

func newOpenSSLED25519Signer(golang ed25519.PrivateKey) (Signer, error) {
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
	pkb := C.CBytes(privKey)
	pk := C.EVP_PKEY_new_raw_private_key(
		C.EVP_PKEY_ED25519,
		nil,
		(*C.uchar)(pkb),
		C.ulong(len(privKey)),
	)
	if err := opensslGetErrors(); err != nil {
		return nil, err
	}
	impl := &opensslED25519Signer{pk, pkb}
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
