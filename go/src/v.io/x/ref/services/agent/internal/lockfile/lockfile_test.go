// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lockfile_test contains an integration test for the lockfile package.
package lockfile_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"v.io/x/lib/gosh"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/agent/internal/lockfile"
	"v.io/x/ref/services/agent/internal/lockutil"
	"v.io/x/ref/test/v23test"
)

var createLockfile = gosh.RegisterFunc("createLockfile", func(file string) {
	err := lockfile.CreateLockfile(file)
	if err == nil {
		fmt.Println("Grabbed lock")
	} else {
		fmt.Println("Lock failed")
	}
})

func TestLockFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "lf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "myfile")
	if err = lockfile.CreateLockfile(file); err != nil {
		t.Fatal(err)
	}
	lockpath := file + "-lock"
	bytes, err := ioutil.ReadFile(lockpath)
	if err != nil {
		t.Fatal(err)
	}
	running, err := lockutil.StillHeld(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !running {
		t.Fatal("expected StillHeld() = true")
	}

	if err = lockfile.CreateLockfile(file); err == nil {
		t.Fatal("Creating 2nd lockfile should fail")
	}

	lockfile.RemoveLockfile(file)
	if _, err = os.Lstat(lockpath); !os.IsNotExist(err) {
		t.Fatalf("%s: expected NotExist, got %v", lockpath, err)
	}
}

func TestOtherProcess(t *testing.T) {
	dir, err := ioutil.TempDir("", "lf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "myfile")

	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	// Start a new child which creates a lockfile and exits.
	output := sh.FuncCmd(createLockfile, file).Stdout()
	if output != "Grabbed lock\n" {
		t.Fatalf("Unexpected output: %s", output)
	}

	// Verify it created a lockfile.
	lockpath := file + "-lock"
	bytes, err := ioutil.ReadFile(lockpath)
	if err != nil {
		t.Fatal(err)
	}
	// And that we know the lockfile is invalid.
	running, err := lockutil.StillHeld(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if running {
		t.Fatal("child process is dead")
	}

	// Now create a lockfile for the process.
	if err = lockfile.CreateLockfile(file); err != nil {
		t.Fatal(err)
	}

	// Now the child should fail to create one.
	output = sh.FuncCmd(createLockfile, file).Stdout()
	if output != "Lock failed\n" {
		t.Fatalf("Unexpected output: %s", output)
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
