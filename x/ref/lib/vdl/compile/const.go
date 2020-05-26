// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"
	"math/big"

	"v.io/v23/vdl"
	"v.io/x/lib/toposort"
	"v.io/x/ref/lib/vdl/opconst"
	"v.io/x/ref/lib/vdl/parse"
)

var (
	// NilConst is defined by the compiler, as are TrueConst and FalseConst.
	NilConst   = vdl.ZeroValue(vdl.AnyType) // nil == any(nil)
	TrueConst  = vdl.BoolValue(nil, true)
	FalseConst = vdl.BoolValue(nil, false)
)

// ConstDef represents a user-defined named const definition in the compiled
// results.
type ConstDef struct {
	NamePos             // name, parse position and docs
	Exported bool       // is this const definition exported?
	Value    *vdl.Value // const value
	File     *File      // parent file that this const is defined in
}

func (x *ConstDef) String() string {
	y := *x
	y.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", y)
}

// compileConstDefs is the "entry point" to the rest of this file.  It takes the
// consts defined in pfiles and compiles them into ConstDefs in pkg.
func compileConstDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	cd := constDefiner{pkg, pfiles, env, make(map[string]*constDefBuilder)}
	if cd.Declare(); !env.Errors.IsEmpty() {
		return
	}
	cd.Define()
}

// constDefiner defines consts in a package.  This is split into two phases:
// 1) Declare ensures local const references can be resolved.
// 2) Define sorts in dependency order, and evaluates and defines each const.
//
// It holds a builders map from const name to constDefBuilder, where the
// constDefBuilder is responsible for compiling and defining a single const.
type constDefiner struct {
	pkg      *Package
	pfiles   []*parse.File
	env      *Env
	builders map[string]*constDefBuilder
}

type constDefBuilder struct {
	def   *ConstDef
	pexpr parse.ConstExpr
}

func printConstBuilderName(ibuilder interface{}) string {
	return ibuilder.(*constDefBuilder).def.Name
}

// Declare creates builders for each const defined in the package.
func (cd constDefiner) Declare() {
	for ix := range cd.pkg.Files {
		file, pfile := cd.pkg.Files[ix], cd.pfiles[ix]
		for _, pdef := range pfile.ConstDefs {
			export, err := validConstIdent(pdef.Name, reservedNormal)
			if err != nil {
				cd.env.prefixErrorf(file, pdef.Pos, err, "const %s invalid name", pdef.Name)
				continue // keep going to catch more errors
			}
			detail := identDetail("const", file, pdef.Pos)
			if err := file.DeclareIdent(pdef.Name, detail); err != nil {
				cd.env.prefixErrorf(file, pdef.Pos, err, "const %s name conflict", pdef.Name)
				continue
			}
			def := &ConstDef{NamePos: NamePos(pdef.NamePos), Exported: export, File: file}
			cd.builders[pdef.Name] = &constDefBuilder{def, pdef.Expr}
		}
	}
}

// Define consts.  We sort by dependencies on other named consts in this
// package.  We don't allow cycles.  The ordering is necessary to perform simple
// single-pass evaluation.
//
// The dependency analysis is performed on consts, not the files they occur in;
// consts in the same package may be defined in any file, even if they cause
// cyclic file dependencies.
func (cd constDefiner) Define() {
	// Populate sorter with dependency information.  The sorting ensures that the
	// list of const defs within each file is topologically sorted, and also
	// deterministic; other than dependencies, const defs are listed in the same
	// order they were defined in the parsed files.
	var sorter toposort.Sorter
	for _, pfile := range cd.pfiles {
		for _, pdef := range pfile.ConstDefs {
			b := cd.builders[pdef.Name]
			sorter.AddNode(b)
			for _, dep := range cd.getLocalDeps(b.pexpr) {
				sorter.AddEdge(b, dep)
			}
		}
	}
	// Sort and check for cycles.
	sorted, cycles := sorter.Sort()
	if len(cycles) > 0 {
		cycleStr := toposort.DumpCycles(cycles, printConstBuilderName)
		first := cycles[0][0].(*constDefBuilder)
		cd.env.Errorf(first.def.File, first.def.Pos, "package %v has cyclic consts: %v", cd.pkg.Name, cycleStr)
		return
	}
	// Define all consts.  Since we add the const defs as we go and evaluate in
	// topological order, dependencies are guaranteed to be resolvable when we get
	// around to evaluating the consts that depend on them.
	for _, ibuilder := range sorted {
		b := ibuilder.(*constDefBuilder)
		def, file := b.def, b.def.File
		def.Value = compileConst("const", nil, b.pexpr, file, cd.env)
		if def.Value == nil {
			continue
		}
		// If the const is exported, make sure that the value type is exported.  We
		// only care about the final value type; we can't perform this check while
		// we're compiling and evaluating the const, since it's fine for sub
		// expressions to use unexported types.
		if def.Exported && !typeIsExported(def.Value.Type(), cd.env) {
			cd.env.Errorf(file, def.Pos, "const %s must have a transitively exported type", def.Name)
		}
		addConstDef(def, cd.env)
	}
}

// addConstDef updates our various structures to add a new const def.
func addConstDef(def *ConstDef, env *Env) {
	def.File.ConstDefs = append(def.File.ConstDefs, def)
	def.File.Package.constDefs = append(def.File.Package.constDefs, def)
	def.File.Package.constMap[def.Name] = def
	if env != nil {
		// env should only be nil during initialization of the built-in package;
		// NewEnv ensures new environments have the built-in consts.
		env.constMap[def.Value] = def
	}
}

// getLocalDeps returns a list of named const dependencies for pexpr, where each
// dependency is defined in this package.
func (cd constDefiner) getLocalDeps(pexpr parse.ConstExpr) (deps []*constDefBuilder) {
	switch pe := pexpr.(type) {
	case nil, *parse.ConstLit, *parse.ConstTypeObject:
		// These have no deps.
	case *parse.ConstNamed:
		// Named references to other consts in this package are all we care about.
		if b := cd.builders[pe.Name]; b != nil {
			deps = append(deps, b)
		}
	case *parse.ConstCompositeLit:
		for _, kv := range pe.KVList {
			deps = append(deps, cd.getLocalDeps(kv.Key)...)
			deps = append(deps, cd.getLocalDeps(kv.Value)...)
		}
	case *parse.ConstIndexed:
		deps = append(deps, cd.getLocalDeps(pe.Expr)...)
		deps = append(deps, cd.getLocalDeps(pe.IndexExpr)...)
	case *parse.ConstTypeConv:
		deps = append(deps, cd.getLocalDeps(pe.Expr)...)
	case *parse.ConstUnaryOp:
		deps = append(deps, cd.getLocalDeps(pe.Expr)...)
	case *parse.ConstBinaryOp:
		deps = append(deps, cd.getLocalDeps(pe.Lexpr)...)
		deps = append(deps, cd.getLocalDeps(pe.Rexpr)...)
	default:
		panic(fmt.Errorf("vdl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
	}
	return
}

// compileConst compiles pexpr into a *vdl.Value.  All named types and consts
// referenced by pexpr must already be defined.
//
// The implicit type is applied to pexpr; untyped consts and composite literals
// with no explicit type assume the implicit type.  Errors are reported if the
// implicit type isn't assignable from the final value.  If the implicit type is
// nil, the exported config const must be explicitly typed.
func compileConst(what string, implicit *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	c := evalConstExpr(implicit, pexpr, file, env)
	if !c.IsValid() {
		return nil
	}
	if implicit != nil &&
		(c.Type() == nil ||
			(implicit.CanBeOptional() && vdl.OptionalType(implicit) == c.Type()) ||
			(c.Type().CanBeOptional() && vdl.OptionalType(c.Type()) == implicit)) {
		// Convert untyped const into the implicit type.
		conv, err := c.Convert(implicit)
		if err != nil {
			env.prefixErrorf(file, pexpr.Pos(), err, "invalid %s", what)
			return nil
		}
		c = conv
	}
	v, err := c.ToValue()
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "invalid %s", what)
		return nil
	}
	if implicit != nil && !implicit.AssignableFrom(v) {
		env.Errorf(file, pexpr.Pos(), "invalid %v (%v not assignable from %v)", what, implicit, v)
		return nil
	}
	return v
}

// compileConstExplicit is similar to compileConst, but instead of an optional
// implicit type, requires a non-nil explicit type.  The compiled const is
// explicitly converted to the explicit type.
func compileConstExplicit(what string, explicit *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	c := evalConstExpr(explicit, pexpr, file, env)
	if !c.IsValid() {
		return nil
	}
	conv, err := c.Convert(explicit)
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "invalid %v", what)
		return nil
	}
	v, err := conv.ToValue()
	if err != nil {
		env.prefixErrorf(file, pexpr.Pos(), err, "invalid %s", what)
		return nil
	}
	return v
}

// evalConstExpr returns the result of evaluating pexpr into a opconst.Const.
// If implicit is non-nil, we apply it to pexpr if it doesn't have an explicit
// type specified.  E.g. composite literals and enum labels with no explicit
// type assume the implicit type.
func evalConstExpr(implicit *vdl.Type, pexpr parse.ConstExpr, file *File, env *Env) opconst.Const { //nolint:gocyclo
	switch pe := pexpr.(type) {
	case *parse.ConstLit:
		// All literal constants start out untyped.
		switch tlit := pe.Lit.(type) {
		case string:
			return opconst.String(tlit)
		case *big.Int:
			return opconst.Integer(tlit)
		case *big.Rat:
			return opconst.Rational(tlit)
		default:
			panic(fmt.Errorf("vdl: unhandled parse.ConstLit %T %#v", tlit, tlit))
		}
	case *parse.ConstCompositeLit:
		t := implicit
		if pe.Type != nil {
			// If an explicit type is specified for the composite literal, it
			// overrides the implicit type.
			t = compileType(pe.Type, file, env)
			if t == nil {
				break
			}
		}
		v := evalCompLit(t, pe, file, env)
		if v == nil {
			break
		}
		return opconst.FromValue(v)
	case *parse.ConstNamed:
		c, err := env.EvalConst(pe.Name, file)
		if err != nil {
			if implicit != nil {
				// Try applying the name as a selector against the implicit type.  This
				// allows a shortened form for enum labels, without redundantly
				// specifying the enum type.
				if c, err2 := env.evalSelectorOnType(implicit, pe.Name); err2 == nil {
					return c
				}
			}
			env.prefixErrorf(file, pe.Pos(), err, "const %s invalid", pe.Name)
			break
		}
		return c
	case *parse.ConstIndexed:
		value := compileConst("const", nil, pe.Expr, file, env)
		if value == nil {
			break
		}
		// TODO(bprosnitz) Should indexing on set also be supported?
		switch value.Kind() {
		case vdl.Array, vdl.List:
			v := evalListIndex(value, pe.IndexExpr, file, env)
			if v != nil {
				return opconst.FromValue(v)
			}
		case vdl.Map:
			v := evalMapIndex(value, pe.IndexExpr, file, env)
			if v != nil {
				return opconst.FromValue(v)
			}
		default:
			env.Errorf(file, pe.Pos(), "illegal use of index operator with unsupported type")
		}
	case *parse.ConstTypeConv:
		t := compileType(pe.Type, file, env)
		x := evalConstExpr(nil, pe.Expr, file, env)
		if t == nil || !x.IsValid() {
			break
		}
		res, err := x.Convert(t)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "invalid type conversion")
			break
		}
		return res
	case *parse.ConstTypeObject:
		t := compileType(pe.Type, file, env)
		if t == nil {
			break
		}
		return opconst.FromValue(vdl.TypeObjectValue(t))
	case *parse.ConstUnaryOp:
		x := evalConstExpr(nil, pe.Expr, file, env)
		op := opconst.ToUnaryOp(pe.Op)
		if op == opconst.InvalidUnaryOp {
			env.Errorf(file, pe.Pos(), "unary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() {
			break
		}
		res, err := opconst.EvalUnary(op, x)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "unary %s invalid", pe.Op)
			break
		}
		return res
	case *parse.ConstBinaryOp:
		x := evalConstExpr(nil, pe.Lexpr, file, env)
		y := evalConstExpr(nil, pe.Rexpr, file, env)
		op := opconst.ToBinaryOp(pe.Op)
		if op == opconst.InvalidBinaryOp {
			env.Errorf(file, pe.Pos(), "binary %s undefined", pe.Op)
			break
		}
		if !x.IsValid() || !y.IsValid() {
			break
		}
		res, err := opconst.EvalBinary(op, x, y)
		if err != nil {
			env.prefixErrorf(file, pe.Pos(), err, "binary %s invalid", pe.Op)
			break
		}
		return res
	default:
		panic(fmt.Errorf("vdl: unhandled parse.ConstExpr %T %#v", pexpr, pexpr))
	}
	return opconst.Const{}
}

// evalListIndex evalutes base[index], where base is a list or array.
func evalListIndex(base *vdl.Value, indexExpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	index := compileConstExplicit(base.Kind().String()+" index", vdl.Uint64Type, indexExpr, file, env)
	if index == nil {
		return nil
	}
	ix := int(index.Uint())
	if ix >= base.Len() {
		env.Errorf(file, indexExpr.Pos(), "index %d out of range", ix)
		return nil
	}
	return base.Index(ix)
}

// evalMapIndex evaluates base[index], where base is a map.
func evalMapIndex(base *vdl.Value, indexExpr parse.ConstExpr, file *File, env *Env) *vdl.Value {
	key := compileConst("map key", base.Type().Key(), indexExpr, file, env)
	if key == nil {
		return nil
	}
	item := base.MapIndex(key)
	if item == nil {
		// Unlike normal go code, it is probably undesirable to return the zero
		// value here.  It is very likely this is an error.
		env.Errorf(file, indexExpr.Pos(), "map key %v not found in map", key)
		return nil
	}
	return item
}

// evalCompLit evaluates a composite literal, returning it as a vdl.Value.  The
// type t is required, but note that subtypes enclosed in a composite type can
// always use the implicit type from the parent composite type.
func evalCompLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	if t == nil {
		env.Errorf(file, lit.Pos(), "missing type for composite literal")
		return nil
	}
	isOptional := false
	if t.Kind() == vdl.Optional {
		isOptional = true
		t = t.Elem()
	}
	var v *vdl.Value
	switch t.Kind() {
	case vdl.Array, vdl.List:
		v = evalListLit(t, lit, file, env)
	case vdl.Set:
		v = evalSetLit(t, lit, file, env)
	case vdl.Map:
		v = evalMapLit(t, lit, file, env)
	case vdl.Struct:
		v = evalStructLit(t, lit, file, env)
	case vdl.Union:
		v = evalUnionLit(t, lit, file, env)
	default:
		env.Errorf(file, lit.Pos(), "%v invalid type for composite literal", t)
		return nil
	}
	if v != nil && isOptional {
		v = vdl.OptionalValue(v)
	}
	return v
}

func evalListLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	listv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v %s literal", t, t.Kind())
	var index int
	assigned := make(map[int]bool)
	for _, kv := range lit.KVList {
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing value in %s", desc)
			return nil
		}
		// Set the index to the key, if it exists.  Semantics are looser than
		// values; we allow any key that's convertible to uint64, even if the key is
		// already typed.
		if kv.Key != nil {
			key := compileConstExplicit("list index", vdl.Uint64Type, kv.Key, file, env)
			if key == nil {
				return nil
			}
			index = int(key.Uint())
		}
		// Make sure the index hasn't been assigned already, and adjust the list
		// length as necessary.
		if assigned[index] {
			env.Errorf(file, kv.Value.Pos(), "duplicate index %d in %s", index, desc)
			return nil
		}
		assigned[index] = true
		if index >= listv.Len() {
			if t.Kind() == vdl.Array {
				env.Errorf(file, kv.Value.Pos(), "index %d out of range in %s", index, desc)
				return nil
			}
			listv.AssignLen(index + 1)
		}
		// Evaluate the value and perform the assignment.
		value := compileConst(t.Kind().String()+" value", t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		listv.Index(index).Assign(value)
		index++
	}
	return listv
}

func evalSetLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	setv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v set literal", t)
	for _, kv := range lit.KVList {
		if kv.Key != nil {
			env.Errorf(file, kv.Key.Pos(), "invalid index in %s", desc)
			return nil
		}
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing key in %s", desc)
			return nil
		}
		// Evaluate the key and make sure it hasn't been assigned already.
		key := compileConst("set key", t.Key(), kv.Value, file, env)
		if key == nil {
			return nil
		}
		if setv.ContainsKey(key) {
			env.Errorf(file, kv.Value.Pos(), "duplicate key %v in %s", key, desc)
			return nil
		}
		setv.AssignSetKey(key)
	}
	return setv
}

func evalMapLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	mapv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v map literal", t)
	for _, kv := range lit.KVList {
		if kv.Key == nil {
			env.Errorf(file, lit.Pos(), "missing key in %s", desc)
			return nil
		}
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing elem in %s", desc)
			return nil
		}
		// Evaluate the key and make sure it hasn't been assigned already.
		key := compileConst("map key", t.Key(), kv.Key, file, env)
		if key == nil {
			return nil
		}
		if mapv.ContainsKey(key) {
			env.Errorf(file, kv.Key.Pos(), "duplicate key %v in %s", key, desc)
			return nil
		}
		// Evaluate the value and perform the assignment.
		value := compileConst("map value", t.Elem(), kv.Value, file, env)
		if value == nil {
			return nil
		}
		mapv.AssignMapIndex(key, value)
	}
	return mapv
}

func evalStructLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	// We require that either all items have keys, or none of them do.
	structv := vdl.ZeroValue(t)
	desc := fmt.Sprintf("%v struct literal", t)
	haskeys := len(lit.KVList) > 0 && lit.KVList[0].Key != nil
	assigned := make(map[int]bool)
	for index, kv := range lit.KVList {
		if kv.Value == nil {
			env.Errorf(file, lit.Pos(), "missing field value in %s", desc)
			return nil
		}
		if haskeys != (kv.Key != nil) {
			env.Errorf(file, kv.Value.Pos(), "mixed key:value and value in %s", desc)
			return nil
		}
		// Get the field description, either from the key or the index.
		var field vdl.Field
		if kv.Key != nil {
			// There is an explicit field name specified.
			fname, ok := kv.Key.(*parse.ConstNamed)
			if !ok {
				env.Errorf(file, kv.Key.Pos(), "invalid field name %q in %s", kv.Key.String(), desc)
				return nil
			}
			field, index = t.FieldByName(fname.Name)
			if index < 0 {
				env.Errorf(file, kv.Key.Pos(), "unknown field %q in %s", fname.Name, desc)
				return nil
			}
		} else {
			// No field names, just use the index position.
			if index >= t.NumField() {
				env.Errorf(file, kv.Value.Pos(), "too many fields in %s", desc)
				return nil
			}
			field = t.Field(index)
		}
		// Make sure the field hasn't been assigned already.
		if assigned[index] {
			env.Errorf(file, kv.Value.Pos(), "duplicate field %q in %s", field.Name, desc)
			return nil
		}
		assigned[index] = true
		// Evaluate the value and perform the assignment.
		value := compileConst("struct field", field.Type, kv.Value, file, env)
		if value == nil {
			return nil
		}
		structv.StructField(index).Assign(value)
	}
	if !haskeys && 0 < len(assigned) && len(assigned) < t.NumField() {
		env.Errorf(file, lit.Pos(), "too few fields in %s", desc)
		return nil
	}
	return structv
}

func evalUnionLit(t *vdl.Type, lit *parse.ConstCompositeLit, file *File, env *Env) *vdl.Value {
	// We require either an empty kv list, or exactly one kv with an explicit key.
	unionv := vdl.ZeroValue(t)
	if len(lit.KVList) == 0 {
		return unionv
	}
	desc := fmt.Sprintf("%v union literal", t)
	if len(lit.KVList) != 1 {
		env.Errorf(file, lit.Pos(), "invalid %s (must have a single entry)", desc)
		return nil
	}
	kv := lit.KVList[0]
	if kv.Key == nil || kv.Value == nil {
		env.Errorf(file, lit.Pos(), "invalid %s (must have explicit key and value)", desc)
		return nil
	}
	// Get the field description.
	fname, ok := kv.Key.(*parse.ConstNamed)
	if !ok {
		env.Errorf(file, kv.Key.Pos(), "invalid field name %q in %s", kv.Key.String(), desc)
		return nil
	}
	field, index := t.FieldByName(fname.Name)
	if index < 0 {
		env.Errorf(file, kv.Key.Pos(), "unknown field %q in %s", fname.Name, desc)
		return nil
	}
	// Evaluate the value and perform the assignment.
	value := compileConst("union field", field.Type, kv.Value, file, env)
	if value == nil {
		return nil
	}
	unionv.AssignField(index, value)
	return unionv
}
