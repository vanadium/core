// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of self source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bytes"
	"log"

	"fmt"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const structTmpl = `{{ .Doc }}///
/// Auto-generated from {{.VdlTypeName}}
{{ .AccessModifier }} class {{.Name}} : VdlStruct, CustomDebugStringConvertible {{ if .IsHashable }}, Hashable {{ end }}{
    /// Vdl type for {@link {{.Name}}}.
    {{ .AccessModifier }} static let vdlTypeName = "{{.VdlTypeName}}"

    {{/* Field declarations */}}
    {{ range $index, $field := .Fields }}
    /// "{{$field.VdlName}}" index {{$index}}
    var {{$field.Name}}: {{$field.Type}}
    {{ end }}

    {{ .AccessModifier }} init({{ range $index, $field := .Fields }}{{ if gt $index 0 }},{{ end }}
        {{$field.Name}}: {{$field.Type}}{{end}}) {

      {{range $field := .Fields}}self.{{$field.Name}} = {{$field.Name}}
      {{end}}{{/* range $field */}}
    }
    {{ if .IsHashable }}
    {{ .AccessModifier }} var hashValue: Int {
      var result = 1
      let prime = 31
      {{ range $field := .Fields }}result = prime * result + Int({{$field.HashcodeComputation}})
      {{ end }}
      return result
    }
    {{ end }}

    {{ .AccessModifier }} var description: String {
      return "{{ .VdlTypeName }}"
    }

    {{ .AccessModifier }} var debugDescription: String {
      var result = "[{{ .VdlTypeName }} "
      {{ range $index, $field := .Fields }}
      {{ if gt $index 0 }}result += ", "{{ end }}
      {{ if .IsAny }}
      result += "{{$field.Name}}:" + (({{$field.Name}} as? CustomDebugStringConvertible)?.debugDescription ?? "\({{$field.Name}})")
      {{ else }}
      result += "{{$field.Name}}:" + {{$field.Name}}.debugDescription
      {{ end }}{{/* IsAny */}}
      {{ end }}{{/* range over fields */}}
      return result + "]"
    }
}

{{ if .IsHashable }}
{{ .AccessModifier }} func ==(lhs: {{.Name}}, rhs: {{.Name}}) -> Bool {
	{{ range $field := .Fields }}
	{{ if $field.IsOptional }}
	switch (lhs.{{$field.Name}}, rhs.{{$field.Name}}) {
		case (nil, nil): break
		case (nil, _): return false
		case (_, nil): return false
		case let (lfield?, rfield?):
		 	if lfield != rfield {
		 	  return false
		 	}
		default: break
	}
	{{ else }}
	if lhs.{{$field.Name}} != rhs.{{$field.Name}} {
		return false
	}
	{{ end }} {{/* optional comparison */}}
	{{ end }} {{/* range over fields */}}
	return true
}{{ end }}`

type structDefinitionField struct {
	AccessModifier      string
	Class               string
	Doc                 string
	HashcodeComputation string
	IsAny               bool
	IsOptional          bool
	Name                string
	Type                string
	VdlName             string
}

func swiftFieldArgStr(tdef *compile.TypeDef, ctx *swiftContext) string {
	var buf bytes.Buffer
	for i := 0; i < tdef.Type.NumField(); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		fld := tdef.Type.Field(i)
		buf.WriteString(ctx.swiftType(fld.Type))
		buf.WriteString(" ")
		buf.WriteString(swiftVariableName(tdef, fld))
	}
	return buf.String()
}

// genSwiftStructFile generates the Swift class file for the provided user-defined type.
func genSwiftStructTdef(tdef *compile.TypeDef, ctx *swiftContext) string {
	isHashable := isTypeHashable(tdef.Type, make(map[*vdl.Type]bool))
	fields := make([]structDefinitionField, tdef.Type.NumField())
	for i := 0; i < tdef.Type.NumField(); i++ {
		fld := tdef.Type.Field(i)
		hashComp := ""
		if isHashable {
			var err error
			hashComp, err = ctx.swiftHashCode("self."+swiftVariableName(tdef, fld), fld.Type)
			if err != nil {
				panic(fmt.Sprintf("Unable to compute hashcode: %v", err))
			}
		}
		fields[i] = structDefinitionField{
			AccessModifier:      swiftAccessModifierForName(fld.Name),
			Class:               ctx.swiftType(fld.Type),
			Doc:                 swiftDoc(tdef.FieldDoc[i], tdef.FieldDocSuffix[i]),
			HashcodeComputation: hashComp,
			IsAny:               fld.Type.Kind() == vdl.Any,
			IsOptional:          fld.Type.Kind() == vdl.Optional,
			Name:                swiftVariableName(tdef, fld),
			Type:                ctx.swiftType(fld.Type),
			VdlName:             fld.Name,
		}
	}

	// Determine if we can create a hashValue out of this. A hashValue implies
	// equality and access to primitives in a useful way. With Any types we cannot
	// do this. One plausible hack is to use "\(x)", but if for some reason the
	// underlying type doesn't convey enough information in its forced string
	// conversion then we're not properly comparing.
	data := struct {
		AccessModifier string
		Doc            string
		Fields         []structDefinitionField
		FieldsAsArgs   string
		IsHashable     bool
		Name           string
		Source         string
		VdlTypeName    string
		VdlTypeString  string
	}{
		AccessModifier: swiftAccessModifier(tdef),
		Doc:            swiftDoc(tdef.Doc, tdef.DocSuffix),
		Fields:         fields,
		FieldsAsArgs:   swiftFieldArgStr(tdef, ctx),
		IsHashable:     isHashable,
		Name:           ctx.swiftTypeName(tdef),
		Source:         tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("swift struct", structTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute struct template: %v", err)
	}
	return formatSwiftCode(buf.String())
}
