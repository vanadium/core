// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import "testing"

func TestDoc(t *testing.T) {
	tests := []struct {
		Name   string
		Doc    string
		Suffix string
		Want   string
	}{
		{"Empty", "", "", ""},
		{"Basic", "// Package signature defines types representing interface and method signatures.\n", "",
			"/// Package signature defines types representing interface and method signatures.\n"},
		{"New line required", "// a", "", ""},
		{"Empty comment is ignored", "//\n", "", ""},
		{"Single-line whitespace only comment is ignored", "//   \n", "", ""},
		{"Multi-line whitespace only comment is ignored", "//   \n//   \n", "", ""},
		{"Empty comment line is respected", "// a\n//\n// b\n", "", "/// a\n///\n/// b\n"},
		{"Multiline respected", "// a\n// b\n", "", "/// a\n/// b\n"},
		{"Whitespace preserved", "//  a\n//b\n", "", "///  a\n///b\n"},
		{"Non-comments ignored", "a\n", "", ""},
		{"Suffix is exchangable", "", "// a\n", "/// a\n"},
		{"Suffix is used", "// a\n", "// a suffix\n", "/// a\n/// a suffix\n"},
		{"Just VDL's // is supported", "/* a */\n", "", ""},
		{"/// goes to ////", "///\n", "", "////\n"},
	}
	for _, test := range tests {
		if got, want := swiftDoc(test.Doc, test.Suffix), test.Want; got != want {
			t.Errorf("%s\n GOT %s\nWANT %s", test.Name, got, want)
		}
	}
}
