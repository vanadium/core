// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb

// #include "leveldb/c.h"
import "C"
import (
	"reflect"
	"unsafe"

	"v.io/v23/verror"
)

// goError copies C error into Go heap and frees C buffer.
func goError(cError *C.char) error {
	if cError == nil {
		return nil
	}
	err := verror.New(verror.ErrInternal, nil, C.GoString(cError))
	C.leveldb_free(unsafe.Pointer(cError))
	return err
}

// cSlice converts Go []byte to C string without copying the data.
// This function behaves similarly to standard Go slice copying or sub-slicing,
// in that the caller need not worry about ownership or garbage collection.
func cSlice(str []byte) (*C.char, C.size_t) {
	if len(str) == 0 {
		return nil, 0
	}
	data := unsafe.Pointer((*reflect.StringHeader)(unsafe.Pointer(&str)).Data)
	return (*C.char)(data), C.size_t(len(str))
}

// goBytes converts C string to Go []byte without copying the data.
// This function behaves similarly to cSlice.
func goBytes(str *C.char, size C.size_t) []byte {
	ptr := unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer(str)),
		Len:  int(size),
		Cap:  int(size),
	})
	return *(*[]byte)(ptr)
}
