// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseFileNameNoError(t *testing.T) {
	testcases := []struct {
		filename string
		lf       *logFile
	}{
		{
			"program.host.user.log.vanadium.INFO.20141204-131502.12345",
			&logFile{false, "program", "host", "user", "vanadium", "INFO", time.Date(2014, 12, 4, 13, 15, 2, 0, time.Local), 12345},
		},
		{
			"prog.test.host-name.user.log.vanadium.ERROR.20141204-131502.12345",
			&logFile{false, "prog.test", "host-name", "user", "vanadium", "ERROR", time.Date(2014, 12, 4, 13, 15, 2, 0, time.Local), 12345},
		},
	}
	for _, tc := range testcases {
		lf, err := parseFileName(tc.filename)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tc.filename, err)
		}
		if !reflect.DeepEqual(tc.lf, lf) {
			t.Errorf("unexpected result: got %+v, expected %+v", lf, tc.lf)
		}
	}
}

func TestParseFileNameError(t *testing.T) {
	testcases := []string{
		"program.host.user.log.vanadium.INFO.20141204-131502",
		"prog.test.host-name.user.log.vanadium.20141204-131502.12345",
		"foo.txt",
	}
	for _, tc := range testcases {
		if _, err := parseFileName(tc); err == nil {
			t.Errorf("unexpected success for %q", tc)
		}
	}
}

func TestParseFileInfo(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "parse-file-info-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	name := "program.host.user.log.vanadium.INFO.20141204-131502.12345"
	if err := ioutil.WriteFile(filepath.Join(tmpdir, name), []byte{}, 0644); err != nil {
		t.Fatalf("ioutil.WriteFile failed: %v", err)
	}
	link := "program.INFO"
	if err := os.Symlink(name, filepath.Join(tmpdir, link)); err != nil {
		t.Fatalf("os.Symlink failed: %v", err)
	}

	// Test regular file.
	{
		fi, err := os.Lstat(filepath.Join(tmpdir, name))
		if err != nil {
			t.Fatalf("os.Lstat failed: %v", err)
		}
		lf, err := parseFileInfo(tmpdir, fi)
		if err != nil {
			t.Errorf("parseFileInfo(%v, %v) failed: %v", tmpdir, fi, err)
		}
		expected := &logFile{false, "program", "host", "user", "vanadium", "INFO", time.Date(2014, 12, 4, 13, 15, 2, 0, time.Local), 12345}
		if !reflect.DeepEqual(lf, expected) {
			t.Errorf("unexpected result: got %+v, expected %+v", lf, expected)
		}
	}

	// Test symlink.
	{
		fi, err := os.Lstat(filepath.Join(tmpdir, link))
		if err != nil {
			t.Fatalf("os.Lstat failed: %v", err)
		}
		lf, err := parseFileInfo(tmpdir, fi)
		if err != nil {
			t.Errorf("parseFileInfo(%v, %v) failed: %v", tmpdir, fi, err)
		}
		expected := &logFile{true, "program", "host", "user", "vanadium", "INFO", time.Date(2014, 12, 4, 13, 15, 2, 0, time.Local), 12345}
		if !reflect.DeepEqual(lf, expected) {
			t.Errorf("unexpected result: got %+v, expected %+v", lf, expected)
		}
	}
}
