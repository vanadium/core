// +build !vdlbootstrapping

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
