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
	"v.io/v23/naming"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
)

var (
	installationUninstalled = device.StatusInstallation{device.InstallationStatus{
		State:   device.InstallationStateUninstalled,
		Version: "director's cut",
	}}
	installationActive = device.StatusInstallation{device.InstallationStatus{
		State:   device.InstallationStateActive,
		Version: "extended cut",
	}}
	instanceUpdating = device.StatusInstance{device.InstanceStatus{
		State:   device.InstanceStateUpdating,
		Version: "theatrical version",
	}}
	instanceRunning = device.StatusInstance{device.InstanceStatus{
		State:   device.InstanceStateRunning,
		Version: "tv version",
	}}
	instanceNotRunning = device.StatusInstance{device.InstanceStatus{
		State:   device.InstanceStateNotRunning,
		Version: "special edition",
	}}
	instanceDeleted = device.StatusInstance{device.InstanceStatus{
		State:   device.InstanceStateDeleted,
		Version: "mini series",
	}}
	deviceService = device.StatusDevice{device.DeviceStatus{
		State:   device.InstanceStateRunning,
		Version: "han shot first",
	}}
	deviceUpdating = device.StatusDevice{device.DeviceStatus{
		State:   device.InstanceStateUpdating,
		Version: "international release",
	}}
)

func testHelper(t *testing.T, lower, upper string) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	tapes := servicetest.NewTapeMap()
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", newDispatcher(t, tapes))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	addr := server.Status().Endpoints[0].String()

	// Setup the command-line.
	cmd := cmd_device.CmdRoot
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	appName := naming.JoinAddressName(addr, "appname")

	// Confirm that we correctly enforce the number of arguments.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{lower}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: "+lower+": incorrect number of arguments, expected 1, got 0", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from %s. Got %q, expected prefix %q", lower, got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	appTape := tapes.ForSuffix("appname")
	appTape.Rewind()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{lower, "nope", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: "+lower+": incorrect number of arguments, expected 1, got 2", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from %s. Got %q, expected prefix %q", lower, got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	appTape.Rewind()

	// Correct operation.
	appTape.SetResponses(nil)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{lower, appName}); err != nil {
		t.Fatalf("%s failed when it shouldn't: %v", lower, err)
	}
	if expected, got := upper+" succeeded", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from %s. Got %q, expected %q", lower, got, expected)
	}
	if expected, got := []interface{}{upper}, appTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	appTape.Rewind()
	stderr.Reset()
	stdout.Reset()

	// Test with bad parameters.
	appTape.SetResponses(verror.New(errOops, nil))
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{lower, appName}); err == nil {
		t.Fatalf("wrongly didn't receive a non-nil error.")
	}
	// expected the same.
	if expected, got := []interface{}{upper}, appTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
}

func joinLines(args ...string) string {
	return strings.Join(args, "\n")
}
