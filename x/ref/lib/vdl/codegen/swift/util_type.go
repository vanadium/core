// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"fmt"
	"log"
	"unicode"
	"unicode/utf8"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// swiftBuiltInType returns the type name for the provided built in type
// definition, forcing the use of a swift class (e.g., swift.lang.Integer) if so
// desired.  This method also returns a boolean value indicating whether the
// returned type is a class.
//
// All swift integers (byte, short, int, long) are signed.  We
// translate signed vdl integers int{16,32,64} into their
// swift equivalents.  We translate unsigned vdl integers
// uint{16,32,64} into our class-based representation
// VdlUint{16,32,64}.
//
// According to this rule, we should translate signed
// vdl int8 into swift byte, and unsigned vdl byte into
// swift VdlUint8.  However we flip the rule, and
// actually translate vdl int8 into swift VdlInt8, and
// vdl byte into swift byte.  We do this because we want the
// common usage of vdl []byte to translate into swift byte[].
func (ctx *swiftContext) swiftBuiltInType(typ *vdl.Type) string { //nolint:gocyclo
	if typ == nil {
		return "Void"
	}
	switch typ.Kind() {
	case vdl.Bool:
		return "Bool"
	case vdl.Byte:
		return "UInt8"
	case vdl.Int8:
		return "Int8"
	case vdl.Uint16:
		return "UInt16"
	case vdl.Int16:
		return "Int16"
	case vdl.Uint32:
		return "UInt32"
	case vdl.Int32:
		return "Int32"
	case vdl.Uint64:
		return "UInt64"
	case vdl.Int64:
		return "Int64"
	case vdl.Float32:
		return "Float"
	case vdl.Float64:
		return "Double"
	case vdl.String:
		return "String"
	case vdl.TypeObject:
		return "VdlTypeObject"
	case vdl.Any:
		return "Any"
	case vdl.List, vdl.Array:
		return fmt.Sprintf("[%v]", ctx.swiftType(typ.Elem()))
	case vdl.Map:
		return fmt.Sprintf("[%v : %v]", ctx.swiftType(typ.Key()), ctx.swiftType(typ.Elem()))
	case vdl.Set:
		return fmt.Sprintf("Set<%v>", ctx.swiftType(typ.Key()))
	default:
		panic("Unsupported built in type")
	}
}

func (ctx *swiftContext) swiftTypeName(tdef *compile.TypeDef) string {
	if name, ok := ctx.memoizedTypeNames[tdef]; ok {
		return name
	}
	var name string
	if tdef.File == compile.BuiltInFile {
		name = ctx.swiftBuiltInType(tdef.Type)
	} else {
		// A typeName is created by walking concatenating a package path with the type name
		// into a camel cased symbol. For example Value in v.io/v23/syncbase/nosql would be
		// SyncbaseNosqlValue (because v23 defines a swift module we stop just before that).
		// Convieniently the Swift package name already does the package path part, so we
		// can just append to that. In cases where we're at the root of our Swift module,
		// the package name will be a zero value and so we correctly end up with just our
		// tdef.Name.
		name = ctx.swiftPackageName(tdef.File.Package) + vdlutil.FirstRuneToUpper(tdef.Name)
		if !ctx.pkgIsSameModule(tdef.File.Package) {
			name = ctx.swiftModule(tdef.File.Package) + "." + name
		}
	}
	ctx.memoizedTypeNames[tdef] = name
	return name
}

// swiftAccessModifier returns the Swift access modifier given the type.
func swiftAccessModifier(tdef *compile.TypeDef) string {
	if tdef.Exported {
		return "public"
	}
	return "internal"
}

// accessModifierForName returns the Swift access modifier given the name.
// It follows VDL naming conventions, indicating that an uppercase name
// denotes a public type and a lowercase name a package-protected type.
func swiftAccessModifierForName(name string) string {
	r, _ := utf8.DecodeRuneInString(name)
	if unicode.IsUpper(r) {
		return "public"
	}
	return "internal"
}

func swiftNativeType(t *vdl.Type, env *compile.Env) (string, bool) {
	if t == vdl.ErrorType {
		// TODO(zinman) Verify this can never be a user-defined error,
		// and determine if we need to prepend the v23 namespace here
		// or if that adjustment will be higher up the stack.
		switch t.Kind() {
		case vdl.Optional:
			return "VError?", true
		case vdl.Struct:
			return "VError", true
		default:
			panic(fmt.Sprintf("Unexpected vdl.Type.Kind() for ErrorType: %v", t.Kind()))
		}
	}
	if tdef := env.FindTypeDef(t); tdef != nil {
		pkg := tdef.File.Package
		if native, ok := pkg.Config.Swift.WireToNativeTypes[tdef.Name]; ok {
			// There is a Swift native type configured for this defined type.
			return native, true
		}
	}
	return "", false
}

// swiftCaseName returns the Swift name of a field translated as a Enum case.
func swiftVariableName(tdef *compile.TypeDef, fld vdl.Field) string {
	// Check if reserved
	return vdlutil.FirstRuneToLower(fld.Name)
}

// swiftCaseName returns the Swift name of a field.
func swiftCaseName(tdef *compile.TypeDef, fld vdl.Field) string {
	// Check if reserved
	return vdlutil.FirstRuneToUpper(fld.Name)
}

func (ctx *swiftContext) swiftType(t *vdl.Type) string {
	if t == nil {
		return ctx.swiftBuiltInType(nil)
	}

	if native, ok := swiftNativeType(t, ctx.env); ok {
		return native
	}
	if tdef := ctx.env.FindTypeDef(t); tdef != nil {
		return ctx.swiftTypeName(tdef)
	}
	switch t.Kind() {
	case vdl.Array, vdl.List:
		val := fmt.Sprintf("[%s]", ctx.swiftType(t.Elem()))
		return val
	case vdl.Set:
		return fmt.Sprintf("%s<%s>", "Set", ctx.swiftType(t.Key()))
	case vdl.Map:
		return fmt.Sprintf("[%s : %s]", ctx.swiftType(t.Key()), ctx.swiftType(t.Elem()))
	case vdl.Optional:
		return fmt.Sprintf("%s?", ctx.swiftType(t.Elem()))
	default:
		log.Fatalf("vdl: swiftType unhandled type %v %v", t.Kind(), t)
		return ""
	}
}

// swiftHashCode returns the swift code for the hashCode() computation for a given type.
func (ctx *swiftContext) swiftHashCode(name string, ty *vdl.Type) (string, error) {
	if def := ctx.env.FindTypeDef(ty); def != nil && def.File == compile.BuiltInFile {
		switch ty.Kind() {
		case vdl.Bool:
			return fmt.Sprintf("%s.hashValue", name), nil
		case vdl.Byte, vdl.Uint16, vdl.Int16:
			return "Int(" + name + ")", nil
		case vdl.Uint32, vdl.Int32:
			return name, nil
		case vdl.Uint64, vdl.Int64:
			return fmt.Sprintf("%s.hashValue", name), nil
		case vdl.Float32:
			return fmt.Sprintf("Float(%s).hashValue", name), nil
		case vdl.Float64:
			return fmt.Sprintf("Double(%s).hashValue", name), nil
		}
	}

	if !isTypeHashable(ty, make(map[*vdl.Type]bool)) {
		return "", fmt.Errorf("Any is not supported with hashCode")
	}

	switch ty.Kind() {
	case vdl.Optional:
		return fmt.Sprintf("(%s?.hashValue ?? 0)", name), nil
	// Primitives are going to access the rawValue inside this box
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Int16, vdl.Uint32, vdl.Int32, vdl.Uint64, vdl.Int64, vdl.Float32, vdl.Float64:
		return fmt.Sprintf("%s.rawValue.hashValue", name), nil
	}
	return fmt.Sprintf("%s.hashValue", name), nil
}

// isTypeHashable returns true if the type provided is hashable in Swift.
// Part of the difficulty in Swift is that as of 2.1 we can't create extensions
// with protocol conformance (Hashable, Equatable) AND type constraints (where Element : Hashable)
// Thus we can't extend Array AND require its elements to conform to Hashable as follows:
// ILLEGAL => extension Array : Hashable where Element : Hashable
//
// Returns false for Any, arrays, maps, and sets. Returns true for primitives,
// strings, enums. Optional, structs, and unions are only hashable if all of their
// contents are hashable.
func isTypeHashable(ty *vdl.Type, seen map[*vdl.Type]bool) bool {
	return !ty.ContainsKind(vdl.WalkAll, vdl.Any, vdl.Array, vdl.List, vdl.Set, vdl.Map, vdl.TypeObject)
}

func bitlen(kind vdl.Kind) int {
	switch kind {
	case vdl.Float32:
		return 32
	case vdl.Float64:
		return 64
	}
	panic(fmt.Errorf("vdl: bitLen unhandled kind %v", kind))
}
