// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build vdltoolbootstrapping

package golang

import (
	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

// validateGoConfig has nothing to do when bootstrapping.
func validateGoConfig(pkg *compile.Package, env *compile.Env) {}

// isNativeType is always false when boostrapping.
func isNativeType(env *compile.Env, t *vdl.Type) bool {
	return false
}

// nativeGoType is an empty place holder when boostrapping.
type nativeGoType struct{}

// asNativeType retruns the supplied type as a native type which
// is always false when boostrapping.
func asNativeType(env *compile.Env, tt *vdl.Type) (nativeGoType, bool) {
	return nativeGoType{}, false
}

func (nt nativeGoType) String() string {
	return ""
}

func (nt nativeGoType) pkg() *compile.Package {
	return nil
}

func (nt nativeGoType) isBool() bool {
	return false
}

func (nt nativeGoType) nativeType(data *goData) string {
	return ""
}

func (nt nativeGoType) constNative(data *goData, v *vdl.Value, typed bool) string {
	return ""
}

func (nt nativeGoType) fieldConst(data *goData, v *vdl.Value) (string, bool) {
	return "", false
}

func (nt nativeGoType) typedConstNativeZero(nType string) string {
	return ""
}

func (nt nativeGoType) untypedConstNativeZero(nType string) string {
	return ""
}

func (nt nativeGoType) isZeroModeUnique() bool {
	return false
}

func (nt nativeGoType) isZeroModeKnown() bool {
	return false
}

func (nt nativeGoType) isDotZero() bool {
	return false
}

func (nt nativeGoType) isZeroValue() string {
	return ""
}

func (nt nativeGoType) zeroUniqueModeValue(data *goData, genIfStatement bool) string {
	return ""
}

// Template function, always returns true.
func typeHasNoCustomNative(data *goData, def *compile.TypeDef) bool {
	return true
}

// Template function, always returns false. All of the other template
// functions below never get called if hasNativeTypes returns
// false.
func hasNativeTypes(data *goData) bool {
	return false
}

// Template function, never gets called.
func nativeType() interface{} {
	panic("not implemented")
}

// Template function, never gets called.
func noCustomNative() interface{} {
	panic("not implemented")
}

// No struct tags will ever be found when bootstrapping.
func structTagFor(data *goData, structName, fieldName string) (string, bool) {
	return "", false
}
