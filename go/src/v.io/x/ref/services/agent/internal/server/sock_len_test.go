// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSockFilePath(t *testing.T, length int) (string, string) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	delta := length - len(d) - 1 // -1 for the path separator.
	if delta <= 0 {
		t.Fatalf("Temp dir path is too long (%s [%d]) to allow creating a file in the dir with path length %d", d, len(d), length)
	}
	filePath := filepath.Join(d, strings.Repeat("s", delta))
	if realLen := len(filePath); realLen != length {
		t.Fatalf("Tried creating a file path of length %d but created %s [%d] instead", length, filePath, realLen)

	}
	return d, filePath
}

func TestListenSuccess(t *testing.T) {
	d, f := makeSockFilePath(t, GetMaxSockPathLen())
	defer os.RemoveAll(d)
	if listener, err := net.Listen("unix", f); err != nil {
		t.Fatalf("Listen(\"unix\", \"%s\") failed: %v", f, err)
	} else {
		listener.Close()
	}
}

func TestListenFail(t *testing.T) {
	d, f := makeSockFilePath(t, GetMaxSockPathLen()+1)
	defer os.RemoveAll(d)
	if listener, err := net.Listen("unix", f); err == nil {
		listener.Close()
		t.Fatalf("Listen(\"unix\", \"%s\") should have failed but succeeded", f)
	}
}
