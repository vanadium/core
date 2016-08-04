// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const unionTmpl = header + `
// Source: {{.Source}}
package {{.PackagePath}};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends io.v.v23.vdl.VdlUnion {
	private static final long serialVersionUID = 1L;

    {{ range $index, $field := .Fields }}
    {{ $field.Doc }}
    @io.v.v23.vdl.GeneratedFromVdl(name = "{{$field.Name}}", index = {{$index}})
    public static class {{$field.Name}} extends {{$.Name}} {
    	private static final long serialVersionUID = 1L;
        private {{$field.Type}} elem;

        public {{$field.Name}}({{$field.Type}} elem) {
            super({{$index}}, elem);
            this.elem = elem;
        }

        public {{$field.Name}}() {
            this({{$field.ZeroValue}});
        }

        @Override
        public {{$field.Class}} getElem() {
            return elem;
        }

        @Override
        public int hashCode() {
            return {{$field.HashcodeComputation}};
        }
    }
    {{ end }}

    public static final io.v.v23.vdl.VdlType VDL_TYPE =
            io.v.v23.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    public {{.Name}}(int index, Object value) {
        super(VDL_TYPE, index, value);
    }
}
`

type unionDefinitionField struct {
	Class               string
	Doc                 string
	HashcodeComputation string
	Name                string
	Type                string
	ZeroValue           string
}

// genJavaUnionFile generates the Java class file for the provided user-defined union type.
func genJavaUnionFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	fields := make([]unionDefinitionField, tdef.Type.NumField())
	for i := 0; i < tdef.Type.NumField(); i++ {
		fld := tdef.Type.Field(i)
		fields[i] = unionDefinitionField{
			Class:               javaType(fld.Type, true, env),
			Doc:                 javaDoc(tdef.FieldDoc[i], tdef.FieldDocSuffix[i]),
			HashcodeComputation: javaHashCode("elem", fld.Type, env),
			Name:                fld.Name,
			Type:                javaType(fld.Type, false, env),
			ZeroValue:           javaZeroValue(fld.Type, env),
		}
	}
	name, access := javaTypeName(tdef, env)
	data := struct {
		FileDoc        string
		AccessModifier string
		Doc            string
		Fields         []unionDefinitionField
		Name           string
		PackagePath    string
		Source         string
		VdlTypeName    string
		VdlTypeString  string
	}{
		FileDoc:        tdef.File.Package.FileDoc,
		AccessModifier: access,
		Doc:            javaDoc(tdef.Doc, tdef.DocSuffix),
		Fields:         fields,
		Name:           name,
		PackagePath:    javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		Source:         tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("union", unionTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute union template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
