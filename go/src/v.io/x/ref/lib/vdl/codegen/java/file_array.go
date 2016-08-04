// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"v.io/x/ref/lib/vdl/compile"
)

const arrayTmpl = header + `
// Source: {{.SourceFile}}

package {{.Package}};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
@io.v.v23.vdl.ArrayLength({{.Length}})
{{ .AccessModifier }} class {{.Name}} extends io.v.v23.vdl.VdlArray<{{.ElemType}}> {
    private static final long serialVersionUID = 1L;

    /**
     * Vdl type for {@link {{.Name}}}.
     */
    public static final io.v.v23.vdl.VdlType VDL_TYPE =
            io.v.v23.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    /**
     * Creates a new instance of {@link {{.Name}}} with the given underlying array.
     *
     * @param arr underlying array
     */
    public {{.Name}}({{.ElemType}}[] arr) {
        super(VDL_TYPE, arr);
    }

    /**
     * Creates a new zero-value instance of {@link {{.Name}}}.
     */
    public {{.Name}}() {
        this({{.ZeroValue}});
    }

    {{ if .ElemIsPrimitive }}
    /**
     * Creates a new instance of {@link {{.Name}}} with the given underlying array.
     *
     * @param arr underlying array
     */
    public {{.Name}}({{ .ElemPrimitiveType }}[] arr) {
        super(VDL_TYPE, convert(arr));
    }

	/**
	 * Converts the array into its primitive (array) type.
	 */
	public {{ .ElemPrimitiveType }}[] toPrimitiveArray() {
		{{ .ElemPrimitiveType }}[] ret = new {{ .ElemPrimitiveType }}[size()];
		for (int i = 0; i < size(); ++i) {
			ret[i] = get(i);
		}
		return ret;
	}

    private static {{ .ElemType }}[] convert({{ .ElemPrimitiveType }}[] arr) {
        {{ .ElemType }}[] ret = new {{ .ElemType }}[arr.length];
        for (int i = 0; i < arr.length; ++i) {
            ret[i] = arr[i];
        }
        return ret;
    }
    {{ end }}
}
`

// genJavaArrayFile generates the Java class file for the provided named array type.
func genJavaArrayFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	name, access := javaTypeName(tdef, env)
	elemType := javaType(tdef.Type.Elem(), true, env)
	elems := strings.TrimSuffix(strings.Repeat(javaZeroValue(tdef.Type.Elem(), env)+", ", tdef.Type.Len()), ", ")
	zeroValue := fmt.Sprintf("new %s[] {%s}", elemType, elems)
	data := struct {
		AccessModifier    string
		Doc               string
		ElemType          string
		ElemIsPrimitive   bool
		ElemPrimitiveType string
		FileDoc           string
		Length            int
		Name              string
		Package           string
		SourceFile        string
		VdlTypeName       string
		VdlTypeString     string
		ZeroValue         string
	}{
		AccessModifier:    access,
		Doc:               javaDoc(tdef.Doc, tdef.DocSuffix),
		ElemType:          elemType,
		ElemIsPrimitive:   !isClass(tdef.Type.Elem(), env),
		ElemPrimitiveType: javaType(tdef.Type.Elem(), false, env),
		FileDoc:           tdef.File.Package.FileDoc,
		Length:            tdef.Type.Len(),
		Name:              name,
		Package:           javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		SourceFile:        tdef.File.BaseName,
		VdlTypeName:       tdef.Type.Name(),
		VdlTypeString:     tdef.Type.String(),
		ZeroValue:         zeroValue,
	}
	var buf bytes.Buffer
	err := parseTmpl("array", arrayTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute array template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
