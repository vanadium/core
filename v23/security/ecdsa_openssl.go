// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

// OpenSSL's libcrypto may have faster implementations of ECDSA signing and
// verification on some architectures (not amd64 after Go 1.6 which includes
// https://go-review.googlesource.com/#/c/8968/). This file enables use
// of libcrypto's implementation of ECDSA operations in those situations.

package security

// #cgo pkg-config: libcrypto
// #include <openssl/bn.h>
// #include <openssl/ec.h>
// #include <openssl/ecdsa.h>
// #include <openssl/err.h>
// #include <openssl/objects.h>
// #include <openssl/opensslv.h>
// #include <openssl/x509.h>
//
// EC_KEY* openssl_d2i_EC_PUBKEY(const unsigned char* data, long len, unsigned long* e);
// EC_KEY* openssl_d2i_ECPrivateKey(const unsigned char* data, long len, unsigned long* e);
// ECDSA_SIG* openssl_ECDSA_do_sign(const unsigned char* data, int len, EC_KEY* key, unsigned long *e);
import "C"

import (
	"crypto/ecdsa"
	"crypto/x509"
	"math/big"
	"runtime"

	"v.io/x/lib/vlog"
)

type opensslECDSAPublicKey struct {
	//k *C.EC_KEY
	k *C.EVP_PKEY
	publicKeyCommon
}

func (k *opensslECDSAPublicKey) finalize() {
	//C.EC_KEY_free(k.k)
	C.EC_PKEY_free(k.k)
}

func (k *opensslECDSAPublicKey) messageDigest(purpose, message []byte) []byte {
	return k.h.sum(messageDigestFields(k.h, k.keyBytes, purpose, message))
}

func (k *opensslECDSAPublicKey) verify(digest []byte, signature *Signature) bool {
	sig := C.ECDSA_SIG_new()
	if C.ECDSA_SIG_set0(sig,
		C.BN_bin2bn(uchar(signature.R), C.int(len(signature.R)), nil),
		C.BN_bin2bn(uchar(signature.S), C.int(len(signature.S)), nil)) == 0 {
		return false
	}
	status := C.ECDSA_do_verify(uchar(digest), C.int(len(digest)), sig, k.k)
	C.ECDSA_SIG_free(sig)
	if status != 1 {
		// Make sure to read out all errors associated with this thread
		// since verification failures are recorded as errors.
		err := opensslGetErrors()
		if status != 0 {
			vlog.Errorf("ECDSA_do_verify: %v", err)
		}
		return false
	}
	return true
}

func newOpenSSLECDSAPublicKey(golang *ecdsa.PublicKey) (PublicKey, error) {
	ret := &opensslECDSAPublicKey{
		publicKeyCommon: newPublicKeyCommon(ecdsaHash(golang), golang),
	}
	if err := ret.keyBytesErr; err != nil {
		return nil, err
	}
	var errno C.ulong
	k := C.openssl_d2i_EC_PUBKEY(uchar(ret.keyBytes), C.long(len(ret.keyBytes)), &errno)
	if k == nil {
		return nil, opensslMakeError(errno)
	}
	runtime.SetFinalizer(ret, func(k *opensslECDSAPublicKey) { k.finalize() })
	ret.k = k
	return ret, nil
}

type opensslECDSASigner struct {
	k *C.EC_KEY
}

func (k *opensslECDSASigner) finalize() {
	C.EC_KEY_free(k.k)
}

func (k *opensslECDSASigner) sign(data []byte) (r, s *big.Int, err error) {
	var errno C.ulong
	sig := C.openssl_ECDSA_do_sign(uchar(data), C.int(len(data)), k.k, &errno)
	if sig == nil {
		return nil, nil, opensslMakeError(errno)
	}
	defer C.ECDSA_SIG_free(sig)
	pr := C.ECDSA_SIG_get0_r(sig)
	ps := C.ECDSA_SIG_get0_s(sig)
	var (
		rlen = (int(C.BN_num_bits(pr)) + 7) / 8
		slen = (int(C.BN_num_bits(ps)) + 7) / 8
		buf  []byte
	)
	if rlen > slen {
		buf = make([]byte, rlen)
	} else {
		buf = make([]byte, slen)
	}
	r = big.NewInt(0).SetBytes(buf[0:int(C.BN_bn2bin(pr, uchar(buf)))])
	s = big.NewInt(0).SetBytes(buf[0:int(C.BN_bn2bin(ps, uchar(buf)))])
	return r, s, nil
}

func newOpenSSLECDSASigner(golang *ecdsa.PrivateKey) (Signer, error) {
	pubkey, err := newOpenSSLECDSAPublicKey(&golang.PublicKey)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(golang)
	if err != nil {
		return nil, err
	}
	var errno C.ulong
	k := C.openssl_d2i_ECPrivateKey(uchar(der), C.long(len(der)), &errno)
	if k == nil {
		return nil, opensslMakeError(errno)
	}

	impl := &opensslECDSASigner{k}
	runtime.SetFinalizer(impl, func(k *opensslECDSASigner) { k.finalize() })
	return &ecdsaSigner{
		sign: impl.sign,
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
