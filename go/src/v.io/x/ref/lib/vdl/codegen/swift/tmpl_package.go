// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bytes"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const packageTmpl = header + `// Source: {{ .Source }}
{{ range $module := .ImportedModules }}
import {{ $module }}
{{ end }}

{{ if .IsModuleRoot }}
{{ if .Doc }}
{{ .Doc }}
{{ .AccessModifier }} enum Docs {}
{{ end }} {{/* if .Doc */}}
{{ else }}
{{ .Doc }}
{{ .AccessModifier }} enum {{ .PackageName }} {
{{ end }} {{/* if .IsModuleRoot */}}

  {{ range $file := .ConstFiles }}
  /* The following constants originate in file: {{ $file.Filename }} */
  {{ range $const := $file.Consts }}
  {{ $const.Doc }}
  {{ $const.AccessModifier }} {{ if not $.IsModuleRoot }}static {{ end }}let {{ $const.Name }}: {{ $const.Type }} = {{ $const.Value }}
  {{ end }} {{/* end range $file.Consts */}}
  {{ end }} {{/* range .ConstFiles */}}

  {{ range $file := .ErrorFiles }}
  /* The following errors originate in file: {{ $file.Filename }} */
  {{ range $error := $file.Errors }}
  {{ $error.Doc }}
  {{ $error.AccessModifier }} {{ if not $.IsModuleRoot }}static {{ end }}let {{ $error.Name }} = VError(
    identity: {{ $error.ID }},
    action: ErrorAction.{{ $error.ActionName }},
    msg: {{ $error.EnglishFmt }},
    stacktrace: nil)
  {{ end }} {{/* end range $file.Errors */}}
  {{ end }} {{/* range .ErrorFiles */}}

{{ if not .IsModuleRoot }}
} {{/* closes the enum { */}}
{{ end }}`

type constTmplDef struct {
	AccessModifier string
	Doc            string
	Type           string
	Name           string
	Value          string
}

type constsInFile struct {
	Filename        string
	Consts          []constTmplDef
	ImportedModules []string
}

func shouldGenerateConstFile(pkg *compile.Package) bool {
	for _, file := range pkg.Files {
		if len(file.ConstDefs) > 0 {
			return true
		}
	}
	return false
}

func parseConstants(pkg *compile.Package, ctx *swiftContext) []constsInFile {
	if !shouldGenerateConstFile(pkg) {
		return nil
	}

	files := []constsInFile{}
	for _, file := range pkg.Files {
		if len(file.ConstDefs) == 0 {
			continue
		}
		consts := make([]constTmplDef, len(file.ConstDefs))
		tdefs := []*compile.TypeDef{}
		for j, cnst := range file.ConstDefs {
			consts[j].AccessModifier = swiftAccessModifierForName(cnst.Name)
			consts[j].Doc = swiftDoc(cnst.Doc, cnst.DocSuffix)
			consts[j].Type = ctx.swiftType(cnst.Value.Type())
			consts[j].Name = vdlutil.FirstRuneToUpper(cnst.Name)
			consts[j].Value = swiftConstVal(cnst.Value, ctx)
			if tdef := ctx.env.FindTypeDef(cnst.Value.Type()); tdef != nil && tdef.Name != "" {
				tdefs = append(tdefs, tdef)
			}
		}

		files = append(files, constsInFile{
			Filename:        file.BaseName,
			Consts:          consts,
			ImportedModules: ctx.importedModules(tdefs),
		})
	}
	return files
}

// highestConstAccessModifier scans across many const definitions looking at
// their access modifier to determine the access modifier for the package
// that contains them. For example, if package foo contains two consts,
// one internal and the other public, the package itself must be public
// in order to access the public variable. If, instead, the package only
// contained internal consts, then the package has no need to be public.
func highestConstAccessModifier(constFiles []constsInFile) string {
	if len(constFiles) == 0 {
		// This is only for documentation at this point. Keep it public for others to read.
		// TODO(zinman): Search for the highest level of any emitted type across the entire
		// package to determine if this would be the only publicly visible artifact.
		return "public"
	}
	levels := map[string]int{"private": 0, "internal": 1, "public": 2}
	modifiers := map[int]string{0: "private", 1: "internal", 2: "public"}
	highest := levels["private"]
	for _, file := range constFiles {
		for _, cnst := range file.Consts {
			level, ok := levels[cnst.AccessModifier]
			if !ok {
				panic(fmt.Sprintf("Invalid access modifier: %v", cnst.AccessModifier))
			}
			if level > highest {
				highest = level
			}
		}
	}
	return modifiers[highest]
}

type errorTmplDef struct {
	AccessModifier string
	ActionName     string
	Doc            string
	EnglishFmt     string
	ID             string
	Name           string
}

type errorsInFile struct {
	Filename string
	Errors   []errorTmplDef
}

func shouldGenerateErrorFile(pkg *compile.Package) bool {
	for _, file := range pkg.Files {
		if len(file.ErrorDefs) > 0 {
			return true
		}
	}
	return false
}

func parseErrors(pkg *compile.Package, ctx *swiftContext) []errorsInFile {
	if !shouldGenerateErrorFile(pkg) {
		return nil
	}
	errorFiles := []errorsInFile{}
	for _, file := range pkg.Files {
		if len(file.ErrorDefs) == 0 {
			continue
		}
		errors := []errorTmplDef{}
		for _, err := range file.ErrorDefs {
			englishFmt := "nil"
			if err.English != "" {
				englishFmt = swiftQuoteString(err.English)
			}
			errors = append(errors, errorTmplDef{
				AccessModifier: swiftAccessModifierForName(err.Name),
				ActionName:     err.RetryCode.String(),
				Doc:            swiftDoc(err.Doc, err.DocSuffix),
				EnglishFmt:     englishFmt,
				ID:             swiftQuoteString(err.ID),
				Name:           vdlutil.FirstRuneToUpper(err.Name) + "Error",
			})
		}
		errorFiles = append(errorFiles, errorsInFile{
			Filename: file.BaseName,
			Errors:   errors,
		})
	}
	return errorFiles
}

func getDocFile(pkg *compile.Package) *compile.File {
	for _, file := range pkg.Files {
		if file.PackageDef.Doc != "" {
			return file
		}
	}
	return nil
}

func importedModulesForPkg(ctx *swiftContext, constFiles []constsInFile, errorFiles []errorsInFile) []string {
	modules := map[string]bool{}
	for _, constFile := range constFiles {
		for _, module := range constFile.ImportedModules {
			modules[module] = true
		}
	}
	if ctx.swiftModule(ctx.pkg) != V23SwiftFrameworkName && errorFiles != nil && len(errorFiles) > 0 {
		// We're going to define VErrors and we aren't generating for VandiumCore,
		// so we must import it since that's where VError is defined
		modules[V23SwiftFrameworkName] = true
	}
	return flattenImportedModuleSet(modules)
}

// genPackageFileSwift generates the Swift package info file, iif any package
// comments were specified in the package's VDL files OR constants OR errors.
func genSwiftPackageFile(pkg *compile.Package, ctx *swiftContext) string {
	constFiles := parseConstants(pkg, ctx)
	errorFiles := parseErrors(pkg, ctx)
	doc := ""
	docFile := getDocFile(pkg)
	if docFile != nil {
		doc = swiftDoc(docFile.PackageDef.Doc, docFile.PackageDef.DocSuffix)
	}
	// If everything is zero value then return
	if len(constFiles) == 0 && len(errorFiles) == 0 && doc == "" {
		return ""
	}
	source := path.Join(ctx.genPathToDir[pkg.GenPath], pkg.Files[0].BaseName)
	vdlRoot, err := ctx.vdlRootForFilePath(source)
	if err != nil {
		log.Fatalf("vdl: couldn't find vdl root for path %v: %v", source, err)
	}
	source = strings.TrimPrefix(source, vdlRoot+"/")
	source = filepath.Dir(source)
	data := struct {
		AccessModifier  string
		ConstFiles      []constsInFile
		Doc             string
		ErrorFiles      []errorsInFile
		FileDoc         string
		IsModuleRoot    bool
		ImportedModules []string
		PackageName     string
		PackagePath     string
		Source          string
	}{
		AccessModifier:  highestConstAccessModifier(constFiles),
		ConstFiles:      constFiles,
		Doc:             doc,
		ErrorFiles:      errorFiles,
		FileDoc:         pkg.FileDoc,
		IsModuleRoot:    ctx.swiftPackageName(pkg) == "",
		ImportedModules: importedModulesForPkg(ctx, constFiles, errorFiles),
		PackageName:     ctx.swiftPackageName(pkg),
		PackagePath:     swiftGenPkgPath(pkg.GenPath),
		Source:          source,
	}
	var buf bytes.Buffer
	err = parseTmpl("swift package", packageTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute package template: %v", err)
	}

	return formatSwiftCode(buf.String())
}
