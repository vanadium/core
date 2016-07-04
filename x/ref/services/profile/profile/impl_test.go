// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/build"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/profile"
	"v.io/x/ref/services/repository"
	"v.io/x/ref/test"
)

var (
	// spec is an example profile specification used throughout the test.
	spec = profile.Specification{
		Arch:        build.ArchitectureAmd64,
		Description: "Example profile to test the profile repository implementation.",
		Format:      build.FormatElf,
		Libraries:   map[profile.Library]struct{}{profile.Library{Name: "foo", MajorVersion: "1", MinorVersion: "0"}: struct{}{}},
		Label:       "example",
		Os:          build.OperatingSystemLinux,
	}
)

type server struct {
	suffix string
}

func (s *server) Label(ctx *context.T, _ rpc.ServerCall) (string, error) {
	ctx.VI(2).Infof("%v.Label() was called", s.suffix)
	if s.suffix != "exists" {
		return "", fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec.Label, nil
}

func (s *server) Description(ctx *context.T, _ rpc.ServerCall) (string, error) {
	ctx.VI(2).Infof("%v.Description() was called", s.suffix)
	if s.suffix != "exists" {
		return "", fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec.Description, nil
}

func (s *server) Specification(ctx *context.T, _ rpc.ServerCall) (profile.Specification, error) {
	ctx.VI(2).Infof("%v.Specification() was called", s.suffix)
	if s.suffix != "exists" {
		return profile.Specification{}, fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return spec, nil
}

func (s *server) Put(ctx *context.T, _ rpc.ServerCall, _ profile.Specification) error {
	ctx.VI(2).Infof("%v.Put() was called", s.suffix)
	return nil
}

func (s *server) Remove(ctx *context.T, _ rpc.ServerCall) error {
	ctx.VI(2).Infof("%v.Remove() was called", s.suffix)
	if s.suffix != "exists" {
		return fmt.Errorf("profile doesn't exist: %v", s.suffix)
	}
	return nil
}

type dispatcher struct {
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return repository.ProfileServer(&server{suffix: suffix}), nil, nil
}

func TestProfileClient(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", &dispatcher{})
	if err != nil {
		return
	}

	// Setup the command-line.
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	exists := naming.JoinAddressName(server.Status().Endpoints[0].String(), "exists")

	// Test the 'label' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"label", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := spec.Label, strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'description' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"description", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := spec.Description, strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'spec' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"specification", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := fmt.Sprintf("%#v", spec), strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'put' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"put", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "Profile added successfully.", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()

	// Test the 'remove' command.
	if err := v23cmd.ParseAndRunForTest(cmdRoot, ctx, env, []string{"remove", exists}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "Profile removed successfully.", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Got %q, expected %q", got, expected)
	}
	stdout.Reset()
}
