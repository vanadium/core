// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package golang implements Go code generation from compiled VDL packages.
package golang

import (
	"bytes"
	"flag"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"v.io/v23/vdl"
	"v.io/v23/vdlroot/vdltool"
	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/parse"
	"v.io/x/ref/lib/vdl/vdlutil"
)

type goData struct {
	Package        *compile.Package
	Env            *compile.Env
	Imports        goImports
	createdTargets map[*vdl.Type]bool // set of types whose Targets have already been created
	anonTargets    map[*vdl.Type]int  // tracks unnamed target numbers
	anonReaders    map[*vdl.Type]int  // tracks unnamed decoder numbers
	anonWriters    map[*vdl.Type]int  // tracks unnamed encoder numbers
	typeOfs        map[*vdl.Type]int  // tracks vdl.TypeOf var numbers

	collectImports bool // is this the import collecting pass instead of normal generation
	importMap      importMap
}

var skipGoMethods = flag.Bool("skip-go-methods", false, "Skip go generation of VDL{Read,Write,Zero} methods.")

// testingMode is only set to true in tests, to make testing simpler.
var testingMode = false

// Reference a package in the generator.
// If goData's collectImports mode is set, empty imports will be generated and the set of
// imported  packages will be collected. If collectImports is unset, the list of imported
// packages will be used to return a valid import.
func (data *goData) Pkg(genPkgPath string) string {
	if testingMode {
		return path.Base(genPkgPath) + "."
	}
	if data.collectImports {
		data.importMap.AddPackage(genPkgPath, data.pkgName(genPkgPath))
		return ""
	}
	// Special-case to avoid adding package qualifiers if we're generating code
	// for that package.
	if data.Package.GenPath == genPkgPath {
		return ""
	}
	if local := data.Imports.LookupLocal(genPkgPath); local != "" {
		return local + "."
	}
	data.Env.Errors.Errorf("%s: unknown generated package %q", data.Package.Name, genPkgPath)
	return ""
}

// Reference a package as forced "_" in the generator.
// During the import pass mode, this is used to collect dependencies on wiretypes
// that must have their native conversions registered.
func (data *goData) AddForcedPkg(genPkgPath string) {
	if data.collectImports {
		data.importMap.AddForcedPackage(genPkgPath)
	}
}

func (data *goData) pkgName(genPkgPath string) string {
	// For user imports, ResolvePackageGenPath should resolve to a
	// valid package.
	if pkg := data.Env.ResolvePackageGenPath(genPkgPath); pkg != nil {
		return pkg.Name
	}
	// For system imports, e.g., v.io/v23/vdl just use the basename.
	return path.Base(genPkgPath)
}

func (data *goData) CtxVar(name string) string {
	return name + " *" + data.Pkg("v.io/v23/context") + "T"
}

func (data *goData) OptsVar(name string) string {
	return name + " ..." + data.Pkg("v.io/v23/rpc") + "CallOpt"
}

func (data *goData) SkipGenZeroReadWrite(def *compile.TypeDef) bool {
	switch {
	case *skipGoMethods:
		return true
	case data.Package.Path == "v.io/v23/vdl/vdltest" && strings.HasPrefix(def.Name, "X"):
		// Don't generate VDL{IsZero,Read,Write} for vdltest types that start with
		// X.  This is hard-coded to make it easy to generate test cases.
		return true
	}
	return false
}

func (data *goData) TypeOf(tt *vdl.Type) string {
	if builtin := builtInTypeVars[tt]; builtin != "" {
		return data.Pkg("v.io/v23/vdl") + builtin
	}
	// Keep track of all other types, so we can define the vars later.
	id := data.typeOfs[tt]
	if id == 0 {
		id = len(data.typeOfs) + 1
		data.typeOfs[tt] = id
	}
	return typeOfVarName(tt, id)
}

func typeOfVarName(tt *vdl.Type, id int) string {
	return fmt.Sprintf("vdlType%s%d", tt.Kind().CamelCase(), id)
}

func (data *goData) idToTypeOfs() map[int]*vdl.Type {
	// Create a reverse typeOfs map, so that we dump the vars in order.
	idToType := make(map[int]*vdl.Type, len(data.typeOfs))
	for tt, id := range data.typeOfs {
		idToType[id] = tt
	}
	return idToType
}

func (data *goData) DeclareTypeOfVars() string {
	idToType := data.idToTypeOfs()
	if len(idToType) == 0 {
		return ""
	}
	s := `
// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (`
	for id := 1; id <= len(idToType); id++ {
		tt := idToType[id]
		s += fmt.Sprintf(`
	%[1]s *%[2]sType`, typeOfVarName(tt, id), data.Pkg("v.io/v23/vdl"))
	}
	return s + `
)`
}

// DefineTypeOfVars defines the vars holding type definitions.  They are
// declared as unexported package-level vars, and defined separately in the
// initializeVDL func, to ensure they are initialized as early as possible.
func (data *goData) DefineTypeOfVars() string {
	idToType := data.idToTypeOfs()
	if len(idToType) == 0 {
		return ""
	}
	s := `
// Initialize type definitions.`
	for id := 1; id <= len(idToType); id++ {
		// We need to generate a Go expression of type *vdl.Type that represents the
		// type.  Since the rest of our logic can already generate the Go code for
		// any value, we just wrap it in vdl.TypeOf to produce the final result.
		//
		// This may seem like a strange roundtrip, but results in less generator and
		// generated code.
		//
		// There's no need to convert the value to its native representation, since
		// it'll just be converted back in vdl.TypeOf.
		tt := idToType[id]
		typeOf := data.InitializationExpression(tt)
		s += fmt.Sprintf(`
	%[1]s = %[2]s`, typeOfVarName(tt, id), typeOf)
	}
	return s
}

// InitializationExpression returns an expression for initializing the value
// of the specified type. It is called by DefineTypeOfVars above and is required
// by the code generator in order to generate initialization values for fields
// in package level variables without relying on initialization order.
func (data *goData) InitializationExpression(tt *vdl.Type) string {
	if builtin, ok := builtInTypeVars[tt]; ok {
		return "vdl." + builtin
	}
	typeOf := typeGoWire(data, tt)
	if tt.Kind() != vdl.Optional {
		typeOf = "*" + typeOf
	}
	typeOf = "((" + typeOf + ")(nil))"
	if tt.CanBeOptional() && tt.Kind() != vdl.Optional {
		typeOf += ".Elem()"
	}
	return fmt.Sprintf(`%[1]sTypeOf%[2]s`, data.Pkg("v.io/v23/vdl"), typeOf)
}

var builtInTypeVars = map[*vdl.Type]string{
	vdl.AnyType:        "AnyType",
	vdl.BoolType:       "BoolType",
	vdl.ByteType:       "ByteType",
	vdl.Uint16Type:     "Uint16Type",
	vdl.Uint32Type:     "Uint32Type",
	vdl.Uint64Type:     "Uint64Type",
	vdl.Int8Type:       "Int8Type",
	vdl.Int16Type:      "Int16Type",
	vdl.Int32Type:      "Int32Type",
	vdl.Int64Type:      "Int64Type",
	vdl.Float32Type:    "Float32Type",
	vdl.Float64Type:    "Float64Type",
	vdl.StringType:     "StringType",
	vdl.TypeObjectType: "TypeObjectType",
	vdl.ErrorType:      "ErrorType",
}

// Generate takes a populated compile.Package and returns a byte slice
// containing the generated Go source code.
func Generate(pkg *compile.Package, env *compile.Env) []byte {
	validateGoConfig(pkg, env)
	data := &goData{
		Package:        pkg,
		Env:            env,
		collectImports: true,
		importMap:      importMap{},
		createdTargets: make(map[*vdl.Type]bool),
		anonTargets:    make(map[*vdl.Type]int),
		anonReaders:    make(map[*vdl.Type]int),
		anonWriters:    make(map[*vdl.Type]int),
		typeOfs:        make(map[*vdl.Type]int),
	}
	// The implementation uses the template mechanism from text/template and
	// executes the template against the goData instance.
	// First pass: run the templates to collect imports.
	var buf bytes.Buffer
	if err := goTemplate.Execute(&buf, data); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}

	// Remove self and built-in package dependencies.  Every package can use
	// itself and the built-in package, so we don't need to record this.
	data.importMap.DeletePackage(pkg)
	data.importMap.DeletePackage(compile.BuiltInPackage)

	// Sort the imports.
	data.Imports = goImports(data.importMap.Sort())

	// Second pass: re-run the templates with the final imports to generate the
	// output file.
	data.collectImports = false
	data.createdTargets = make(map[*vdl.Type]bool)
	data.anonTargets = make(map[*vdl.Type]int)
	data.anonReaders = make(map[*vdl.Type]int)
	data.anonWriters = make(map[*vdl.Type]int)
	data.typeOfs = make(map[*vdl.Type]int)
	buf.Reset()
	if err := goTemplate.Execute(&buf, data); err != nil {
		// We shouldn't see an error; it means our template is buggy.
		panic(fmt.Errorf("vdl: couldn't execute template: %v", err))
	}
	// Use goimports to format the generated source and imports.
	pretty, err := runner(buf.Bytes(), "gofmt", "-s")
	if err != nil {
		// We shouldn't see an error; it means we generated invalid code.
		fmt.Printf("%s", buf.Bytes())
		panic(fmt.Errorf("vdl: generated invalid Go code for %v: %v", pkg.Path, err))
	}
	// Use goimports to format the generated source and imports.
	pretty, err = runner(pretty, "goimports")
	if err != nil {
		// We shouldn't see an error; it means we generated invalid code.
		fmt.Printf("%s", pretty)
		panic(fmt.Errorf("vdl: generated invalid Go code for %v: %v", pkg.Path, err))
	}
	return pretty
}

func runner(buf []byte, binary string, args ...string) ([]byte, error) {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = bytes.NewBuffer(buf)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run %v: %v", strings.Join(cmd.Args, " "), err)
	}
	return output, nil
}

// The native types feature is hard to use correctly.  E.g. the package
// containing the wire type must be imported into your Go binary in order for
// the wire<->native registration to work, which is hard to ensure.  E.g.
//
//   package base    // VDL package
//   type Wire int   // has native type native.Int
//
//   package dep     // VDL package
//   import "base"
//   type Foo struct {
//     X base.Wire
//   }
//
// The Go code for package "dep" imports "native", rather than "base":
//
//   package dep     // Go package generated from VDL package
//   import "native"
//   type Foo struct {
//     X native.Int
//   }
//
// Note that when you import the "dep" package in your own code, you always use
// native.Int, rather than base.Wire; the base.Wire representation is only used
// as the wire format, but doesn't appear in generated code.  But in order for
// this to work correctly, the "base" package must imported.  This is tricky.
//
// Restrict the feature to these whitelisted VDL packages for now.
var nativeTypePackageWhitelist = map[string]bool{
	"math":                                   true,
	"time":                                   true,
	"v.io/x/ref/lib/vdl/testdata/nativetest": true,
	"v.io/v23/security":                      true,
	"v.io/v23/vdl":                           true,
	"v.io/v23/vdl/vdltest":                   true,
}

func validateGoConfig(pkg *compile.Package, env *compile.Env) {
	vdlconfig := path.Join(pkg.GenPath, "vdl.config")
	// Validate native type configuration.  Since native types are hard to use, we
	// restrict them to a built-in whitelist of packages for now.
	if len(pkg.Config.Go.WireToNativeTypes) > 0 && !nativeTypePackageWhitelist[pkg.Path] {
		env.Errors.Errorf("%s: Go.WireToNativeTypes is restricted to whitelisted VDL packages", vdlconfig)
	}
	// Make sure each wire type is actually defined in the package, and required
	// fields are all filled in.
	for wire, native := range pkg.Config.Go.WireToNativeTypes {
		baseWire := wire
		if strings.HasPrefix(wire, "*") {
			baseWire = strings.TrimPrefix(wire, "*")
		}
		if def := pkg.ResolveType(baseWire); def == nil {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes undefined", vdlconfig, wire)
		}
		if native.Type == "" {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (empty GoType.Type)", vdlconfig, wire)
		}
		for _, imp := range native.Imports {
			if imp.Path == "" || imp.Name == "" {
				env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (empty GoImport.Path or Name)", vdlconfig, wire)
				continue
			}
			importPrefix := imp.Name + "."
			if !strings.Contains(native.Type, importPrefix) &&
				!strings.Contains(native.ToNative, importPrefix) &&
				!strings.Contains(native.FromNative, importPrefix) {
				env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (native type %q doesn't contain import prefix %q)", vdlconfig, wire, native.Type, importPrefix)
			}
		}
		if native.Zero.Mode != vdltool.GoZeroModeUnique && native.Zero.IsZero == "" {
			env.Errors.Errorf("%s: type %s specified in Go.WireToNativeTypes invalid (native type %q must be either Mode:Unique or have a non-empty IsZero)", vdlconfig, wire, native.Type)
		}
	}
}

var goTemplate *template.Template

// The template mechanism is great at high-level formatting and simple
// substitution, but is bad at more complicated logic.  We define some functions
// that we can use in the template so that when things get complicated we back
// off to a regular function.
func init() {
	funcMap := template.FuncMap{
		"firstRuneToExport":     vdlutil.FirstRuneToExportCase,
		"firstRuneToUpper":      vdlutil.FirstRuneToUpper,
		"firstRuneToLower":      vdlutil.FirstRuneToLower,
		"vdlZeroValue":          vdl.ZeroValue,
		"errorName":             errorName,
		"errorComment":          errorComment,
		"nativeType":            nativeType,
		"noCustomNative":        noCustomNative,
		"typeHasNoCustomNative": typeHasNoCustomNative,
		"typeGo":                typeGo,
		"defineType":            defineType,
		"defineIsZero":          defineIsZero,
		"defineWrite":           defineWrite,
		"defineRead":            defineRead,
		"defineConst":           defineConst,
		"genValueOf":            genValueOf,
		"embedGo":               embedGo,
		"isStreamingMethod":     isStreamingMethod,
		"hasStreamingMethods":   hasStreamingMethods,
		"docBreak":              docBreak,
		"quoteStripDoc":         parse.QuoteStripDoc,
		"argNames":              argNames,
		"argTypes":              argTypes,
		"argNameTypes":          argNameTypes,
		"argParens":             argParens,
		"uniqueName":            uniqueName,
		"uniqueNameImpl":        uniqueNameImpl,
		"serverCallVar":         serverCallVar,
		"serverCallStubVar":     serverCallStubVar,
		"outArgsClient":         outArgsClient,
		"clientStubImpl":        clientStubImpl,
		"clientFinishImpl":      clientFinishImpl,
		"serverStubImpl":        serverStubImpl,
		"reInitStreamValue":     reInitStreamValue,
		"callVerror":            callVerror,
		"paramNamedResults":     paramNamedResults,
		"errorsI18n":            errorsI18n,
	}
	goTemplate = template.Must(template.New("genGo").Funcs(funcMap).Parse(genGo))
}

func errorName(def *compile.ErrorDef) string {
	switch {
	case def.Exported:
		return "Err" + def.Name
	default:
		return "err" + vdlutil.FirstRuneToUpper(def.Name)
	}
}

func errorComment(def *compile.ErrorDef) string {
	parts := strings.Fields(def.Doc)
	errName := errorName(def)
	// Make sure the comment starts with Err<name>....
	if len(parts) > 0 && "Err"+parts[1] == errName {
		return strings.Replace(def.Doc, parts[1], "Err"+parts[1], 1)
	}
	return def.Doc
}

func errorsI18n(data *goData) bool {
	return data.Env.ErrorI18nSupport()
}

func isStreamingMethod(method *compile.Method) bool {
	return method.InStream != nil || method.OutStream != nil
}

func hasStreamingMethods(methods []*compile.Method) bool {
	for _, method := range methods {
		if isStreamingMethod(method) {
			return true
		}
	}
	return false
}

// docBreak adds a "//\n" break to separate previous comment lines and doc.  If
// doc is empty it returns the empty string.
func docBreak(doc string) string {
	if doc == "" {
		return ""
	}
	return "//\n" + doc
}

// argTypes returns a comma-separated list of each type from args.
func argTypes(first, last string, data *goData, args []*compile.Field) string {
	var result []string
	if first != "" {
		result = append(result, first)
	}
	for _, arg := range args {
		result = append(result, typeGo(data, arg.Type))
	}
	if last != "" {
		result = append(result, last)
	}
	return strings.Join(result, ", ")
}

// argNames returns a comma-separated list of each name from args.  If argPrefix
// is empty, the name specified in args is used; otherwise the name is prefixD,
// where D is the position of the argument.
func argNames(boxPrefix, argPrefix, first, second, last string, args []*compile.Field) string {
	var result []string
	if first != "" {
		result = append(result, first)
	}
	if second != "" {
		result = append(result, second)
	}
	for ix, arg := range args {
		name := arg.Name
		if argPrefix != "" {
			name = fmt.Sprintf("%s%d", argPrefix, ix)
		}
		if arg.Type == vdl.ErrorType {
			// TODO(toddw): Also need to box user-defined external interfaces.  Or can
			// we remove this special-case now?
			name = boxPrefix + name
		}
		result = append(result, name)
	}
	if last != "" {
		result = append(result, last)
	}
	return strings.Join(result, ", ")
}

// argNameTypes returns a comma-separated list of "name type" from args.  If
// argPrefix is empty, the name specified in args is used; otherwise the name is
// prefixD, where D is the position of the argument.  If argPrefix is empty and
// no names are specified in args, no names will be output.
func argNameTypes(argPrefix, first, second, last string, data *goData, args []*compile.Field) string {
	noNames := argPrefix == "" && !hasArgNames(args)
	var result []string
	if first != "" {
		result = append(result, maybeStripArgName(first, noNames))
	}
	if second != "" {
		result = append(result, maybeStripArgName(second, noNames))
	}
	for ax, arg := range args {
		var name string
		switch {
		case noNames:
			break
		case argPrefix == "":
			name = arg.Name + " "
		default:
			name = fmt.Sprintf("%s%d ", argPrefix, ax)
		}
		result = append(result, name+typeGo(data, arg.Type))
	}
	if last != "" {
		result = append(result, maybeStripArgName(last, noNames))
	}
	return strings.Join(result, ", ")
}

func hasArgNames(args []*compile.Field) bool {
	// VDL guarantees that either all args are named, or none of them are.
	return len(args) > 0 && args[0].Name != ""
}

// maybeStripArgName strips away the first space-terminated token from arg, only
// if strip is true.
func maybeStripArgName(arg string, strip bool) string {
	if index := strings.Index(arg, " "); index != -1 && strip {
		return arg[index+1:]
	}
	return arg
}

// paramNamedResults returns a comma-separated list of "name type" from args
// in the order (first, second, args*, last)
func paramNamedResults(first, second, last string, data *goData, args []*compile.Field) string {
	var result []string
	if first != "" {
		result = append(result, first)
	}
	if second != "" {
		result = append(result, second)
	}
	for _, arg := range args {
		result = append(result, arg.Name+" "+typeGo(data, arg.Type))
	}
	if last != "" {
		result = append(result, last)
	}
	return strings.Join(result, ", ")
}

// argParens takes a list of 0 or more arguments, and adds parens only when
// necessary; if args contains any commas or spaces, we must add parens.
func argParens(argList string) string {
	if strings.ContainsAny(argList, ", ") {
		return "(" + argList + ")"
	}
	return argList
}

// uniqueName returns a unique name based on the interface, method and suffix.
func uniqueName(iface *compile.Interface, method *compile.Method, suffix string) string {
	return iface.Name + method.Name + suffix
}

// uniqueNameImpl returns uniqueName with an "impl" prefix.
func uniqueNameImpl(iface *compile.Interface, method *compile.Method, suffix string) string {
	return "impl" + uniqueName(iface, method, suffix)
}

// The first arg of every server method is a context; the type is either a typed
// context for streams, or rpc.ServerCall for non-streams.
func serverCallVar(name string, data *goData, iface *compile.Interface, method *compile.Method) string {
	if isStreamingMethod(method) {
		return name + " " + uniqueName(iface, method, "ServerCall")
	}
	return name + " " + data.Pkg("v.io/v23/rpc") + "ServerCall"
}

// The first arg of every server stub method is a context; the type is either a
// typed context stub for streams, or rpc.ServerCall for non-streams.
func serverCallStubVar(name string, data *goData, iface *compile.Interface, method *compile.Method) string {
	if isStreamingMethod(method) {
		return name + " *" + uniqueName(iface, method, "ServerCallStub")
	}
	return name + " " + data.Pkg("v.io/v23/rpc") + "ServerCall"
}

// outArgsClient returns the out args of an interface method on the client,
// wrapped in parens if necessary.  The client side always returns a final
// error, in addition to the regular out-args.
func outArgsClient(argPrefix string, errName string, data *goData, iface *compile.Interface, method *compile.Method) string {
	first, args := "", method.OutArgs
	if isStreamingMethod(method) {
		first, args = "ocall "+uniqueName(iface, method, "ClientCall"), nil
	}
	return argParens(argNameTypes(argPrefix, first, "", errName+" error", data, args))
}

// clientStubImpl returns the interface method client stub implementation.
func clientStubImpl(data *goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	inargs := "nil"
	if len(method.InArgs) > 0 {
		inargs = "[]interface{}{" + argNames("&", "i", "", "", "", method.InArgs) + "}"
	}
	switch {
	case isStreamingMethod(method):
		fmt.Fprint(&buf, "\tvar call "+data.Pkg("v.io/v23/rpc")+"ClientCall\n")
		fmt.Fprintf(&buf, "\tif call, err = "+data.Pkg("v.io/v23")+"GetClient(ctx).StartCall(ctx, c.name, %q, %s, opts...); err != nil {\n\t\treturn\n\t}\n", method.Name, inargs)
		fmt.Fprintf(&buf, "ocall = &%s{ClientCall: call}\n", uniqueNameImpl(iface, method, "ClientCall"))
	default:
		outargs := "nil"
		if len(method.OutArgs) > 0 {
			outargs = "[]interface{}{" + argNames("", "&o", "", "", "", method.OutArgs) + "}"
		}
		fmt.Fprintf(&buf, "\terr = "+data.Pkg("v.io/v23")+"GetClient(ctx).Call(ctx, c.name, %q, %s, %s, opts...)\n", method.Name, inargs, outargs)
	}
	fmt.Fprint(&buf, "\treturn")
	return buf.String() // the caller writes the trailing newline
}

// clientFinishImpl returns the client finish implementation for method.
func clientFinishImpl(varname string, method *compile.Method) string {
	outargs := argNames("", "&o", "", "", "", method.OutArgs)
	return fmt.Sprintf("\terr = %s.Finish(%s)", varname, outargs)
}

// serverStubImpl returns the interface method server stub implementation.
func serverStubImpl(data *goData, iface *compile.Interface, method *compile.Method) string {
	var buf bytes.Buffer
	inargs := argNames("", "i", "ctx", "call", "", method.InArgs)
	fmt.Fprintf(&buf, "\treturn s.impl.%s(%s)", method.Name, inargs)
	return buf.String() // the caller writes the trailing newline
}

func reInitStreamValue(data *goData, t *vdl.Type, name string) string {
	switch t.Kind() {
	case vdl.Struct:
		return name + " = " + typeGo(data, t) + "{}\n"
	case vdl.Any:
		return name + " = nil\n"
	}
	return ""
}

func callVerror(data *goData, call string) string {
	if data.Package.Name == "verror" {
		return call
	}
	return "verror." + call
}

// The template that we execute against a goData instance to generate our
// code.  Most of this is fairly straightforward substitution and ranges; more
// complicated logic is delegated to the helper functions above.
//
// We try to generate code that has somewhat reasonable formatting, and leave
// the fine-tuning to the go/format package.  Note that go/format won't fix
// some instances of spurious newlines, so we try to keep it reasonable.
const genGo = `
{{$data := .}}
{{$pkg := $data.Package}}
{{$pkg.FileDoc}}

// This file was auto-generated by the vanadium vdl tool.
// Package: {{$pkg.Name}}

{{$pkg.Doc}}//nolint:golint
package {{$pkg.Name}}

{{if $data.Imports}}
import (
	{{range $imp := $data.Imports}}
	{{if $imp.Name}}{{$imp.Name}} {{end}}"{{$imp.Path}}"{{end}}
){{end}}

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

{{if $pkg.TypeDefs}}
// Type definitions
// ================

{{range $tdef := $pkg.TypeDefs}}
{{defineType $data $tdef}}
{{if not ($data.SkipGenZeroReadWrite $tdef)}}
{{defineIsZero $data $tdef}}
{{defineWrite $data $tdef}}
{{defineRead $data $tdef}}
{{end}}
{{end}}

{{if $pkg.Config.Go.WireToNativeTypes}}
// Type-check native conversion functions.
var (
{{range $wire, $native := $pkg.Config.Go.WireToNativeTypes}}{{if noCustomNative $native}}{{$nat := nativeType $data $native $pkg}}
	_ func({{$wire}}, *{{$nat}}) error = {{$wire}}ToNative
	_ func(*{{$wire}}, {{$nat}}) error = {{$wire}}FromNative{{end}}{{end}}
)
{{end}}
{{end}}

{{if $pkg.ConstDefs}}
// Const definitions
// =================

{{range $cdef := $pkg.ConstDefs}}
{{defineConst $data $cdef}}{{end}}
{{end}}

{{if $pkg.ErrorDefs}}
// Error definitions
// =================

var (
{{range $edef := $pkg.ErrorDefs}}
	{{errorComment $edef}}{{errorName $edef}}	= {{$data.Pkg "v.io/v23/verror"}}NewIDAction("{{$edef.ID}}", {{$data.Pkg "v.io/v23/verror"}}{{$edef.RetryCode}}){{end}}
)

{{range $edef := $pkg.ErrorDefs}}
{{$errName := errorName $edef}}
{{$newErr := print (firstRuneToExport "New" $edef.Exported) (firstRuneToUpper $errName)}}
{{$errorf := print (firstRuneToExport "Errorf" $edef.Exported) (firstRuneToUpper  $edef.Name)}}
{{$message := print (firstRuneToExport "Message" $edef.Exported) (firstRuneToUpper  $edef.Name)}}
{{if errorsI18n $data}}
// {{$newErr}} returns an error with the {{$errName}} ID.
// Deprecated: this function will be removed in the future,
// use {{$errorf}} or {{$message}} instead.
func {{$newErr}}(ctx {{argNameTypes "" (print "*" ($data.Pkg "v.io/v23/context") "T") "" "" $data $edef.Params}}) error {
	return {{$data.Pkg "v.io/v23/verror"}}New({{$errName}}, {{argNames "" "" "ctx" "" "" $edef.Params}})
}
{{end}}
// {{$errorf}} calls {{$errName}}.Errorf with the supplied arguments.
func {{$errorf}}(ctx {{(print "*" ($data.Pkg "v.io/v23/context") "T")}}, format string,  {{argNameTypes "" "" "" "" $data $edef.Params}}) error {
	return {{$errName}}.Errorf({{argNames "" "" "ctx" "format" "" $edef.Params}})
}

// {{$message}} calls {{$errName}}.Message with the supplied arguments.
func {{$message}}(ctx {{(print "*" ($data.Pkg "v.io/v23/context") "T")}}, message string,  {{argNameTypes "" "" "" "" $data $edef.Params}}) error {
	return {{$errName}}.Message({{argNames "" "" "ctx" "message" "" $edef.Params}})
}

{{$params := print (firstRuneToExport "Params" $edef.Exported) (firstRuneToUpper  $errName)}}
// {{$params}} extracts the expected parameters from the error's ParameterList.
func {{$params}}(argumentError error) ({{paramNamedResults "verrorComponent string" "verrorOperation string"  "returnErr error" $data $edef.Params}}) {
	params := {{callVerror $data "Params(argumentError)"}}
	if params == nil {
		returnErr = fmt.Errorf("no parameters found in: %T: %v", argumentError, argumentError)
		return
	}
	iter := &paramListIterator{params: params, max: len(params)}

	if verrorComponent, verrorOperation, returnErr = iter.preamble(); returnErr != nil {
		return
	}
	{{if $edef.Params}}
	var (
		tmp interface{}
		ok bool
	)
	{{range $edef.Params}}tmp, returnErr = iter.next()
	if {{.Name}}, ok = tmp.({{typeGo $data .Type}}); !ok {
		if returnErr != nil {
			return
		}
		returnErr = fmt.Errorf("parameter list contains the wrong type for return value {{.Name}}, has %T and not {{typeGo $data .Type}}", tmp)
		return
	}
	{{end}}{{end}}
	return
}
{{end}}
{{end}}

{{if $pkg.ErrorDefs}}
type paramListIterator struct {
	err      error
	idx, max int
	params   []interface{}
}

func (pl *paramListIterator) next() (interface{}, error) {
	if pl.err != nil {
		return nil, pl.err
	}
	if pl.idx+1 > pl.max {
		pl.err = fmt.Errorf("too few parameters: have %v", pl.max)
		return nil, pl.err
	}
	pl.idx++
	return  pl.params[pl.idx-1], nil
}

func (pl *paramListIterator) preamble() (component, operation string, err error) {
	var tmp interface{}
	if tmp, err = pl.next(); err != nil {
		return
	}
	var ok bool
	if component, ok = tmp.(string); !ok {
		return "", "", fmt.Errorf("ParamList[0]: component name is not a string: %T", tmp)
	}
	if tmp, err = pl.next(); err != nil {
		return
	}
	if operation, ok = tmp.(string); !ok {
		return "", "", fmt.Errorf("ParamList[1]: operation name is not a string: %T", tmp)
	}
	return
}
{{end}}

{{if $pkg.Interfaces}}
// Interface definitions
// =====================
{{range $iface := $pkg.Interfaces}}
{{$ifaceStreaming := hasStreamingMethods $iface.AllMethods}}
{{$rpc_ := $data.Pkg "v.io/v23/rpc"}}
// {{$iface.Name}}ClientMethods is the client interface
// containing {{$iface.Name}} methods.
{{docBreak $iface.Doc}}type {{$iface.Name}}ClientMethods interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ClientMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" ($data.CtxVar "_") "" ($data.OptsVar "_") $data $method.InArgs}}) {{outArgsClient "" "_" $data $iface $method}}{{$method.DocSuffix}}{{end}}
}

// {{$iface.Name}}ClientStub embeds {{$iface.Name}}ClientMethods and is a
// placeholder for additional management operations.
type {{$iface.Name}}ClientStub interface {
	{{$iface.Name}}ClientMethods
}

// {{$iface.Name}}Client returns a client stub for {{$iface.Name}}.
func {{$iface.Name}}Client(name string) {{$iface.Name}}ClientStub {
	return impl{{$iface.Name}}ClientStub{name{{range $embed := $iface.Embeds}}, {{embedGo $data $embed}}Client(name){{end}} }
}

type impl{{$iface.Name}}ClientStub struct {
	name   string
{{range $embed := $iface.Embeds}}
	{{embedGo $data $embed}}ClientStub{{end}}
}

{{range $method := $iface.Methods}}
func (c impl{{$iface.Name}}ClientStub) {{$method.Name}}({{argNameTypes "i" ($data.CtxVar "ctx") "" ($data.OptsVar "opts") $data $method.InArgs}}) {{outArgsClient "o" "err" $data $iface $method}} {
{{clientStubImpl $data $iface $method}}
}
{{end}}

{{range $method := $iface.Methods}}{{if isStreamingMethod $method}}
{{$clientStream := uniqueName $iface $method "ClientStream"}}
{{$clientCall := uniqueName $iface $method "ClientCall"}}
{{$clientCallImpl := uniqueNameImpl $iface $method "ClientCall"}}
{{$clientRecvImpl := uniqueNameImpl $iface $method "ClientCallRecv"}}
{{$clientSendImpl := uniqueNameImpl $iface $method "ClientCallSend"}}

// {{$clientStream}} is the client stream for {{$iface.Name}}.{{$method.Name}}.
type {{$clientStream}} interface { {{if $method.OutStream}}
	// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} client stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() {{typeGo $data $method.OutStream}}
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	} {{end}}{{if $method.InStream}}
	// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} client stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors
		// encountered while sending, or if Send is called after Close or
		// the stream has been canceled.  Blocks if there is no buffer
		// space; will unblock when buffer space is available or after
		// the stream has been canceled.
		Send(item {{typeGo $data $method.InStream}}) error
		// Close indicates to the server that no more items will be sent;
		// server Recv calls will receive io.EOF after all sent items.
		// This is an optional call - e.g. a client might call Close if it
		// needs to continue receiving items from the server after it's
		// done sending.  Returns errors encountered while closing, or if
		// Close is called after the stream has been canceled.  Like Send,
		// blocks if there is no buffer space available.
		Close() error
	} {{end}}
}

// {{$clientCall}} represents the call returned from {{$iface.Name}}.{{$method.Name}}.
type {{$clientCall}} interface {
	{{$clientStream}} {{if $method.InStream}}
	// Finish performs the equivalent of SendStream().Close, then blocks until
	// the server is done, and returns the positional return values for the call.{{else}}
	// Finish blocks until the server is done, and returns the positional return
	// values for call.{{end}}
	//
	// Finish returns immediately if the call has been canceled; depending on the
	// timing the output could either be an error signaling cancelation, or the
	// valid positional return values from the server.
	//
	// Calling Finish is mandatory for releasing stream resources, unless the call
	// has been canceled or any of the other methods return an error.  Finish should
	// be called at most once.
	Finish() {{argParens (argNameTypes "" "" "" "_ error" $data $method.OutArgs)}}
}

type {{$clientCallImpl}} struct {
	{{$rpc_}}ClientCall{{if $method.OutStream}}
	valRecv {{typeGo $data $method.OutStream}}
	errRecv error{{end}}
}

{{if $method.OutStream}}func (c *{{$clientCallImpl}}) RecvStream() interface {
	Advance() bool
	Value() {{typeGo $data $method.OutStream}}
	Err() error
} {
	return {{$clientRecvImpl}}{c}
}

type {{$clientRecvImpl}} struct {
	c *{{$clientCallImpl}}
}

func (c {{$clientRecvImpl}}) Advance() bool {
	{{reInitStreamValue $data $method.OutStream "c.c.valRecv"}}c.c.errRecv = c.c.Recv(&c.c.valRecv)
	return c.c.errRecv == nil
}
func (c {{$clientRecvImpl}}) Value() {{typeGo $data $method.OutStream}} {
	return c.c.valRecv
}
func (c {{$clientRecvImpl}}) Err() error {
	if c.c.errRecv == {{$data.Pkg "io"}}EOF {
		return nil
	}
	return c.c.errRecv
}
{{end}}{{if $method.InStream}}func (c *{{$clientCallImpl}}) SendStream() interface {
	Send(item {{typeGo $data $method.InStream}}) error
	Close() error
} {
	return {{$clientSendImpl}}{c}
}

type {{$clientSendImpl}} struct {
	c *{{$clientCallImpl}}
}

func (c {{$clientSendImpl}}) Send(item {{typeGo $data $method.InStream}}) error {
	return c.c.Send(item)
}
func (c {{$clientSendImpl}}) Close() error {
	return c.c.CloseSend()
}
{{end}}func (c *{{$clientCallImpl}}) Finish() {{argParens (argNameTypes "o" "" "" "err error" $data $method.OutArgs)}} {
{{clientFinishImpl "c.ClientCall" $method}}
	return
}
{{end}}
{{end}}

// {{$iface.Name}}ServerMethods is the interface a server writer
// implements for {{$iface.Name}}.
{{docBreak $iface.Doc}}type {{$iface.Name}}ServerMethods interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ServerMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" ($data.CtxVar "_") (serverCallVar "_" $data $iface $method) "" $data $method.InArgs}}) {{argParens (argNameTypes "" "" "" "_ error" $data $method.OutArgs)}}{{$method.DocSuffix}}{{end}}
}

// {{$iface.Name}}ServerStubMethods is the server interface containing
// {{$iface.Name}} methods, as expected by rpc.Server.{{if $ifaceStreaming}}
// The only difference between this interface and {{$iface.Name}}ServerMethods
// is the streaming methods.{{else}}
// There is no difference between this interface and {{$iface.Name}}ServerMethods
// since there are no streaming methods.{{end}}
type {{$iface.Name}}ServerStubMethods {{if $ifaceStreaming}}interface { {{range $embed := $iface.Embeds}}
	{{$embed.Doc}}{{embedGo $data $embed}}ServerStubMethods{{$embed.DocSuffix}}{{end}}{{range $method := $iface.Methods}}
	{{$method.Doc}}{{$method.Name}}({{argNameTypes "" ($data.CtxVar "_") (serverCallStubVar "_" $data $iface $method) "" $data $method.InArgs}}) {{argParens (argNameTypes "" "" "" "_ error" $data $method.OutArgs)}}{{$method.DocSuffix}}{{end}}
}
{{else}}{{$iface.Name}}ServerMethods
{{end}}

// {{$iface.Name}}ServerStub adds universal methods to {{$iface.Name}}ServerStubMethods.
type {{$iface.Name}}ServerStub interface {
	{{$iface.Name}}ServerStubMethods
	// DescribeInterfaces the {{$iface.Name}} interfaces.
	Describe__() []{{$rpc_}}InterfaceDesc
}

// {{$iface.Name}}Server returns a server stub for {{$iface.Name}}.
// It converts an implementation of {{$iface.Name}}ServerMethods into
// an object that may be used by rpc.Server.
func {{$iface.Name}}Server(impl {{$iface.Name}}ServerMethods) {{$iface.Name}}ServerStub {
	stub := impl{{$iface.Name}}ServerStub{
		impl: impl,{{range $embed := $iface.Embeds}}
		{{$embed.Name}}ServerStub: {{embedGo $data $embed}}Server(impl),{{end}}
	}
	// Initialize GlobState; always check the stub itself first, to handle the
	// case where the user has the Glob method defined in their VDL source.
	if gs := {{$rpc_}}NewGlobState(stub); gs != nil {
		stub.gs = gs
	} else if gs := {{$rpc_}}NewGlobState(impl); gs != nil {
		stub.gs = gs
	}
	return stub
}

type impl{{$iface.Name}}ServerStub struct {
	impl {{$iface.Name}}ServerMethods{{range $embed := $iface.Embeds}}
	{{embedGo $data $embed}}ServerStub{{end}}
	gs *{{$rpc_}}GlobState
}

{{range $method := $iface.Methods}}
func (s impl{{$iface.Name}}ServerStub) {{$method.Name}}({{argNameTypes "i" ($data.CtxVar "ctx") (serverCallStubVar "call" $data $iface $method) "" $data $method.InArgs}}) {{argParens (argTypes "" "error" $data $method.OutArgs)}} {
{{serverStubImpl $data $iface $method}}
}
{{end}}

func (s impl{{$iface.Name}}ServerStub) Globber() *{{$rpc_}}GlobState {
	return s.gs
}

func (s impl{{$iface.Name}}ServerStub) Describe__() []{{$rpc_}}InterfaceDesc {
	return []{{$rpc_}}InterfaceDesc{ {{$iface.Name}}Desc{{range $embed := $iface.TransitiveEmbeds}}, {{embedGo $data $embed}}Desc{{end}} }
}

// {{$iface.Name}}Desc describes the {{$iface.Name}} interface.
var {{$iface.Name}}Desc {{$rpc_}}InterfaceDesc = desc{{$iface.Name}}

// desc{{$iface.Name}} hides the desc to keep godoc clean.
var desc{{$iface.Name}} = {{$rpc_}}InterfaceDesc{ {{if $iface.Name}}
	Name: "{{$iface.Name}}",{{end}}{{if $iface.File.Package.Path}}
	PkgPath: "{{$iface.File.Package.Path}}",{{end}}{{if $iface.Doc}}
	Doc: {{quoteStripDoc $iface.Doc}},{{end}}{{if $iface.Embeds}}
	Embeds: []{{$rpc_}}EmbedDesc{ {{range $embed := $iface.Embeds}}
		{ Name: "{{$embed.Name}}", PkgPath: "{{$embed.File.Package.Path}}", Doc: {{quoteStripDoc $embed.Doc}} },{{end}}
	},{{end}}{{if $iface.Methods}}
	Methods: []{{$rpc_}}MethodDesc{ {{range $method := $iface.Methods}}
		{ {{if $method.Name}}
			Name: "{{$method.Name}}",{{end}}{{if $method.Doc}}
			Doc: {{quoteStripDoc $method.Doc}},{{end}}{{if $method.InArgs}}
			InArgs: []{{$rpc_}}ArgDesc{ {{range $arg := $method.InArgs}}
				{ Name: "{{$arg.Name}}", Doc: {{quoteStripDoc $arg.Doc}} }, // {{typeGo $data $arg.Type}}{{end}}
			},{{end}}{{if $method.OutArgs}}
			OutArgs: []{{$rpc_}}ArgDesc{ {{range $arg := $method.OutArgs}}
				{ Name: "{{$arg.Name}}", Doc: {{quoteStripDoc $arg.Doc}} }, // {{typeGo $data $arg.Type}}{{end}}
			},{{end}}{{if $method.Tags}}
			Tags: []*{{$data.Pkg "v.io/v23/vdl"}}Value{ {{range $tag := $method.Tags}}{{genValueOf $data $tag}} ,{{end}} },{{end}}
		},{{end}}
	},{{end}}
}

{{range $method := $iface.Methods}}
{{if isStreamingMethod $method}}
{{$serverStream := uniqueName $iface $method "ServerStream"}}
{{$serverCall := uniqueName $iface $method "ServerCall"}}
{{$serverCallStub := uniqueName $iface $method "ServerCallStub"}}
{{$serverRecvImpl := uniqueNameImpl $iface $method "ServerCallRecv"}}
{{$serverSendImpl := uniqueNameImpl $iface $method "ServerCallSend"}}

// {{$serverStream}} is the server stream for {{$iface.Name}}.{{$method.Name}}.
type {{$serverStream}} interface { {{if $method.InStream}}
	// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} server stream.
	RecvStream() interface {
		// Advance stages an item so that it may be retrieved via Value.  Returns
		// true iff there is an item to retrieve.  Advance must be called before
		// Value is called.  May block if an item is not available.
		Advance() bool
		// Value returns the item that was staged by Advance.  May panic if Advance
		// returned false or was not called.  Never blocks.
		Value() {{typeGo $data $method.InStream}}
		// Err returns any error encountered by Advance.  Never blocks.
		Err() error
	} {{end}}{{if $method.OutStream}}
	// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} server stream.
	SendStream() interface {
		// Send places the item onto the output stream.  Returns errors encountered
		// while sending.  Blocks if there is no buffer space; will unblock when
		// buffer space is available.
		Send(item {{typeGo $data $method.OutStream}}) error
	} {{end}}
}

// {{$serverCall}} represents the context passed to {{$iface.Name}}.{{$method.Name}}.
type {{$serverCall}} interface {
	{{$rpc_}}ServerCall
	{{$serverStream}}
}

// {{$serverCallStub}} is a wrapper that converts rpc.StreamServerCall into
// a typesafe stub that implements {{$serverCall}}.
type {{$serverCallStub}} struct {
	{{$rpc_}}StreamServerCall{{if $method.InStream}}
	valRecv {{typeGo $data $method.InStream}}
	errRecv error{{end}}
}

// Init initializes {{$serverCallStub}} from rpc.StreamServerCall.
func (s *{{$serverCallStub}}) Init(call {{$rpc_}}StreamServerCall) {
	s.StreamServerCall = call
}

{{if $method.InStream}}// RecvStream returns the receiver side of the {{$iface.Name}}.{{$method.Name}} server stream.
func (s  *{{$serverCallStub}}) RecvStream() interface {
	Advance() bool
	Value() {{typeGo $data $method.InStream}}
	Err() error
} {
	return {{$serverRecvImpl}}{s}
}

type {{$serverRecvImpl}} struct {
	s *{{$serverCallStub}}
}

func (s {{$serverRecvImpl}}) Advance() bool {
	{{reInitStreamValue $data $method.InStream "s.s.valRecv"}}s.s.errRecv = s.s.Recv(&s.s.valRecv)
	return s.s.errRecv == nil
}
func (s {{$serverRecvImpl}}) Value() {{typeGo $data $method.InStream}} {
	return s.s.valRecv
}
func (s {{$serverRecvImpl}}) Err() error {
	if s.s.errRecv == {{$data.Pkg "io"}}EOF {
		return nil
	}
	return s.s.errRecv
}
{{end}}{{if $method.OutStream}}// SendStream returns the send side of the {{$iface.Name}}.{{$method.Name}} server stream.
func (s *{{$serverCallStub}}) SendStream() interface {
	Send(item {{typeGo $data $method.OutStream}}) error
} {
	return {{$serverSendImpl}}{s}
}

type {{$serverSendImpl}} struct {
	s *{{$serverCallStub}}
}

func (s {{$serverSendImpl}}) Send(item {{typeGo $data $method.OutStream}}) error {
	return s.s.Send(item)
}
{{end}}{{end}}{{end}}{{end}}{{end}}

{{$data.DeclareTypeOfVars}}

var initializeVDLCalled bool

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = initializeVDL()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func initializeVDL() struct{} {
	if initializeVDLCalled {
		return struct{}{}
	}
	initializeVDLCalled = true
{{if $pkg.Config.Go.WireToNativeTypes}}
	// Register native type conversions first, so that vdl.TypeOf works.{{range $wire, $native := $pkg.Config.Go.WireToNativeTypes}}{{if noCustomNative $native}}
	{{$data.Pkg "v.io/v23/vdl"}}RegisterNative({{$wire}}ToNative, {{$wire}}FromNative){{end}}{{end}}
{{end}}
{{if $pkg.TypeDefs}}
	// Register types.{{range $tdef := $pkg.TypeDefs}}{{if typeHasNoCustomNative $data $tdef}}
	{{$data.Pkg "v.io/v23/vdl"}}Register((*{{$tdef.Name}})(nil)){{end}}{{end}}
{{end}}
{{$data.DefineTypeOfVars}}
{{if $pkg.ErrorDefs}}
{{if errorsI18n $data}}
	// Set error format strings.{{/* TODO(toddw): Don't set "en-US" or "en" again, since it's already set by the original verror.Register call. */}}{{range $edef := $pkg.ErrorDefs}}{{range $lf := $edef.Formats}}
	{{$data.Pkg "v.io/v23/i18n"}}Cat().SetWithBase({{$data.Pkg "v.io/v23/i18n"}}LangID("{{$lf.Lang}}"), {{$data.Pkg "v.io/v23/i18n"}}MsgID({{errorName $edef}}.ID), "{{$lf.Fmt}}"){{end}}{{end}}
{{end}}
{{end}}
	return struct{}{}
}
`
