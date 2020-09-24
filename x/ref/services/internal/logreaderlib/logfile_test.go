// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !android

package logreaderlib_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/logreader"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/logreaderlib"
	"v.io/x/ref/test"
)

type logFileDispatcher struct {
	root string
}

func (d *logFileDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return logreaderlib.NewLogFileService(d.root, suffix), nil, nil
}

func writeAndSync(t *testing.T, w *os.File, s string) {
	if _, err := w.WriteString(s); err != nil {
		t.Fatalf("w.WriteString failed: %v", err)
	}
	if err := w.Sync(); err != nil {
		t.Fatalf("w.Sync failed: %v", err)
	}
}

func TestReadLogImplNoFollow(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	workdir, err := ioutil.TempDir("", "logreadertest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(workdir)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", &logFileDispatcher{workdir})
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0].String()
	const testFile = "mylogfile.INFO"
	writer, err := os.Create(path.Join(workdir, testFile))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	tests := []string{
		"Hello World!",
		"Life is too short",
		"Have fun",
		"Play hard",
		"Break something",
		"Fix it later",
	}
	for _, s := range tests {
		writeAndSync(t, writer, s+"\n")
	}

	// Try to access a file that doesn't exist.
	lf := logreader.LogFileClient(naming.JoinAddressName(endpoint, "doesntexist"))
	_, err = lf.Size(ctx)
	if expected := verror.ErrNoExist; !errors.Is(err, expected) {
		t.Errorf("unexpected error value, got %v, want: %v", err, expected)
	}

	// Try to access a file that does exist.
	lf = logreader.LogFileClient(naming.JoinAddressName(endpoint, testFile))
	_, err = lf.Size(ctx)
	if err != nil {
		t.Errorf("Size failed: %v", err)
	}

	// Read without follow.
	stream, err := lf.ReadLog(ctx, 0, logreader.AllEntries, false)
	if err != nil {
		t.Errorf("ReadLog failed: %v", err)
	}
	rStream := stream.RecvStream()
	expectedPosition := int64(0)
	for count := 0; rStream.Advance(); count++ {
		entry := rStream.Value()
		if entry.Position != expectedPosition {
			t.Errorf("unexpected position. Got %v, want %v", entry.Position, expectedPosition)
		}
		if expected := tests[count]; entry.Line != expected {
			t.Errorf("unexpected content. Got %q, want %q", entry.Line, expected)
		}
		expectedPosition += int64(len(entry.Line)) + 1
	}

	if err := rStream.Err(); err != nil {
		t.Errorf("unexpected stream error: %v", rStream.Err())
	}
	offset, err := stream.Finish()
	if err != nil {
		t.Errorf("Finish failed: %v", err)
	}
	if offset != expectedPosition {
		t.Errorf("unexpected offset. Got %q, want %q", offset, expectedPosition)
	}

	// Read with follow from EOF (where the previous read ended).
	stream, err = lf.ReadLog(ctx, offset, logreader.AllEntries, false)
	if err != nil {
		t.Errorf("ReadLog failed: %v", err)
	}
	_, err = stream.Finish()
	if !errors.Is(err, verror.ErrEndOfFile) {
		t.Errorf("unexpected error, got %#v, want EOF", err)
	}
}

func TestReadLogImplWithFollow(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	workdir, err := ioutil.TempDir("", "logreadertest")
	if err != nil {
		t.Fatalf("ioutil.TempDir: %v", err)
	}
	defer os.RemoveAll(workdir)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", &logFileDispatcher{workdir})
	if err != nil {
		t.Fatalf("NewDispatchingServer failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0].String()

	const testFile = "mylogfile.INFO"
	writer, err := os.Create(path.Join(workdir, testFile))
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	tests := []string{
		"Hello World!",
		"Life is too short",
		"Have fun",
		"Play hard",
		"Break something",
		"Fix it later",
	}

	lf := logreader.LogFileClient(naming.JoinAddressName(endpoint, testFile))
	_, err = lf.Size(ctx)
	if err != nil {
		t.Errorf("Size failed: %v", err)
	}

	// Read with follow.
	stream, err := lf.ReadLog(ctx, 0, int32(len(tests)), true)
	if err != nil {
		t.Errorf("ReadLog failed: %v", err)
	}
	rStream := stream.RecvStream()
	writeAndSync(t, writer, tests[0]+"\n")
	for count, pos := 0, int64(0); rStream.Advance(); count++ {
		entry := rStream.Value()
		if entry.Position != pos {
			t.Errorf("unexpected position. Got %v, want %v", entry.Position, pos)
		}
		if expected := tests[count]; entry.Line != expected {
			t.Errorf("unexpected content. Got %q, want %q", entry.Line, expected)
		}
		pos += int64(len(entry.Line)) + 1
		if count+1 < len(tests) {
			writeAndSync(t, writer, tests[count+1]+"\n")
		}
	}

	if err := rStream.Err(); err != nil {
		t.Errorf("unexpected stream error: %v", rStream.Err())
	}
	_, err = stream.Finish()
	if err != nil {
		t.Errorf("Finish failed: %v", err)
	}
}
