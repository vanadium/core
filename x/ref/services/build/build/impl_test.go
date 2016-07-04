// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/services/binary"
	"v.io/v23/services/build"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mock struct{}

func (mock) Build(ctx *context.T, call build.BuilderBuildServerCall, arch build.Architecture, opsys build.OperatingSystem) ([]byte, error) {
	ctx.VI(2).Infof("Build(%v, %v) was called", arch, opsys)
	iterator := call.RecvStream()
	for iterator.Advance() {
	}
	if err := iterator.Err(); err != nil {
		ctx.Errorf("Advance() failed: %v", err)
		return nil, verror.New(verror.ErrInternal, ctx)
	}
	return nil, nil
}

func (mock) Describe(ctx *context.T, _ rpc.ServerCall, name string) (binary.Description, error) {
	ctx.VI(2).Infof("Describe(%v) was called", name)
	return binary.Description{}, nil
}

type dispatcher struct{}

func startServer(ctx *context.T, t *testing.T) naming.Endpoint {
	unpublished := ""
	ctx, server, err := v23.WithNewServer(ctx, unpublished, build.BuilderServer(&mock{}), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	return server.Status().Endpoints[0]
}

func TestBuildClient(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	endpoint := startServer(ctx, t)

	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	args := []string{"build", naming.JoinAddressName(endpoint.String(), ""), "v.io/x/ref/services/build/build"}
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, args); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if got, want := strings.TrimSpace(stdout.String()), ""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
