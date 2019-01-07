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
	"v.io/v23/naming"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestDebugCommand(t *testing.T) {
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
	rootTape.SetResponses(GlobResponse{results: []string{"app"}})

	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}

	debugMessage := "the secrets of the universe, revealed"
	appTape.SetResponses(instanceRunning, debugMessage)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"debug", globName}); err != nil {
		t.Fatalf("%v", err)
	}
	line := strings.Repeat("*", len(appName)+4)
	expected := fmt.Sprintf("%s\n* %s *\n%s\n%s", line, appName, line, debugMessage)
	if got := strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from debug. Got:\n%v\nExpected:\n%v", got, expected)
	}
	if got, expected := appTape.Play(), []interface{}{"Status", "Debug"}; !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
}
