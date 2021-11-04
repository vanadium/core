// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Template functions that differ when bootstrapping. This
// file contains the functions used for normal operation.
//
//go:build !vdltoolbootstrapping
// +build !vdltoolbootstrapping

package golang

import (
	"strings"

	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

func nativeType(data *goData, native vdltool.GoType, wirePkg *compile.Package) string {
	result := native.Type
	for _, imp := range native.Imports {
		// Translate the packages specified in the native type into local package
		// identifiers.  E.g. if the native type is "foo.Type" with import
		// "path/to/foo", we need to replace "foo." in the native type with the
		// local package identifier for "path/to/foo".
		if strings.Contains(result, imp.Name+".") {
			// Add the import dependency if there is a match.
			pkg := data.Pkg(imp.Path)
			result = strings.ReplaceAll(result, imp.Name+".", pkg)
		}
	}
	data.AddForcedPkg(wirePkg.GenPath)
	return result
}

func hasNativeTypes(data *goData) bool {
	return len(data.Package.Config.Go.WireToNativeTypes) > 0
}

func noCustomNative(native vdltool.GoType) bool {
	return native.ToNative == "" && native.FromNative == ""
}

func typeHasNoCustomNative(data *goData, def *compile.TypeDef) bool {
	if native, ok := asNativeType(data.Env, def.Type); ok {
		return native.typ.ToNative == "" && native.typ.FromNative == ""
	}
	return true
}
