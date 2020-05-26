// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"fmt"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// makeTypeDefinitionsString generates a string that defines the specified types.
// It consists of the following sections:
// - Definitions. e.g. "var _typeNamedBool = new Type();"
// - Field assignments. e.g. "_typeNamedBool.name = \"NamedBool\";"
// - Type Freezes, e.g. "_typedNamedBool.freeze();"
// - Constructor definitions. e.g. "types.NamedBool = Registry.lookupOrCreateConstructor(_typeNamedBool)"
func makeTypeDefinitionsString(jsnames typeNames) string {
	str := ""
	sortedDefs := jsnames.SortedList()

	for _, def := range sortedDefs {
		str += makeDefString(def.Name)
	}

	for _, def := range sortedDefs {
		str += makeTypeFieldAssignmentString(def.Name, def.Type, jsnames)
	}

	for _, def := range sortedDefs {
		str += makeTypeFreezeString(def.Name)
	}

	for _, def := range sortedDefs {
		if def.Type.Name() != "" {
			if def.Type.Kind() == vdl.Enum {
				str += makeEnumLabelString(def.Type, jsnames)
			} else {
				str += makeConstructorDefinitionString(def.Type, jsnames)
			}
		}
	}

	return str
}

// makeDefString generates a type definition for the specified type name.
// e.g. "var _typeNamedBool = new Type();"
func makeDefString(jsname string) string {
	return fmt.Sprintf("var %s = new vdl.Type();\n", jsname)
}

// makeTypeFreezeString calls the type's freeze function to finalize it.
// e.g. "typeNamedBool.freeze();"
func makeTypeFreezeString(jsname string) string {
	return fmt.Sprintf("%s.freeze();\n", jsname)
}

// makeTypeFieldAssignmentString generates assignments for type fields.
// e.g. "_typeNamedBool.name = \"NamedBool\";"
func makeTypeFieldAssignmentString(jsname string, t *vdl.Type, jsnames typeNames) string {
	// kind
	str := fmt.Sprintf("%s.kind = %s;\n", jsname, jsKind(t.Kind()))

	// name
	str += fmt.Sprintf("%s.name = %q;\n", jsname, t.Name())

	// labels
	if t.Kind() == vdl.Enum {
		str += fmt.Sprintf("%s.labels = [", jsname)
		for i := 0; i < t.NumEnumLabel(); i++ {
			if i > 0 {
				str += ", "
			}
			str += fmt.Sprintf("%q", t.EnumLabel(i))
		}
		str += "];\n"
	}

	// len
	if t.Kind() == vdl.Array { // Array is the only type where len is valid.
		str += fmt.Sprintf("%s.len = %d;\n", jsname, t.Len())
	}

	// elem
	switch t.Kind() {
	case vdl.Optional, vdl.Array, vdl.List, vdl.Map:
		str += fmt.Sprintf("%s.elem = %s;\n", jsname, jsnames.LookupType(t.Elem()))
	}

	// key
	switch t.Kind() {
	case vdl.Set, vdl.Map:
		str += fmt.Sprintf("%s.key = %s;\n", jsname, jsnames.LookupType(t.Key()))
	}

	// fields
	switch t.Kind() {
	case vdl.Struct, vdl.Union:
		str += fmt.Sprintf("%s.fields = [", jsname)
		for i := 0; i < t.NumField(); i++ {
			if i > 0 {
				str += ", "
			}
			field := t.Field(i)
			str += fmt.Sprintf("{name: %q, type: %s}", field.Name, jsnames.LookupType(field.Type))
		}
		str += "];\n"
	}

	return str
}

// makeConstructorDefinitionString creates a string that defines the constructor for the type.
// e.g. "module.exports.NamedBool = Registry.lookupOrCreateConstructor(_typeNamedBool)"
func makeConstructorDefinitionString(t *vdl.Type, jsnames typeNames) string {
	_, name := vdl.SplitIdent(t.Name())
	ctorName := jsnames.LookupConstructor(t)
	return fmt.Sprintf("module.exports.%s = %s;\n", name, ctorName)
}

// makeEnumLabelString creates a string that defines the labels in an enum.
// e.g. `module.Exports.MyEnum = {
//   ALabel: (Registry.lookupOrCreateConstructor(_typeMyEnum))("ALabel"),
//   BLabel: (Registry.lookupOrCreateConstructor(_typeMyEnum))("BLabel"),
// }`

func makeEnumLabelString(t *vdl.Type, jsnames typeNames) string {
	_, name := vdl.SplitIdent(t.Name())
	str := fmt.Sprintf("module.exports.%s = {\n", name)
	for i := 0; i < t.NumEnumLabel(); i++ {
		enumVal := vdl.ZeroValue(t)
		enumVal.AssignEnumIndex(i)
		str += fmt.Sprintf("  %s: %s,\n", vdlutil.ToConstCase(t.EnumLabel(i)), typedConst(jsnames, enumVal))
	}
	str += "};\n"
	return str
}

func jsKind(k vdl.Kind) string { //nolint:gocyclo
	switch k {
	case vdl.Any:
		return "vdl.kind.ANY"
	case vdl.Union:
		return "vdl.kind.UNION"
	case vdl.Optional:
		return "vdl.kind.OPTIONAL"
	case vdl.Bool:
		return "vdl.kind.BOOL"
	case vdl.Byte:
		return "vdl.kind.BYTE"
	case vdl.Uint16:
		return "vdl.kind.UINT16"
	case vdl.Uint32:
		return "vdl.kind.UINT32"
	case vdl.Uint64:
		return "vdl.kind.UINT64"
	case vdl.Int8:
		return "vdl.kind.INT8"
	case vdl.Int16:
		return "vdl.kind.INT16"
	case vdl.Int32:
		return "vdl.kind.INT32"
	case vdl.Int64:
		return "vdl.kind.INT64"
	case vdl.Float32:
		return "vdl.kind.FLOAT32"
	case vdl.Float64:
		return "vdl.kind.FLOAT64"
	case vdl.String:
		return "vdl.kind.STRING"
	case vdl.Enum:
		return "vdl.kind.ENUM"
	case vdl.TypeObject:
		return "vdl.kind.TYPEOBJECT"
	case vdl.Array:
		return "vdl.kind.ARRAY"
	case vdl.List:
		return "vdl.kind.LIST"
	case vdl.Set:
		return "vdl.kind.SET"
	case vdl.Map:
		return "vdl.kind.MAP"
	case vdl.Struct:
		return "vdl.kind.STRUCT"
	}
	panic(fmt.Errorf("val: unhandled kind: %d", k))
}

// builtinJSType indicates whether a vdl.Type has built-in type definition in vdl.js
// If true, then it returns a pointer to the type definition in javascript/types.js
// It assumes a variable named "vdl.Types" is already pointing to vom.Types
func builtinJSType(t *vdl.Type) (string, bool) { //nolint:gocyclo
	_, n := vdl.SplitIdent(t.Name())

	if t == vdl.ErrorType {
		return "vdl.types.ERROR", true
	}

	// named types are not built-in.
	if n != "" {
		return "", false
	}

	// switch on supported types in vdl.js
	switch t.Kind() {
	case vdl.Any:
		return "vdl.types.ANY", true
	case vdl.Bool:
		return "vdl.types.BOOL", true
	case vdl.Byte:
		return "vdl.types.BYTE", true
	case vdl.Uint16:
		return "vdl.types.UINT16", true
	case vdl.Uint32:
		return "vdl.types.UINT32", true
	case vdl.Uint64:
		return "vdl.types.UINT64", true
	case vdl.Int8:
		return "vdl.types.INT8", true
	case vdl.Int16:
		return "vdl.types.INT16", true
	case vdl.Int32:
		return "vdl.types.INT32", true
	case vdl.Int64:
		return "vdl.types.INT64", true
	case vdl.Float32:
		return "vdl.types.FLOAT32", true
	case vdl.Float64:
		return "vdl.types.FLOAT64", true
	case vdl.String:
		return "vdl.types.STRING", true
	case vdl.TypeObject:
		return "vdl.types.TYPEOBJECT", true
	}

	return "", false
}
