// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const listTmpl = header + `
// Source: {{.SourceFile}}
package {{.Package}};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends io.v.v23.vdl.VdlList<{{.ElemType}}> {
    private static final long serialVersionUID = 1L;

    /**
     * Vdl type for {@link {{.Name}}}.
     */
    public static final io.v.v23.vdl.VdlType VDL_TYPE =
            io.v.v23.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    /**
     * Creates a new instance of {@link {{.Name}}} with the given underlying value.
     *
     * @param impl underlying value
     */
    public {{.Name}}(java.util.List<{{.ElemType}}> impl) {
        super(VDL_TYPE, impl);
    }

    /**
     * Creates a new empty instance of {@link {{.Name}}}.
     */
    public {{.Name}}() {
        this(new java.util.ArrayList<{{.ElemType}}>());
    }
}
`

// genJavaListFile generates the Java class file for the provided named list type.
func genJavaListFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	name, access := javaTypeName(tdef, env)
	data := struct {
		AccessModifier string
		Doc            string
		ElemType       string
		FileDoc        string
		Name           string
		Package        string
		SourceFile     string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: access,
		Doc:            javaDoc(tdef.Doc, tdef.DocSuffix),
		ElemType:       javaType(tdef.Type.Elem(), true, env),
		FileDoc:        tdef.File.Package.FileDoc,
		Name:           name,
		Package:        javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		SourceFile:     tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("list", listTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute list template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
