// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlgen

// TODO(toddw): Add tests

import (
	"fmt"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/codegen"
)

// Type returns t using VDL syntax, returning the qualified name if t is named,
// otherwise returning the base type of t.  The pkgPath and imports are used to
// determine the local package qualifier to add to named types; if a local
// package qualifier cannot be found, a full package path qualifier is added.
func Type(t *vdl.Type, pkgPath string, imports codegen.Imports) string {
	if t.Name() == "" {
		return BaseType(t, pkgPath, imports)
	}
	path, name := vdl.SplitIdent(t.Name())
	if path == "" && name == "" {
		return "<empty>"
	}
	if path == "" || path == pkgPath {
		return name
	}
	if local := imports.LookupLocal(path); local != "" {
		return local + "." + name
	}
	return `"` + path + `".` + name
}

// BaseType returns the base type of t using VDL syntax, where the base type is
// the type of t disregarding its name.  Subtypes contained in t are output via
// calls to Type.
func BaseType(t *vdl.Type, pkgPath string, imports codegen.Imports) string {
	if t == vdl.ErrorType {
		return "error"
	}
	switch k := t.Kind(); k {
	case vdl.Any, vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.String, vdl.TypeObject:
		// Built-in types are named the same as their kind.
		return k.String()
	case vdl.Optional:
		return "?" + Type(t.Elem(), pkgPath, imports)
	case vdl.Enum:
		ret := "enum{"
		for i := 0; i < t.NumEnumLabel(); i++ {
			if i > 0 {
				ret += ";"
			}
			ret += t.EnumLabel(i)
		}
		return ret + "}"
	case vdl.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), Type(t.Elem(), pkgPath, imports))
	case vdl.List:
		return fmt.Sprintf("[]%s", Type(t.Elem(), pkgPath, imports))
	case vdl.Set:
		return fmt.Sprintf("set[%s]", Type(t.Key(), pkgPath, imports))
	case vdl.Map:
		key := Type(t.Key(), pkgPath, imports)
		elem := Type(t.Elem(), pkgPath, imports)
		return fmt.Sprintf("map[%s]%s", key, elem)
	case vdl.Struct, vdl.Union:
		ret := k.String() + " {\n"
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			ftype := Type(f.Type, pkgPath, imports)
			ftype = strings.ReplaceAll(ftype, "\n", "\n\t")
			ret += fmt.Sprintf("\t%s %s\n", f.Name, ftype)
		}
		return ret + "}"
	default:
		panic(fmt.Errorf("vdlgen.BaseType: unhandled type: %v %v", k, t))
	}
}
