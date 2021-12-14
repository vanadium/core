// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

// OpenSSL's libcrypto may have faster implementations of ECDSA signing and
// verification on some architectures (not amd64 after Go 1.6 which includes
// https://go-review.googlesource.com/#/c/8968/). This file enables use
// of libcrypto's implementation of ECDSA operations in those situations.

package security

// #cgo CFLAGS: -DOPENSSL_API_COMPAT=30000 -DOPENSSL_NO_DEPRECATED
// #cgo pkg-config: libcrypto
// #include <openssl/evp.h>
//
// EVP_PKEY *openssl_evp_private_key(int keyType, const unsigned char* data, long len, unsigned long* e);
import "C"

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"math/big"
	"runtime"
)

type opensslECDSAPublicKey struct {
	opensslPublicKeyCommon
}

func (k *opensslECDSAPublicKey) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslECDSAPublicKey) messageDigest(purpose, message []byte) []byte {
	// NOTE that the openssl EVP API will hash the result of this digest.
	return messageDigestFields(k.h, k.keyBytes, purpose, message)
}

func (k *opensslECDSAPublicKey) verify(digest []byte, signature *Signature) bool {
	tmp := struct {
		R, S *big.Int
	}{
		R: new(big.Int).SetBytes(signature.R),
		S: new(big.Int).SetBytes(signature.S),
	}
	sig, err := asn1.Marshal(tmp)
	if err != nil {
		return false
	}
	ok, _ := evpVerify(k.k, k.oh, digest, sig)
	return ok

}

func newOpenSSLECDSAPublicKey(golang *ecdsa.PublicKey) (PublicKey, error) {
	pc, err := newOpensslPublicKeyCommon(ecdsaHash(golang), golang)
	if err != nil {
		return nil, err
	}
	ret := &opensslECDSAPublicKey{pc}
	runtime.SetFinalizer(ret, func(k *opensslECDSAPublicKey) { k.finalize() })
	return ret, nil
}

type opensslECDSASigner struct {
	k *C.EVP_PKEY
	h *C.EVP_MD // no need to free this.
}

func (k *opensslECDSASigner) finalize() {
	C.EVP_PKEY_free(k.k)
}

func (k *opensslECDSASigner) sign(data []byte) (r, s *big.Int, err error) {
	sig, err := evpSign(k.k, k.h, data)
	if err != nil {
		return nil, nil, err
	}
	tmp := struct {
		R, S *big.Int
	}{}
	rest, err := asn1.Unmarshal(sig, &tmp)
	if err != nil {
		return nil, nil, err
	}
	if r := len(rest); r > 0 {
		return nil, nil, fmt.Errorf("signature has %v spurious bytes", r)
	}
	return tmp.R, tmp.S, nil
}

func newOpenSSLECDSASigner(golang *ecdsa.PrivateKey) (Signer, error) {
	pubkey, err := newOpenSSLECDSAPublicKey(&golang.PublicKey)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	k := C.openssl_evp_private_key(C.EVP_PKEY_EC, uchar(der), C.long(len(der)), &errno)
	if k == nil {
		return nil, opensslMakeError(errno)
	}
	impl := &opensslECDSASigner{k, opensslHash(pubkey.hash())}
	runtime.SetFinalizer(impl, func(k *opensslECDSASigner) { k.finalize() })
	return &ecdsaSigner{
		sign:   impl.sign,
		pubkey: pubkey,
		impl:   impl,
	}, nil
}

func newInMemoryECDSASignerImpl(key *ecdsa.PrivateKey) (Signer, error) {
	return newOpenSSLECDSASigner(key)
}

func newECDSAPublicKeyImpl(key *ecdsa.PublicKey) PublicKey {
	if key, err := newOpenSSLECDSAPublicKey(key); err == nil {
		return key
	}
	return newGoStdlibECDSAPublicKey(key)
}
