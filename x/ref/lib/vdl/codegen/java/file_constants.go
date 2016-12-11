// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
	"v.io/x/ref/lib/vdl/vdlutil"
)

const constTmpl = header + `
// Source(s): {{ .Source }}
package {{ .PackagePath }};

/**
 * Constants defined in all VDL files in this package.
 */
public final class {{ .ClassName }} {
    {{ range $file := .Files }}

    /* The following constants originate in file: {{ $file.Name }} */
    {{ range $const := $file.Consts }}
    {{ $const.Doc }}
    {{ $const.AccessModifier }} static final {{ $const.Type }} {{ $const.Name }} = {{ $const.Value }};
    {{ end }} {{/* end range $file.Consts */}}
    {{ end }} {{/* range .Files */}}

    private {{ .ClassName }}() {}
}
`

type constConst struct {
	AccessModifier string
	Doc            string
	Type           string
	Name           string
	Value          string
}

type constFile struct {
	Name   string
	Consts []constConst
}

func shouldGenerateConstFile(pkg *compile.Package) bool {
	for _, file := range pkg.Files {
		if len(file.ConstDefs) > 0 {
			return true
		}
	}
	return false
}

// genJavaConstFile generates the (single) Java file that contains constant
// definitions from all the VDL files.
func genJavaConstFile(pkg *compile.Package, env *compile.Env) *JavaFileInfo {
	if !shouldGenerateConstFile(pkg) {
		return nil
	}

	className := "Constants"

	files := make([]constFile, len(pkg.Files))
	for i, file := range pkg.Files {
		consts := make([]constConst, len(file.ConstDefs))
		for j, cnst := range file.ConstDefs {
			consts[j].AccessModifier = accessModifierForName(cnst.Name)
			consts[j].Doc = javaDoc(cnst.Doc, cnst.DocSuffix)
			consts[j].Type = javaType(cnst.Value.Type(), false, env)
			consts[j].Name = vdlutil.ToConstCase(cnst.Name)
			consts[j].Value = javaConstVal(cnst.Value, env)
		}
		files[i].Name = file.BaseName
		files[i].Consts = consts
	}

	data := struct {
		ClassName   string
		FileDoc     string
		Files       []constFile
		PackagePath string
		Source      string
	}{
		ClassName:   className,
		FileDoc:     pkg.FileDoc,
		Files:       files,
		PackagePath: javaPath(javaGenPkgPath(pkg.GenPath)),
		Source:      javaFileNames(pkg.Files),
	}
	var buf bytes.Buffer
	err := parseTmpl("const", constTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute const template: %v", err)
	}
	return &JavaFileInfo{
		Name: className + ".java",
		Data: buf.Bytes(),
	}
}
