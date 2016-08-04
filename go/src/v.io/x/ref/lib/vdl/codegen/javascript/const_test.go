// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"testing"

	"v.io/v23/vdl"
)

func TestTypedConst(t *testing.T) {
	names, structType, _, _, err := getTestTypes()
	if err != nil {
		t.Fatalf("Error in getTestTypes(): %v", err)
	}
	structValue := vdl.ZeroValue(structType)
	_, index := structValue.Type().FieldByName(unnamedTypeFieldName)
	structValue.StructField(index).AssignLen(1)
	structValue.StructField(index).Index(0).AssignString("AStringVal")

	tests := []struct {
		name       string
		inputValue *vdl.Value
		expected   string
	}{
		{
			name:       "struct test",
			inputValue: structValue,
			expected: `canonicalize.reduce(new (vdl.registry.lookupOrCreateConstructor(_typeNamedStruct))({
  'list': [
],
  'bool': false,
  'unnamedTypeField': [
"AStringVal",
],
}, true), _typeNamedStruct)`,
		},
		{
			name:       "bytes test",
			inputValue: vdl.BytesValue(nil, []byte{1, 2, 3, 4}),
			expected: `canonicalize.reduce(new (vdl.registry.lookupOrCreateConstructor(_type2))(new Uint8Array([
1,
2,
3,
4,
]), true), _type2)`,
		},
	}
	for _, test := range tests {
		strVal := typedConst(names, test.inputValue)
		if strVal != test.expected {
			t.Errorf("In %q, expected %q, but got %q", test.name, test.expected, strVal)
		}
	}
}

// TODO(bjornick) Add more thorough tests.
