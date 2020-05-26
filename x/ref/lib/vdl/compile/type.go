// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"

	"v.io/v23/vdl"
	"v.io/x/lib/toposort"
	"v.io/x/ref/lib/vdl/parse"
)

// TypeDef represents a user-defined named type definition in the compiled
// results.
type TypeDef struct {
	NamePos            // name, parse position and docs
	Exported bool      // is this type definition exported?
	Type     *vdl.Type // type of this type definition

	// BaseType is the type that Type is based on.  The BaseType may be named or
	// unnamed.  E.g.
	//                                 BaseType
	//   type Bool    bool;            bool
	//   type Bool2   Bool;            Bool
	//   type List    []int32;         []int32
	//   type List2   List;            List
	//   type Struct  struct{A bool};  struct{A bool}
	//   type Struct2 Struct;          Struct
	BaseType *vdl.Type

	LabelDoc       []string // [valid for enum] docs for each label
	LabelDocSuffix []string // [valid for enum] suffix docs for each label
	FieldDoc       []string // [valid for struct, union] docs for each field
	FieldDocSuffix []string // [valid for struct, union] suffix docs for each field
	File           *File    // parent file that this type is defined in
}

func (x *TypeDef) String() string {
	y := *x
	y.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", y)
}

// compileTypeDefs is the "entry point" to the rest of this file.  It takes the
// types defined in pfiles and compiles them into TypeDefs in pkg.
func compileTypeDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	td := typeDefiner{
		pkg:      pkg,
		pfiles:   pfiles,
		env:      env,
		vtb:      new(vdl.TypeBuilder),
		builders: make(map[string]*typeDefBuilder),
	}
	if td.Declare(); !env.Errors.IsEmpty() {
		return
	}
	if td.Describe(); !env.Errors.IsEmpty() {
		return
	}
	if td.Define(); !env.Errors.IsEmpty() {
		return
	}
	td.AttachDoc()
}

// typeDefiner defines types in a package.  This is split into three phases:
// 1) Declare ensures local type references can be resolved.
// 2) Describe describes each type, resolving named references.
// 3) Define sorts in dependency order, and builds and defines all types.
//
// It holds a builders map from type name to typeDefBuilder, where the
// typeDefBuilder is responsible for compiling and defining a single type.
type typeDefiner struct {
	pkg      *Package
	pfiles   []*parse.File
	env      *Env
	vtb      *vdl.TypeBuilder
	builders map[string]*typeDefBuilder
}

type typeDefBuilder struct {
	def     *TypeDef
	ptype   parse.Type
	pending vdl.PendingNamed // named type that's being built
	base    vdl.PendingType  // base type that pending is based on
}

// Declare creates builders for each type defined in the package.
func (td typeDefiner) Declare() {
	for ix := range td.pkg.Files {
		file, pfile := td.pkg.Files[ix], td.pfiles[ix]
		for _, pdef := range pfile.TypeDefs {
			detail := identDetail("type", file, pdef.Pos)
			if err := file.DeclareIdent(pdef.Name, detail); err != nil {
				td.env.prefixErrorf(file, pdef.Pos, err, "type %s name conflict", pdef.Name)
				continue
			}
			td.builders[pdef.Name] = td.makeTypeDefBuilder(file, pdef)
		}
	}
}

func (td typeDefiner) makeTypeDefBuilder(file *File, pdef *parse.TypeDef) *typeDefBuilder {
	export, err := validIdent(pdef.Name, reservedNormal)
	if err != nil {
		td.env.prefixErrorf(file, pdef.Pos, err, "type %s invalid name", pdef.Name)
		return nil
	}
	b := new(typeDefBuilder)
	b.def = &TypeDef{NamePos: NamePos(pdef.NamePos), Exported: export, File: file}
	b.ptype = pdef.Type
	// We use the qualified name to actually name the type, to ensure types
	// defined in separate packages are hash-consed separately.
	qname := file.Package.QualifiedName(pdef.Name)
	b.pending = td.vtb.Named(qname)
	switch pt := pdef.Type.(type) {
	case *parse.TypeEnum:
		b.def.LabelDoc = make([]string, len(pt.Labels))
		b.def.LabelDocSuffix = make([]string, len(pt.Labels))
		for index, plabel := range pt.Labels {
			if err := validExportedIdent(plabel.Name, reservedFirstRuneLower); err != nil {
				td.env.prefixErrorf(file, plabel.Pos, err, "invalid enum label name %s", plabel.Name)
				return nil
			}
			b.def.LabelDoc[index] = plabel.Doc
			b.def.LabelDocSuffix[index] = plabel.DocSuffix
		}
	case *parse.TypeStruct:
		b = attachFieldDoc(b, pt.Fields, file, td.env)
	case *parse.TypeUnion:
		b = attachFieldDoc(b, pt.Fields, file, td.env)
	}
	return b
}

func attachFieldDoc(b *typeDefBuilder, fields []*parse.Field, file *File, env *Env) *typeDefBuilder {
	b.def.FieldDoc = make([]string, len(fields))
	b.def.FieldDocSuffix = make([]string, len(fields))
	for index, pfield := range fields {
		if err := validExportedIdent(pfield.Name, reservedFirstRuneLower); err != nil {
			env.prefixErrorf(file, pfield.Pos, err, "invalid field name %s", pfield.Name)
			return nil
		}
		b.def.FieldDoc[index] = pfield.Doc
		b.def.FieldDocSuffix[index] = pfield.DocSuffix
	}
	return b
}

// Describe uses the builders to describe each type.  Named types defined in
// other packages must have already been compiled, and in env.  Named types
// defined in this package are represented by the builders.
func (td typeDefiner) Describe() {
	for _, b := range td.builders {
		def, file := b.def, b.def.File
		c := typeCompiler{
			file:        file,
			env:         td.env,
			vtb:         td.vtb,
			builders:    td.builders,
			transExport: def.Exported,
		}
		base := c.compileDef(b.ptype)
		switch tbase := base.(type) {
		case nil:
			continue // keep going to catch more errors
		case *vdl.Type:
			if tbase == vdl.ErrorType {
				td.env.Errorf(file, def.Pos, "error cannot be renamed")
				continue // keep going to catch more errors
			}
			def.BaseType = tbase
		case vdl.PendingType:
			b.base = tbase
		default:
			panic(fmt.Errorf("vdl: typeDefiner.Define unhandled TypeOrPending %T %v", tbase, tbase))
		}
		b.pending.AssignBase(base)
	}
}

// compileType returns the *vdl.Type corresponding to ptype.  All named types
// referenced by ptype must already be defined.
func compileType(ptype parse.Type, file *File, env *Env) *vdl.Type {
	c := typeCompiler{
		file: file,
		env:  env,
		vtb:  new(vdl.TypeBuilder),
	}
	typeOrPending := c.compileLit(ptype)
	c.vtb.Build()
	switch top := typeOrPending.(type) {
	case nil:
		return nil
	case *vdl.Type:
		return top
	case vdl.PendingType:
		t, err := top.Built()
		if err != nil {
			env.prefixErrorf(file, ptype.Pos(), err, "invalid type")
			return nil
		}
		return t
	default:
		panic(fmt.Errorf("vdl: compileType unhandled TypeOrPending %T %v", top, top))
	}
}

func compileExportedType(ptype parse.Type, file *File, env *Env) *vdl.Type {
	t := compileType(ptype, file, env)
	if t == nil {
		return nil
	}
	if !typeIsExported(t, env) {
		env.Errorf(file, ptype.Pos(), "type must be transitively exported")
		return nil
	}
	return t
}

// typeCompiler holds the state to compile a single type.
type typeCompiler struct {
	file        *File
	env         *Env
	vtb         *vdl.TypeBuilder
	builders    map[string]*typeDefBuilder
	transExport bool
}

// compileDef compiles the ptype defined type.  It handles definitions based on
// array, enum, struct and union, as well as any literal type.
func (c typeCompiler) compileDef(ptype parse.Type) vdl.TypeOrPending {
	switch pt := ptype.(type) {
	case *parse.TypeArray:
		elem := c.compileLit(pt.Elem)
		if elem == nil {
			return nil
		}
		return c.vtb.Array().AssignLen(pt.Len).AssignElem(elem)
	case *parse.TypeEnum:
		enum := c.vtb.Enum()
		for _, plabel := range pt.Labels {
			enum.AppendLabel(plabel.Name)
		}
		return enum
	case *parse.TypeStruct:
		st := c.vtb.Struct()
		for _, pfield := range pt.Fields {
			ftype := c.compileLit(pfield.Type)
			if ftype == nil {
				return nil
			}
			st.AppendField(pfield.Name, ftype)
		}
		return st
	case *parse.TypeUnion:
		union := c.vtb.Union()
		for _, pfield := range pt.Fields {
			ftype := c.compileLit(pfield.Type)
			if ftype == nil {
				return nil
			}
			union.AppendField(pfield.Name, ftype)
		}
		return union
	}
	lit := c.compileLit(ptype)
	if _, ok := lit.(vdl.PendingOptional); ok {
		// Don't allow Optional at the top-level of a type definition.  The purpose
		// of this rule is twofold:
		// 1) Reduce confusion; the Optional modifier cannot be hidden in a type
		//    definition, it must be explicitly mentioned on each use.
		// 2) The Optional concept is typically translated to pointers in generated
		//    languages, and many languages don't support named pointer types.
		//
		//   type A string            // ok
		//   type B []?string         // ok
		//   type C struct{X ?string} // ok
		//   type D ?string           // bad
		//   type E ?struct{X string} // bad
		c.env.Errorf(c.file, ptype.Pos(), "can't define type based on top-level optional")
		return nil
	}
	return lit
}

// compileLit compiles the ptype literal type.  It handles any literal type.
// Note that array, enum, struct and union are required to be defined and named,
// and aren't allowed as regular literal types.
func (c typeCompiler) compileLit(ptype parse.Type) vdl.TypeOrPending { //nolint:gocyclo
	switch pt := ptype.(type) {
	case *parse.TypeNamed:
		// Try to resolve the named type from the already-compiled packages in env.
		if def, matched := c.env.ResolveType(pt.Name, c.file); def != nil {
			if len(matched) < len(pt.Name) {
				c.env.Errorf(c.file, pt.Pos(), "type %s invalid (%s unmatched)", pt.Name, pt.Name[len(matched):])
				return nil
			}
			return def.Type
		}
		// Try to resolve the named type from the builders in this local package.
		// This resolve types that haven't been described yet, to handle arbitrary
		// ordering of types within the package, as well as cyclic types.
		if b, ok := c.builders[pt.Name]; ok {
			// If transExport is set, ensure all subtypes are exported.  We only need
			// to check local names, since names resolved from the packages in env
			// could only be resolved if they were exported to begin with.
			if c.transExport && !b.def.Exported {
				c.env.Errorf(c.file, pt.Pos(), "type %s must be transitively exported", pt.Name)
				return nil
			}
			return b.pending
		}
		c.env.Errorf(c.file, pt.Pos(), "type %s undefined", pt.Name)
		return nil
	case *parse.TypeList:
		elem := c.compileLit(pt.Elem)
		if elem == nil {
			return nil
		}
		return c.vtb.List().AssignElem(elem)
	case *parse.TypeSet:
		key := c.compileLit(pt.Key)
		if key == nil {
			return nil
		}
		return c.vtb.Set().AssignKey(key)
	case *parse.TypeMap:
		key, elem := c.compileLit(pt.Key), c.compileLit(pt.Elem)
		if key == nil || elem == nil {
			return nil
		}
		return c.vtb.Map().AssignKey(key).AssignElem(elem)
	case *parse.TypeOptional:
		elem := c.compileLit(pt.Base)
		if elem == nil {
			return nil
		}
		return c.vtb.Optional().AssignElem(elem)
	default:
		c.env.Errorf(c.file, pt.Pos(), "unnamed %s type invalid (type must be defined)", ptype.Kind())
		return nil
	}
}

// Define types.  We sort by dependencies on other types in this package.  If
// there are cycles, the order is arbitrary; otherwise the types are ordered
// topologically.
//
// The order that we call Built on each pending type doesn't actually matter;
// the v.io/v23/vdl package deals with arbitrary orders, and supports recursive
// types.  However we want the order to be deterministic, otherwise the output
// will constantly change.  And it's nicer for some code generators to get types
// in dependency order when possible.
func (td typeDefiner) Define() {
	td.vtb.Build()
	// Sort all types.
	var sorter toposort.Sorter
	for _, pfile := range td.pfiles {
		for _, pdef := range pfile.TypeDefs {
			b := td.builders[pdef.Name]
			sorter.AddNode(b)
			for _, dep := range td.getLocalDeps(b.ptype) {
				sorter.AddEdge(b, dep)
			}
		}
	}
	sorted, _ := sorter.Sort()
	// Define all types.
	for _, ibuilder := range sorted {
		b := ibuilder.(*typeDefBuilder)
		def, file := b.def, b.def.File
		if b.base != nil {
			base, err := b.base.Built()
			if err != nil {
				td.env.prefixErrorf(file, b.ptype.Pos(), err, "%s base type invalid", def.Name)
				return
			}
			def.BaseType = base
		}
		t, err := b.pending.Built()
		if err != nil {
			td.env.prefixErrorf(file, def.Pos, err, "%s invalid", def.Name)
			return
		}
		def.Type = t
		addTypeDef(def, td.env)
	}
}

// addTypeDef updates our various structures to add a new type def.
func addTypeDef(def *TypeDef, env *Env) {
	def.File.TypeDefs = append(def.File.TypeDefs, def)
	def.File.Package.typeDefs = append(def.File.Package.typeDefs, def)
	def.File.Package.typeMap[def.Name] = def
	if env != nil {
		// env should only be nil during initialization of the built-in package;
		// NewEnv ensures new environments have the built-in types.
		env.typeMap[def.Type] = def
	}
}

// getLocalDeps returns a list of named type dependencies for ptype, where each
// dependency is defined in this package.
func (td typeDefiner) getLocalDeps(ptype parse.Type) (deps []*typeDefBuilder) {
	switch pt := ptype.(type) {
	case *parse.TypeNamed:
		// Named references to other types in this package are all we care about.
		if b := td.builders[pt.Name]; b != nil {
			deps = append(deps, b)
		}
	case *parse.TypeEnum:
		// No deps.
	case *parse.TypeArray:
		deps = append(deps, td.getLocalDeps(pt.Elem)...)
	case *parse.TypeList:
		deps = append(deps, td.getLocalDeps(pt.Elem)...)
	case *parse.TypeSet:
		deps = append(deps, td.getLocalDeps(pt.Key)...)
	case *parse.TypeMap:
		deps = append(deps, td.getLocalDeps(pt.Key)...)
		deps = append(deps, td.getLocalDeps(pt.Elem)...)
	case *parse.TypeStruct:
		for _, field := range pt.Fields {
			deps = append(deps, td.getLocalDeps(field.Type)...)
		}
	case *parse.TypeUnion:
		for _, field := range pt.Fields {
			deps = append(deps, td.getLocalDeps(field.Type)...)
		}
	case *parse.TypeOptional:
		deps = append(deps, td.getLocalDeps(pt.Base)...)
	default:
		panic(fmt.Errorf("vdl: unhandled parse.Type %T %#v", ptype, ptype))
	}
	return
}

// AttachDoc makes another pass to fill in doc and doc suffix slices for enums,
// structs and unions.  Typically these are initialized in makeTypeDefBuilder,
// based on the underlying parse data.  But type definitions based on other
// named types can't be updated until the base type is actually compiled.
//
// TODO(toddw): This doesn't actually attach comments from the base type, it
// just leaves everything empty.  This is fine for now, but we should revamp the
// vdl parsing / comment attaching strategy in the future.
func (td typeDefiner) AttachDoc() {
	for _, file := range td.pkg.Files {
		for _, def := range file.TypeDefs {
			switch t := def.Type; t.Kind() {
			case vdl.Enum:
				if len(def.LabelDoc) == 0 {
					def.LabelDoc = make([]string, t.NumEnumLabel())
				}
				if len(def.LabelDocSuffix) == 0 {
					def.LabelDocSuffix = make([]string, t.NumEnumLabel())
				}
			case vdl.Struct, vdl.Union:
				if len(def.FieldDoc) == 0 {
					def.FieldDoc = make([]string, t.NumField())
				}
				if len(def.FieldDocSuffix) == 0 {
					def.FieldDocSuffix = make([]string, t.NumField())
				}
			}
		}
	}
}

// typeIsExported returns true iff t and all subtypes of t are exported.
func typeIsExported(t *vdl.Type, env *Env) bool {
	// Stop at the first defined type that we encounter; if it is exported, we are
	// already guaranteed that all subtypes are also exported.
	//
	// Note that all types are composed of defined types; the built-in types
	// bootstrap the whole system.
	if def := env.FindTypeDef(t); def != nil {
		return def.Exported
	}
	// Check composite types recursively.
	switch t.Kind() {
	case vdl.Optional, vdl.Array, vdl.List:
		return typeIsExported(t.Elem(), env)
	case vdl.Set:
		return typeIsExported(t.Key(), env)
	case vdl.Map:
		return typeIsExported(t.Key(), env) && typeIsExported(t.Elem(), env)
	case vdl.Struct, vdl.Union:
		for ix := 0; ix < t.NumField(); ix++ {
			if !typeIsExported(t.Field(ix).Type, env) {
				return false
			}
		}
		return true
	}
	panic(fmt.Errorf("vdl: typeIsExported unhandled type %v", t))
}
