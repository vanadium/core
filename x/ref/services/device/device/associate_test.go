// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestListCommand(t *testing.T) {
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
	deviceName := server.Status().Endpoints[0].Name()

	rootTape := tapes.ForSuffix("")
	// Test the 'list' command.
	rootTape.SetResponses(ListAssociationResponse{
		na: []device.Association{
			{
				"root/self",
				"alice_self_account",
			},
			{
				"root/other",
				"alice_other_account",
			},
		},
		err: nil,
	})

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"associate", "list", deviceName}); err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := "root/self alice_self_account\nroot/other alice_other_account", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	if got, expected := rootTape.Play(), []interface{}{"ListAssociations"}; !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()

	// Test list with bad parameters.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"associate", "list", deviceName, "hello"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if got, expected := len(rootTape.Play()), 0; got != expected {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
}

func TestAddCommand(t *testing.T) {
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
	deviceName := server.Status().Endpoints[0].Name()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"add", "one"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	rootTape := tapes.ForSuffix("")
	if got, expected := len(rootTape.Play()), 0; got != expected {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()

	rootTape.SetResponses(nil)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"associate", "add", deviceName, "alice", "root/self"}); err != nil {
		t.Fatalf("%v", err)
	}
	expected := []interface{}{
		AddAssociationStimulus{"AssociateAccount", []string{"root/self"}, "alice"},
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()

	rootTape.SetResponses(nil)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"associate", "add", deviceName, "alice", "root/other", "root/self"}); err != nil {
		t.Fatalf("%v", err)
	}
	expected = []interface{}{
		AddAssociationStimulus{"AssociateAccount", []string{"root/other", "root/self"}, "alice"},
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
}

func TestRemoveCommand(t *testing.T) {
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
	deviceName := server.Status().Endpoints[0].Name()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"remove", "one"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	rootTape := tapes.ForSuffix("")
	if got, expected := len(rootTape.Play()), 0; got != expected {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()

	rootTape.SetResponses(nil)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"associate", "remove", deviceName, "root/self"}); err != nil {
		t.Fatalf("%v", err)
	}
	expected := []interface{}{
		AddAssociationStimulus{"AssociateAccount", []string{"root/self"}, ""},
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
}
