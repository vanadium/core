// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestKillCommand(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	tapes := servicetest.NewTapeMap()
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", newDispatcher(t, tapes))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Setup the command-line.
	cmd := cmd_device.CmdRoot
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	appName := naming.JoinAddressName(server.Status().Endpoints[0].String(), "appname")

	// Confirm that we correctly enforce the number of arguments.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"kill"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: kill: incorrect number of arguments, expected 1, got 0", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from kill. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	appTape := tapes.ForSuffix("appname")
	appTape.Rewind()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"kill", "nope", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: kill: incorrect number of arguments, expected 1, got 2", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from kill. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	appTape.Rewind()

	// Test the 'kill' command.
	appTape.SetResponses(nil)

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"kill", appName}); err != nil {
		t.Fatalf("kill failed when it shouldn't: %v", err)
	}
	if expected, got := "Kill succeeded", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	expected := []interface{}{
		KillStimulus{"Kill", 10 * time.Second},
	}
	if got := appTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	appTape.Rewind()
	stderr.Reset()
	stdout.Reset()

	// Test kill with bad parameters.
	appTape.SetResponses(verror.New(errOops, nil))
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"kill", appName}); err == nil {
		t.Fatalf("wrongly didn't receive a non-nil error.")
	}
	// expected the same.
	if got := appTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
}
