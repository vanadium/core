// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"reflect"
	"testing"
)

type (
	wireA   struct{}
	nativeA struct{}

	nativeError struct{} //nolint:deadcode,unused
)

func equalNativeInfo(a, b nativeInfo) bool {
	// We don't care about comparing the stack traces.
	a.stack = nil
	b.stack = nil
	return reflect.DeepEqual(a, b)
}

func TestRegisterNative(t *testing.T) {
	tests := []nativeInfo{
		{
			WireType:       reflect.TypeOf(wireA{}),
			NativeType:     reflect.TypeOf(nativeA{}),
			toNativeFunc:   reflect.ValueOf(func(wireA, *nativeA) error { return nil }),
			fromNativeFunc: reflect.ValueOf(func(*wireA, nativeA) error { return nil }),
			stack:          nil,
		},
		// TODO(toddw): Add tests where the wire type is a VDL union.

		// We can only register the error conversion once, and it's registered in
		// convert_test via the verror package, so we can't check registration.
	}
	for _, test := range tests {
		name := fmt.Sprintf("[%v %v]", test.WireType, test.NativeType)
		RegisterNative(test.toNativeFunc.Interface(), test.fromNativeFunc.Interface())
		if got, want := nativeInfoFromWire(test.WireType), test; !equalNativeInfo(*got, want) {
			t.Errorf("%s nativeInfoFromWire got %#v, want %#v", name, got, want)
		}
		if got, want := nativeInfoFromNative(test.NativeType), test; !equalNativeInfo(*got, want) {
			t.Errorf("%s nativeInfoFromNative got %#v, want %#v", name, got, want)
		}
	}
}

func TestDeriveNativeInfo(t *testing.T) {
	tests := []nativeInfo{
		{
			WireType:       reflect.TypeOf(wireA{}),
			NativeType:     reflect.TypeOf(nativeA{}),
			toNativeFunc:   reflect.ValueOf(func(wireA, *nativeA) error { return nil }),
			fromNativeFunc: reflect.ValueOf(func(*wireA, nativeA) error { return nil }),
			stack:          nil,
		},
		{
			WireType:       reflect.TypeOf(WireError{}),
			NativeType:     reflect.TypeOf((*error)(nil)).Elem(),
			toNativeFunc:   reflect.ValueOf(func(WireError, *error) error { return nil }),
			fromNativeFunc: reflect.ValueOf(func(*WireError, error) error { return nil }),
			stack:          nil,
		},
	}
	for _, test := range tests {
		name := fmt.Sprintf("[%v %v]", test.WireType, test.NativeType)
		ni, err := deriveNativeInfo(test.toNativeFunc.Interface(), test.fromNativeFunc.Interface())
		if err != nil {
			t.Fatalf("%s got error: %v", name, err)
		}
		if got, want := ni, test; !equalNativeInfo(*got, want) {
			t.Errorf("%s got %v, want %v", name, got, want)
		}
	}
}
