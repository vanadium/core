// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package javascript

import (
	"testing"

	"v.io/v23/i18n"
	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/compile"
)

func TestError(t *testing.T) {
	e := &compile.ErrorDef{
		NamePos: compile.NamePos{
			Name: "Test",
		},
		ID:        "v.io/x/ref/lib/vdl/codegen/javascript.Test",
		RetryCode: vdl.WireRetryCodeNoRetry,
		Params: []*compile.Field{
			{
				NamePos: compile.NamePos{
					Name: "x",
				},
				Type: vdl.BoolType,
			},
			{
				NamePos: compile.NamePos{
					Name: "y",
				},
				Type: vdl.Int32Type,
			},
		},
		Formats: []compile.LangFmt{
			{
				Lang: i18n.LangID("en-US"),
				Fmt:  "english string",
			},
			{
				Lang: i18n.LangID("fr"),
				Fmt:  "french string",
			},
		},
	}
	var names typeNames
	result := generateErrorConstructor(names, e)
	expected := `module.exports.TestError = makeError('v.io/x/ref/lib/vdl/codegen/javascript.Test', actions.NO_RETRY, {
  'en-US': 'english string',
  'fr': 'french string',
}, [
  vdl.types.BOOL,
  vdl.types.INT32,
]);
`
	if result != expected {
		t.Errorf("got %s, want %s", result, expected)
	}
}
