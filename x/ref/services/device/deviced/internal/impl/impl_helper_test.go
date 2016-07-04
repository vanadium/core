// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl_test

// Separate from impl_test to avoid contributing further to impl_test bloat.
// TODO(rjkroege): Move all helper-related tests to here.

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"v.io/v23/context"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/device/deviced/internal/impl"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

func TestBaseCleanupDir(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ctx = context.WithLogger(ctx, logger.Global())
	dir, err := ioutil.TempDir("", "impl_helper_test")
	if err != nil {
		t.Fatalf("ioutil.TempDir() failed: %v", err)
	}
	defer os.RemoveAll(dir)

	// Setup some files to delete.
	helperTarget := path.Join(dir, "helper_target")
	if err := os.MkdirAll(helperTarget, os.FileMode(0700)); err != nil {
		t.Fatalf("os.MkdirAll(%s) failed: %v", helperTarget, err)
	}

	nohelperTarget := path.Join(dir, "nohelper_target")
	if err := os.MkdirAll(nohelperTarget, os.FileMode(0700)); err != nil {
		t.Fatalf("os.MkdirAll(%s) failed: %v", nohelperTarget, err)
	}

	// Setup a helper.
	helper := utiltest.GenerateSuidHelperScript(t, dir)

	impl.BaseCleanupDir(ctx, helperTarget, helper)
	if _, err := os.Stat(helperTarget); err == nil || os.IsExist(err) {
		t.Fatalf("%s should be missing but isn't", helperTarget)
	}

	impl.BaseCleanupDir(ctx, nohelperTarget, "")
	if _, err := os.Stat(nohelperTarget); err == nil || os.IsExist(err) {
		t.Fatalf("%s should be missing but isn't", nohelperTarget)
	}
}
