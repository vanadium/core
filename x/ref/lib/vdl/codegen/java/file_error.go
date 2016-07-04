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

const errorTmpl = header + `
// Source(s): {{ .Source }}
package {{ .PackagePath }};

{{ .Doc }}
{{ .AccessModifier}} final class {{ .ClassName }} extends io.v.v23.verror.VException {
    {{ .AccessModifier }} static final io.v.v23.verror.VException.IDAction ID_ACTION = io.v.v23.verror.VException.register("{{ .ID }}", io.v.v23.verror.VException.ActionCode.{{ .ActionName }}, "{{ .EnglishFmt }}");

    {{ if gt (len .Formats) 0 }}
    static {
        {{ range $format := .Formats }}
        io.v.v23.i18n.Language.getDefaultCatalog().setWithBase("{{ $format.Lang }}", ID_ACTION.getID(), "{{ $format.Fmt }}");
        {{ end }}
    }
    {{ end }} {{/* if */}}

    {{ .AccessModifier }} {{ .ClassName }}(io.v.v23.context.VContext _ctx{{ .MethodArgs}}) {
        super(ID_ACTION, _ctx, new java.lang.Object[] { {{ .Params }} }, new java.lang.reflect.Type[]{ {{ .ParamTypes }} });
    }

    {{ .AccessModifier }} {{ .ClassName }}(String _language, String _componentName, String _opName{{ .MethodArgs}}) {
        super(ID_ACTION, _language, _componentName, _opName, new java.lang.Object[] { {{ .Params }} }); //, new java.lang.reflect.Type[]{ {{ .ParamTypes }} });
    }

    private {{ .ClassName }}(io.v.v23.verror.VException e) {
        super(e);
    }
}
`

type errorFormat struct {
	Lang string
	Fmt  string
}

// genJavaErrorFile generates the Java file for the provided error type.
func genJavaErrorFile(file *compile.File, err *compile.ErrorDef, env *compile.Env) JavaFileInfo {
	className := vdlutil.FirstRuneToUpper(err.Name) + "Exception"
	formats := make([]errorFormat, len(err.Formats))
	for k, format := range err.Formats {
		formats[k].Lang = string(format.Lang)
		formats[k].Fmt = format.Fmt
	}
	data := struct {
		AccessModifier string
		ActionName     string
		ClassName      string
		FileDoc        string
		Doc            string
		EnglishFmt     string
		Formats        []errorFormat
		ID             string
		MethodArgs     string
		PackagePath    string
		Params         string
		ParamTypes     string
		Source         string
	}{
		AccessModifier: accessModifierForName(err.Name),
		ActionName:     vdlutil.ToConstCase(err.RetryCode.String()),
		ClassName:      className,
		Doc:            javaDoc(err.Doc, err.DocSuffix),
		EnglishFmt:     err.English,
		FileDoc:        file.Package.FileDoc,
		Formats:        formats,
		ID:             err.ID,
		MethodArgs:     javaDeclarationArgStr(err.Params, env, true),
		PackagePath:    javaPath(javaGenPkgPath(file.Package.GenPath)),
		Params:         javaCallingArgStr(err.Params, false),
		ParamTypes:     javaCallingArgTypeStr(err.Params, env),
		Source:         file.BaseName,
	}
	var buf bytes.Buffer
	if e := parseTmpl("error", errorTmpl).Execute(&buf, data); e != nil {
		log.Fatalf("vdl: couldn't execute error template: %v", e)
	}
	return JavaFileInfo{
		Name: className + ".java",
		Data: buf.Bytes(),
	}
}
