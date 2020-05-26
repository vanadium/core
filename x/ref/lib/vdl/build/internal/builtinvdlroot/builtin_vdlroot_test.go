// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package builtinvdlroot

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/x/lib/gosh"
	"v.io/x/ref/test/testutil"
)

// Ensures the vdlroot data built-in to the binary matches the current sources.
func TestBuiltInVDLRootIsUpToDate(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()
	gotRoot := sh.MakeTempDir()

	if err := RestoreAssets(gotRoot, ""); err != nil {
		t.Fatalf("Couldn't extract vdlroot: %v", err)
	}
	wantRoot := filepath.Join("..", "..", "..", "..", "..", "..", "..", "v23", "vdlroot")
	var debug bytes.Buffer
	opts := testutil.FileTreeOpts{
		Debug: &debug,
		FileB: regexp.MustCompile(`((\.vdl)|(vdl\.config))$`),
	}
	switch ok, err := testutil.FileTreeEqual(gotRoot, wantRoot, opts); {
	case err != nil:
		t.Error(err)
	case !ok:
		t.Errorf("%v is not the same as %v\n%v", gotRoot, wantRoot, debug.String())
	}
}
