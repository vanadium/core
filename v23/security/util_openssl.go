// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

// #cgo pkg-config: libcrypto
// #include <openssl/bn.h>
// #include <openssl/ec.h>
// #include <openssl/err.h>
// #include <openssl/x509.h>
//
// EC_KEY* openssl_d2i_EC_PUBKEY(const unsigned char* data, long len, unsigned long* e);
import "C"

import (
	"fmt"
	"unsafe"
)

func opensslMakeError(errno C.ulong) error {
	return fmt.Errorf("OpenSSL error (%v): %v in %v:%v",
		errno,
		C.GoString(C.ERR_func_error_string(errno)),
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
		errno := C.ERR_get_error_line(&file, &line)
		if errno == 0 {
			break
		}
		nerr := fmt.Errorf("OpenSSL error (%v): %v in %v:%v, %v:%v",
			errno,
			C.GoString(C.ERR_func_error_string(errno)),
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
