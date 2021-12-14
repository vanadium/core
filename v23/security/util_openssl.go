// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <openssl/err.h>
// #include <openssl/evp.h>
import "C"

import (
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
	return fmt.Sprintf("%v (CFLAGS:%v)", C.GoString(C.SSLeay_version(C.SSLEAY_VERSION)), C.GoString(C.SSLeay_version(C.SSLEAY_CFLAGS)))
}

func opensslGetErrors() error {
	var err error
	for {
		var file *C.char
		var line C.int
		errno := C.ERR_get_error_all(&file, &line, nil, nil, nil)
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
	k  *C.EVP_PKEY
	oh *C.EVP_MD
	publicKeyCommon
}

func newOpensslPublicKeyCommon(h Hash, key interface{}) opensslPublicKeyCommon {
	pc := newPublicKeyCommon(h, key)
	if pc.keyBytesErr != nil {
		return opensslPublicKeyCommon{}
	}
	oh := opensslHash(h)
	return opensslPublicKeyCommon{
		oh:              oh,
		publicKeyCommon: pc,
	}
}
