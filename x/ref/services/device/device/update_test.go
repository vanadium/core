// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

// TODO(caprita): Rename to update_revert_test.go

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return ""
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// TestUpdateAndRevertCommands verifies the device update and revert commands.
func TestUpdateAndRevertCommands(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	tapes := servicetest.NewTapeMap()
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", newDispatcher(t, tapes))
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	addr := server.Status().Endpoints[0].String()
	root := cmd_device.CmdRoot
	appName := naming.JoinAddressName(addr, "app")
	rootTape := tapes.ForSuffix("")
	globName := naming.JoinAddressName(addr, "glob")
	// TODO(caprita): Move joinLines to a common place.
	joinLines := func(args ...string) string {
		return strings.Join(args, "\n")
	}
	for _, cmd := range []string{"update", "revert"} {
		for _, c := range []struct {
			globResponses   []string
			statusResponses map[string][]interface{}
			expectedStimuli map[string][]interface{}
			expectedStdout  string
			expectedStderr  string
			expectedError   string
		}{
			{ // Everything succeeds.
				[]string{"app/2", "app/1", "app/5", "app/3", "app/4"},
				map[string][]interface{}{
					"app/1": {instanceRunning, nil, nil, nil},
					"app/2": {instanceNotRunning, nil},
					"app/3": {installationActive, nil},
					// The uninstalled installation and the
					// deleted instance should be excluded
					// from the Update and Revert as per the
					// default GlobSettings for the update
					// and revert commands.
					"app/4": {installationUninstalled, nil},
					"app/5": {instanceDeleted, nil},
				},
				map[string][]interface{}{
					"app/1": {"Status", KillStimulus{"Kill", 10 * time.Second}, capitalize(cmd), "Run"},
					"app/2": {"Status", capitalize(cmd)},
					"app/3": {"Status", capitalize(cmd)},
				},
				joinLines(
					fmt.Sprintf("Successful %s of version for installation \"%s/3\".", cmd, appName),
					fmt.Sprintf("Successful %s of version for instance \"%s/1\".", cmd, appName),
					fmt.Sprintf("Successful %s of version for instance \"%s/2\".", cmd, appName)),
				"",
				"",
			},
			{ // Assorted failure modes.
				[]string{"app/1", "app/2", "app/3", "app/4", "app/5"},
				map[string][]interface{}{
					// Starts as running, fails Kill, but then
					// recovers. This ultimately counts as a success.
					"app/1": {instanceRunning, fmt.Errorf("Simulate Kill failing"), instanceNotRunning, nil, nil},
					// Starts as running, fails Kill, and stays running.
					"app/2": {instanceRunning, fmt.Errorf("Simulate Kill failing"), instanceRunning},
					// Starts as running, Kill and Update succeed, but Run fails.
					"app/3": {instanceRunning, nil, nil, fmt.Errorf("Simulate Run failing")},
					// Starts as running, Kill succeeds, Update fails, but Run succeeds.
					"app/4": {instanceRunning, nil, fmt.Errorf("Simulate %s failing", capitalize(cmd)), nil},
					// Starts as running, Kill succeeds, Update fails, and Run fails.
					"app/5": {instanceRunning, nil, fmt.Errorf("Simulate %s failing", capitalize(cmd)), fmt.Errorf("Simulate Run failing")},
				},
				map[string][]interface{}{
					"app/1": {"Status", KillStimulus{"Kill", 10 * time.Second}, "Status", capitalize(cmd), "Run"},
					"app/2": {"Status", KillStimulus{"Kill", 10 * time.Second}, "Status"},
					"app/3": {"Status", KillStimulus{"Kill", 10 * time.Second}, capitalize(cmd), "Run"},
					"app/4": {"Status", KillStimulus{"Kill", 10 * time.Second}, capitalize(cmd), "Run"},
					"app/5": {"Status", KillStimulus{"Kill", 10 * time.Second}, capitalize(cmd), "Run"},
				},
				joinLines(
					fmt.Sprintf("Successful %s of version for instance \"%s/1\".", cmd, appName),
					fmt.Sprintf("Successful %s of version for instance \"%s/3\".", cmd, appName),
				),
				joinLines(
					fmt.Sprintf("WARNING for \"%s/1\": recovered from Kill error (device.test:<rpc.Client>\"%s/1\".Kill: Error: Simulate Kill failing). Proceeding with %s.", appName, appName, cmd),
					fmt.Sprintf("ERROR for \"%s/2\": Kill failed: device.test:<rpc.Client>\"%s/2\".Kill: Error: Simulate Kill failing.", appName, appName),
					fmt.Sprintf("ERROR for \"%s/3\": Run failed: device.test:<rpc.Client>\"%s/3\".Run: Error: Simulate Run failing.", appName, appName),
					fmt.Sprintf("ERROR for \"%s/4\": %s failed: device.test:<rpc.Client>\"%s/4\".%s: Error: Simulate %s failing.", appName, capitalize(cmd), appName, capitalize(cmd), capitalize(cmd)),
					fmt.Sprintf("ERROR for \"%s/5\": Run failed: device.test:<rpc.Client>\"%s/5\".Run: Error: Simulate Run failing.", appName, appName),
					fmt.Sprintf("ERROR for \"%s/5\": %s failed: device.test:<rpc.Client>\"%s/5\".%s: Error: Simulate %s failing.", appName, capitalize(cmd), appName, capitalize(cmd), capitalize(cmd)),
				),
				"encountered a total of 4 error(s)",
			},
		} {
			var stdout, stderr bytes.Buffer
			env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
			tapes.Rewind()
			rootTape.SetResponses(GlobResponse{results: c.globResponses})
			for n, r := range c.statusResponses {
				tapes.ForSuffix(n).SetResponses(r...)
			}
			args := []string{cmd, globName}
			if err := v23cmd.ParseAndRunForTest(root, ctx, env, args); err != nil {
				if want, got := c.expectedError, err.Error(); want != got {
					t.Errorf("Unexpected error: want %v, got %v", want, got)
				}
			} else {
				if c.expectedError != "" {
					t.Errorf("Expected to get error %v, but didn't get any error.", c.expectedError)
				}
			}

			if expected, got := c.expectedStdout, strings.TrimSpace(stdout.String()); got != expected {
				t.Errorf("Unexpected stdout output from %s.\nGot:\n%v\nExpected:\n%v", cmd, got, expected)
			}
			if expected, got := c.expectedStderr, strings.TrimSpace(stderr.String()); got != expected {
				t.Errorf("Unexpected stderr output from %s.\nGot:\n%v\nExpected:\n%v", cmd, got, expected)
			}
			for n, m := range c.expectedStimuli {
				if want, got := m, tapes.ForSuffix(n).Play(); !reflect.DeepEqual(want, got) {
					t.Errorf("Unexpected stimuli for %v. Want: %v, got %v.", n, want, got)
				}
			}
			cmd_device.ResetGlobSettings()
		}
	}
}
