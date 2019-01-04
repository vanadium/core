// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"encoding/json"
	"strings"
	"testing"

	"v.io/v23/naming"
	"v.io/v23/services/application"
	"v.io/x/ref/test/v23test"
)

func helper(t *testing.T, sh *v23test.Shell, clientBin string, clientCreds *v23test.Credentials, expectError bool, cmd string, args ...string) string {
	args = append([]string{cmd}, args...)
	c := sh.Cmd(clientBin, args...).WithCredentials(clientCreds)
	c.ExitErrorIsOk = true
	stdout := c.Stdout()
	if c.Err != nil && !expectError {
		t.Fatalf("%s %q failed: %v\n%v", clientBin, strings.Join(args, " "), c.Err, stdout)
	}
	if c.Err == nil && expectError {
		t.Fatalf("%s %q did not fail when it should", clientBin, strings.Join(args, " "))
	}
	return strings.TrimSpace(stdout)
}

func matchEnvelope(t *testing.T, sh *v23test.Shell, clientBin string, clientCreds *v23test.Credentials, expectError bool, name, suffix string) string {
	return helper(t, sh, clientBin, clientCreds, expectError, "match", naming.Join(name, suffix), "test-profile")
}

func putEnvelope(t *testing.T, sh *v23test.Shell, clientBin string, clientCreds *v23test.Credentials, name, suffix, envelope string) string {
	return helper(t, sh, clientBin, clientCreds, false, "put", naming.Join(name, suffix), "test-profile", envelope)
}

func removeEnvelope(t *testing.T, sh *v23test.Shell, clientBin string, clientCreds *v23test.Credentials, name, suffix string) string {
	return helper(t, sh, clientBin, clientCreds, false, "remove", naming.Join(name, suffix), "test-profile")
}

func TestV23ApplicationRepository(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start the application repository.
	appRepoName := "test-app-repo"
	sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/application/applicationd"),
		"-name="+appRepoName,
		"-store="+sh.MakeTempDir(),
		"-v=2",
		"-v23.tcp.address=127.0.0.1:0").WithCredentials(sh.ForkCredentials("applicationd")).Start()

	// Build the client binary (must be a delegate of the server to pass
	// the default authorization policy).
	clientBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/application/application")
	clientCreds := sh.ForkCredentials("applicationd:client")

	// Generate publisher blessings
	publisher := sh.ForkCredentials("publisher")
	sig, err := publisher.Principal.Sign([]byte("binarycontents"))
	if err != nil {
		t.Fatal(err)
	}
	// Create an application envelope.
	appRepoSuffix := "test-application/v1"
	appEnvelopeFile := sh.MakeTempFile()
	publisherBlessings, _ := publisher.Principal.BlessingStore().Default()
	wantEnvelope, err := json.MarshalIndent(application.Envelope{
		Title: "title",
		Binary: application.SignedFile{
			File:      "foo",
			Signature: sig,
		},
		Publisher: publisherBlessings,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appEnvelopeFile.Write([]byte(wantEnvelope)); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}
	putEnvelope(t, sh, clientBin, clientCreds, appRepoName, appRepoSuffix, appEnvelopeFile.Name())

	// Match the application envelope.
	if got, want := matchEnvelope(t, sh, clientBin, clientCreds, false, appRepoName, appRepoSuffix), string(wantEnvelope); got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Remove the application envelope.
	removeEnvelope(t, sh, clientBin, clientCreds, appRepoName, appRepoSuffix)

	// Check that the application envelope no longer exists.
	matchEnvelope(t, sh, clientBin, clientCreds, true, appRepoName, appRepoSuffix)
}
