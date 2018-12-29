// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/services/application"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestInstallCommand(t *testing.T) {
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
	appId := "myBestAppID"
	cfg := device.Config{"someflag": "somevalue"}
	pkg := application.Packages{"pkg": application.SignedFile{File: "somename"}}
	rootTape := tapes.ForSuffix("")
	for i, c := range []struct {
		args         []string
		config       device.Config
		packages     application.Packages
		shouldErr    bool
		tapeResponse interface{}
		expectedTape interface{}
	}{
		{
			[]string{"blech"},
			nil,
			nil,
			true,
			nil,
			nil,
		},
		{
			[]string{"blech1", "blech2", "blech3", "blech4"},
			nil,
			nil,
			true,
			nil,
			nil,
		},
		{
			[]string{deviceName, appNameNoFetch, "not-valid-json"},
			nil,
			nil,
			true,
			nil,
			nil,
		},
		{
			[]string{deviceName, appNameNoFetch},
			nil,
			nil,
			false,
			InstallResponse{appId, nil},
			InstallStimulus{"Install", appNameNoFetch, nil, nil, application.Envelope{}, nil},
		},
		{
			[]string{deviceName, appNameNoFetch},
			cfg,
			pkg,
			false,
			InstallResponse{appId, nil},
			InstallStimulus{"Install", appNameNoFetch, cfg, pkg, application.Envelope{}, nil},
		},
	} {
		rootTape.SetResponses(c.tapeResponse)
		if c.config != nil {
			jsonConfig, err := json.Marshal(c.config)
			if err != nil {
				t.Fatalf("test case %d: Marshal(%v) failed: %v", i, c.config, err)
			}
			c.args = append([]string{fmt.Sprintf("--config=%s", string(jsonConfig))}, c.args...)
		}
		if c.packages != nil {
			jsonPackages, err := json.Marshal(c.packages)
			if err != nil {
				t.Fatalf("test case %d: Marshal(%v) failed: %v", i, c.packages, err)
			}
			c.args = append([]string{fmt.Sprintf("--packages=%s", string(jsonPackages))}, c.args...)
		}
		c.args = append([]string{"install"}, c.args...)
		err := v23cmd.ParseAndRunForTest(cmd, ctx, env, c.args)
		if c.shouldErr {
			if err == nil {
				t.Fatalf("test case %d: wrongly failed to receive a non-nil error.", i)
			}
			if got, expected := len(rootTape.Play()), 0; got != expected {
				t.Errorf("test case %d: invalid call sequence. Got %v, want %v", i, got, expected)
			}
		} else {
			if err != nil {
				t.Fatalf("test case %d: %v", i, err)
			}
			if expected, got := naming.Join(deviceName, appId), strings.TrimSpace(stdout.String()); got != expected {
				t.Fatalf("test case %d: Unexpected output from Install. Got %q, expected %q", i, got, expected)
			}
			if got, expected := rootTape.Play(), []interface{}{c.expectedTape}; !reflect.DeepEqual(expected, got) {
				t.Errorf("test case %d: invalid call sequence. Got %#v, want %#v", i, got, expected)
			}
		}
		rootTape.Rewind()
		stdout.Reset()
	}
}
