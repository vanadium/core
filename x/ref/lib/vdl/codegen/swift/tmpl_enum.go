// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bytes"
	"log"

	"v.io/x/ref/lib/vdl/compile"
)

const enumTmpl = `{{ .Doc }}///
/// Auto-generated from {{.VdlTypeName}}
{{ .AccessModifier }} enum {{ .Name }} : String, VdlEnum {
    /// Vdl type for {@link {{ .Name }}}.
    {{ .AccessModifier }} static let vdlTypeName = "{{ .VdlTypeName }}"

	{{ range $index, $label := .EnumLabels }}
	{{ $label.Doc }}
	case {{ $label.Name }} = "{{ $label.Name }}"
    {{ end }}

    {{ .AccessModifier }} var vdlName: String {
    	return self.rawValue
    }

    {{ .AccessModifier }} var description: String {
    	return vdlName
    }

    {{ .AccessModifier }} var debugDescription: String {
    	return "{{ .Name }}.\(description)"
    }
}`

type enumLabel struct {
	Doc  string
	Name string
}

// genSwiftEnumFile generates the Swift class file for the provided user-defined enum type.
func genSwiftEnumTdef(tdef *compile.TypeDef, ctx *swiftContext) string {
	labels := make([]enumLabel, tdef.Type.NumEnumLabel())
	for i := 0; i < tdef.Type.NumEnumLabel(); i++ {
		labels[i] = enumLabel{
			Doc:  swiftDoc(tdef.LabelDoc[i], tdef.LabelDocSuffix[i]),
			Name: tdef.Type.EnumLabel(i),
		}
	}
	data := struct {
		AccessModifier string
		Doc            string
		EnumLabels     []enumLabel
		Name           string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: swiftAccessModifier(tdef),
		Doc:            swiftDoc(tdef.Doc, tdef.DocSuffix),
		EnumLabels:     labels,
		Name:           ctx.swiftTypeName(tdef),
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("swift enum", enumTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute enum template: %v", err)
	}
	return formatSwiftCode(buf.String())
}
