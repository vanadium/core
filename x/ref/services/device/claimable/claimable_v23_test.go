// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libsec "v.io/x/ref/lib/security"
	"v.io/x/ref/test/v23test"
)

func TestV23ClaimableServer(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	workdir, err := ioutil.TempDir("", "claimable-test-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	permsDir := filepath.Join(workdir, "perms")

	serverCreds := sh.ForkCredentials("child")
	if err := libsec.InitDefaultBlessings(serverCreds.Principal, "server"); err != nil {
		t.Fatalf("Failed to create server credentials: %v", err)
	}
	legitClientCreds := sh.ForkCredentials("legit")
	badClientCreds1 := sh.ForkCredentials("child")
	badClientCreds2 := sh.ForkCredentials("other-guy")

	serverBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/device/claimable")
	server := sh.Cmd(serverBin,
		"--v23.tcp.address=127.0.0.1:0",
		"--perms-dir="+permsDir,
		"--root-blessings="+rootBlessings(t, sh, legitClientCreds),
		"--v23.permissions.literal={\"Admin\":{\"In\":[\"root:legit\"]}}",
	).WithCredentials(serverCreds)
	server.Start()
	addr := server.S.ExpectVar("NAME")

	clientBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/device/device")

	testcases := []struct {
		creds      *v23test.Credentials
		success    bool
		permsExist bool
	}{
		{badClientCreds1, false, false},
		{badClientCreds2, false, false},
		{legitClientCreds, true, true},
	}

	for _, tc := range testcases {
		client := sh.Cmd(clientBin, "claim", addr, "my-device").WithCredentials(tc.creds)
		client.ExitErrorIsOk = true
		if client.Run(); (client.Err == nil) != tc.success {
			t.Errorf("Unexpected exit value. Expected success=%v, got err=%v", tc.success, err)
		}
		if _, err := os.Stat(permsDir); (client.Err == nil) != tc.permsExist {
			t.Errorf("Unexpected permsDir state. Got %v, expected %v", err == nil, tc.permsExist)
		}
	}

	// Server should exit cleanly after the successful Claim.
	server.Wait()
}

// Note: Identical to rootBlessings in
// v.io/x/ref/services/cluster/cluster_agentd/cluster_agentd_v23_test.go.
func rootBlessings(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials) string {
	principalBin := v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")
	blessings := strings.TrimSpace(sh.Cmd(principalBin, "get", "default").WithCredentials(creds).Stdout())
	cmd := sh.Cmd(principalBin, "dumproots", "-")
	cmd.SetStdinReader(strings.NewReader(blessings))
	return strings.Replace(strings.TrimSpace(cmd.Stdout()), "\n", ",", -1)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
