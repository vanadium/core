// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"fmt"
	"log"
	"path"
	"strings"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

func javaFullyQualifiedNamedType(def *compile.TypeDef, forceClass bool, env *compile.Env) string {
	if def.File == compile.BuiltInFile {
		name, _ := javaBuiltInType(def.Type, forceClass)
		return name
	}
	name, _ := javaTypeName(def, env)
	return javaPath(path.Join(javaGenPkgPath(def.File.Package.GenPath), name))
}

// javaReflectType returns java.reflect.Type string for provided VDL type.
func javaReflectType(t *vdl.Type, env *compile.Env) string {
	if t != nil {
		switch t.Kind() {
		case vdl.List, vdl.Set, vdl.Map, vdl.Optional:
			if javaType(t, false, env) == "byte[]" {
				return "byte[].class"
			}
			return fmt.Sprintf("new com.google.common.reflect.TypeToken<%s>(){}.getType()", javaType(t, true, env))
		}
	}
	return fmt.Sprintf("%s.class", javaType(t, true, env))
}

// javaBuiltInType returns the type name for the provided built in type
// definition, forcing the use of a java class (e.g., java.lang.Integer) if so
// desired.  This method also returns a boolean value indicating whether the
// returned type is a class.
//
// All java integers (byte, short, int, long) are signed.  We
// translate signed vdl integers int{16,32,64} into their
// java equivalents.  We translate unsigned vdl integers
// uint{16,32,64} into our class-based representation
// VdlUint{16,32,64}.
//
// According to this rule, we should translate signed
// vdl int8 into java byte, and unsigned vdl byte into
// java VdlUint8.  However we flip the rule, and
// actually translate vdl int8 into java VdlInt8, and
// vdl byte into java byte.  We do this because we want the
// common usage of vdl []byte to translate into java byte[].
func javaBuiltInType(typ *vdl.Type, forceClass bool) (string, bool) { //nolint:gocyclo
	if typ == nil {
		if forceClass {
			return "java.lang.Void", true
		}
		return "void", false
	}
	switch typ.Kind() {
	case vdl.Bool:
		if forceClass {
			return "java.lang.Boolean", true
		}
		return "boolean", false
	case vdl.Byte:
		if forceClass {
			return "java.lang.Byte", true
		}
		return "byte", false
	case vdl.Int8:
		return "io.v.v23.vdl.VdlInt8", true
	case vdl.Uint16:
		return "io.v.v23.vdl.VdlUint16", true
	case vdl.Int16:
		if forceClass {
			return "java.lang.Short", true
		}
		return "short", false
	case vdl.Uint32:
		return "io.v.v23.vdl.VdlUint32", true
	case vdl.Int32:
		if forceClass {
			return "java.lang.Integer", true
		}
		return "int", false
	case vdl.Uint64:
		return "io.v.v23.vdl.VdlUint64", true
	case vdl.Int64:
		if forceClass {
			return "java.lang.Long", true
		}
		return "long", false
	case vdl.Float32:
		if forceClass {
			return "java.lang.Float", true
		}
		return "float", false
	case vdl.Float64:
		if forceClass {
			return "java.lang.Double", true
		}
		return "double", false
	case vdl.String:
		return "java.lang.String", true
	case vdl.TypeObject:
		return "io.v.v23.vdl.VdlTypeObject", true
	case vdl.Any:
		return "io.v.v23.vdl.VdlAny", true
	default:
		return "", false
	}
}

func javaTypeName(def *compile.TypeDef, env *compile.Env) (string, string) {
	if native, ok := def.File.Package.Config.Java.WireTypeRenames[def.Name]; ok {
		return native, accessModifierForName(native)
	}
	return vdlutil.FirstRuneToUpper(def.Name), accessModifierForName(def.Name)
}

func javaNativeType(t *vdl.Type, env *compile.Env) (string, bool) {
	if t == vdl.ErrorType {
		return "io.v.v23.verror.VException", true
	}
	if def := env.FindTypeDef(t); def != nil {
		pkg := def.File.Package
		name, _ := javaTypeName(def, env)
		if native, ok := pkg.Config.Java.WireToNativeTypes[name]; ok {
			// There is a Java native type configured for this defined type.
			return native, true
		}
	}
	return "", false
}

func javaType(t *vdl.Type, forceClass bool, env *compile.Env) string {
	if t == nil {
		name, _ := javaBuiltInType(nil, forceClass)
		return name
	}
	if native, ok := javaNativeType(t, env); ok {
		return native
	}
	if def := env.FindTypeDef(t); def != nil {
		return javaFullyQualifiedNamedType(def, forceClass, env)
	}
	switch t.Kind() {
	case vdl.Array:
		return fmt.Sprintf("%s[]", javaType(t.Elem(), false, env))
	case vdl.List:
		// NOTE(spetrovic): We represent byte lists as Java byte arrays, as it's doubtful anybody
		// would want to use them as Java lists.
		if javaType(t.Elem(), false, env) == "byte" {
			return "byte[]"
		}
		return fmt.Sprintf("%s<%s>", "java.util.List", javaType(t.Elem(), true, env))
	case vdl.Set:
		return fmt.Sprintf("%s<%s>", "java.util.Set", javaType(t.Key(), true, env))
	case vdl.Map:
		return fmt.Sprintf("%s<%s, %s>", "java.util.Map", javaType(t.Key(), true, env), javaType(t.Elem(), true, env))
	case vdl.Optional:
		return fmt.Sprintf("io.v.v23.vdl.VdlOptional<%s>", javaType(t.Elem(), true, env))
	default:
		log.Fatalf("vdl: javaType unhandled type %v %v", t.Kind(), t)
		return ""
	}
}

func javaVdlPrimitiveType(kind vdl.Kind) string {
	switch kind {
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64, vdl.String:
		return "io.v.v23.vdl.Vdl" + vdlutil.FirstRuneToUpper(kind.String())
	}
	log.Fatalf("val: unhandled kind: %v", kind)
	return ""
}

// javaHashCode returns the java code for the hashCode() computation for a given type.
func javaHashCode(name string, ty *vdl.Type, env *compile.Env) string {
	if isJavaNativeArray(ty, env) {
		return fmt.Sprintf("java.util.Arrays.hashCode(%s)", name)
	}
	if def := env.FindTypeDef(ty); def != nil && def.File == compile.BuiltInFile {
		switch ty.Kind() {
		case vdl.Bool:
			return fmt.Sprintf("java.lang.Boolean.valueOf(%s).hashCode()", name)
		case vdl.Byte, vdl.Int16:
			return "(int)" + name
		case vdl.Int32:
			return name
		case vdl.Int64:
			return fmt.Sprintf("java.lang.Long.valueOf(%s).hashCode()", name)
		case vdl.Float32:
			return fmt.Sprintf("java.lang.Float.valueOf(%s).hashCode()", name)
		case vdl.Float64:
			return fmt.Sprintf("java.lang.Double.valueOf(%s).hashCode()", name)
		}
	}
	return fmt.Sprintf("(%s == null ? 0 : %s.hashCode())", name, name)
}

// isClass returns true iff the provided type is represented by a Java class.
func isClass(t *vdl.Type, env *compile.Env) bool {
	if t == nil { // void type
		return false
	}
	if def := env.FindTypeDef(t); def != nil && def.File == compile.BuiltInFile {
		// Built-in type.  See if it's represented by a class.
		if tname, isClass := javaBuiltInType(t, false); tname != "" && !isClass {
			return false
		}
	}
	return true
}

// isJavaNativeArray returns true iff the provided type is represented by a Java array.
func isJavaNativeArray(t *vdl.Type, env *compile.Env) bool {
	typeStr := javaType(t, false, env)
	return strings.HasSuffix(typeStr, "[]")
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
