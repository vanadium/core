// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suid

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"
)

func TestChown(t *testing.T) {
	dir, err := ioutil.TempDir("", "chown_test")
	if err != nil {
		t.Fatalf("ioutil.TempDir() failed: %v", err)
	}
	defer os.RemoveAll(dir)

	for _, p := range []string{"a/b/c", "c/d"} {
		fp := path.Join(dir, p)
		if err := os.MkdirAll(fp, os.FileMode(0700)); err != nil {
			t.Fatalf("os.MkdirAll(%s) failed: %v", fp, err)
		}
	}

	wp := &WorkParameters{
		uid:       42,
		gid:       7,
		logDir:    path.Join(dir, "a"),
		workspace: path.Join(dir, "c"),

		dryrun: true,
	}

	// Collect the log entries.
	b := new(bytes.Buffer)
	log.SetOutput(b)
	log.SetFlags(0)
	defer log.SetOutput(os.Stderr)
	defer log.SetFlags(log.LstdFlags)

	// Mock-chown the tree.
	if err := wp.Chown(); err != nil {
		t.Fatalf("wp.Chown() wrongly failed: %v", err)
	}

	// Edit the log buffer to remove the invocation dependent output.
	pb := bytes.TrimSpace(bytes.Replace(b.Bytes(), []byte(dir), []byte("$PATH"), -1))

	cmds := bytes.Split(pb, []byte{'\n'})
	for i, _ := range cmds {
		cmds[i] = bytes.TrimSpace(cmds[i])
	}

	expected := []string{
		"[dryrun] os.Chown($PATH/c, 42, 7)",
		"[dryrun] os.Chown($PATH/c/d, 42, 7)",
		"[dryrun] os.Chown($PATH/a, 42, 7)",
		"[dryrun] os.Chown($PATH/a/b, 42, 7)",
		"[dryrun] os.Chown($PATH/a/b/c, 42, 7)",
	}
	if got, expected := len(cmds), len(expected); got != expected {
		t.Fatalf("bad length. got: %d, expected %d", got, expected)
	}
	for i, _ := range expected {
		if expected, got := expected[i], string(cmds[i]); expected != got {
			t.Fatalf("wp.Chown output %d: got %v, expected %v", i, got, expected)
		}
	}
}
