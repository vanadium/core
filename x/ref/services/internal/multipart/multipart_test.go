// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multipart_test

import (
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"v.io/x/ref/services/internal/multipart"
)

func read(t *testing.T, m http.File, thisMuch int) string {
	buf := make([]byte, thisMuch)
	bytesRead := 0
	for {
		n, err := m.Read(buf[bytesRead:])
		bytesRead += n
		if bytesRead == thisMuch {
			return string(buf)
		}
		switch err {
		case nil:
		case io.EOF:
			return string(buf[:bytesRead])
		default:
			t.Fatalf("Read failed: %v", err)
		}
	}
}

// TestFile verifies the http.File operations on the multipart file.
func TestFile(t *testing.T) { //nolint:gocyclo
	contents := []string{"v", "is", "for", "vanadium"}
	files := make([]*os.File, len(contents))
	d, err := ioutil.TempDir("", "multiparts")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(d)
	contentsSize := 0
	for i, c := range contents {
		contentsSize += len(c)
		fPath := filepath.Join(d, strconv.Itoa(i))
		if err := ioutil.WriteFile(fPath, []byte(c), 0600); err != nil {
			t.Fatalf("WriteFile(%v) failed: %v", fPath, err)
		}
		var err error
		if files[i], err = os.Open(fPath); err != nil {
			t.Fatalf("Open(%v) failed: %v", fPath, err)
		}
	}
	m, err := multipart.NewFile("bunnies", files)
	if err != nil {
		t.Fatalf("NewFile failed: %v", err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()
	stat, err := m.Stat()
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	if want, got := "bunnies", stat.Name(); want != got {
		t.Fatalf("Name returned %s, expected %s", got, want)
	}
	if want, got := int64(contentsSize), stat.Size(); want != got {
		t.Fatalf("Size returned %d, expected %d", got, want)
	}
	if want, got := strings.Join(contents, ""), read(t, m, 1024); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if want, got := "", read(t, m, 1024); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if pos, err := m.Seek(0, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(0), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	if want, got := strings.Join(contents, ""), read(t, m, 1024); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if pos, err := m.Seek(0, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(0), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	for _, c := range contents {
		if want, got := c, read(t, m, len(c)); want != got {
			t.Fatalf("Read %v, wanted %v instead", got, want)
		}
	}
	if want, got := "", read(t, m, 1024); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if pos, err := m.Seek(1, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(1), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	if want, got := "isfo", read(t, m, 4); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if pos, err := m.Seek(2, 1); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(7), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	if want, got := "anadi", read(t, m, 5); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if _, err := m.Seek(100, 1); err == nil {
		t.Fatalf("Seek expected to fail")
	}
	if want, got := "u", read(t, m, 1); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if pos, err := m.Seek(8, 2); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(6), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	if want, got := "vanad", read(t, m, 5); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}
	if _, err := m.Seek(100, 2); err == nil {
		t.Fatalf("Seek expected to fail")
	}
	if pos, err := m.Seek(9, 2); err != nil {
		t.Fatalf("Seek failed: %v", err)
	} else if want, got := int64(5), pos; want != got {
		t.Fatalf("Pos is %d, wanted %d", got, want)
	}
	if want, got := "rvana", read(t, m, 5); want != got {
		t.Fatalf("Read %v, wanted %v instead", got, want)
	}

	// TODO(caprita): Add some auto-generated test cases where we seek/read
	// using various combinations of indices.  These can be exhaustive or
	// randomized, the idea is to get better coverage.
}
