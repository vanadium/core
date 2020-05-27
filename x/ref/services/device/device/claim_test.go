// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/security"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/test"

	cmd_device "v.io/x/ref/services/device/device"
	"v.io/x/ref/services/internal/servicetest"
)

func TestClaimCommand(t *testing.T) { //nolint:gocyclo
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
	deviceKey, err := v23.GetPrincipal(ctx).PublicKey().MarshalBinary()
	if err != nil {
		t.Fatalf("Failed to marshal principal public key: %v", err)
	}

	// Confirm that we correctly enforce the number of arguments.
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"claim", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: claim: incorrect number of arguments, expected atleast 2 (max: 4), got 1", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from claim. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	rootTape := tapes.ForSuffix("")
	rootTape.Rewind()

	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"claim", "nope", "nope", "nope", "nope", "nope"}); err == nil {
		t.Fatalf("wrongly failed to receive a non-nil error.")
	}
	if expected, got := "ERROR: claim: incorrect number of arguments, expected atleast 2 (max: 4), got 5", strings.TrimSpace(stderr.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from claim. Got %q, expected prefix %q", got, expected)
	}
	stdout.Reset()
	stderr.Reset()
	rootTape.Rewind()

	// Incorrect operation
	var pairingToken string
	var deviceKeyWrong []byte
	if publicKey, _, err := security.NewPrincipalKey(); err != nil {
		t.Fatalf("NewPrincipalKey failed: %v", err)
	} else {
		if deviceKeyWrong, err = publicKey.MarshalBinary(); err != nil {
			t.Fatalf("Failed to marshal principal public key: %v", err)
		}
	}
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"claim", deviceName, "grant", pairingToken, base64.URLEncoding.EncodeToString(deviceKeyWrong)}); verror.ErrorID(err) != verror.ErrNotTrusted.ID {
		t.Fatalf("wrongly failed to receive correct error on claim with incorrect device key:%v id:%v", err, verror.ErrorID(err))
	}
	stdout.Reset()
	stderr.Reset()
	rootTape.Rewind()

	// Correct operation.
	rootTape.SetResponses(nil)
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"claim", deviceName, "grant", pairingToken, base64.URLEncoding.EncodeToString(deviceKey)}); err != nil {
		t.Fatalf("Claim(%s, %s, %s) failed: %v", deviceName, "grant", pairingToken, err)
	}
	if got, expected := len(rootTape.Play()), 1; got != expected {
		t.Errorf("invalid call sequence. Got %v, want %v", got, expected)
	}
	if expected, got := "Successfully claimed.", strings.TrimSpace(stdout.String()); !strings.HasPrefix(got, expected) {
		t.Fatalf("Unexpected output from claim. Got %q, expected prefix %q", got, expected)
	}
	expected := []interface{}{
		"Claim",
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
	rootTape.Rewind()
	stdout.Reset()
	stderr.Reset()

	// Error operation.
	rootTape.SetResponses(verror.New(errOops, nil))
	if err := v23cmd.ParseAndRunForTest(cmd, ctx, env, []string{"claim", deviceName, "grant", pairingToken}); err == nil {
		t.Fatal("claim() failed to detect error:", err)
	}
	expected = []interface{}{
		"Claim",
	}
	if got := rootTape.Play(); !reflect.DeepEqual(expected, got) {
		t.Errorf("unexpected result. Got %v want %v", got, expected)
	}
}
