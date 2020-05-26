// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golang

import (
	"fmt"
	"path"
	"strings"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
)

// defineIsZero returns the VDLIsZero method for the def type.
func defineIsZero(data *goData, def *compile.TypeDef) string {
	g := genIsZero{goData: data}
	return g.Gen(def)
}

type genIsZero struct {
	*goData
}

func (g *genIsZero) Gen(def *compile.TypeDef) string {
	// Special-case array and struct to check each elem or field for zero.
	// Special-case union to generate methods for each concrete union struct.
	//
	// Types that have a unique Go zero value can bypass these special-cases, and
	// perform a direct comparison against that zero value.  Note that def.Type
	// always represents a wire type here, since we're generating the VDLIsZero
	// method on the wire type.
	if !isGoZeroValueUniqueWire(g.goData, def.Type) {
		switch def.Type.Kind() {
		case vdl.Array:
			return g.genArrayDef(def)
		case vdl.Struct:
			return g.genStructDef(def)
		case vdl.Union:
			return g.genUnionDef(def)
		}
	}
	return g.genDef(def)
}

func (g *genIsZero) genDef(def *compile.TypeDef) string {
	tt, arg := def.Type, namedArg{"x", false}
	expr := g.ExprWire(returnEqZero, tt, arg, "")
	return fmt.Sprintf(`
func (x %[1]s) VDLIsZero() bool { //nolint:gocyclo
	return %[2]s
}
`, def.Name, expr)
}

func (g *genIsZero) genArrayDef(def *compile.TypeDef) string {
	tt := def.Type
	elemArg := typedArg("elem", tt.Elem())
	expr := g.Expr(ifNeZero, tt.Elem(), elemArg, "")
	return fmt.Sprintf(`
func (x %[1]s) VDLIsZero() bool { //nolint:gocyclo
	for _, elem := range x {
		if %[2]s {
			return false
		}
	}
	return true
}
`, def.Name, expr)
}

func (g *genIsZero) genStructDef(def *compile.TypeDef) string {
	tt, arg := def.Type, namedArg{"x", false}
	s := fmt.Sprintf(`
func (x %[1]s) VDLIsZero() bool { //nolint:gocyclo`, def.Name)
	if tt.NumField() == 1 {
		field := tt.Field(0)
		expr := g.Expr(returnEqZero, field.Type, arg.Field(field), field.Name)
		s += fmt.Sprintf(`
	return %[1]s
}
`, expr)
		return s
	}
	for ix := 0; ix < tt.NumField(); ix++ {
		field := tt.Field(ix)
		expr := g.Expr(ifNeZero, field.Type, arg.Field(field), field.Name)
		s += fmt.Sprintf(`
	if %[1]s {
		return false
	}`, expr)
	}
	s += `
	return true
}
`
	return s
}

func (g *genIsZero) genUnionDef(def *compile.TypeDef) string {
	// The 0th field needs a real zero check.
	tt := def.Type
	field0 := tt.Field(0)
	fieldArg := typedArg("x.Value", field0.Type)
	expr := g.Expr(returnEqZero, field0.Type, fieldArg, "")
	s := fmt.Sprintf(`
func (x %[1]s%[2]s) VDLIsZero() bool { //nolint:gocyclo
	return %[3]s
}
`, def.Name, field0.Name, expr)
	// All other fields simply return false.
	for ix := 1; ix < tt.NumField(); ix++ {
		s += fmt.Sprintf(`
func (x %[1]s%[2]s) VDLIsZero() bool {
	return false
}
`, def.Name, tt.Field(ix).Name)
	}
	return s
}

// zeroExpr configures what type of zero expression to generate.
type zeroExpr int

const (
	ifEqZero     = iota // Generate expression for "if x == 0 {" statement
	ifNeZero            // Generate expression for "if x != 0 {" statement
	returnEqZero        // Generate expression for "return x == 0" statement
)

func (ze zeroExpr) GenEqual() bool {
	return ze == ifEqZero || ze == returnEqZero
}
func (ze zeroExpr) GenNotEqual() bool {
	return !ze.GenEqual()
}
func (ze zeroExpr) GenIfStmt() bool {
	return ze == ifEqZero || ze == ifNeZero
}
func (ze zeroExpr) GenReturnStmt() bool {
	return !ze.GenIfStmt()
}

// Expr generates the Go code to check whether the arg, which has type tt, is
// equal or not-equal to zero.  The tmp string is appended to temporary variable
// names to make them unique.
//
// The returned expression is a boolean Go expression that evaluates whether arg
// is zero or non-zero.  It is meant to be used like this:
//     if <expr> {
//       ...
//     }
// Or like this:
//     return <expr>
//
// The first argument ze describes whether the expression will be used in an
// "if" or "return" statement, and whether it should evaluate to equal or
// not-equal to zero.  The kind of statement affects the expression because of
// Go's parsing and type safety rules.
func (g *genIsZero) Expr(ze zeroExpr, tt *vdl.Type, arg namedArg, tmp string) string {
	if native, wirePkg, ok := findNativeType(g.Env, tt); ok {
		opNot, eq, ref := "", "==", arg.Ref()
		if ze.GenNotEqual() {
			opNot, eq = "!", "!="
		}
		switch {
		case native.Kind == vdltool.GoKindBool:
			return ref
		case native.Zero.Mode == vdltool.GoZeroModeUnique:
			// We use an untyped const as the zero value, because Go only allows
			// comparison of slices with nil.  E.g.
			//   type MySlice []string
			//   pass := MySlice(nil) == nil          // valid
			//   fail := MySlice(nil) == MySlice(nil) // invalid
			nType := nativeType(g.goData, native, wirePkg)
			zeroValue := untypedConstNativeZero(native.Kind, nType)
			if ze.GenIfStmt() {
				if k := native.Kind; k == vdltool.GoKindStruct || k == vdltool.GoKindArray {
					// Without a special-case, we'll get a statement like:
					//   if x == Foo{} {
					// But that isn't valid Go code, so we change it to:
					//   if x == (Foo{}) {
					zeroValue = "(" + zeroValue + ")"
				}
			}
			return ref + eq + zeroValue
		case strings.HasPrefix(native.Zero.IsZero, "."):
			return opNot + arg.Name + native.Zero.IsZero
		case native.Zero.IsZero != "":
			// TODO(toddw): Handle the function form of IsZero, including IsZeroImports.
			vdlconfig := path.Join(wirePkg.GenPath, "vdl.config")
			g.Env.Errors.Errorf("%s: native type %s uses function form of IsZero, which isn't implemented", vdlconfig, native.Type)
			return ""
		}
	}
	return g.ExprWire(ze, tt, arg, tmp)
}

// ExprWire is like Expr, but generates code for the wire type tt.
func (g *genIsZero) ExprWire(ze zeroExpr, tt *vdl.Type, arg namedArg, tmp string) string { //nolint:gocyclo
	opNot, eq, cond, ref := "", "==", "||", arg.Ref()
	if ze.GenNotEqual() {
		opNot, eq, cond = "!", "!=", "&&"
	}
	// Handle everything other than Array and Struct.
	switch tt.Kind() {
	case vdl.Bool:
		expr := "!" + ref // false is zero, while true is non-zero
		if ze.GenNotEqual() {
			expr = ref
		}
		if ze.GenReturnStmt() && tt.Name() != "" {
			// Special-case named bool types, since we'll get an expression like "x",
			// but we need an explicit conversion since the type of x is the named
			// bool type, not the built-in bool type.
			expr = "bool(" + expr + ")"
		}
		return expr
	case vdl.String:
		return ref + eq + `""`
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		return ref + eq + "0"
	case vdl.Enum:
		return ref + eq + typeGoWire(g.goData, tt) + tt.EnumLabel(0)
	case vdl.TypeObject:
		return ref + eq + "nil" + cond + ref + eq + g.Pkg("v.io/v23/vdl") + "AnyType"
	case vdl.List, vdl.Set, vdl.Map:
		return "len(" + ref + ")" + eq + "0"
	case vdl.Optional:
		return arg.Name + eq + "nil"
	}
	// The interface{} representation of any is special-cased, since it behaves
	// differently than the *vdl.Value and *vom.RawBytes representations.
	if tt.Kind() == vdl.Any && goAnyRepMode(g.Package) == goAnyRepInterface {
		return ref + eq + "nil"
	}
	switch tt.Kind() {
	case vdl.Union, vdl.Any:
		// Union is always named, and Any is either *vdl.Value or *vom.RawBytes, so
		// we call VDLIsZero directly.  A slight complication is the fact that all
		// of these might be nil, which we need to protect against before making the
		// VDLIsZero call.
		return ref + eq + "nil" + cond + opNot + arg.Name + ".VDLIsZero()"
	}
	// Only Array and Struct are left.
	//
	// If there is a unique Go zero value, we generate a fastpath that simply
	// compares against that value.  Note that tt always represents a wire type
	// here, since native types were handled in Expr.
	if isGoZeroValueUniqueWire(g.goData, tt) {
		zeroValue := typedConstWire(g.goData, vdl.ZeroValue(tt))
		if ze.GenIfStmt() {
			// Without a special-case, we'll get a statement like:
			//   if x == Foo{} {
			// But that isn't valid Go code, so we change it to:
			//   if x == (Foo{}) {
			zeroValue = "(" + zeroValue + ")"
		}
		return ref + eq + zeroValue
	}
	// Otherwise we call VDLIsZero directly.  This takes advantage of the fact
	// that Array and Struct are always named, so will always have a VDLIsZero
	// method defined.
	return opNot + arg.Name + ".VDLIsZero()"
}

// isGoZeroValueUniqueWire returns true iff the Go zero value of the wire type
// tt represents the VDL zero value, and is the *only* value that represents the
// VDL zero value.
func isGoZeroValueUniqueWire(data *goData, tt *vdl.Type) bool {
	// Not unique if tt contains inline native subtypes that don't have a unique
	// zero representation.  This doesn't apply if tt itself is native, but has no
	// inline native subtypes, since this function only considers the wire type
	// form of tt.
	if containsInlineNativeNonUniqueSubTypes(data, tt, true) {
		return false
	}
	// Not unique if tt contains types where there is more than one VDL zero value
	// representation:
	//   Any:            nil, or VDLIsZero on vdl.Value/vom.RawBytes
	//   TypeObject:     nil, or AnyType
	//   Union:          nil, or zero value of field 0
	//   List, Set, Map: nil, or empty
	//
	// Note that the interface{} representation of Any uses nil as the only VDL
	// zero value, while the vdl.Value/vom.RawBytes pointers can either be nil, or
	// represent VDL zero through their non-nil pointer.
	kkNotUnique := []vdl.Kind{vdl.TypeObject, vdl.Union, vdl.List, vdl.Set, vdl.Map}
	if goAnyRepMode(data.Package) != goAnyRepInterface {
		kkNotUnique = append(kkNotUnique, vdl.Any)
	}
	if tt.ContainsKind(vdl.WalkInline, kkNotUnique...) {
		return false
	}
	return true
}

// isGoZeroValueCanonical returns true iff the Go zero value of the type tt
// (which may be a native type) is the canonical representation of the VDL zero
// value.  This differs from isGoZeroValueUniqueWire since e.g. the canonical
// zero value of list is nil, which is the Go zero value, but it isn't unique
// since it isn't the only zero value representation.  Also this checks if tt is
// a native type, while isGoZeroValueUniqueWire assumes tt is a wire type.
func isGoZeroValueCanonical(data *goData, tt *vdl.Type) bool {
	// If tt is a native type in either Canonical or Unique zero mode, the Go zero
	// value is canonical.  Note that Unique zero mode is stronger than Canonical;
	// not only is the Go zero value canonical, it's the only representation.
	if native, _, ok := findNativeType(data.Env, tt); ok {
		if native.Zero.Mode != vdltool.GoZeroModeUnknown {
			return true
		}
	}
	// Not canonical if tt contains inline native subtypes that don't have a
	// Canonical or Unique zero mode.
	if containsInlineNativeUnknownSubTypes(data, tt, false) {
		return false
	}
	// Not canonical if tt contains types where the Go zero value isn't the
	// canonical VDL zero value.
	//
	// Note that The interface{} representation of Any uses nil as the only VDL
	// zero value, while the vdl.Value/vom.RawBytes pointers use a non-nil pointer
	// as the canonical VDL zero value.
	kkNotCanonical := []vdl.Kind{vdl.TypeObject, vdl.Union}
	if goAnyRepMode(data.Package) != goAnyRepInterface {
		kkNotCanonical = append(kkNotCanonical, vdl.Any)
	}
	if tt.ContainsKind(vdl.WalkInline, kkNotCanonical...) {
		return false
	}
	return true
}

func containsInlineNativeNonUniqueSubTypes(data *goData, tt *vdl.Type, wireOnly bool) bool {
	// The walk early-exits if the visitor functor returns false.  We want the
	// early-exit when we detect the first native type, so we use false to mean
	// that we've seen a native type, and true if we haven't.
	return !tt.Walk(vdl.WalkInline, func(visit *vdl.Type) bool {
		if wireOnly && visit == tt {
			// We don't want the native check to fire for tt itself, so we return true
			// when we visit tt, meaning we haven't detected a native type yet.
			return true
		}
		if native, _, ok := findNativeType(data.Env, visit); ok {
			return native.Zero.Mode == vdltool.GoZeroModeUnique
		}
		return true
	})
}

func containsInlineNativeUnknownSubTypes(data *goData, tt *vdl.Type, wireOnly bool) bool {
	// The walk early-exits if the visitor functor returns false.  We want the
	// early-exit when we detect the first native type, so we use false to mean
	// that we've seen a native type, and true if we haven't.
	return !tt.Walk(vdl.WalkInline, func(visit *vdl.Type) bool {
		if wireOnly && visit == tt {
			// We don't want the native check to fire for tt itself, so we return true
			// when we visit tt, meaning we haven't detected a native type yet.
			return true
		}
		if native, _, ok := findNativeType(data.Env, visit); ok {
			return native.Zero.Mode != vdltool.GoZeroModeUnknown
		}
		return true
	})
}

func findNativeType(env *compile.Env, tt *vdl.Type) (vdltool.GoType, *compile.Package, bool) {
	if def := env.FindTypeDef(tt); def != nil {
		pkg := def.File.Package
		key := def.Name
		if tt.Kind() == vdl.Optional {
			key = "*" + key
		}
		native, ok := pkg.Config.Go.WireToNativeTypes[key]
		return native, pkg, ok
	}
	return vdltool.GoType{}, nil, false
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
