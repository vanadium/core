// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

import (
	"bytes"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/repository"
	"v.io/x/ref/services/internal/packages"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

const (
	v23Prefix = "vanadium_binary_repository"
)

func setupRepository(t *testing.T, ctx *context.T) (string, func()) {
	// Setup the root of the binary repository.
	rootDir, err := ioutil.TempDir("", v23Prefix)
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	path, perm := filepath.Join(rootDir, VersionFile), os.FileMode(0600)
	if err := ioutil.WriteFile(path, []byte(Version), perm); err != nil {
		ctx.Fatalf("WriteFile(%v, %v, %v) failed: %v", path, Version, perm, err)
	}
	// Setup and start the binary repository server.
	depth := 2
	state, err := NewState(rootDir, "http://test-root-url", depth)
	if err != nil {
		t.Fatalf("NewState(%v, %v) failed: %v", rootDir, depth, err)
	}
	dispatcher, err := NewDispatcher(ctx, state)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v\n", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	von := naming.JoinAddressName(server.Status().Endpoints[0].String(), "test")
	return von, func() {
		if err := os.Remove(path); err != nil {
			t.Fatalf("Remove(%v) failed: %v", path, err)
		}
		// Check that any directories and files that were created to
		// represent the binary objects have been garbage collected.
		if err := os.RemoveAll(rootDir); err != nil {
			t.Fatalf("Remove(%v) failed: %v", rootDir, err)
		}
		cancel()
		<-server.Closed()
	}
}

// TestBufferAPI tests the binary repository client-side library
// interface using buffers.
func TestBufferAPI(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	rg := testutil.NewRandGenerator(t.Logf)

	von, cleanup := setupRepository(t, ctx)
	defer cleanup()
	data := rg.RandomBytes(rg.RandomIntn(10 << 20))
	mediaInfo := repository.MediaInfo{Type: "application/octet-stream"}
	sig, err := Upload(ctx, von, data, mediaInfo)
	if err != nil {
		t.Fatalf("Upload(%v) failed: %v", von, err)
	}
	p := v23.GetPrincipal(ctx)
	if sig != nil {
		// verify the principal signature
		h := sha256.Sum256(data)
		if !sig.Verify(p.PublicKey(), h[:]) {
			t.Fatalf("Failed to verify upload signature(%v)", sig)
		}
		// verify that Sign called directly also produces a working signature
		reader := bytes.NewReader(data)
		sig2, err := Sign(ctx, reader)
		if err != nil {
			t.Fatalf("Sign failed: %v", err)
		}
		if !sig2.Verify(p.PublicKey(), h[:]) {
			t.Fatalf("Failed to verify signature from Sign(%v, %v)", sig, sig2)
		}
	} else {
		t.Fatalf("Upload(%v) failed to generate principal(%v) signature", von, p)
	}
	output, outInfo, err := Download(ctx, von)
	if err != nil {
		t.Fatalf("Download(%v) failed: %v", von, err)
	}
	if bytes.Compare(data, output) != 0 {
		t.Errorf("Data mismatch:\nexpected %v %v\ngot %v %v", len(data), data[:100], len(output), output[:100])
	}
	if err := Delete(ctx, von); err != nil {
		t.Errorf("Delete(%v) failed: %v", von, err)
	}
	if _, _, err := Download(ctx, von); err == nil {
		t.Errorf("Download(%v) did not fail", von)
	}
	if !reflect.DeepEqual(mediaInfo, outInfo) {
		t.Errorf("unexpected media info: expected %v, got %v", mediaInfo, outInfo)
	}
}

// TestFileAPI tests the binary repository client-side library
// interface using files.
func TestFileAPI(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	rg := testutil.NewRandGenerator(t.Logf)

	von, cleanup := setupRepository(t, ctx)
	defer cleanup()
	// Create up to 10MB of random bytes.
	data := rg.RandomBytes(rg.RandomIntn(10 << 20))
	dir, prefix := "", ""
	src, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		t.Fatalf("TempFile(%v, %v) failed: %v", dir, prefix, err)
	}
	defer os.Remove(src.Name())
	defer src.Close()
	dstdir, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%v, %v) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(dstdir)
	dst, err := ioutil.TempFile(dstdir, prefix)
	if err != nil {
		t.Fatalf("TempFile(%v, %v) failed: %v", dstdir, prefix, err)
	}
	defer dst.Close()
	if _, err := src.Write(data); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}
	if _, err := UploadFromFile(ctx, von, src.Name()); err != nil {
		t.Fatalf("UploadFromFile(%v, %v) failed: %v", von, src.Name(), err)
	}
	if err := DownloadToFile(ctx, von, dst.Name()); err != nil {
		t.Fatalf("DownloadToFile(%v, %v) failed: %v", von, dst.Name(), err)
	}
	output, err := ioutil.ReadFile(dst.Name())
	if err != nil {
		t.Errorf("ReadFile(%v) failed: %v", dst.Name(), err)
	}
	if bytes.Compare(data, output) != 0 {
		t.Errorf("Data mismatch:\nexpected %v %v\ngot %v %v", len(data), data[:100], len(output), output[:100])
	}
	jMediaInfo, err := ioutil.ReadFile(packages.MediaInfoFile(dst.Name()))
	if err != nil {
		t.Errorf("ReadFile(%v) failed: %v", packages.MediaInfoFile(dst.Name()), err)
	}
	if expected := `{"Type":"application/octet-stream","Encoding":""}`; string(jMediaInfo) != expected {
		t.Errorf("unexpected media info: expected %q, got %q", expected, string(jMediaInfo))
	}
	if err := Delete(ctx, von); err != nil {
		t.Errorf("Delete(%v) failed: %v", von, err)
	}
}

// TestDownloadUrl tests the binary repository client-side library
// DownloadUrl method.
func TestDownloadUrl(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	von, cleanup := setupRepository(t, ctx)
	defer cleanup()
	url, _, err := DownloadUrl(ctx, von)
	if err != nil {
		t.Fatalf("DownloadUrl(%v) failed: %v", von, err)
	}
	if got, want := url, "http://test-root-url/test"; got != want {
		t.Fatalf("unexpect output: got %v, want %v", got, want)
	}
}
