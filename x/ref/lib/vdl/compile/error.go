// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/parse"
)

// ErrorDef represents a user-defined error definition in the compiled results.
type ErrorDef struct {
	NamePos                     // name, parse position and docs
	Exported  bool              // is this error definition exported?
	ID        string            // error ID
	RetryCode vdl.WireRetryCode // retry action to be performed by client
	Params    []*Field          // list of positional parameter names and types
	File      *File             // parent file that this error is defined in
}

func (x *ErrorDef) String() string {
	y := *x
	y.File = nil
	return fmt.Sprintf("%+v", y)
}

// compileErrorDefs fills in pkg with compiled error definitions.
func compileErrorDefs(pkg *Package, pfiles []*parse.File, env *Env) {
	for index := range pkg.Files {
		file, pfile := pkg.Files[index], pfiles[index]
		for _, ped := range pfile.ErrorDefs {
			name, detail := ped.Name, identDetail("error", file, ped.Pos)
			export, err := validIdent(name, reservedNormal)
			if err != nil {
				env.prefixErrorf(file, ped.Pos, err, "error %s invalid name", name)
				continue
			}
			if err := file.DeclareIdent(name, detail); err != nil {
				env.prefixErrorf(file, ped.Pos, err, "error %s name conflict", name)
				continue
			}
			// The error id should not be changed; the whole point of error defs is
			// that the id is stable.
			id := pkg.Path + "." + name
			ed := &ErrorDef{NamePos: NamePos(ped.NamePos), Exported: export, ID: id, File: file}
			defineErrorActions(ed, name, ped.Actions, file, env)
			ed.Params = defineErrorParams(name, ped.Params, export, file, env)
			addErrorDef(ed, env)
		}
	}
}

// addErrorDef updates our various structures to add a new error def.
func addErrorDef(def *ErrorDef, env *Env) {
	def.File.ErrorDefs = append(def.File.ErrorDefs, def)
	def.File.Package.errorDefs = append(def.File.Package.errorDefs, def)
}

func defineErrorActions(ed *ErrorDef, name string, pactions []parse.StringPos, file *File, env *Env) {
	// We allow multiple actions to be specified in the parser, so that it's easy
	// to add new actions in the future.
	seenRetry := false
	for _, pact := range pactions {
		if retry, err := vdl.WireRetryCodeFromString(pact.String); err == nil {
			if seenRetry {
				env.Errorf(file, pact.Pos, "error %s action %s invalid (retry action specified multiple times)", name, pact.String)
				continue
			}
			seenRetry = true
			ed.RetryCode = retry
			continue
		}
		env.Errorf(file, pact.Pos, "error %s action %s invalid (unknown action)", name, pact.String)
	}
}

func defineErrorParams(name string, pparams []*parse.Field, export bool, file *File, env *Env) []*Field {
	var params []*Field
	seen := make(map[string]*parse.Field)
	for _, pparam := range pparams {
		pname, pos := pparam.Name, pparam.Pos
		if pname == "" {
			env.Errorf(file, pos, "error %s invalid (parameters must be named)", name)
			return nil
		}
		if dup := seen[pname]; dup != nil {
			env.Errorf(file, pos, "error %s param %s duplicate name (previous at %s)", name, pname, dup.Pos)
			continue
		}
		seen[pname] = pparam
		if _, err := validIdent(pname, reservedFirstRuneLower); err != nil {
			env.prefixErrorf(file, pos, err, "error %s param %s invalid", name, pname)
			continue
		}
		var paramType *vdl.Type
		if export {
			paramType = compileExportedType(pparam.Type, file, env)
		} else {
			paramType = compileType(pparam.Type, file, env)
		}
		param := &Field{NamePos(pparam.NamePos), paramType}
		params = append(params, param)
	}
	return params
}
