// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
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
// EVP_PKEY* openssl_d2i_RSAPrivateEVPKey(const unsigned char *data, long len, unsigned long *e);
// EVP_PKEY* openssl_d2i_RSAPublicEVPKey(const unsigned char *data, long len, unsigned long *e);
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
	ret := &opensslRSAPublicKey{
		opensslPublicKeyCommon: newOpensslPublicKeyCommon(SHA512Hash, golang),
	}
	if err := ret.keyBytesErr; err != nil {
		return nil, err
	}
	var errno C.ulong
	ret.k = C.openssl_d2i_RSAPublicEVPKey(uchar(ret.keyBytes), C.long(len(ret.keyBytes)), &errno)
	if ret.k == nil {
		return nil, opensslMakeError(errno)
	}
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
	der, err := x509.MarshalPKCS8PrivateKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	key := C.openssl_d2i_RSAPrivateEVPKey(uchar(der), C.long(len(der)), &errno)
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
