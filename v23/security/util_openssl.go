// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl

package security

// #cgo CFLAGS: -DOPENSSL_API_COMPAT=30000 -DOPENSSL_NO_DEPRECATED
// #include <openssl/err.h>
// #include <openssl/evp.h>
// EVP_PKEY *openssl_evp_public_key(const unsigned char *data, long len, unsigned long *e);
import "C"

import (
	"crypto"
	"fmt"
	"unsafe"
)

func opensslMakeError(errno C.ulong) error {
	if errno == 0 {
		return nil
	}
	return fmt.Errorf("OpenSSL error %v in %v:%v",
		errno,
		C.GoString(C.ERR_lib_error_string(errno)),
		C.GoString(C.ERR_reason_error_string(errno)))
}

func uchar(b []byte) *C.uchar {
	if len(b) == 0 {
		return nil
	}
	return (*C.uchar)(unsafe.Pointer(&b[0]))
}

func openssl_version() string {
	return fmt.Sprintf("%v (CFLAGS:%v)", C.GoString(C.OpenSSL_version(C.OPENSSL_VERSION)), C.GoString(C.OpenSSL_version(C.OPENSSL_CFLAGS)))
}

func opensslGetErrors() error {
	var err error
	for {
		var file *C.char
		var line C.int
		errno := C.ERR_peek_error_line(&file, &line)
		C.ERR_get_error()
		if errno == 0 {
			break
		}
		nerr := fmt.Errorf("OpenSSL error %v in %v:%v, %v:%v",
			errno,
			C.GoString(C.ERR_lib_error_string(errno)),
			C.GoString(C.ERR_reason_error_string(errno)),
			C.GoString(file),
			line)
		if err == nil {
			err = nerr
			continue
		}
		err = fmt.Errorf("%v\n%v", err, nerr)
	}
	return err
}

func opensslHash(h Hash) *C.EVP_MD {
	switch h {
	case SHA1Hash:
		return C.EVP_sha1()
	case SHA256Hash:
		return C.EVP_sha256()
	case SHA384Hash:
		return C.EVP_sha384()
	case SHA512Hash:
		return C.EVP_sha512()
	}
	panic(fmt.Sprintf("unsupported hash function %v", h))
}

type opensslPublicKeyCommon struct {
	osslKey  *C.EVP_PKEY
	osslHash *C.EVP_MD
	key      interface{}
	publicKeyCommon
}

func (pk opensslPublicKeyCommon) cryptoKey() crypto.PublicKey {
	return pk.key
}

func newOpensslPublicKeyCommon(key interface{}, h Hash) (opensslPublicKeyCommon, error) {
	pc := newPublicKeyCommon(key, h)
	if pc.keyBytesErr != nil {
		return opensslPublicKeyCommon{}, pc.keyBytesErr
	}
	var errno C.ulong
	k := C.openssl_evp_public_key(
		uchar(pc.keyBytes),
		C.long(len(pc.keyBytes)),
		&errno)
	if k == nil {
		return opensslPublicKeyCommon{}, opensslMakeError(errno)
	}
	oh := opensslHash(h)
	return opensslPublicKeyCommon{
		publicKeyCommon: pc,
		key:             key,
		osslHash:        oh,
		osslKey:         k,
	}, nil
}
