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

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestStatusCommand(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	tapes := servicetest.NewTapeMap()
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", newDispatcher(t, tapes))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	cmd := cmd_device.CmdRoot
	addr := server.Status().Endpoints[0].String()
	globName := naming.JoinAddressName(addr, "glob")
	appName := naming.JoinAddressName(addr, "app")

	rootTape, appTape := tapes.ForSuffix(""), tapes.ForSuffix("app")
	for _, c := range []struct {
		tapeResponse device.Status
		expected     string
	}{
		{
			installationUninstalled,
			fmt.Sprintf("Installation %v [State:Uninstalled,Version:director's cut]", appName),
		},
		{
			instanceUpdating,
			fmt.Sprintf("Instance %v [State:Updating,Version:theatrical version]", appName),
		},
		{
			deviceService,
			fmt.Sprintf("Device Service %v [State:Running,Version:han shot first]", appName),
		},
	} {
		var stdout, stderr bytes.Buffer
		env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
		tapes.Rewind()
		rootTape.SetResponses(GlobResponse{results: []string{"app"}})
		appTape.SetResponses(c.tapeResponse)
		if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"status", globName}); err != nil {
			t.Errorf("%v", err)
		}
		if expected, got := c.expected, strings.TrimSpace(stdout.String()); got != expected {
			t.Errorf("Unexpected output from status. Got %q, expected %q", got, expected)
		}
		if got, expected := rootTape.Play(), []interface{}{GlobStimulus{"glob"}}; !reflect.DeepEqual(expected, got) {
			t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
		}
		if got, expected := appTape.Play(), []interface{}{"Status"}; !reflect.DeepEqual(expected, got) {
			t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
		}
		cmd_device.ResetGlobSettings()
	}
}
