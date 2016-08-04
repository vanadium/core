// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const enumTmpl = header + `
// Source: {{.Source}}
package {{.PackagePath}};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends io.v.v23.vdl.VdlEnum {
    {{ range $index, $label := .EnumLabels }}
        @io.v.v23.vdl.GeneratedFromVdl(name = "{{$label.Name}}", index = {{$index}})
        {{ $label.Doc }}
        public static final {{$.Name}} {{$label.Name}};
    {{ end }}

    /**
     * Vdl type for {@link {{.Name}}}.
     */
    public static final io.v.v23.vdl.VdlType VDL_TYPE =
            io.v.v23.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    static {
        {{ range $label := .EnumLabels }}
            {{$label.Name}} = new {{$.Name}}("{{$label.Name}}");
        {{ end }}
    }

    private {{.Name}}(String name) {
        super(VDL_TYPE, name);
    }

    /**
     * Returns the enum with the given name.
     */
    public static {{.Name}} valueOf(String name) {
        {{ range $label := .EnumLabels }}
            if ("{{$label.Name}}".equals(name)) {
                return {{$label.Name}};
            }
        {{ end }}
        throw new java.lang.IllegalArgumentException();
    }
}
`

type enumLabel struct {
	Doc  string
	Name string
}

// genJavaEnumFile generates the Java class file for the provided user-defined enum type.
func genJavaEnumFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	labels := make([]enumLabel, tdef.Type.NumEnumLabel())
	for i := 0; i < tdef.Type.NumEnumLabel(); i++ {
		labels[i] = enumLabel{
			Doc:  javaDoc(tdef.LabelDoc[i], tdef.LabelDocSuffix[i]),
			Name: tdef.Type.EnumLabel(i),
		}
	}
	name, access := javaTypeName(tdef, env)
	data := struct {
		AccessModifier string
		Doc            string
		EnumLabels     []enumLabel
		FileDoc        string
		Name           string
		PackagePath    string
		Source         string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: access,
		Doc:            javaDoc(tdef.Doc, tdef.DocSuffix),
		EnumLabels:     labels,
		FileDoc:        tdef.File.Package.FileDoc,
		Name:           name,
		PackagePath:    javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		Source:         tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("enum", enumTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute enum template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
