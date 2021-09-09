// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <openssl/crypto.h>
// #include <openssl/err.h>
// #include <openssl/evp.h>
//
// unsigned long openssl_EVP_sign_oneshot(EVP_PKEY* key, EVP_MD* dt, const unsigned char*  digest, size_t digestLen, unsigned char* sig, size_t siglen);
// unsigned long openssl_EVP_sign(EVP_PKEY* key, EVP_MD* dt, const unsigned char*  digest, size_t digestLen, unsigned char** sig, size_t *siglen);
// int openssl_EVP_verify(EVP_PKEY *key, EVP_MD *dt, const unsigned char *digest, size_t digestLen, unsigned char *sig, size_t siglen, unsigned long *e);
import "C"

import (
	"fmt"
	"unsafe"

	"v.io/x/lib/vlog"
)

func evpVerify(k *C.EVP_PKEY, algo string, dt *C.EVP_MD, digest []byte, signature []byte) bool {
	sig := uchar(signature)
	siglen := C.size_t(len(signature))
	dig := uchar(digest)
	diglen := C.size_t(len(digest))
	var errno C.ulong
	if rc := C.openssl_EVP_verify(k, dt, dig, diglen, sig, siglen, &errno); rc != 1 {
		if err := opensslMakeError(errno); err != nil {
			vlog.Errorf("EVP_DigestVerifyInit: %v: %v", algo, err)
		}
		return false
	}
	return true
}

func evpSignOneShot(k *C.EVP_PKEY, algo string, signatureSize int, digestType *C.EVP_MD, data []byte) ([]byte, error) {
	siglen := C.size_t(signatureSize)
	sig := C.CRYPTO_zalloc(siglen, C.CString("evp_openssl.go"), C.int(42))
	defer C.CRYPTO_free(sig, C.CString("evp_openssl.go"), C.int(43))
	if errno := C.openssl_EVP_sign_oneshot(k, digestType,
		uchar(data), C.size_t(len(data)),
		(*C.uchar)(sig), C.size_t(siglen)); errno != 0 {
		return nil, opensslMakeError(errno)
	}
	gosig := make([]byte, signatureSize)
	copy(gosig, C.GoBytes(sig, C.int(siglen)))
	return gosig, nil
}

func evpSign(k *C.EVP_PKEY, algo string, digestType *C.EVP_MD, data []byte) ([]byte, error) {
	var siglen C.size_t
	var sig *C.uchar
	if errno := C.openssl_EVP_sign(k, digestType,
		uchar(data), C.size_t(len(data)),
		&sig, &siglen); errno != 0 {
		return nil, opensslMakeError(errno)
	}
	gosig := make([]byte, siglen)
	copy(gosig, C.GoBytes(unsafe.Pointer(sig), C.int(siglen)))
	C.CRYPTO_free(unsafe.Pointer(sig), C.CString("evp_openssl.go"), C.int(64))
	return gosig, nil
}

func newEVPKey(algo string, setter func(evpKey *C.EVP_PKEY) C.int) (*C.EVP_PKEY, error) {
	evpKey := C.EVP_PKEY_new()
	if evpKey == nil {
		return nil, fmt.Errorf("EVP_PKEY_new: %s: %v", algo, opensslGetErrors())
	}
	if rc := setter(evpKey); rc != 1 {
		if err := opensslGetErrors(); err != nil {
			vlog.Errorf("EVP_PKEY_set1: %s: %v", algo, err)
			return nil, err
		}
		vlog.Errorf("EVP_PKEY_set1_RSA:unrecognised error code: %s: %v", algo, rc)
		return nil, fmt.Errorf("EVP_PKEY_set1_RSA:unrecognised error code: %s: %v", algo, rc)
	}
	return evpKey, nil
}
