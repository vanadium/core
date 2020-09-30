// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/opconst"
	"v.io/x/ref/lib/vdl/parse"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// Env is the environment for compilation.  It contains all errors that were
// collected during the execution - you can pass Errors to the parse phase to
// collect all errors together.  As packages are compiled it also collects the
// output; after a sequence of dependent packages is compiled, all compiled
// output will be collected.
//
// Always create a new Env via NewEnv; the zero Env is invalid.
type Env struct {
	Errors   *vdlutil.Errors
	Warnings *vdlutil.Errors
	pkgs     map[string]*Package
	typeMap  map[*vdl.Type]*TypeDef
	constMap map[*vdl.Value]*ConstDef

	disallowPathQualifiers bool // Disallow syntax like "a/b/c".Type
	noI18nErrorSupport     bool
}

// NewEnv creates a new Env, allowing up to maxErrors errors before we stop.
func NewEnv(maxErrors int) *Env {
	return NewEnvWithErrors(
		vdlutil.NewErrors(maxErrors),
		vdlutil.NewErrors(maxErrors),
	)
}

// NewEnvWithErrors creates a new Env, using the given errs to collect errors.
func NewEnvWithErrors(errs, warnings *vdlutil.Errors) *Env {
	env := &Env{
		Errors:   errs,
		Warnings: warnings,
		pkgs:     make(map[string]*Package),
		typeMap:  make(map[*vdl.Type]*TypeDef),
		constMap: make(map[*vdl.Value]*ConstDef),
	}
	// The env always starts out with the built-in package.
	env.pkgs[BuiltInPackage.Name] = BuiltInPackage
	for _, def := range BuiltInFile.TypeDefs {
		env.typeMap[def.Type] = def
	}
	for _, def := range BuiltInFile.ConstDefs {
		env.constMap[def.Value] = def
	}
	return env
}

// FindTypeDef returns the type definition corresponding to t, or nil if t isn't
// a defined type.  All built-in and user-defined named types are considered
// defined; e.g. unnamed lists don't have a corresponding type def.
func (e *Env) FindTypeDef(t *vdl.Type) *TypeDef { return e.typeMap[t] }

// FindConstDef returns the const definition corresponding to v, or nil if v
// isn't a defined const.  All user-defined named consts are considered defined;
// e.g. method tags don't have a corresponding const def.
func (e *Env) FindConstDef(v *vdl.Value) *ConstDef { return e.constMap[v] }

// ResolvePackage resolves a package path to its previous compiled results.
func (e *Env) ResolvePackage(path string) *Package {
	return e.pkgs[path]
}

// ResolvePackageGenPath resolves a package gen path to its previous compiled
// results.
func (e *Env) ResolvePackageGenPath(genPath string) *Package {
	for _, pkg := range e.pkgs {
		if pkg.GenPath == genPath {
			return pkg
		}
	}
	return nil
}

// Resolves a name against the current package and imported package namespace.
func (e *Env) resolve(name string, file *File) (val interface{}, matched string) {
	// First handle package-path qualified identifiers, which look like this:
	//   "a/b/c".Ident   (qualified with package path "a/b/c")
	// These must be handled first, since the package-path may include dots.
	if strings.HasPrefix(name, `"`) {
		if parts := strings.SplitN(name[1:], `".`, 2); len(parts) == 2 {
			path, remain := parts[0], parts[1]
			if e.disallowPathQualifiers {
				// TODO(toddw): Add real position.
				e.Errorf(file, parse.Pos{}, "package path qualified identifier %s not allowed", name)
			}
			if file.ValidateImportPackagePath(path) {
				if pkg := e.ResolvePackage(path); pkg != nil {
					if dotParts := strings.Split(remain, "."); len(dotParts) > 0 {
						if val := pkg.resolve(dotParts[0], false); val != nil {
							return val, `"` + path + `".` + dotParts[0]
						}
					}
				}
			}
		}
	}
	// Now handle built-in and package-local identifiers.  Examples:
	//   string
	//   TypeName
	//   EnumType.Label
	//   ConstName
	//   StructConst.Field
	//   InterfaceName
	nameParts := strings.Split(name, ".")
	if len(nameParts) == 0 {
		return nil, ""
	}
	if builtin := BuiltInPackage.resolve(nameParts[0], false); builtin != nil {
		return builtin, nameParts[0]
	}
	if local := file.Package.resolve(nameParts[0], true); local != nil {
		return local, nameParts[0]
	}
	// Now handle package qualified identifiers, which look like this:
	//   pkg.Ident   (qualified with local package identifier pkg)
	if len(nameParts) > 1 {
		if path := file.LookupImportPath(nameParts[0]); path != "" {
			if pkg := e.ResolvePackage(path); pkg != nil {
				if val := pkg.resolve(nameParts[1], false); val != nil {
					return val, nameParts[0] + "." + nameParts[1]
				}
			}
		}
	}
	// No match found.
	return nil, ""
}

// ResolveType resolves a name to a type definition.
// Returns the type def and the portion of name that was matched.
func (e *Env) ResolveType(name string, file *File) (td *TypeDef, matched string) {
	v, matched := e.resolve(name, file)
	td, _ = v.(*TypeDef)
	if td == nil {
		return nil, ""
	}
	return td, matched
}

// ResolveConst resolves a name to a const definition.
// Returns the const def and the portion of name that was matched.
func (e *Env) ResolveConst(name string, file *File) (cd *ConstDef, matched string) {
	v, matched := e.resolve(name, file)
	cd, _ = v.(*ConstDef)
	if cd == nil {
		return nil, ""
	}
	return cd, matched
}

// ResolveInterface resolves a name to an interface definition.
// Returns the interface and the portion of name that was matched.
func (e *Env) ResolveInterface(name string, file *File) (i *Interface, matched string) {
	v, matched := e.resolve(name, file)
	i, _ = v.(*Interface)
	if i == nil {
		return nil, ""
	}
	return i, matched
}

// evalSelectorOnValue evaluates the selector on v.
func (e *Env) evalSelectorOnValue(v *vdl.Value, selector string) (opconst.Const, error) {
	for _, fieldName := range strings.Split(selector, ".") {
		if v.Kind() != vdl.Struct {
			return opconst.Const{}, fmt.Errorf("invalid selector on const of kind: %v", v.Type().Kind())
		}
		next := v.StructFieldByName(fieldName)
		if next == nil {
			return opconst.Const{}, fmt.Errorf("invalid field name on struct %s: %s", v, fieldName)
		}
		v = next
	}
	return opconst.FromValue(v), nil
}

// evalSelectorOnType evaluates the selector on t.
func (e *Env) evalSelectorOnType(t *vdl.Type, selector string) (opconst.Const, error) {
	if t.Kind() != vdl.Enum {
		return opconst.Const{}, fmt.Errorf("invalid selector on type of kind: %v", t.Kind())
	}
	index := t.EnumIndex(selector)
	if index < 0 {
		return opconst.Const{}, fmt.Errorf("invalid label on enum %s: %s", t.Name(), selector)
	}
	return opconst.FromValue(vdl.EnumValue(t, index)), nil
}

// EvalConst resolves and evaluates a name to a const.
func (e *Env) EvalConst(name string, file *File) (opconst.Const, error) {
	if cd, matched := e.ResolveConst(name, file); cd != nil {
		if matched == name {
			return opconst.FromValue(cd.Value), nil
		}
		remainder := name[len(matched)+1:]
		c, err := e.evalSelectorOnValue(cd.Value, remainder)
		if err != nil {
			return opconst.Const{}, err
		}
		return c, nil
	}
	if td, matched := e.ResolveType(name, file); td != nil {
		if matched == name {
			return opconst.Const{}, fmt.Errorf("%s is a type", name)
		}
		remainder := name[len(matched)+1:]
		c, err := e.evalSelectorOnType(td.Type, remainder)
		if err != nil {
			return opconst.Const{}, err
		}
		return c, nil
	}
	return opconst.Const{}, fmt.Errorf("%s undefined", name)
}

// Errorf is a helper for error reporting, to consistently contain the file and
// position of the error when possible.
func (e *Env) Errorf(file *File, pos parse.Pos, format string, v ...interface{}) {
	e.Errors.Error(fpStringf(file, pos, format, v...))
}

// Warningf is like errorf but for warning messages that will not cause the
// compilation to fail.
func (e *Env) Warningf(file *File, pos parse.Pos, format string, v ...interface{}) {
	e.Warnings.Error(fpStringf(file, pos, "Warning: "+format, v...))
}

func (e *Env) prefixErrorf(file *File, pos parse.Pos, err error, format string, v ...interface{}) {
	e.Errors.Error(fpStringf(file, pos, format, v...) + " (" + err.Error() + ")")
}

func fpString(file *File, pos parse.Pos) string {
	return path.Join(file.Package.Path, file.BaseName) + ":" + pos.String()
}

func fpStringf(file *File, pos parse.Pos, format string, v ...interface{}) string {
	return fmt.Sprintf(fpString(file, pos)+" "+format, v...)
}

// DisallowPathQualifiers disables syntax like "a/b/c".Type.
func (e *Env) DisallowPathQualifiers() *Env {
	e.disallowPathQualifiers = true
	return e
}

// DisallowI18nErrorSupport disables i18n formats for errors.
func (e *Env) DisallowI18nErrorSupport() *Env {
	e.noI18nErrorSupport = true
	return e
}

// ErrorI18nSupport returns true if i18n support is enabled for errors.
func (e *Env) ErrorI18nSupport() bool {
	return !e.noI18nErrorSupport
}

// Representation of the components of an vdl file.  These data types represent
// the results of the compilation, used by generators for different languages.

// Package represents a vdl package, containing a list of files.
type Package struct {
	// Name is the name of the package, specified in the vdl files.
	// E.g. "bar"
	Name string
	// Path is the package path; the path used in VDL import clauses.
	// E.g. "foo/bar".
	Path string
	// GenPath is the package path to use for code generation.  It is typically
	// the same as Path, except for vdlroot standard packages.
	// E.g. "v.io/v23/vdlroot/time"
	GenPath string
	// Files holds the files contained in the package.
	Files []*File
	// FileDoc holds the top-level file documentation, which must be the same for
	// every file in the package.  This is typically used to hold boilerplate that
	// must appear in every generated file, e.g. a copyright notice.
	FileDoc string
	// Config holds the configuration for this package, specifying options used
	// during compilation and code generation.
	Config vdltool.Config

	// We hold some internal maps to make local name resolution cheap and easy.
	typeMap  map[string]*TypeDef
	constMap map[string]*ConstDef
	ifaceMap map[string]*Interface

	// We also hold some internal slices, to remember the dependency ordering.
	typeDefs  []*TypeDef
	constDefs []*ConstDef
	errorDefs []*ErrorDef
	ifaceDefs []*Interface

	// lowercaseIdents maps from lowercased identifier to a detail string; it's
	// used to detect and report identifier conflicts.
	lowercaseIdents map[string]string
}

func newPackage(name, pkgPath, genPath string, config vdltool.Config) *Package {
	return &Package{
		Name:            name,
		Path:            pkgPath,
		GenPath:         genPath,
		Config:          config,
		typeMap:         make(map[string]*TypeDef),
		constMap:        make(map[string]*ConstDef),
		ifaceMap:        make(map[string]*Interface),
		lowercaseIdents: make(map[string]string),
	}
}

// QualifiedName returns the fully-qualified name of an identifier, by
// prepending the identifier with the package path.
func (p *Package) QualifiedName(id string) string {
	if p.Path == "" {
		return id
	}
	return p.Path + "." + id
}

// ResolveType resolves the type name to its definition.
func (p *Package) ResolveType(name string) *TypeDef { return p.typeMap[name] }

// ResolveConst resolves the const name to its definition.
func (p *Package) ResolveConst(name string) *ConstDef { return p.constMap[name] }

// ResolveInterface resolves the interface name to its definition.
func (p *Package) ResolveInterface(name string) *Interface { return p.ifaceMap[name] }

// resolve resolves a name against the package.
// Checks for duplicate definitions should be performed before this is called.
func (p *Package) resolve(name string, isLocal bool) interface{} {
	if t := p.ResolveType(name); t != nil && (t.Exported || isLocal) {
		return t
	}
	if c := p.ResolveConst(name); c != nil && (c.Exported || isLocal) {
		return c
	}
	if i := p.ResolveInterface(name); i != nil && (i.Exported || isLocal) {
		return i
	}
	return nil
}

// Doc returns the package documentation, which should only exist in a single
// file in the package.
func (p *Package) Doc() string {
	for _, file := range p.Files {
		if doc := file.PackageDef.Doc; doc != "" {
			return doc
		}
	}
	return ""
}

// TypeDefs returns all types defined in this package, in dependency order.  If
// type B refers to type A, B depends on A, and A will appear before B.  Types
// may have cyclic dependencies; the ordering of cyclicly dependent types is
// arbitrary.
func (p *Package) TypeDefs() []*TypeDef { return p.typeDefs }

// ConstDefs returns all consts defined in this package, in dependency order.
// If const B refers to const A, B depends on A, and A will appear before B.
// Consts cannot have cyclic dependencies.
func (p *Package) ConstDefs() (x []*ConstDef) { return p.constDefs }

// ErrorDefs returns all errors defined in this package.  Errors don't have
// dependencies.
func (p *Package) ErrorDefs() (x []*ErrorDef) { return p.errorDefs }

// Interfaces returns all interfaces defined in this package, in dependency
// order.  If interface B embeds interface A, B depends on A, and A will appear
// before B.  Interfaces cannot have cyclic dependencies.
func (p *Package) Interfaces() (x []*Interface) { return p.ifaceDefs }

// File represents a compiled vdl file.
type File struct {
	BaseName   string       // Base name of the vdl file, e.g. "foo.vdl"
	PackageDef NamePos      // Name, position and docs of the "package" clause
	TypeDefs   []*TypeDef   // Types defined in this file
	ConstDefs  []*ConstDef  // Consts defined in this file
	ErrorDefs  []*ErrorDef  // Errors defined in this file
	Interfaces []*Interface // Interfaces defined in this file
	Package    *Package     // Parent package

	TypeDeps    map[*vdl.Type]bool // Types the file depends on
	PackageDeps []*Package         // Packages the file depends on, sorted by path

	// Imports maps the user-supplied imports from local package name to package
	// path.  They may be different from PackageDeps since we evaluate all consts
	// to their final typed value.  E.g. let's say we have three vdl files:
	//
	//   a/a.vdl  type Foo int32; const A1 = Foo(1)
	//   b/b.vdl  import "a";     const B1 = a.Foo(1); const B2 = a.A1 + 1
	//   c/c.vdl  import "b";     const C1 = b.B1;     const C2 = b.B1 + 1
	//
	// The final type and value of the constants:
	//   A1 = a.Foo(1); B1 = a.Foo(1); C1 = a.Foo(1)
	//                  B2 = a.Foo(2); C2 = a.Foo(2)
	//
	// Note that C1 and C2 both have final type a.Foo, even though c.vdl doesn't
	// explicitly import "a", and the generated c.go shouldn't import "b" since
	// it's not actually used anymore.
	imports map[string]*importPath
}

type importPath struct {
	path string
	pos  parse.Pos
	used bool // was this import path ever used?
}

// LookupImportPath translates local into a package path name, based on the
// imports associated with the file.  Returns the empty string "" if local
// couldn't be found; every valid package path is non-empty.
func (f *File) LookupImportPath(local string) string {
	if imp, ok := f.imports[local]; ok {
		imp.used = true
		return imp.path
	}
	return ""
}

// ValidateImportPackagePath returns true iff path is listed in the file's
// imports, and marks the import as used.
func (f *File) ValidateImportPackagePath(path string) bool {
	for _, imp := range f.imports {
		if imp.path == path {
			imp.used = true
			return true
		}
	}
	return false
}

// identDetail formats a detail string for calls to DeclareIdent.
func identDetail(kind string, file *File, pos parse.Pos) string {
	return fmt.Sprintf("%s at %s:%s", kind, file.BaseName, pos)
}

// DeclareIdent declares ident with the given detail string.  Returns an error
// if ident conflicts with an existing identifier in this file or package, where
// the error includes the the previous declaration detail.
func (f *File) DeclareIdent(ident, detail string) error {
	// Identifiers must be distinct from the the import names used in this file,
	// but can differ by only their capitalization.  E.g.
	//   import "foo"
	//   type foo string // BAD, type "foo" collides with import "foo"
	//   type Foo string //  OK, type "Foo" distinct from import "foo"
	//   type FoO string //  OK, type "FoO" distinct from import "foo"
	if i, ok := f.imports[ident]; ok {
		return fmt.Errorf("previous import at %s", i.pos)
	}
	// Identifiers must be distinct from all other identifiers within this
	// package, and cannot differ by only their capitalization.  E.g.
	//   type foo string
	//   const foo = "a" // BAD, const "foo" collides with type "foo"
	//   const Foo = "A" // BAD, const "Foo" collides with type "foo"
	//   const FoO = "A" // BAD, const "FoO" collides with type "foo"
	lower := strings.ToLower(ident)
	if prevDetail := f.Package.lowercaseIdents[lower]; prevDetail != "" {
		return fmt.Errorf("previous %s", prevDetail)
	}
	f.Package.lowercaseIdents[lower] = detail
	return nil
}

// Interface represents a set of embedded interfaces and methods.
type Interface struct {
	NamePos               // interface name, pos and doc
	Exported bool         // is this interface exported?
	Embeds   []*Interface // list of embedded interfaces
	Methods  []*Method    // list of methods
	File     *File        // parent file
}

// Method represents a method in an interface.
type Method struct {
	NamePos                // method name, pos and doc
	InArgs    []*Field     // list of positional in-args
	OutArgs   []*Field     // list of positional out-args
	InStream  *vdl.Type    // in-stream type, may be nil
	OutStream *vdl.Type    // out-stream type, may be nil
	Tags      []*vdl.Value // list of method tags
	Interface *Interface   // parent interface
}

// Field represents method arguments and error params.
type Field struct {
	NamePos           // arg name, pos and doc
	Type    *vdl.Type // arg type, never nil
}

// NamePos represents a name, its associated position and documentation.
type NamePos parse.NamePos

func (x *Field) String() string   { return fmt.Sprintf("%+v", *x) }
func (x *NamePos) String() string { return fmt.Sprintf("%+v", *x) }
func (p *Package) String() string {
	c := *p
	c.typeMap = nil
	c.constMap = nil
	c.ifaceMap = nil
	c.typeDefs = nil
	c.constDefs = nil
	c.errorDefs = nil
	c.ifaceDefs = nil
	return fmt.Sprintf("%+v", c)
}
func (f *File) String() string {
	c := *f
	c.Package = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}
func (x *Interface) String() string {
	c := *x
	c.File = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}
func (x *Method) String() string {
	c := *x
	c.Interface = nil // avoid infinite loop
	return fmt.Sprintf("%+v", c)
}

func (x *Interface) AllMethods() []*Method {
	result := make([]*Method, len(x.Methods))
	copy(result, x.Methods)
	for _, embed := range x.Embeds {
		result = append(result, embed.AllMethods()...)
	}
	return result
}

func (x *Interface) FindMethod(name string) *Method {
	for _, m := range x.AllMethods() {
		if name == m.Name {
			return m
		}
	}
	return nil
}

func (x *Interface) TransitiveEmbeds() []*Interface {
	return x.transitiveEmbeds(make(map[*Interface]bool))
}

func (x *Interface) transitiveEmbeds(seen map[*Interface]bool) []*Interface {
	var ret []*Interface
	for _, e := range x.Embeds {
		if !seen[e] {
			seen[e] = true
			ret = append(ret, e)
			ret = append(ret, e.transitiveEmbeds(seen)...)
		}
	}
	return ret
}

// We might consider allowing more characters, but we'll need to ensure they're
// allowed in all our codegen languages.
var (
	regexpIdent = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_]*$")
)

// hasUpperAcronym returns true if s contains an uppercase acronym; if s
// contains a run of two uppercase letters not followed by a lowercase letter.
// The lowercase letter special-case is to allow identifiers like "AMethod".
func hasUpperAcronym(s string) bool {
	upperRun := 0
	for _, r := range s {
		switch {
		case upperRun == 2 && !unicode.IsLower(r):
			return true
		case unicode.IsUpper(r):
			upperRun++
		default:
			upperRun = 0
		}
	}
	return upperRun >= 2
}

// validConstIdent returns (exported, err) where err is non-nil iff the
// identifier is valid as the name of a const.  Exported is true if the
// identifier is exported.  Valid: "^[A-Za-z][A-Za-z0-9_]*$"
func validConstIdent(ident string, mode reservedMode) (bool, error) {
	if re := regexpIdent; !re.MatchString(ident) {
		return false, fmt.Errorf("allowed regexp: %q", re)
	}
	if reservedWord(ident, mode) {
		return false, fmt.Errorf("reserved word in a generated language")
	}
	return ident[0] >= 'A' && ident[0] <= 'Z', nil
}

// validIdent is like validConstIdent, but applies to all non-const identifiers.
// It adds an additional check for uppercase acronyms.
func validIdent(ident string, mode reservedMode) (bool, error) {
	exported, err := validConstIdent(ident, mode)
	if err != nil {
		return false, err
	}
	if hasUpperAcronym(ident) {
		// TODO(toddw): Link to documentation explaining why.
		return false, fmt.Errorf("acronyms must use CamelCase")
	}
	return exported, nil
}

// validExportedIdent returns a non-nil error iff the identifier is valid and
// exported.
func validExportedIdent(ident string, mode reservedMode) error {
	exported, err := validIdent(ident, mode)
	if err != nil {
		return err
	}
	if !exported {
		return fmt.Errorf("must be exported")
	}
	return nil
}
