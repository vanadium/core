// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"fmt"
	"testing"

	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

const unnamedTypeFieldName = "UnnamedTypeField"

func getTestTypes() (names typeNames, tyStruct, tyList, tyBool *vdl.Type, outErr error) {
	var builder vdl.TypeBuilder
	namedBool := builder.Named("otherPkg.NamedBool").AssignBase(vdl.BoolType)
	listType := builder.List()
	namedList := builder.Named("NamedList").AssignBase(listType)
	structType := builder.Struct()
	namedStruct := builder.Named("NamedStruct").AssignBase(structType)
	structType.AppendField("List", namedList)
	structType.AppendField("Bool", namedBool)
	structType.AppendField(unnamedTypeFieldName, builder.List().AssignElem(vdl.StringType))
	listType.AssignElem(namedStruct)
	builder.Build()

	builtBool, err := namedBool.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedBool: %v", err)
		return
	}

	builtList, err := namedList.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedList %v", err)
		return
	}

	builtStruct, err := namedStruct.Built()
	if err != nil {
		outErr = fmt.Errorf("Error creating NamedStruct: %v", err)
		return
	}

	pkg := &compile.Package{
		Files: []*compile.File{
			{
				TypeDefs: []*compile.TypeDef{
					{
						Type: builtList,
					},
					{
						Type: builtStruct,
					},
					{
						Type: vdl.ListType(vdl.ByteType),
					},
					{
						Type: vdl.NamedType("ColorsBeginningWithAOrB", vdl.EnumType("Aqua", "Beige")),
					},
				},
			},
		},
	}

	return newTypeNames(pkg), builtStruct, builtList, builtBool, nil
}

// TestType tests that the output string of generated types is what we expect.
func TestType(t *testing.T) {
	jsnames, _, _, _, err := getTestTypes()
	if err != nil {
		t.Fatalf("Error in getTestTypes(): %v", err)
	}
	result := makeTypeDefinitionsString(jsnames)

	expectedResult := `var _type1 = new vdl.Type();
var _type2 = new vdl.Type();
var _typeColorsBeginningWithAOrB = new vdl.Type();
var _typeNamedList = new vdl.Type();
var _typeNamedStruct = new vdl.Type();
_type1.kind = vdl.kind.LIST;
_type1.name = "";
_type1.elem = vdl.types.STRING;
_type2.kind = vdl.kind.LIST;
_type2.name = "";
_type2.elem = vdl.types.BYTE;
_typeColorsBeginningWithAOrB.kind = vdl.kind.ENUM;
_typeColorsBeginningWithAOrB.name = "ColorsBeginningWithAOrB";
_typeColorsBeginningWithAOrB.labels = ["Aqua", "Beige"];
_typeNamedList.kind = vdl.kind.LIST;
_typeNamedList.name = "NamedList";
_typeNamedList.elem = _typeNamedStruct;
_typeNamedStruct.kind = vdl.kind.STRUCT;
_typeNamedStruct.name = "NamedStruct";
_typeNamedStruct.fields = [{name: "List", type: _typeNamedList}, {name: "Bool", type: new otherPkg.NamedBool()._type}, {name: "UnnamedTypeField", type: _type1}];
_type1.freeze();
_type2.freeze();
_typeColorsBeginningWithAOrB.freeze();
_typeNamedList.freeze();
_typeNamedStruct.freeze();
module.exports.ColorsBeginningWithAOrB = {
  AQUA: canonicalize.reduce(new (vdl.registry.lookupOrCreateConstructor(_typeColorsBeginningWithAOrB))('Aqua', true), _typeColorsBeginningWithAOrB),
  BEIGE: canonicalize.reduce(new (vdl.registry.lookupOrCreateConstructor(_typeColorsBeginningWithAOrB))('Beige', true), _typeColorsBeginningWithAOrB),
};
module.exports.NamedList = (vdl.registry.lookupOrCreateConstructor(_typeNamedList));
module.exports.NamedStruct = (vdl.registry.lookupOrCreateConstructor(_typeNamedStruct));
`

	if result != expectedResult {
		t.Errorf("Expected %q, but got %q", expectedResult, result)
	}
}
