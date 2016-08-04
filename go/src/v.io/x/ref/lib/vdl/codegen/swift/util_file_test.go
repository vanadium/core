// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"errors"
	"testing"

	"v.io/x/ref/lib/vdl/compile"
)

func newVdlRootTestSwiftContext(srcDirs []string) *swiftContext {
	srcDirsSet := map[string]bool{}
	for _, srcDir := range srcDirs {
		srcDirsSet[srcDir] = true
	}
	ctx := swiftContext{nil, nil, map[string]string{}, srcDirsSet, map[*compile.TypeDef]string{}}
	return &ctx
}

func TestVdlRootForFilePath(t *testing.T) {
	tests := []struct {
		Name         string
		AbsolutePath string
		Ctx          *swiftContext
		WantRoot     string
		WantErr      error
	}{
		{"Must be absolute path", "not/absolute", newVdlRootTestSwiftContext(nil), "", errors.New("Path not absolute or unsupported file system")},
		{"Basic",
			"/home/user/vanadium/release/go/src/v.io/v23/syncbase/syncbase.vdl",
			newVdlRootTestSwiftContext([]string{"/home/user/vanadium/release/go/src/v.io/v23"}),
			"/home/user/vanadium/release/go/src/v.io/v23", nil},
		{"Basic two places",
			"/home/user/vanadium/release/go/src/v.io/v23/syncbase/syncbase.vdl",
			newVdlRootTestSwiftContext([]string{"/home/user/vanadium/release/go/src/v.io/v23", "/home/users/vanadium/third-party"}),
			"/home/user/vanadium/release/go/src/v.io/v23", nil},
		{"Longest path wins",
			"/home/user/vanadium/release/go/src/v.io/v23/services/syncbase/syncbase.vdl",
			newVdlRootTestSwiftContext([]string{"/home/user/vanadium/release/go/src/v.io/v23", "/home/user/vanadium/release/go/src/v.io/v23/services"}),
			"/home/user/vanadium/release/go/src/v.io/v23/services", nil},
		{"Vdlroot NOT found",
			"/home/user/vanadium/release/go/src/v.io/v23/services/syncbase/syncbase.vdl",
			newVdlRootTestSwiftContext([]string{"/somewhere/else"}),
			"", errors.New("Hit the root before any vdl paths were found")},
	}
	for _, test := range tests {
		gotRoot, gotErr := test.Ctx.vdlRootForFilePath(test.AbsolutePath)
		if gotRoot != test.WantRoot {
			t.Errorf("%s\n GOT %s\nWANT %s", test.Name, gotRoot, test.WantRoot)
		}
		// Errors can be nil in want or what's producted. Test if they're not both nil or not nil (XOR), or if is not expected IF we want an error
		if (gotErr != nil) != (test.WantErr != nil) || (test.WantErr != nil && gotErr.Error() != test.WantErr.Error()) {
			t.Errorf("%s\n GOT ERROR %v\nWANT ERROR %v", test.Name, gotErr, test.WantErr)
		}
	}
}
