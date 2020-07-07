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
	"crypto/x509"
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"
)

type opensslED25519PublicKey struct {
	k        *C.EVP_PKEY
	keyBytes unsafe.Pointer
	der      []byte // the result of MarshalPKIXPublicKey on *ed25519.PublicKey.
}

func (k *opensslED25519PublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(k.keyBytes)
}

func (k *opensslED25519PublicKey) MarshalBinary() ([]byte, error) {
	cpy := make([]byte, len(k.der))
	copy(cpy, k.der)
	return cpy, nil
}

func (k *opensslED25519PublicKey) String() string { return publicKeyString(k) }

func (k *opensslED25519PublicKey) hash() Hash { return SHA512Hash }

func (k *opensslED25519PublicKey) verify(digest []byte, signature *Signature) bool {
	return false
}

func newOpenSSLED25519PublicKey(golang ed25519.PublicKey) (PublicKey, error) {
	der, err := x509.MarshalPKIXPublicKey(golang)
	if err != nil {
		return nil, err
	}
	pkb := C.CBytes(golang)
	pk := C.EVP_PKEY_new_raw_public_key(
		C.EVP_PKEY_ED25519,
		nil,
		(*C.uchar)(pkb),
		C.ulong(len(golang)),
	)
	if err := opensslGetErrors(); err != nil {
		return nil, fmt.Errorf("newOpenSSLED25519PublicKey: %v", err)
	}
	impl := &opensslED25519PublicKey{pk, pkb, der}
	runtime.SetFinalizer(impl, func(k *opensslED25519PublicKey) { k.finalize() })
	return impl, nil
}

type opensslED25519Signer struct {
	k        *C.EVP_PKEY
	keyBytes unsafe.Pointer
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(k.keyBytes)
}

func (k *opensslED25519Signer) sign(data []byte) (s []byte, err error) {
	md_ctx := C.EVP_MD_CTX_new()
	_, file, line, _ := runtime.Caller(1)
	file = filepath.Base(file)
	sig := C.CRYPTO_secure_zalloc(
		C.ulong(ed25519.PrivateKeySize),
		C.CString(file),
		C.int(line))
	var siglen C.ulong
	siglen = ed25519.PrivateKeySize
	fmt.Printf("XXX %p %d.. %p.. %p\n", sig, siglen, uchar(data), k.k)
	if C.EVP_DigestSignInit(md_ctx, nil, nil, nil, k.k) != 1 {
		if err := opensslGetErrors(); err != nil {
			return nil, fmt.Errorf("EVP_DigestSignInit: %v", err)
		}
	}
	if C.EVP_DigestSign(md_ctx, (*C.uchar)(sig), &siglen, uchar(data), C.ulong(len(data))) != 1 {
		if err := opensslGetErrors(); err != nil {
			return nil, fmt.Errorf("EVP_DigestSign: %v", err)
		}
		fmt.Printf("FAILED: sign .... \n")
	}
	gosig := make([]byte, siglen)
	copy(gosig, C.GoBytes(sig, C.int(siglen)))
	_, _, line, _ = runtime.Caller(1)
	C.CRYPTO_secure_free(sig, C.CString(file), C.int(line))
	C.EVP_MD_CTX_free(md_ctx)
	return gosig, nil
}

func newOpenSSLED25519Signer(golang ed25519.PrivateKey) (Signer, error) {
	pubkey, err := newOpenSSLED25519PublicKey(golang.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, err
	}
	privKey := golang[:ed25519.SeedSize]
	pkb := C.CBytes(privKey)
	pkl := C.ulong(len(privKey))
	pk := C.EVP_PKEY_new_raw_private_key(
		C.EVP_PKEY_ED25519,
		nil,
		(*C.uchar)(pkb),
		pkl,
	)
	if err := opensslGetErrors(); err != nil {
		return nil, err
	}
	impl := &opensslED25519Signer{pk, pkb}
	runtime.SetFinalizer(impl, func(k *opensslED25519Signer) { k.finalize() })
	return &ed25519Signer{
		sign: func(data []byte) ([]byte, error) {
			return impl.sign(data)
		},
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
