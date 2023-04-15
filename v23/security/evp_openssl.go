// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl

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
	"unsafe"
)

func evpVerify(k *C.EVP_PKEY, dt *C.EVP_MD, digest []byte, signature []byte) (bool, error) {
	sig := uchar(signature)
	siglen := C.size_t(len(signature))
	dig := uchar(digest)
	diglen := C.size_t(len(digest))
	var errno C.ulong
	if rc := C.openssl_EVP_verify(k, dt, dig, diglen, sig, siglen, &errno); rc != 1 {
		return false, opensslMakeError(errno)
	}
	return true, nil
}

func evpSignOneShot(k *C.EVP_PKEY, signatureSize int, digestType *C.EVP_MD, data []byte) ([]byte, error) {
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

func evpSign(k *C.EVP_PKEY, digestType *C.EVP_MD, data []byte) ([]byte, error) {
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
