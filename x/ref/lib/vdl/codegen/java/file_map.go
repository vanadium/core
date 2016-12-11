// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const mapTmpl = header + `
// Source: {{.SourceFile}}

package {{.Package}};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends io.v.v23.vdl.VdlMap<{{.KeyType}}, {{.ElemType}}> {
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
    public {{.Name}}(java.util.Map<{{.KeyType}}, {{.ElemType}}> impl) {
        super(VDL_TYPE, impl);
    }

    /**
     * Creates a new empty instance of {@link {{.Name}}}.
     */
    public {{.Name}}() {
        this(new java.util.HashMap<{{.KeyType}}, {{.ElemType}}>());
    }
}
`

// genJavaMapFile generates the Java class file for the provided named map type.
func genJavaMapFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	name, access := javaTypeName(tdef, env)
	data := struct {
		AccessModifier string
		Doc            string
		ElemType       string
		FileDoc        string
		KeyType        string
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
		KeyType:        javaType(tdef.Type.Key(), true, env),
		Name:           name,
		Package:        javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		SourceFile:     tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.BaseType.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("map", mapTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute map template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
