// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store/util"
)

func makeRootDir(t *testing.T) string {
	rootDir, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		t.Fatalf("can't create temp dir: %v", err)
	}
	return rootDir
}

func TestOpenNoCreate(t *testing.T) {
	rootDir := makeRootDir(t)
	_, err := util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: false,
		ErrorIfExists:   false,
	})
	if err == nil {
		t.Fatalf("expected error opening nonexistent leveldb")
	}
}

func TestOpenDestroy(t *testing.T) {
	rootDir := makeRootDir(t)
	st, err := util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: true,
		ErrorIfExists:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := util.DestroyStore("leveldb", rootDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	files, err := ioutil.ReadDir(rootDir)
	// ReadDir() returns an error if the directory does not exist.
	if err == nil && len(files) != 0 {
		t.Fatalf("unexpected files in rootDir: %v", files)
	}

	// Trying to reopen the leveldb should fail.
	_, err = util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: false,
		ErrorIfExists:   false,
	})
	if err == nil {
		t.Fatalf("expected error opening nonexistent leveldb")
	}
}

func TestCorruptStore(t *testing.T) {
	rootDir := makeRootDir(t)

	// Open the store, put some data, close.
	st, err := util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: true,
		ErrorIfExists:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := st.Put([]byte("the key"), []byte("the value")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Corrupt a log file.
	var fileToCorrupt string
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if fileToCorrupt != "" {
			return nil
		}
		if match, _ := regexp.MatchString(`.*\.log$`, path); match {
			fileToCorrupt = path
			return errors.New("found match, stop walking")
		}
		return nil
	})
	if fileToCorrupt == "" {
		t.Fatalf("Could not find file")
	}
	fileBytes, err := ioutil.ReadFile(fileToCorrupt)
	if err != nil {
		t.Fatalf("Could not read log file: %v", err)
	}
	// Overwrite last 20 bytes.
	offset := len(fileBytes) - 20 - 1
	if offset < 0 {
		t.Fatalf("Expected bigger log file.  Found: %d", len(fileBytes))
	}
	for i := 0; i < 20; i++ {
		fileBytes[i+offset] = 0x80
	}
	if err := ioutil.WriteFile(fileToCorrupt, fileBytes, 0); err != nil {
		t.Fatalf("Could not corrupt file: %v", err)
	}

	// The leveldb should fail to open since it is corrupt.
	_, err = util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: false,
		ErrorIfExists:   false,
	})
	if verror.ErrorID(err) != syncbase.ErrCorruptDatabase.ID {
		t.Fatalf("wrong error opening corrupt leveldb: %v", err)
	}
	// Look for the leveldb directory to be moved aside.
	matches, err := filepath.Glob(rootDir + ".corrupt.*")
	if err != nil {
		t.Fatalf("bad Glob %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Got matches: %v, want one match", matches)
	}

	// Opening (with create) a second time should succeed.
	st, err = util.OpenStore("leveldb", rootDir, util.OpenOptions{
		CreateIfMissing: true,
		ErrorIfExists:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
