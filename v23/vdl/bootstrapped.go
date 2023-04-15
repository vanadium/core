// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !vdlbootstrapping && !vdltoolbootstrapping

package vdl

import "reflect"

var (
	rtCache = &rtCacheT{
		rtmap: map[reflect.Type]*Type{
			// Ensure TypeOf(WireError{}) returns the built-in VDL error type.
			reflect.TypeOf(WireError{}): ErrorType.Elem(),
		},
	}
	rtCacheEnabled = true
	rtWireError    = reflect.TypeOf(WireError{})
)
