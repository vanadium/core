// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestInstantiateCommand(t *testing.T) {
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
	appName := server.Status().Endpoints[0].Name()

	// Confirm that we correctly enforce the number of arguments.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"instantiate", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: instantiate: incorrect number of arguments, expected 2, got 1", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from instantiate. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	rootTape := tapes.ForSuffix("")
	rootTape.Rewind()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"instantiate", "nope", "nope", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: instantiate: incorrect number of arguments, expected 2, got 3", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from instantiate. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	rootTape.Rewind()

	// Correct operation.
	rootTape.SetResponses(InstantiateResponse{
		err:        nil,
		instanceID: "app1",
	})
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"instantiate", appName, "grant"}); err != nil {
		t.Fatalf("instantiate %s %s failed: %v", appName, "grant", err)
	}

	b := new(bytes.Buffer)
	fmt.Fprintf(b, "%s", appName+"/app1")
	if expected, got := b.String(), strings.TrimSpace(stdout.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from instantiate. Got %q, expected prefix %q", got, expected)
	}
	expected := []interface{}{
		"Instantiate",
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// Error operation.
	rootTape.SetResponses(InstantiateResponse{
		verror.New(errOops, nil),
		"",
	})
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"instantiate", appName, "grant"}); err == nil {
		t.Fatalf("instantiate failed to detect error")
	}
	expected = []interface{}{
		"Instantiate",
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
}
