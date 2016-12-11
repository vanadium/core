// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"bytes"
	"log"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const primitiveTmpl = `{{ .Doc }}///
/// Auto-generated from {{.VdlTypeName}}
{{ .AccessModifier }} struct {{.Name}} : VdlPrimitive {{ if .IsHashable }}, Hashable {{ end }} {
  /// Vdl type for {@link {{.Name}}}.
  {{ .AccessModifier }} static let vdlTypeName = "{{.VdlTypeName}}"

  // Conform to RawRepresentable for the underlying Type
  {{ .AccessModifier }} associatedtype RawValue = {{.RawValue}}
  {{ .AccessModifier }} let rawValue: RawValue

  /// Creates a new instance of {@link {{.Name}}} with the given value.
  {{ .AccessModifier }} init(rawValue: {{.RawValue}}) {
    self.rawValue = rawValue
  }

  {{ .AccessModifier }} var debugDescription: String {
    return "{{ .Name }}(\(self.rawValue))"
  }

  {{ if .IsHashable }}
  {{ .AccessModifier }} var hashValue: Int {
    return self.rawValue.hashValue
  }
  {{ end }}
}

{{ if .IsHashable }}
{{ .AccessModifier }} func ==(lhs: {{.Name}}, rhs: {{.Name}}) -> Bool {
	return lhs.rawValue == rhs.rawValue
}{{ end }}`

// genSwiftPrimitiveFile generates the Swift class file for the provided user-defined type.
func genSwiftPrimitiveTdef(tdef *compile.TypeDef, ctx *swiftContext) string {
	data := struct {
		AccessModifier string
		Doc            string
		IsHashable     bool
		Name           string
		Source         string
		RawValue       string
		VdlTypeName    string
		VdlTypeKind    string
		VdlTypeString  string
	}{
		AccessModifier: swiftAccessModifier(tdef),
		Doc:            swiftDoc(tdef.Doc, tdef.DocSuffix),
		IsHashable:     isTypeHashable(tdef.Type, make(map[*vdl.Type]bool)),
		Name:           ctx.swiftTypeName(tdef),
		Source:         tdef.File.BaseName,
		RawValue:       ctx.swiftBuiltInType(tdef.Type),
		VdlTypeName:    tdef.Type.Name(),
		VdlTypeKind:    tdef.Type.Kind().String(),
		VdlTypeString:  tdef.Type.String(),
	}
	var buf bytes.Buffer
	err := parseTmpl("swift primitive", primitiveTmpl).Execute(&buf, data)
	if err != nil {
		log.Fatalf("vdl: couldn't execute primitive template: %v", err)
	}
	return formatSwiftCode(buf.String())
}
