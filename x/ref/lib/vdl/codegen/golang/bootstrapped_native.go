// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Native types are not supported in bootstrapping mode.  This
// file contains the functions used for normal operation.
//
//go:build !vdltoolbootstrapping
// +build !vdltoolbootstrapping

package golang

import (
	"path"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

// The native types feature is hard to use correctly.  E.g. the package
// containing the wire type must be imported into your Go binary in order for
// the wire<->native registration to work, which is hard to ensure.  E.g.
//
//   package base    // VDL package
//   type Wire int   // has native type native.Int
//
//   package dep     // VDL package
//   import "base"
//   type Foo struct {
//     X base.Wire
//   }
//
// The Go code for package "dep" imports "native", rather than "base":
//
//   package dep     // Go package generated from VDL package
//   import "native"
//   type Foo struct {
//     X native.Int
//   }
//
// Note that when you import the "dep" package in your own code, you always use
// native.Int, rather than base.Wire; the base.Wire representation is only used
// as the wire format, but doesn't appear in generated code.  But in order for
// this to work correctly, the "base" package must imported.  This is tricky.
//
// Restrict the feature to these whitelisted VDL packages for now.
var nativeTypePackageWhitelist = map[string]bool{
	"math":                                   true,
	"time":                                   true,
	"v.io/x/ref/lib/vdl/testdata/nativetest": true,
	"v.io/v23/security":                      true,
	"v.io/v23/vdl":                           true,
	"v.io/v23/vdl/vdltest":                   true,
}

// validateGoConfig ensures a valid vdl.Config.
func validateGoConfig(pkg *compile.Package, env *compile.Env) {
	vdlconfig := path.Join(pkg.GenPath, pkg.ConfigName)
	// Validate native type configuration.  Since native types are hard to use, we
	// restrict them to a built-in whitelist of packages for now.
	if len(pkg.Config.Go.WireToNativeTypes) > 0 && !nativeTypePackageWhitelist[pkg.Path] {
		env.Errors.Errorf("%s: Go.WireToNativeTypes is restricted to whitelisted VDL packages", vdlconfig)
	}
	// Make sure each wire type is actually defined in the package, and required
	// fields are all filled in.
	for wire, native := range pkg.Config.Go.WireToNativeTypes {
		baseWire := wire
		if strings.HasPrefix(wire, "*") {
			baseWire = strings.TrimPrefix(wire, "*")
		}
		if def := pkg.ResolveType(baseWire); def == nil {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes undefined", vdlconfig, wire)
		}
		if native.Type == "" {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (empty GoType.Type)", vdlconfig, wire)
		}
		for _, imp := range native.Imports {
			if imp.Path == "" || imp.Name == "" {
				env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (empty GoImport.Path or Name)", vdlconfig, wire)
				continue
			}
			importPrefix := imp.Name + "."
			if !strings.Contains(native.Type, importPrefix) &&
				!strings.Contains(native.ToNative, importPrefix) &&
				!strings.Contains(native.FromNative, importPrefix) {
				env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (native type %q doesn't contain import prefix %q)", vdlconfig, wire, native.Type, importPrefix)
			}
		}
		if native.Zero.Mode != vdltool.GoZeroModeUnique && native.Zero.IsZero == "" {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (native type %q must be either Mode:Unique or have a non-empty IsZero)", vdlconfig, wire, native.Type)
		}
	}
}

// isNativeType returns true iff t is a native type.
func isNativeType(env *compile.Env, t *vdl.Type) bool {
	if def := env.FindTypeDef(t); def != nil {
		key := def.Name
		if t.Kind() == vdl.Optional {
			key = "*" + key
		}
		_, ok := def.File.Package.Config.Go.WireToNativeTypes[key]
		return ok
	}
	return false
}

// asNativeType retruns the supplied type as a native type if
// it is configured as such.
func asNativeType(env *compile.Env, tt *vdl.Type) (nativeGoType, bool) {
	if def := env.FindTypeDef(tt); def != nil {
		pkg := def.File.Package
		key := def.Name
		if tt.Kind() == vdl.Optional {
			key = "*" + key
		}
		native, ok := pkg.Config.Go.WireToNativeTypes[key]
		return nativeGoType{native, pkg}, ok
	}
	return nativeGoType{}, false
}

// nativeGoType abstracts native types so that they can be implemented
// differently when bootstrapping.
type nativeGoType struct {
	typ     vdltool.GoType
	wirePkg *compile.Package
}

func (nt nativeGoType) String() string {
	return nt.typ.Type
}

func (nt nativeGoType) pkg() *compile.Package {
	return nt.wirePkg
}

func (nt nativeGoType) isBool() bool {
	return nt.typ.Kind == vdltool.GoKindBool
}

// nativeType returns the type name for the native type.
func (nt nativeGoType) nativeType(data *goData) string {
	native := nt.typ
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
	data.AddForcedPkg(nt.wirePkg.GenPath)
	return result
}

// constNative returns a const value for the native type.
func (nt nativeGoType) constNative(data *goData, v *vdl.Value, typed bool) string {
	native := nt.typ
	nType := nt.nativeType(data)
	if native.Zero.Mode != vdltool.GoZeroModeUnknown && v.IsZero() {
		// This is the case where the value is zero, and the zero mode is either
		// Canonical or Unique, which means that the Go zero value of the native
		// type is sufficient to represent the value.
		if typed {
			return nt.typedConstNativeZero(nType)
		}
		return nt.untypedConstNativeZero(nType)
	}
	return constNativeConversion(data, v, nType, toNative(data, native, v.Type()))
}

func toNative(data *goData, native vdltool.GoType, ttWire *vdl.Type) string {
	if native.ToNative != "" {
		result := native.ToNative
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
		return result
	}
	return typeGoWire(data, ttWire) + "ToNative"
}

// fieldConst will return the typed field const of the supplied
// value if it has a native type configured, or "" otherwise.
func (nt nativeGoType) fieldConst(data *goData, v *vdl.Value) (string, bool) {
	switch nt.typ.Kind {
	case vdltool.GoKindArray, vdltool.GoKindSlice, vdltool.GoKindMap, vdltool.GoKindStruct:
		return typedConst(data, v), true
	}
	return "", false
}

func (nt nativeGoType) isZeroModeUnique() bool {
	return nt.typ.Zero.Mode == vdltool.GoZeroModeUnique
}

func (nt nativeGoType) isZeroModeKnown() bool {
	return nt.typ.Zero.Mode != vdltool.GoZeroModeUnknown
}

func (nt nativeGoType) isDotZero() bool {
	return strings.HasPrefix(nt.typ.Zero.IsZero, ".")
}

func (nt nativeGoType) isZeroValue() string {
	return nt.typ.Zero.IsZero
}

func (nt nativeGoType) zeroUniqueModeValue(data *goData, genIfStatement bool) string {
	// We use an untyped const as the zero value, because Go only allows
	// comparison of slices with nil.  E.g.
	//   type MySlice []string
	//   pass := MySlice(nil) == nil          // valid
	//   fail := MySlice(nil) == MySlice(nil) // invalid
	nType := nt.nativeType(data)
	zeroValue := nt.untypedConstNativeZero(nType)
	if genIfStatement {
		if k := nt.typ.Kind; k == vdltool.GoKindStruct || k == vdltool.GoKindArray {
			// Without a special-case, we'll get a statement like:
			//   if x == Foo{} {
			// But that isn't valid Go code, so we change it to:
			//   if x == (Foo{}) {
			zeroValue = "(" + zeroValue + ")"
		}
	}
	return zeroValue
}

func (nt nativeGoType) typedConstNativeZero(nType string) string {
	zero := nt.untypedConstNativeZero(nType)
	switch nt.typ.Kind {
	case vdltool.GoKindStruct, vdltool.GoKindArray:
		return zero // untyped const is already typed, e.g. NativeType{}
	default:
		return nType + "(" + zero + ")" // e.g. NativeType(0)
	}
}

func (nt nativeGoType) untypedConstNativeZero(nType string) string {
	switch nt.typ.Kind {
	case vdltool.GoKindStruct, vdltool.GoKindArray:
		return nType + "{}" // No way to create an untyped zero struct or array.
	case vdltool.GoKindBool:
		return "false"
	case vdltool.GoKindNumber:
		return "0"
	case vdltool.GoKindString:
		return `""`
	default:
		return "nil"
	}
}
