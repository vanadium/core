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
	"runtime"
	"unsafe"

	"v.io/x/lib/vlog"
)

type opensslED25519PublicKey struct {
	k        *C.EVP_PKEY
	keyBytes *C.uchar
	der      []byte // the result of MarshalPKIXPublicKey on *ed25519.PublicKey.
}

func (k *opensslED25519PublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(unsafe.Pointer(k.keyBytes))
}

func (k *opensslED25519PublicKey) MarshalBinary() ([]byte, error) {
	cpy := make([]byte, len(k.der))
	copy(cpy, k.der)
	return cpy, nil
}

func (k *opensslED25519PublicKey) String() string { return publicKeyString(k) }

func (k *opensslED25519PublicKey) hash() Hash { return SHA512Hash }

func (k *opensslED25519PublicKey) verify(digest []byte, signature *Signature) bool {
	md_ctx := C.EVP_MD_CTX_new()
	defer C.EVP_MD_CTX_free(md_ctx)
	if C.EVP_DigestVerifyInit(md_ctx, nil, nil, nil, k.k) != 1 {
		if err := opensslGetErrors(); err != nil {
			vlog.Errorf("EVP_DigestVerifyInit: %v", err)
		}
		return false
	}
	sig := C.CBytes(signature.Ed25519)
	siglen := C.ulong(len(signature.Ed25519))
	dig := C.CBytes(digest)
	diglen := C.ulong(len(digest))
	if rc := C.EVP_DigestVerify(md_ctx, (*C.uchar)(sig), siglen, (*C.uchar)(dig), diglen); rc != 1 {
		if rc == 0 {
			return false
		}
		if err := opensslGetErrors(); err != nil {
			vlog.Errorf("EVP_DigestVerifyInit: %v", err)
		}
		vlog.Errorf("EVP_DigestVerifyInit: unrecognised error return: %v", rc)
		return false
	}
	return true
}

func newOpenSSLED25519PublicKey(golang ed25519.PublicKey) (PublicKey, error) {
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
	der, err := x509.MarshalPKIXPublicKey(golang)
	if err != nil {
		return nil, err
	}
	dercpy := make([]byte, len(der))
	copy(dercpy, der)
	ret := &opensslED25519PublicKey{k, pkb, dercpy}
	runtime.SetFinalizer(ret, func(k *opensslED25519PublicKey) { k.finalize() })
	return ret, nil
}

type opensslED25519Signer struct {
	k        *C.EVP_PKEY
	keyBytes unsafe.Pointer
}

func (k *opensslED25519Signer) finalize() {
	C.EVP_PKEY_free(k.k)
	C.free(k.keyBytes)
}

func (k *opensslED25519Signer) sign(data []byte) ([]byte, error) {
	md_ctx := C.EVP_MD_CTX_new()
	defer C.EVP_MD_CTX_free(md_ctx)
	if rc := C.EVP_DigestSignInit(md_ctx, nil, nil, nil, k.k); rc != 1 {
		if err := opensslGetErrors(); err != nil {
			return nil, fmt.Errorf("EVP_DigestSignInit: %v", err)
		}
		return nil, fmt.Errorf("EVP_DigestSignInit: unrecognised error return: %v", rc)
	}

	siglen := C.ulong(ed25519.SignatureSize)
	sig := C.CRYPTO_secure_zalloc(siglen, C.CString("opensslED25519Signer.sign.zalloc"), C.int(0))
	defer C.CRYPTO_secure_free(sig, C.CString("opensslED25519Signer.sign.free"), C.int(0))
	if rc := C.EVP_DigestSign(md_ctx, (*C.uchar)(sig), &siglen, uchar(data), C.ulong(len(data))); rc != 1 {
		if err := opensslGetErrors(); err != nil {
			return nil, fmt.Errorf("EVP_DigestSign: %v", err)
		}
		return nil, fmt.Errorf("EVP_DigestSign: unrecognised error return: %v", rc)
	}
	gosig := make([]byte, siglen)
	copy(gosig, C.GoBytes(sig, C.int(siglen)))
	return gosig, nil
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
