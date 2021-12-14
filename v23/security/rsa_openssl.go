// Copyright 2021 The Vanadium Authors. All rights reserved.
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
	"crypto/rsa"
	"crypto/x509"
	"runtime"
)

type opensslRSAPublicKey struct {
	opensslPublicKeyCommon
}

func (k *opensslRSAPublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslRSAPublicKey) messageDigest(purpose, message []byte) []byte {
	// NOTE: the openssl rsa signer/verifier KCS1v15 implementation always
	// 	     hashes the message it receives, whereas the go implementation
	//       assumes a prehashed version. Consequently this method returns
	//       the results of messageDigestFields and leaves it to the
	//       implementation of the signer to hash that value or not.
	//       For this go implementation, the results returned by this
	//       function are therefore hashed again below (see the sign method
	//       implementation provided when the signer is created).
	return messageDigestFields(k.h, k.keyBytes, purpose, message)
}

func (k *opensslRSAPublicKey) verify(digest []byte, signature *Signature) bool {
	ok, _ := evpVerify(k.k, C.EVP_sha512(), digest, signature.Rsa)
	return ok
}

func newOpenSSLRSAPublicKey(golang *rsa.PublicKey) (PublicKey, error) {
	pc, err := newOpensslPublicKeyCommon(SHA512Hash, golang)
	if err != nil {
		return nil, err
	}
	ret := &opensslRSAPublicKey{pc}
	runtime.SetFinalizer(ret, func(k *opensslRSAPublicKey) { k.finalize() })
	return ret, nil
}

type opensslRSASigner struct {
	k *C.EVP_PKEY
	h *C.EVP_MD // no need to free this
}

func (k *opensslRSASigner) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslRSASigner) sign(data []byte) ([]byte, error) {
	return evpSign(k.k, k.h, data)
}

func newOpenSSLRSASigner(golang *rsa.PrivateKey) (Signer, error) {
	pubkey, err := newOpenSSLRSAPublicKey(&golang.PublicKey)
	if err != nil {
		return nil, err
	}
	der := x509.MarshalPKCS1PrivateKey(golang)
	var errno C.ulong
	key := C.openssl_evp_private_key(C.EVP_PKEY_RSA, uchar(der), C.long(len(der)), &errno)
	if key == nil {
		return nil, opensslMakeError(errno)
	}
	impl := &opensslRSASigner{key, C.EVP_sha512()}
	runtime.SetFinalizer(impl, func(k *opensslRSASigner) { k.finalize() })
	return &rsaSigner{
		sign:   impl.sign,
		pubkey: pubkey,
		impl:   impl,
	}, nil
}

func newInMemoryRSASignerImpl(key *rsa.PrivateKey) (Signer, error) {
	return newOpenSSLRSASigner(key)
}

func newRSAPublicKeyImpl(key *rsa.PublicKey) PublicKey {
	if pk, err := newOpenSSLRSAPublicKey(key); err == nil {
		return pk
	}
	return newGoStdlibRSAPublicKey(key)
}
