// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package apilog_test

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/v23/context"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/apilog"
)

func readLogFiles(dir string) ([]string, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var contents []string
	for _, fi := range files {
		// Skip symlinks to avoid double-counting log lines.
		if !fi.Mode().IsRegular() {
			continue
		}
		file, err := os.Open(filepath.Join(dir, fi.Name()))
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if line := scanner.Text(); len(line) > 0 && line[0] == 'I' {
				contents = append(contents, line)
			}
		}
	}
	return contents, nil
}

func myLoggedFunc(ctx *context.T) {
	f := apilog.LogCallf(ctx, apilog.CallerName(), "entry")
	f(ctx, "exit")
}

func TestLogCall(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	l := vlog.NewLogger("testHeader")
	l.Configure(vlog.LogDir(dir), vlog.Level(2)) //nolint:errcheck
	ctx, _ := context.RootContext()
	ctx = context.WithLogger(ctx, l)
	myLoggedFunc(ctx)
	ctx.FlushLog()
	testLogOutput(t, dir)
}

func TestLogCallNoContext(t *testing.T) {
	dir, err := ioutil.TempDir("", "logtest")
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	l := vlog.NewLogger("testHeader")
	l.Configure(vlog.LogDir(dir), vlog.Level(2)) //nolint:errcheck
	saved := vlog.Log
	vlog.Log = l
	defer func() { vlog.Log = saved }()
	myLoggedFunc(nil)
	vlog.FlushLog()
	testLogOutput(t, dir)
}

func testLogOutput(t *testing.T, dir string) {
	contents, err := readLogFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %q %s", dir, err)
	}
	if want, got := 2, len(contents); want != got {
		t.Errorf("Expected %d info lines, got %d instead", want, got)
	}
	logCallLineRE := regexp.MustCompile(`\S+ \S+\s+\S+ ([^:]*):.*(call|return)\[(\S*)`)
	for _, line := range contents {
		match := logCallLineRE.FindStringSubmatch(line)
		if len(match) != 4 {
			t.Errorf("failed to match %s", line)
			continue
		}
		fileName, callType, funcName := match[1], match[2], match[3]
		if fileName != "apilog_test.go" {
			t.Errorf("unexpected file name: %s", fileName)
			continue
		}
		if callType != "call" && callType != "return" {
			t.Errorf("unexpected call type: %s", callType)
		}
		if funcName != "apilog_test.myLoggedFunc" {
			t.Errorf("unexpected func name: %s", funcName)
		}
	}
}
