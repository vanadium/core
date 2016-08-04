// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bytes"
	"fmt"
	"log"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const unionTmpl = `{{ .Doc }}///
/// Auto-generated from {{.VdlTypeName}}
{{ .AccessModifier }} enum {{.Name}} : VdlUnion {{ if .IsHashable }}, Hashable {{ end }} {
  /// Vdl type for {@link {{.Name}}}.
  {{ .AccessModifier }} static let vdlTypeName = "{{.VdlTypeName}}"

  {{ range $index, $field := .Fields }}
  {{ $field.Doc }}
  /// GeneratedFromVdl(name = "{{$field.VdlName}}", index = {{$index}})
  case {{$field.Name}}(elem:{{$field.Type}})
  {{ end }}

  {{ if .IsHashable }}
  {{ .AccessModifier }} var hashValue: Int {
    switch self {
    {{ range $field := .Fields }}
    case let .{{ $field.Name }}(elem): return {{$field.HashcodeComputation}}
    {{ end }}
    }
  }
  {{ end }}

  {{ .AccessModifier }} var vdlName: String {
    switch self {
    {{ range $field := .Fields }}
    case .{{ $field.Name }}: return "{{$field.VdlName}}"
    {{ end }}
    }
  }

  {{ .AccessModifier }} var description: String {
    return vdlName
  }

  {{ .AccessModifier }} var debugDescription: String {
    switch self {
    {{ range $field := .Fields }}
    {{ if .IsAny }}
    case let .{{ $field.Name }}(elem): return "{{$field.VdlName}}(\(elem))"
    {{ else }}
    case let .{{ $field.Name }}(elem): return "{{$field.VdlName}}(\(elem.debugDescription))"
    {{ end }}{{/* IsAny */}}
    {{ end }}{{/* range */}}
    }
  }
}

{{ if .IsHashable }}
{{ .AccessModifier }} func ==(lhs: {{.Name}}, rhs: {{.Name}}) -> Bool {
  switch (lhs, rhs) {
  {{ $EnumName := .Name }}
  {{ range $field := .Fields }}
  case ({{$EnumName}}.{{ $field.Name }}(let lElem), {{$EnumName}}.{{ $field.Name }}(let rElem)):
  {{ if $field.IsOptional }}
    switch (lElem, rElem) {
    case (nil, nil): return true
    case (nil, _): return false
    case (_, nil): return false
    case let (lfield?, rfield?): return lfield == rfield
    default: return false
    }
  {{ else }}
    return lElem == rElem
  {{ end }} {{/* optional comparison */}}
  {{ end }}
  default: return false
  }
}{{ end }}`

type unionDefinitionField struct {
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

// genSwiftUnionFile generates the Swift class file for the provided user-defined union type.
func genSwiftUnionTdef(tdef *compile.TypeDef, ctx *swiftContext) string {
	fields := make([]unionDefinitionField, tdef.Type.NumField())
	isHashable := isTypeHashable(tdef.Type, make(map[*vdl.Type]bool))
	for i := 0; i < tdef.Type.NumField(); i++ {
		fld := tdef.Type.Field(i)
		hashComp := ""
		if isHashable {
			var err error
			hashComp, err = ctx.swiftHashCode("elem", fld.Type)
			if err != nil {
				panic(fmt.Sprintf("Unable to compute hashcode: %v", err))
			}
		}
		fields[i] = unionDefinitionField{
			AccessModifier:      swiftAccessModifierForName(fld.Name),
			Class:               ctx.swiftType(fld.Type),
			Doc:                 swiftDoc(tdef.FieldDoc[i], tdef.FieldDocSuffix[i]),
			HashcodeComputation: hashComp,
			IsAny:               fld.Type.Kind() == vdl.Any,
			IsOptional:          fld.Type.Kind() == vdl.Optional,
			Name:                swiftCaseName(tdef, fld),
			Type:                ctx.swiftType(fld.Type),
			VdlName:             fld.Name,
		}
	}
	data := struct {
		AccessModifier string
		Doc            string
		Fields         []unionDefinitionField
		IsHashable     bool
		Name           string
		Source         string
		VdlTypeName    string
	}{
		AccessModifier: swiftAccessModifier(tdef),
		Doc:            swiftDoc(tdef.Doc, tdef.DocSuffix),
		Fields:         fields,
		IsHashable:     isHashable,
		Name:           ctx.swiftTypeName(tdef),
		Source:         tdef.File.BaseName,
		VdlTypeName:    tdef.Type.Name(),
	}
	var buf bytes.Buffer
	err := parseTmpl("swift union", unionTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute union template: %v", err)
	}
	return formatSwiftCode(buf.String())
}
