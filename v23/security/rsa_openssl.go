// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <openssl/rsa.h>
// #include <openssl/evp.h>
// #include <openssl/err.h>
// #include <openssl/objects.h>
// #include <openssl/opensslv.h>
// #include <openssl/x509.h>
//
// void *openssl_d2i_RSAPrivateKey(const unsigned char *data, long len, unsigned long *e);
// RSA *openssl_d2i_RSA_PUBKEY(const unsigned char *data, long len, unsigned long *e);
import "C"

import (
	"crypto/rsa"
	"crypto/x509"
	"runtime"
)

type opensslRSAPublicKey struct {
	k *C.EVP_PKEY
	publicKeyCommon
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
	//       For this openssl implementation, the results returned by this
	//       function are therefore not hashed again below (see the sign method
	//       implementation provided when the signer is created).
	return messageDigestFields(k.h, k.keyBytes, purpose, message)
}

func (k *opensslRSAPublicKey) verify(digest []byte, signature *Signature) bool {
	return evpVerify(k.k, "RSA", opensslHash(k.h), digest, signature.Rsa)
}

func opensslHash(h Hash) *C.EVP_MD {
	if h == SHA256Hash {
		return C.EVP_sha256()
	}
	return C.EVP_sha512()
}

func newOpenSSLRSAPublicKey(golang *rsa.PublicKey) (PublicKey, error) {
	ret := &opensslRSAPublicKey{
		publicKeyCommon: newPublicKeyCommon(rsaHash(golang), golang),
	}
	if err := ret.keyBytesErr; err != nil {
		return nil, err
	}
	var errno C.ulong
	rsaKey := C.openssl_d2i_RSA_PUBKEY(uchar(ret.keyBytes), C.long(len(ret.keyBytes)), &errno)
	if rsaKey == nil {
		return nil, opensslMakeError(errno)
	}
	evpKey, err := newEVPKey("RSA", func(ek *C.EVP_PKEY) C.int {
		return C.EVP_PKEY_set1_RSA(ek, rsaKey)
	})
	if err != nil {
		return nil, err
	}
	ret.k = evpKey
	runtime.SetFinalizer(ret, func(k *opensslRSAPublicKey) { k.finalize() })
	return ret, nil
}

type opensslRSASigner struct {
	k             *C.EVP_PKEY
	h             Hash
	signatureSize int
}

func (k *opensslRSASigner) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslRSASigner) sign(data []byte) ([]byte, error) {
	return evpSign(k.k, "RSA", opensslHash(k.h), data)
}

func newOpenSSLRSASigner(golang *rsa.PrivateKey) (Signer, error) {
	pubkey, err := newOpenSSLRSAPublicKey(&golang.PublicKey)
	if err != nil {
		return nil, err
	}
	der := x509.MarshalPKCS1PrivateKey(golang)
	var errno C.ulong
	rsaKey := C.openssl_d2i_RSAPrivateKey(uchar(der), C.long(len(der)), &errno)
	if rsaKey == nil {
		return nil, opensslMakeError(errno)
	}
	evpKey, err := newEVPKey("RSA", func(ek *C.EVP_PKEY) C.int {
		return C.EVP_PKEY_assign(ek, C.EVP_PKEY_RSA, rsaKey)
	})
	if err != nil {
		return nil, err
	}
	impl := &opensslRSASigner{evpKey, pubkey.hash(), golang.PublicKey.Size()}
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
