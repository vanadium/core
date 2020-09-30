// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"
	"regexp"
	"strconv"

	"v.io/v23/i18n"
	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/parse"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// ErrorDef represents a user-defined error definition in the compiled results.
type ErrorDef struct {
	NamePos                     // name, parse position and docs
	Exported  bool              // is this error definition exported?
	ID        string            // error ID
	RetryCode vdl.WireRetryCode // retry action to be performed by client
	Params    []*Field          // list of positional parameter names and types
	Formats   []LangFmt         // list of language / format pairs
	English   string            // English format text from Formats
	File      *File             // parent file that this error is defined in
}

// LangFmt represents a language / format string pair.
type LangFmt struct {
	Lang i18n.LangID // IETF language tag
	Fmt  string      // i18n format string in the given language.
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
			ed.Formats = defineErrorFormats(name, ped.Formats, ed.Params, file, env)
			// We require the "en" base language for at least one of the Formats, and
			// favor "en-US" if it exists.  This requirement is an attempt to ensure
			// there is at least one common language across all errors.
			for _, lf := range ed.Formats {
				if lf.Lang == i18n.LangID("en-US") {
					ed.English = lf.Fmt
					break
				}
				if ed.English == "" && i18n.BaseLangID(lf.Lang) == i18n.LangID("en") {
					ed.English = lf.Fmt
				}
			}
			upperName := vdlutil.FirstRuneToUpper(ed.Name)
			errName := "Err" + upperName
			newName := "NewErr" + upperName
			errorfName := "ErrorfErr" + upperName
			errorID := pkg.Path + "." + errName
			if !ed.Exported {
				errName = "err" + upperName
				newName = "newErr" + upperName
				errorfName = "errorfErr" + upperName
				errorID = errName
			}
			if ed.English == "" {
				if !env.noI18nErrorSupport {
					env.Warningf(file, ed.Pos, "error %s does not include an i18n message, make sure that %s is being used to create errors with ID %s and not %s", errName, errorfName, errorID, newName)
				}
			} else {
				env.Warningf(file, ed.Pos, "error %s includes an i18n format which is now deprecated, remove this and use %s to create errors with ID %s", errName, errorfName, errorID)
			}
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

func defineErrorFormats(name string, plfs []parse.LangFmt, params []*Field, file *File, env *Env) []LangFmt {
	var lfs []LangFmt
	seen := make(map[i18n.LangID]parse.LangFmt)
	for _, plf := range plfs {
		pos, lang, fmt := plf.Pos(), i18n.LangID(plf.Lang.String), plf.Fmt.String
		if lang == "" {
			env.Errorf(file, pos, "error %s has empty language identifier", name)
			continue
		}
		if dup, ok := seen[lang]; ok {
			env.Errorf(file, pos, "error %s duplicate language %s (previous at %s)", name, lang, dup.Pos())
			continue
		}
		seen[lang] = plf
		xfmt, err := xlateErrorFormat(fmt, params)
		if err != nil {
			env.prefixErrorf(file, pos, err, "error %s language %s format invalid", name, lang)
			continue
		}
		lfs = append(lfs, LangFmt{lang, xfmt})
	}
	return lfs
}

var tagRE = regexp.MustCompile(`\{\:?([0-9a-zA-Z_]+)\:?\}`)

// xlateErrorFormat translates the user-supplied format into the format
// expected by i18n, mainly translating parameter names into numeric indexes.
func xlateErrorFormat(format string, params []*Field) (string, error) {
	const prefix = "{1:}{2:}"
	if format == "" {
		return prefix, nil
	}
	// Create a map from param name to index.  The index numbering starts at 3,
	// since the first two params are the component and op name, and i18n formats
	// use 1-based indices.
	pmap := make(map[string]string)
	for ix, param := range params {
		pmap[param.Name] = strconv.Itoa(ix + 3)
	}

	result, pos := prefix+" ", 0
	for _, match := range tagRE.FindAllStringSubmatchIndex(format, -1) {
		// The tag submatch indices are available as match[2], match[3]
		if len(match) != 4 || match[2] < pos || match[2] > match[3] {
			return "", fmt.Errorf("internal error: bad regexp indices %v", match)
		}
		beg, end := match[2], match[3]
		tag := format[beg:end]
		if tag == "_" {
			continue // Skip underscore tags.
		}
		if _, err := strconv.Atoi(tag); err == nil {
			continue // Skip number tags.
		}
		xtag, ok := pmap[tag]
		if !ok {
			return "", fmt.Errorf("unknown param %q", tag)
		}
		// Replace tag with xtag in the result.
		result += format[pos:beg]
		result += xtag
		pos = end
	}
	if end := len(format); pos < end {
		result += format[pos:end]
	}
	return result, nil
}
