// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"strconv"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

func localIdent(data *goData, file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return data.Pkg(file.Package.GenPath) + ident
}

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

func noCustomNative(native vdltool.GoType) bool {
	return native.ToNative == "" && native.FromNative == ""
}

func typeHasNoCustomNative(data *goData, def *compile.TypeDef) bool {
	if native, _, ok := findNativeType(data.Env, def.Type); ok {
		return noCustomNative(native)
	}
	return true
}

func packageIdent(file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return file.Package.Name + "." + ident
}

func qualifiedIdent(file *compile.File, ident string) string {
	if testingMode {
		return ident
	}
	return file.Package.QualifiedName(ident)
}

// typeGo translates vdl.Type into a Go type.
func typeGo(data *goData, t *vdl.Type) string {
	if native, pkg, ok := findNativeType(data.Env, t); ok {
		return nativeType(data, native, pkg)
	}
	return typeGoWire(data, t)
}

func typeGoWire(data *goData, t *vdl.Type) string { //nolint:gocyclo
	if testingMode {
		if t.Name() != "" {
			return t.Name()
		}
	}
	// Terminate recursion at defined types, which include both user-defined types
	// (enum, struct, union) and built-in types.
	if def := data.Env.FindTypeDef(t); def != nil {
		switch {
		case t == vdl.AnyType:
			switch goAnyRepMode(data.Package) {
			case goAnyRepRawBytes:
				return "*" + data.Pkg("v.io/v23/vom") + "RawBytes"
			case goAnyRepValue:
				return "*" + data.Pkg("v.io/v23/vdl") + "Value"
			default:
				return "interface{}"
			}
		case t == vdl.TypeObjectType:
			return "*" + data.Pkg("v.io/v23/vdl") + "Type"
		case def.File == compile.BuiltInFile:
			// Built-in primitives just use their name.
			return def.Name
		}
		return localIdent(data, def.File, def.Name)
	}
	// Special-cases to allow error constants.
	switch {
	case t == vdl.ErrorType.Elem():
		return data.Pkg("v.io/v23/vdl") + "WireError"
	case t == vdl.ErrorType.Elem().Field(1).Type:
		return data.Pkg("v.io/v23/vdl") + "WireRetryCode"
	}
	// Otherwise recurse through the type.
	switch t.Kind() {
	case vdl.Optional:
		return "*" + typeGo(data, t.Elem())
	case vdl.Array:
		return "[" + strconv.Itoa(t.Len()) + "]" + typeGo(data, t.Elem())
	case vdl.List:
		return "[]" + typeGo(data, t.Elem())
	case vdl.Set:
		return "map[" + typeGo(data, t.Key()) + "]struct{}"
	case vdl.Map:
		return "map[" + typeGo(data, t.Key()) + "]" + typeGo(data, t.Elem())
	default:
		panic(fmt.Errorf("vdl: typeGo unhandled type %v %v", t.Kind(), t))
	}
}

type goAnyRep int

const (
	goAnyRepRawBytes  goAnyRep = iota // Use vom.RawBytes to represent any.
	goAnyRepValue                     // Use vdl.Value to represent any.
	goAnyRepInterface                 // Use interface{} to represent any.
)

// goAnyRepMode returns the representation of the any type.  By default we use
// vom.RawBytes, but for some hard-coded cases we use vdl.Value or interface{}.
//
// This is hard-coded because we don't want to allow the user to configure this
// behavior, since it's subtle and tricky.  E.g. if the user picks the
// interface{} representation, their generated server stub would fail on vom
// decoding if the type isn't registered.
func goAnyRepMode(pkg *compile.Package) goAnyRep {
	if strings.HasPrefix(pkg.Path, "v.io/v23/vom/testdata") {
		// The vom/testdata/... packages use vdl.Value due to an import cycle: vom
		// imports testdata/...
		return goAnyRepValue
	}
	switch pkg.Path {
	case "v.io/v23/vdl":
		// The vdl package uses vdl.Value due to an import cycle: vom imports vdl.
		return goAnyRepValue
	case "signature":
		// The signature package uses vdl.Value for two reasons:
		// - an import cycle: vom imports vdlroot imports signature
		// - any is used for method tags, and these are used via reflection,
		//   although interface{} is a reasonable alternative.
		return goAnyRepValue
	case "v.io/v23/vdl/vdltest", "v.io/v23/vom/vomtest":
		// The vdltest and vomtest packages use interface{} for convenience in
		// setting up test values.
		return goAnyRepInterface
	}
	return goAnyRepRawBytes
}

func genEnumType(data *goData, def *compile.TypeDef) string {
	t := def.Type
	s := &strings.Builder{}
	fmt.Fprintf(s, "%stype %s int%s\nconst (", def.Doc, def.Name, def.DocSuffix)
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		fmt.Fprintf(s, "\n\t%s%s%s", def.LabelDoc[ix], def.Name, t.EnumLabel(ix))
		if ix == 0 {
			fmt.Fprintf(s, " %s = iota", def.Name)
		}
		s.WriteString(def.LabelDocSuffix[ix])
	}
	disableGoCycloLint := ""
	if t.NumEnumLabel() > 10 {
		disableGoCycloLint = "//nolint:gocyclo"
	}
	fmt.Fprintf(s, "\n)"+
		"\n\n// %[1]sAll holds all labels for %[1]s."+
		"\nvar %[1]sAll = [...]%[1]s{%[2]s}"+
		"\n\n// %[1]sFromString creates a %[1]s from a string label."+
		"\n//nolint:deadcode,unused"+
		"\nfunc %[1]sFromString(label string) (x %[1]s, err error) {"+
		"\n\terr = x.Set(label)"+
		"\n\treturn"+
		"\n}"+
		"\n\n// Set assigns label to x."+
		"\nfunc (x *%[1]s) Set(label string) error {"+
		disableGoCycloLint+
		"\n\tswitch label {",
		def.Name,
		commaEnumLabels(def.Name, t))
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		fmt.Fprintf(s, "\n\tcase %[2]q, %[3]q:"+
			"\n\t\t*x = %[1]s%[2]s"+
			"\n\t\treturn nil", def.Name, t.EnumLabel(ix), strings.ToLower(t.EnumLabel(ix)))
	}
	fmt.Fprintf(s, "\n\t}"+
		"\n\t*x = -1"+
		"\n\treturn "+data.Pkg("fmt")+"Errorf(\"unknown label %%q in %[2]s\", label)"+
		"\n}"+
		"\n\n// String returns the string label of x."+
		"\nfunc (x %[1]s) String() string {"+
		disableGoCycloLint+
		"\n\tswitch x {", def.Name, packageIdent(def.File, def.Name))
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		fmt.Fprintf(s, "\n\tcase %[1]s%[2]s:"+
			"\n\t\treturn %[2]q", def.Name, t.EnumLabel(ix))
	}
	fmt.Fprintf(s, "\n\t}"+
		"\n\treturn \"\""+
		"\n}"+
		"\n\nfunc (%[1]s) VDLReflect(struct{"+
		"\n\tName string `vdl:%[3]q`"+
		"\n\tEnum struct{ %[2]s string }"+
		"\n}) {"+
		"\n}",
		def.Name, commaEnumLabels("", t), qualifiedIdent(def.File, def.Name))
	return s.String()
}

func genStructType(data *goData, def *compile.TypeDef) string {
	t := def.Type
	var structTags map[string][]vdltool.GoStructTag
	if data.Package != nil && data.Package.Config.Go.StructTags != nil {
		structTags = data.Package.Config.Go.StructTags
	}
	if structTags == nil {
		structTags = map[string][]vdltool.GoStructTag{}
	}
	s := &strings.Builder{}
	fmt.Fprintf(s, "%stype %s struct {", def.Doc, def.Name)
	for ix := 0; ix < t.NumField(); ix++ {
		f := t.Field(ix)
		fmt.Fprintf(s, "\n\t%s %s", def.FieldDoc[ix]+f.Name, typeGo(data, f.Type)+def.FieldDocSuffix[ix])
		if tags, ok := structTags[def.Name]; ok {
			for _, tag := range tags {
				if tag.Field == f.Name {
					s.WriteString(" `" + tag.Tag + "`")
				}
			}
		}
	}
	fmt.Fprintf(s, "\n}%s", def.DocSuffix)
	fmt.Fprintf(s, "\n"+
		"\nfunc (%[1]s) VDLReflect(struct{"+
		"\n\tName string `vdl:%[2]q`"+
		"\n}) {"+
		"\n}",
		def.Name, qualifiedIdent(def.File, def.Name))
	return s.String()
}

func genUnionType(data *goData, def *compile.TypeDef) string {
	t := def.Type
	s := &strings.Builder{}
	fmt.Fprintf(s, "type ("+
		"\n\t// %[1]s represents any single field of the %[1]s union type."+
		"\n\t%[2]s%[1]s interface {"+
		"\n\t\t// Index returns the field index."+
		"\n\t\tIndex() int"+
		"\n\t\t// Interface returns the field value as an interface."+
		"\n\t\tInterface() interface{}"+
		"\n\t\t// Name returns the field name."+
		"\n\t\tName() string"+
		"\n\t\t// VDLReflect describes the %[1]s union type."+
		"\n\t\tVDLReflect(vdl%[1]sReflect)", def.Name, docBreak(def.Doc))
	if !data.SkipGenZeroReadWrite(def) {
		fmt.Fprintf(s, "\n\t\tVDLIsZero() bool"+
			"\n\t\tVDLWrite(%[1]sEncoder) error", data.Pkg("v.io/v23/vdl"))
	}
	fmt.Fprintf(s, "\n\t}%[1]s", def.DocSuffix)
	for ix := 0; ix < t.NumField(); ix++ {
		f := t.Field(ix)
		fmt.Fprintf(s, "\n\t// %[1]s%[2]s represents field %[2]s of the %[1]s union type."+
			"\n\t%[4]s%[1]s%[2]s struct{ Value %[3]s }%[5]s",
			def.Name, f.Name, typeGo(data, f.Type),
			docBreak(def.FieldDoc[ix]), def.FieldDocSuffix[ix])
	}
	fmt.Fprintf(s, "\n\t// vdl%[1]sReflect describes the %[1]s union type."+
		"\n\tvdl%[1]sReflect struct {"+
		"\n\t\tName string `vdl:%[2]q`"+
		"\n\t\tType %[1]s", def.Name, qualifiedIdent(def.File, def.Name))
	s.WriteString("\n\t\tUnion struct {")
	for ix := 0; ix < t.NumField(); ix++ {
		fmt.Fprintf(s, "\n\t\t\t%[2]s %[1]s%[2]s", def.Name, t.Field(ix).Name)
	}
	s.WriteString("\n\t\t}\n\t}\n)")
	for ix := 0; ix < t.NumField(); ix++ {
		f := t.Field(ix)
		fmt.Fprintf(s, "\n\nfunc (x %[1]s%[2]s) Index() int { return %[3]d }"+
			"\nfunc (x %[1]s%[2]s) Interface() interface{} { return x.Value }"+
			"\nfunc (x %[1]s%[2]s) Name() string { return \"%[2]s\" }"+
			"\nfunc (x %[1]s%[2]s) VDLReflect(vdl%[1]sReflect) {}",
			def.Name, f.Name, ix)
	}
	return s.String()
}

// defineType returns the type definition for def.
func defineType(data *goData, def *compile.TypeDef) string {
	switch t := def.Type; t.Kind() {
	case vdl.Enum:
		return genEnumType(data, def)
	case vdl.Struct:
		return genStructType(data, def)
	case vdl.Union:
		return genUnionType(data, def)
	default:
		s := &strings.Builder{}
		fmt.Fprintf(s, "%stype %s %s", def.Doc, def.Name, typeGo(data, def.BaseType)+def.DocSuffix)
		fmt.Fprintf(s, "\n"+
			"\nfunc (%[1]s) VDLReflect(struct{"+
			"\n\tName string `vdl:%[2]q`"+
			"\n}) {"+
			"\n}",
			def.Name, qualifiedIdent(def.File, def.Name))
		return s.String()
	}
}

func commaEnumLabels(prefix string, t *vdl.Type) (s string) {
	for ix := 0; ix < t.NumEnumLabel(); ix++ {
		if ix > 0 {
			s += ", "
		}
		s += prefix
		s += t.EnumLabel(ix)
	}
	return
}

func embedGo(data *goData, embed *compile.Interface) string {
	return localIdent(data, embed.File, embed.Name)
}
