// Boostrapping mode is required to generate all vdl output files starting
// with no pre-existing vdl generated files. Currently the vdl tool
// chain depends on the generated output of this package and
// v.io/v23/vdlroot/vdltool.
//
// +build vdlbootstrapping

package vdl

import "reflect"

// Disable the cache when bootstrapping and use a dummy WireError, WireRetryCode.
var (
	rtCache        rtCacheT
	rtCacheEnabled = false
	rtWireError    = reflect.TypeOf(WireError{})
)

// WireError is the minimal, ie. empty, definiton used when bootstrapping.
type WireError struct{}

// WireRetryCode is the minimal, ie. empty, definiton used when bootstrapping.
type WireRetryCode int

// String implements stringer.
func (WireRetryCode) String() string {
	panic("not implemented when bootstrapping")
	return ""
}

// WireRetryCodeFromString mimics the vdl generated function for parsing
// WireRetryCodes.
func WireRetryCodeFromString(string) (WireRetryCode, error) {
	panic("not implemented when bootstrapping")
	return 0, nil
}
