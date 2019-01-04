// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/binary"
	"v.io/v23/services/repository"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type server struct {
	suffix string
}

func (s *server) Create(ctx *context.T, _ rpc.ServerCall, _ int32, _ repository.MediaInfo) error {
	ctx.Infof("Create() was called. suffix=%v", s.suffix)
	return nil
}

func (s *server) Delete(ctx *context.T, _ rpc.ServerCall) error {
	ctx.Infof("Delete() was called. suffix=%v", s.suffix)
	if s.suffix != "exists" {
		return fmt.Errorf("binary doesn't exist: %v", s.suffix)
	}
	return nil
}

func (s *server) Download(ctx *context.T, call repository.BinaryDownloadServerCall, _ int32) error {
	ctx.Infof("Download() was called. suffix=%v", s.suffix)
	sender := call.SendStream()
	sender.Send([]byte("Hello"))
	sender.Send([]byte("World"))
	return nil
}

func (s *server) DownloadUrl(ctx *context.T, _ rpc.ServerCall) (string, int64, error) {
	ctx.Infof("DownloadUrl() was called. suffix=%v", s.suffix)
	if s.suffix != "" {
		return "", 0, fmt.Errorf("non-empty suffix: %v", s.suffix)
	}
	return "test-download-url", 0, nil
}

func (s *server) Stat(ctx *context.T, _ rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	ctx.Infof("Stat() was called. suffix=%v", s.suffix)
	h := md5.New()
	text := "HelloWorld"
	h.Write([]byte(text))
	part := binary.PartInfo{Checksum: hex.EncodeToString(h.Sum(nil)), Size: int64(len(text))}
	return []binary.PartInfo{part}, repository.MediaInfo{Type: "text/plain"}, nil
}

func (s *server) Upload(ctx *context.T, call repository.BinaryUploadServerCall, _ int32) error {
	ctx.Infof("Upload() was called. suffix=%v", s.suffix)
	rStream := call.RecvStream()
	for rStream.Advance() {
	}
	return nil
}

func (s *server) GetPermissions(ctx *context.T, _ rpc.ServerCall) (perms access.Permissions, version string, err error) {
	return nil, "", nil
}

func (s *server) SetPermissions(ctx *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return nil
}

type dispatcher struct {
}

func NewDispatcher() rpc.Dispatcher {
	return &dispatcher{}
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return repository.BinaryServer(&server{suffix: suffix}), nil, nil
}

func TestBinaryClient(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	_, server, err := v23.WithNewDispatchingServer(ctx, "", NewDispatcher())
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0]

	// Setup the command-line.
	var out bytes.Buffer
	env := &cmdline.Env{Stdout: &out, Stderr: &out}

	// Test the 'delete' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"delete", naming.JoinAddressName(endpoint.String(), "exists")}); err != nil {
		t.Fatalf("%v failed: %v\n%v", "delete", err, out.String())
	}
	if expected, got := "Binary deleted successfully", strings.TrimSpace(out.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	out.Reset()

	// Test the 'download' command.
	dir, err := ioutil.TempDir("", "binaryimpltest")
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer os.RemoveAll(dir)
	file := path.Join(dir, "testfile")
	defer os.Remove(file)
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"download", naming.JoinAddressName(endpoint.String(), "exists"), file}); err != nil {
		t.Fatalf("%v failed: %v\n%v", "download", err, out.String())
	}
	if expected, got := "Binary installed as "+file, strings.TrimSpace(out.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if expected := "HelloWorld"; string(buf) != expected {
		t.Errorf("Got %q, expected %q", string(buf), expected)
	}
	out.Reset()

	// Test the 'upload' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"upload", naming.JoinAddressName(endpoint.String(), "exists"), file}); err != nil {
		t.Fatalf("%v failed: %v\n%v", "upload", err, out.String())
	}
	out.Reset()

	// Test the 'url' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"url", naming.JoinAddressName(endpoint.String(), "")}); err != nil {
		t.Fatalf("%v failed: %v\n%v", "url", err, out.String())
	}
	if expected, got := "test-download-url", strings.TrimSpace(out.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
}
