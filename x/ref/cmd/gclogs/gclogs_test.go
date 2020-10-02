// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"v.io/x/lib/cmdline"
)

func setup(t *testing.T, workdir, username string) (tmpdir string) {
	var err error
	tmpdir, err = ioutil.TempDir(workdir, "gclogs-test-setup-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	logfiles := []struct {
		name string
		link string
		age  time.Duration
	}{
		{"prog1.host.%USER%.log.vanadium.INFO.20141204-131502.12345", "", 4 * time.Hour},
		{"prog1.host.%USER%.log.vanadium.INFO.20141204-145040.23456", "prog1.INFO", 1 * time.Hour},
		{"prog1.host.%USER%.log.vanadium.ERROR.20141204-145040.23456", "prog1.ERROR", 1 * time.Hour},
		{"prog2.host.%USER%.log.vanadium.INFO.20141204-135040.23456", "prog2.INFO", 4 * time.Hour},
		{"prog3.host.otheruser.log.vanadium.INFO.20141204-135040.23456", "prog3.INFO", 1 * time.Hour},
		{"foo.txt", "", 1 * time.Hour},
		{"bar.txt", "", 1 * time.Hour},
	}
	for _, l := range logfiles {
		l.name = strings.ReplaceAll(l.name, "%USER%", username)
		filename := filepath.Join(tmpdir, l.name)
		if err := ioutil.WriteFile(filename, []byte{}, 0644); err != nil {
			t.Fatalf("ioutil.WriteFile failed: %v", err)
		}
		mtime := time.Now().Add(-l.age)
		if err := os.Chtimes(filename, mtime, mtime); err != nil {
			t.Fatalf("os.Chtimes failed: %v", err)
		}
		if l.link != "" {
			if err := os.Symlink(l.name, filepath.Join(tmpdir, l.link)); err != nil {
				t.Fatalf("os.Symlink failed: %v", err)
			}
		}
	}
	if err := os.Mkdir(filepath.Join(tmpdir, "subdir"), 0700); err != nil {
		t.Fatalf("os.Mkdir failed: %v", err)
	}
	return
}

func TestGCLogs(t *testing.T) {
	workdir, err := ioutil.TempDir("", "parse-file-info-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	u, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current failed: %v", err)
	}

	testcases := []struct {
		cutoff   time.Duration
		verbose  bool
		dryrun   bool
		expected []string
	}{
		{
			cutoff:  6 * time.Hour,
			verbose: false,
			dryrun:  false,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Number of files deleted: 0`,
			},
		},
		{
			cutoff:  2 * time.Hour,
			verbose: false,
			dryrun:  true,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Would delete "%TESTDIR%/prog1.host.%USER%.log.vanadium.INFO.20141204-131502.12345"`,
				`Would delete "%TESTDIR%/prog2.host.%USER%.log.vanadium.INFO.20141204-135040.23456"`,
				`Number of files deleted: 0`,
			},
		},
		{
			cutoff:  2 * time.Hour,
			verbose: false,
			dryrun:  false,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Number of files deleted: 3`,
			},
		},
		{
			cutoff:  2 * time.Hour,
			verbose: true,
			dryrun:  false,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Deleting "%TESTDIR%/prog1.host.%USER%.log.vanadium.INFO.20141204-131502.12345"`,
				`Deleting "%TESTDIR%/prog2.host.%USER%.log.vanadium.INFO.20141204-135040.23456"`,
				`Deleting symlink "%TESTDIR%/prog2.INFO"`,
				`Not a log file: "%TESTDIR%/bar.txt"`,
				`Not a log file: "%TESTDIR%/foo.txt"`,
				`Skipped directory: "%TESTDIR%/subdir"`,
				`Skipped log file created by other user: "%TESTDIR%/prog3.INFO"`,
				`Skipped log file created by other user: "%TESTDIR%/prog3.host.otheruser.log.vanadium.INFO.20141204-135040.23456"`,
				`Number of files deleted: 3`,
			},
		},
		{
			cutoff:  time.Minute,
			verbose: true,
			dryrun:  false,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Deleting "%TESTDIR%/prog1.host.%USER%.log.vanadium.ERROR.20141204-145040.23456"`,
				`Deleting "%TESTDIR%/prog1.host.%USER%.log.vanadium.INFO.20141204-131502.12345"`,
				`Deleting "%TESTDIR%/prog1.host.%USER%.log.vanadium.INFO.20141204-145040.23456"`,
				`Deleting "%TESTDIR%/prog2.host.%USER%.log.vanadium.INFO.20141204-135040.23456"`,
				`Deleting symlink "%TESTDIR%/prog1.ERROR"`,
				`Deleting symlink "%TESTDIR%/prog1.INFO"`,
				`Deleting symlink "%TESTDIR%/prog2.INFO"`,
				`Not a log file: "%TESTDIR%/bar.txt"`,
				`Not a log file: "%TESTDIR%/foo.txt"`,
				`Skipped directory: "%TESTDIR%/subdir"`,
				`Skipped log file created by other user: "%TESTDIR%/prog3.INFO"`,
				`Skipped log file created by other user: "%TESTDIR%/prog3.host.otheruser.log.vanadium.INFO.20141204-135040.23456"`,
				`Number of files deleted: 7`,
			},
		},
		{
			cutoff:  time.Minute,
			verbose: false,
			dryrun:  false,
			expected: []string{
				`Processing: "%TESTDIR%"`,
				`Number of files deleted: 7`,
			},
		},
	}
	for _, tc := range testcases {
		testdir := setup(t, workdir, u.Username)
		cutoff := fmt.Sprintf("--cutoff=%s", tc.cutoff)
		verbose := fmt.Sprintf("--verbose=%v", tc.verbose)
		dryrun := fmt.Sprintf("--n=%v", tc.dryrun)
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		if err := cmdline.ParseAndRun(cmdGCLogs, env, []string{cutoff, verbose, dryrun, testdir}); err != nil {
			t.Fatalf("%v: %v", stderr.String(), err)
		}
		gotsl := strings.Split(stdout.String(), "\n")
		if len(gotsl) >= 2 {
			sort.Strings(gotsl[1 : len(gotsl)-2])
		}
		got := strings.Join(gotsl, "\n")
		expected := strings.Join(tc.expected, "\n") + "\n"
		expected = strings.ReplaceAll(expected, "%TESTDIR%", testdir)
		expected = strings.ReplaceAll(expected, "%USER%", u.Username)
		if got != expected {
			t.Errorf("Unexpected result for (%v, %v): got %q, expected %q", tc.cutoff, tc.verbose, got, expected)
		}
	}
}
