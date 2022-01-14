// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compatibility_test

import (
	"os"
	"testing"

	"v.io/v23/context"
	"v.io/x/ref/test/compatibility"
)

func TestBuild(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	for _, main := range []string{
		"gosh/internal/gosh_example/main.go",
		"gosh/internal/gosh_example",
	} {
		tmpDir, err := os.MkdirTemp("", "testing")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)
		binary, cleanup, err := compatibility.BuildWithDependencies(ctx,
			"v.io/x/lib",
			compatibility.GOPATH(tmpDir),
			compatibility.Main(main),
			compatibility.Verbose(true),
			compatibility.Require("github.com/spf13/pflag", "v1.0.5-rc1"),
		)
		defer cleanup()
		if err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(binary)
		if !fi.Mode().IsRegular() {
			t.Errorf("%v is not a regular file", binary)
		}
		if (fi.Mode().Perm() & 0x100) == 0 {
			t.Errorf("%v is not an executable file", binary)
		}
	}
}
