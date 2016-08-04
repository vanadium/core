// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logreaderlib

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func writeAndSync(t *testing.T, w *os.File, s string) {
	if _, err := w.WriteString(s); err != nil {
		t.Fatalf("w.WriteString failed: %v", err)
	}
	if err := w.Sync(); err != nil {
		t.Fatalf("w.Sync failed: %v", err)
	}
}

func TestFollowReaderNoFollow(t *testing.T) {
	w, err := ioutil.TempFile("", "reader-test-")
	if err != nil {
		t.Fatalf("ioutil.TempFile: unexpected error: %v", err)
	}
	defer w.Close()
	defer os.Remove(w.Name())

	tests := []string{
		"Hello world",
		"Hello world Two",
		"Hello world Three",
	}
	for _, s := range tests {
		writeAndSync(t, w, s+"\n")
	}
	writeAndSync(t, w, "Partial line with no newline")

	r, err := os.Open(w.Name())
	if err != nil {
		t.Fatalf("os.Open: unexpected error: %v", err)
	}
	defer r.Close()

	f := newFollowReader(nil, r, 0, false)
	if f == nil {
		t.Fatalf("newFollowReader return nil")
	}

	var expectedOffset int64
	for _, s := range tests {
		line, offset, err := f.readLine()
		if err != nil {
			t.Errorf("readLine, unexpected error: %v", err)
		}
		if line != s {
			t.Errorf("unexpected result. Got %v, want %v", line, s)
		}
		if offset != expectedOffset {
			t.Errorf("unexpected result. Got %v, want %v", offset, expectedOffset)
		}
		expectedOffset += int64(len(s)) + 1
	}

	// Attempt to read the partial line.
	if line, _, err := f.readLine(); line != "" || err != io.EOF {
		t.Errorf("unexpected result. Got %v:%v, want \"\":EOF", line, err)
	}
}

func sleep() {
	time.Sleep(500 * time.Millisecond)
}

func TestFollowReaderWithFollow(t *testing.T) {
	w, err := ioutil.TempFile("", "reader-test-")
	if err != nil {
		t.Fatalf("ioutil.TempFile: unexpected error: %v", err)
	}
	defer os.Remove(w.Name())

	tests := []string{
		"Hello world",
		"Hello world Two",
		"Hello world Three",
	}
	go func() {
		for _, s := range tests {
			sleep()
			writeAndSync(t, w, s+"\n")
		}
		sleep()
		writeAndSync(t, w, "Hello ")
		sleep()
		writeAndSync(t, w, "world ")
		sleep()
		writeAndSync(t, w, "Four\n")
		w.Close()
	}()

	r, err := os.Open(w.Name())
	if err != nil {
		t.Fatalf("os.Open: unexpected error: %v", err)
	}
	defer r.Close()

	f := newFollowReader(nil, r, 0, true)
	if f == nil {
		t.Fatalf("newFollowReader return nil")
	}

	var expectedOffset int64
	for _, s := range tests {
		line, offset, err := f.readLine()
		if err != nil {
			t.Errorf("readLine, unexpected error: %v", err)
		}
		if line != s {
			t.Errorf("unexpected result. Got %v, want %v", line, s)
		}
		if offset != expectedOffset {
			t.Errorf("unexpected result. Got %v, want %v", offset, expectedOffset)
		}
		expectedOffset += int64(len(s)) + 1
	}

	line, offset, err := f.readLine()
	if err != nil {
		t.Errorf("readLine, unexpected error: %v", err)
	}
	if expected := "Hello world Four"; line != expected {
		t.Errorf("unexpected result. Got %q, want %q", line, expected)
	}
	if offset != expectedOffset {
		t.Errorf("unexpected result. Got %v, want %v", offset, expectedOffset)
	}
}
