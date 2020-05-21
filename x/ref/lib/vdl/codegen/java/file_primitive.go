// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package java

import (
	"bytes"
	"log"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const primitiveTmpl = header + `
// Source: {{ .Source }}
package {{ .PackagePath }};

{{ .Doc }}
@io.v.v23.vdl.GeneratedFromVdl(name = "{{.VdlTypeName}}")
{{ .AccessModifier }} class {{.Name}} extends {{.VdlType}} {
    private static final long serialVersionUID = 1L;

    /**
     * Vdl type for {@link {{.Name}}}.
     */
    public static final io.v.v23.vdl.VdlType VDL_TYPE =
            io.v.v23.vdl.Types.getVdlTypeFromReflect({{.Name}}.class);

    /**
     * Creates a new instance of {@link {{.Name}}} with the given value.
     *
     * @param value value
     */
    public {{.Name}}({{.ConstructorType}} value) {
        super(VDL_TYPE, value);
    }

    /**
     * Creates a new zero-value instance of {@link {{.Name}}}.
     */
    public {{.Name}}() {
        super(VDL_TYPE);
    }
}
`

// javaConstructorType returns java type that is used as a constructor argument
// type for a VDL primitive.
func javaConstructorType(t *vdl.Type) string {
	switch t.Kind() {
	case vdl.Byte, vdl.Int8:
		return "byte"
	case vdl.Uint16, vdl.Int16:
		return "short"
	case vdl.Uint32, vdl.Int32:
		return "int"
	case vdl.Uint64, vdl.Int64:
		return "long"
	default:
		constructorType, _ := javaBuiltInType(t, false)
		return constructorType
	}
}

// javaConstructorType returns java class that is used as a type adapter delegate
// argument for a VDL primitive.
func javaTypeAdapterDelegateClass(t *vdl.Type) string {
	switch t.Kind() {
	case vdl.Byte, vdl.Int8:
		return "java.lang.Byte"
	case vdl.Uint16, vdl.Int16:
		return "java.lang.Short"
	case vdl.Uint32, vdl.Int32:
		return "java.lang.Integer"
	case vdl.Uint64, vdl.Int64:
		return "java.lang.Long"
	default:
		typeAdapterDelegateClass, _ := javaBuiltInType(t, true)
		return typeAdapterDelegateClass
	}
}

// genJavaPrimitiveFile generates the Java class file for the provided user-defined type.
func genJavaPrimitiveFile(tdef *compile.TypeDef, env *compile.Env) JavaFileInfo {
	name, access := javaTypeName(tdef, env)
	data := struct {
		AccessModifier           string
		ConstructorType          string
		Doc                      string
		FileDoc                  string
		Name                     string
		PackagePath              string
		Source                   string
		TypeAdapterDelegateClass string
		VdlType                  string
		VdlTypeName              string
		VdlTypeString            string
	}{
		AccessModifier:           access,
		Doc:                      javaDoc(tdef.Doc, tdef.DocSuffix),
		ConstructorType:          javaConstructorType(tdef.Type),
		FileDoc:                  tdef.File.Package.FileDoc,
		Name:                     name,
		PackagePath:              javaPath(javaGenPkgPath(tdef.File.Package.GenPath)),
		Source:                   tdef.File.BaseName,
		TypeAdapterDelegateClass: javaTypeAdapterDelegateClass(tdef.Type),
		VdlType:                  javaVdlPrimitiveType(tdef.Type.Kind()),
		VdlTypeName:              tdef.Type.Name(),
		VdlTypeString:            tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("primitive", primitiveTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute primitive template: %v", err)
	}
	return JavaFileInfo{
		Name: name + ".java",
		Data: buf.Bytes(),
	}
}
