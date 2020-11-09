// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltool

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
)

// Ensure that a config can be loaded to a concrete value.
// The reason this test is useful is to ensure that either the generated VdlConfig
// target works correctly, or if it is not generated it is still possible to write
// to the config using reflect.
// If this is not the case (perhaps because of a broken vdltool.vdl.go),
// unexpected behavior can result when generating files.
func TestLoadingVdlConfig(t *testing.T) {
	config := Config{
		Go: GoConfig{
			WireToNativeTypes: map[string]GoType{
				"WireString":       {Type: "string"},
				"WireMapStringInt": {Type: "map[string]int"},
				"WireTime": {
					Type:    "time.Time",
					Imports: []GoImport{{Path: "time", Name: "time"}},
				},
				"WireSamePkg": {
					Type:    "nativetest.NativeSamePkg",
					Imports: []GoImport{{Path: "v.io/v23/vdl/testdata/nativetest", Name: "nativetest"}},
				},
				"WireMultiImport": {
					Type: "map[nativetest.NativeSamePkg]time.Time",
					Imports: []GoImport{
						{Path: "v.io/v23/vdl/testdata/nativetest", Name: "nativetest"},
						{Path: "time", Name: "time"},
					},
				},
			},
			StructTags: map[string][]GoStructTag{
				"StructTypeName": {
					{Field: "NameOfField", Tag: `json:"name,omitempty" other:"name,,'quoted'"`},
				},
			},
		},
		Java: JavaConfig{
			WireTypeRenames: map[string]string{
				"WireRenameMe": "WireRenamed",
			},

			WireToNativeTypes: map[string]string{
				"WireString":       "java.lang.String",
				"WireMapStringInt": "java.util.Map<java.lang.String, java.lang.Integer>",
				"WireTime":         "org.joda.time.DateTime",
				"WireSamePkg":      "io.v.v23.vdl.testdata.nativetest.NativeSamePkg",
				"WireMultiImport":  "java.util.Map<io.v.v23.vdl.testdata.nativetest.NativeSamePkg, org.joda.time.DateTime>",
				"WireRenamed":      "java.lang.Long",
			},
		},
	}

	configVdlValue := vdl.ValueOf(config)

	var finalConfig Config
	if err := vdl.Convert(&finalConfig, configVdlValue); err != nil {
		t.Fatalf("error in Convert: %v", err)
	}

	if got, want := finalConfig, config; !reflect.DeepEqual(got, want) {
		t.Errorf("\n got: %#v\nwant: %#v", got, want)
	}
}
