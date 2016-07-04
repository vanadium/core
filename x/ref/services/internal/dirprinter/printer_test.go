// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dirprinter_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/x/ref/services/internal/dirprinter"
)

func TestDumpDir(t *testing.T) {
	// In addition to the tree checked in under testdata, we add the
	// following:

	// An empty directory, dir.2.
	dirPath := filepath.Join("testdata", "todump", "dir.2")
	if err := os.Mkdir(dirPath, os.ModePerm); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	defer os.RemoveAll(dirPath)

	// A directory with no permissions, dir.3/dir.3.1.
	dirPath = filepath.Join("testdata", "todump", "dir.3", "dir.3.1")
	if err := os.Mkdir(dirPath, 0); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	defer os.RemoveAll(dirPath)

	// A file with no permissions, dir.1/dir.1.1/file.1.1.3.
	filePath := filepath.Join("testdata", "todump", "dir.1", "dir.1.1", "file.1.1.3")
	if err := ioutil.WriteFile(filePath, []byte("can't read me"), 0); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	defer os.Remove(filePath)

	var out bytes.Buffer
	if err := dirprinter.DumpDir(&out, filepath.Join("testdata", "todump")); err != nil {
		t.Fatalf("DumpDir failed: %v", err)
	}
	re := regexp.MustCompile("[\\d][\\d]:[\\d][\\d]:[\\d][\\d].[\\d][\\d][\\d][\\d][\\d][\\d]")
	cleaned := re.ReplaceAll(out.Bytes(), []byte("hh:mm:ss.xxxxxx"))
	expected, err := ioutil.ReadFile(filepath.Join("testdata", "expected"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !bytes.Equal(expected, cleaned) {
		t.Fatalf("Expected:\n\n%s\nGot:\n\n%s\n", string(expected), out.String())
	}
}
