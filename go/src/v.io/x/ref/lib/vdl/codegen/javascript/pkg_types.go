// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"fmt"
	"sort"
	"strings"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"

	"v.io/v23/vdl"
)

// typeNames holds a mapping between VDL type and generated type name.
type typeNames map[*vdl.Type]string

// LookupConstructor returns a string representing the constructor of the type.
// Several cases:
// - Local package type (and has been added to tn), return
// Registry.lookupOrCreateConstructor(_typeNameHere)
// - Builtin
// This is not supported. Fail.
// - Type in other package
// Return pkgName.ConstructorName
func (tn typeNames) LookupConstructor(t *vdl.Type) string {
	if builtInName, ok := builtinJSType(t); ok {
		return tn.constructorFromTypeName(builtInName)
	}

	if name, ok := tn[t]; ok {
		return tn.constructorFromTypeName(name)
	}

	return qualifiedName(t)
}

func (tn typeNames) constructorFromTypeName(name string) string {
	return "(vdl.registry.lookupOrCreateConstructor(" + name + "))"
}

// Is this type defined in a different package?
func (tn typeNames) IsDefinedInExternalPkg(t *vdl.Type) bool {
	if _, ok := builtinJSType(t); ok {
		return false
	}
	if _, ok := tn[t]; ok {
		return false
	}
	return true
}

// qualifiedName returns a name representing the type prefixed by
// its package name (e.g. "a.X")
func qualifiedName(t *vdl.Type) string {
	pkgPath, name := vdl.SplitIdent(t.Name())
	pkgParts := strings.Split(pkgPath, "/")
	pkgName := pkgParts[len(pkgParts)-1]
	return fmt.Sprintf("%s.%s", pkgName, name)
}

// LookupType returns a string representing the type.
// - If it is a built in type, return the name.
// - Otherwise get type type from the constructor.
func (tn typeNames) LookupType(t *vdl.Type) string {
	if builtInName, ok := builtinJSType(t); ok {
		return builtInName
	}

	if name, ok := tn[t]; ok {
		return name
	}

	if t.Kind() == vdl.Enum {
		return fmt.Sprintf("%s.%s._type", tn.LookupConstructor(t), vdlutil.ToConstCase(t.EnumLabel(0)))
	}

	return "new " + tn.LookupConstructor(t) + "()._type"
}

// SortedList returns a list of type and name pairs, sorted by name.
// This is needed to make the output stable.
func (tn typeNames) SortedList() typeNamePairList {
	pairs := typeNamePairList{}
	for t, name := range tn {
		pairs = append(pairs, typeNamePair{t, name})
	}
	sort.Sort(pairs)
	return pairs
}

type typeNamePairList []typeNamePair
type typeNamePair struct {
	Type *vdl.Type
	Name string
}

func (l typeNamePairList) Len() int           { return len(l) }
func (l typeNamePairList) Less(i, j int) bool { return l[i].Name < l[j].Name }
func (l typeNamePairList) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }

// newTypeNames generates typeNames for all new types in a package.
func newTypeNames(pkg *compile.Package) typeNames {
	ptn := pkgTypeNames{
		nextIndex: 1,
		names:     typeNames{},
		pkg:       pkg,
	}
	return ptn.getNames()
}

// pkgTypeNames tracks information necessary to define JS types from VDL.
// The next index that a new type will be auto-generated with previously auto-generated type names
// package being generated
type pkgTypeNames struct {
	nextIndex int
	names     typeNames
	pkg       *compile.Package
}

// getNames generates typeNames for all new types in a package.
func (p pkgTypeNames) getNames() typeNames {
	for _, file := range p.pkg.Files {
		for _, def := range file.TypeDefs {
			p.addInnerTypes(def.Type)
		}
		for _, constdef := range file.ConstDefs {
			p.addInnerTypes(constdef.Value.Type())
			p.addTypesInConst(constdef.Value)
		}
		for _, errordef := range file.ErrorDefs {
			for _, field := range errordef.Params {
				p.addInnerTypes(field.Type)
			}
		}
		for _, interfacedef := range file.Interfaces {
			for _, method := range interfacedef.AllMethods() {
				for _, inarg := range method.InArgs {
					p.addInnerTypes(inarg.Type)
				}
				for _, outarg := range method.OutArgs {
					p.addInnerTypes(outarg.Type)
				}
				if method.InStream != nil {
					p.addInnerTypes(method.InStream)
				}
				if method.OutStream != nil {
					p.addInnerTypes(method.OutStream)
				}
			}
		}
	}

	return p.names
}

// addName generates a new name for t if necessary.  Returns true if a new name
// was generated, and thus names for inner-types should also be generated.
func (p *pkgTypeNames) addName(t *vdl.Type) bool {
	// Name has already been generated.
	if _, ok := p.names[t]; ok {
		return false
	}

	// Do not create name for built-in JS types (primitives, any, etc..)
	if _, ok := builtinJSType(t); ok {
		return false
	}

	var name string
	if t.Name() != "" {
		pp, n := vdl.SplitIdent(t.Name())
		// Do not create name for types from other packages.
		if pp != p.pkg.Path {
			return false
		}
		name = "_type" + n
	} else {
		name = fmt.Sprintf("_type%d", p.nextIndex)
		p.nextIndex++
	}
	p.names[t] = name
	return true
}

func (p *pkgTypeNames) addInnerTypes(t *vdl.Type) {
	if !p.addName(t) {
		return
	}
	switch t.Kind() {
	case vdl.Optional, vdl.Array, vdl.List:
		p.addInnerTypes(t.Elem())
	case vdl.Set:
		p.addInnerTypes(t.Key())
	case vdl.Map:
		p.addInnerTypes(t.Key())
		p.addInnerTypes(t.Elem())
	case vdl.Struct, vdl.Union:
		for i := 0; i < t.NumField(); i++ {
			p.addInnerTypes(t.Field(i).Type)
		}
	}
}

func (p *pkgTypeNames) addTypesInConst(v *vdl.Value) {
	// Generate the type if it is a typeobject or any.
	switch v.Kind() {
	case vdl.TypeObject:
		p.addInnerTypes(v.TypeObject())
	case vdl.Any:
		if !v.IsNil() {
			p.addInnerTypes(v.Elem().Type())
		}
	}

	// Recurse.
	switch v.Kind() {
	case vdl.List, vdl.Array:
		for i := 0; i < v.Len(); i++ {
			p.addTypesInConst(v.Index(i))
		}
	case vdl.Set:
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			p.addTypesInConst(key)
		}
	case vdl.Map:
		for _, key := range vdl.SortValuesAsString(v.Keys()) {
			p.addTypesInConst(key)
			p.addTypesInConst(v.MapIndex(key))

		}
	case vdl.Struct:
		for i := 0; i < v.Type().NumField(); i++ {
			p.addTypesInConst(v.StructField(i))
		}
	case vdl.Union:
		_, innerVal := v.UnionField()
		p.addTypesInConst(innerVal)
	case vdl.Any, vdl.Optional:
		if !v.IsNil() {
			p.addTypesInConst(v.Elem())
		}
	}
}
