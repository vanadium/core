// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

const pkgPath = "v.io/x/ref/services/device/main"

var (
	errOops = verror.Register(pkgPath+".errOops", verror.NoRetry, "oops!")
)

func TestAccessListGetCommand(t *testing.T) {
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

	// Test the 'get' command.
	rootTape := tapes.ForSuffix("")
	rootTape.SetResponses(GetPermissionsResponse{
		perms: access.Permissions{
			"Admin": access.AccessList{
				In:    []security.BlessingPattern{"self"},
				NotIn: []string{"self/bad"},
			},
			"Read": access.AccessList{
				In: []security.BlessingPattern{"other", "self"},
			},
		},
		version: "aVersionForToday",
		err:     nil,
	})

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "get", deviceName}); err != nil {
		t.Fatalf("error: %v", err)
	}
	if expected, got := strings.TrimSpace(`
other Read
self Admin,Read
self/bad !Admin
`), strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Unexpected output from get. Got %q, expected %q", got, expected)
	}
	if got, expected := rootTape.Play(), []interface{}{"GetPermissions"}; !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
}

func TestAccessListSetCommand(t *testing.T) {
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

	// Some tests to validate parse.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 1, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName, "foo"}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 2, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName, "foo", "bar", "ohno"}); err == nil {
		t.Fatalf("failed to correctly detect insufficient parameters")
	}
	if expected, got := "ERROR: set: incorrect number of arguments 4, must be 1 + 2n", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	stderr.Reset()
	stdout.Reset()
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName, "foo", "!"}); err == nil {
		t.Fatalf("failed to detect invalid parameter")
	}
	if expected, got := "ERROR: failed to parse access tags for \"foo\": empty access tag", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Errorf("Unexpected output from list. Got %q, expected prefix %q", got, expected)
	}

	// Correct operation in the absence of errors.
	stderr.Reset()
	stdout.Reset()
	rootTape := tapes.ForSuffix("")
	rootTape.SetResponses(
		GetPermissionsResponse{
			perms: access.Permissions{
				"Admin": access.AccessList{
					In: []security.BlessingPattern{"self"},
				},
				"Read": access.AccessList{
					In:    []security.BlessingPattern{"other", "self"},
					NotIn: []string{"other/bob"},
				},
			},
			version: "aVersionForToday",
			err:     nil,
		},
		verror.NewErrBadVersion(nil),
		GetPermissionsResponse{
			perms: access.Permissions{
				"Admin": access.AccessList{
					In: []security.BlessingPattern{"self"},
				},
				"Read": access.AccessList{
					In:    []security.BlessingPattern{"other", "self"},
					NotIn: []string{"other/bob/baddevice"},
				},
			},
			version: "aVersionForTomorrow",
			err:     nil,
		},
		nil,
	)

	// set command that:
	// - Adds entry for "friends" to "Write" & "Admin"
	// - Adds a blacklist entry for "friend/alice"  for "Admin"
	// - Edits existing entry for "self" (adding "Write" access)
	// - Removes entry for "other/bob/baddevice"
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{
		"acl",
		"set",
		deviceName,
		"friends", "Admin,Write",
		"friends/alice", "!Admin,Write",
		"self", "Admin,Write,Read",
		"other/bob/baddevice", "^",
	}); err != nil {
		t.Fatalf("SetPermissions failed: %v", err)
	}

	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	if expected, got := "WARNING: trying again because of asynchronous change", strings.TrimSpace(stderr.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	expected := []interface{}{
		"GetPermissions",
		SetPermissionsStimulus{
			fun: "SetPermissions",
			perms: access.Permissions{
				"Admin": access.AccessList{
					In:    []security.BlessingPattern{"friends", "self"},
					NotIn: []string{"friends/alice"},
				},
				"Read": access.AccessList{
					In:    []security.BlessingPattern{"other", "self"},
					NotIn: []string{"other/bob"},
				},
				"Write": access.AccessList{
					In:    []security.BlessingPattern{"friends", "friends/alice", "self"},
					NotIn: []string(nil),
				},
			},
			version: "aVersionForToday",
		},
		"GetPermissions",
		SetPermissionsStimulus{
			fun: "SetPermissions",
			perms: access.Permissions{
				"Admin": access.AccessList{
					In:    []security.BlessingPattern{"friends", "self"},
					NotIn: []string{"friends/alice"},
				},
				"Read": access.AccessList{
					In:    []security.BlessingPattern{"other", "self"},
					NotIn: []string(nil),
				},
				"Write": access.AccessList{
					In:    []security.BlessingPattern{"friends", "friends/alice", "self"},
					NotIn: []string(nil),
				},
			},
			version: "aVersionForTomorrow",
		},
	}

	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// GetPermissions fails.
	rootTape.SetResponses(GetPermissionsResponse{
		perms:   access.Permissions{},
		version: "aVersionForToday",
		err:     verror.New(errOops, nil),
	})

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName, "vana/bad", "Read"}); err == nil {
		t.Fatalf("GetPermissions RPC inside perms set command failed but error wrongly not detected")
	} else if expected, got := `^GetPermissions\(`+deviceName+`\) failed:.*oops!`, err.Error(); !regexp.MustCompile(expected).MatchString(got) {
		t.Fatalf("Unexpected output from list. Got %q, regexp %q", got, expected)
	}
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}
	expected = []interface{}{
		"GetPermissions",
	}

	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// SetPermissions fails with something other than a bad version failure.
	rootTape.SetResponses(
		GetPermissionsResponse{
			perms: access.Permissions{
				"Read": access.AccessList{
					In: []security.BlessingPattern{"other", "self"},
				},
			},
			version: "aVersionForToday",
			err:     nil,
		},
		verror.New(errOops, nil),
	)

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"acl", "set", deviceName, "friend", "Read"}); err == nil {
		t.Fatalf("SetPermissions should have failed: %v", err)
	} else if expected, got := `^SetPermissions\(`+deviceName+`\) failed:.*oops!`, err.Error(); !regexp.MustCompile(expected).MatchString(got) {
		t.Fatalf("Unexpected output from list. Got %q, regexp %q", got, expected)
	}
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Fatalf("Unexpected output from list. Got %q, expected %q", got, expected)
	}

	expected = []interface{}{
		"GetPermissions",
		SetPermissionsStimulus{
			fun: "SetPermissions",
			perms: access.Permissions{
				"Read": access.AccessList{
					In:    []security.BlessingPattern{"friend", "other", "self"},
					NotIn: []string(nil),
				},
			},
			version: "aVersionForToday",
		},
	}

	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("invalid call sequence. Got %#v, want %#v", got, expected)
	}
}
