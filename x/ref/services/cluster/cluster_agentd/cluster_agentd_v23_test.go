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

	"v.io/v23/security"
	"v.io/x/ref"
	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/v23test"
)

func start(t *testing.T, c *v23test.Cmd) *expect.Session {
	c.Start()
	return c.S
}

func TestV23ClusterAgentD(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	workdir, err := ioutil.TempDir("", "cluster-agentd-test-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	agentCreds := sh.ForkCredentials("agent")
	aliceCreds := sh.ForkCredentials("alice")
	bobCreds := sh.ForkCredentials("bob")

	// Create a blessing (root:alice:prod) that alice will use to talk to
	// the cluster agent.
	alicePrincipal := aliceCreds.Principal
	aliceBlessings, _ := alicePrincipal.BlessingStore().Default()
	if prodBlessing, err := alicePrincipal.Bless(alicePrincipal.PublicKey(), aliceBlessings, "prod", security.UnconstrainedUse()); err != nil {
		t.Fatalf("Failed to create alice:prod blessing: %v", err)
	} else if _, err := alicePrincipal.BlessingStore().Set(prodBlessing, security.BlessingPattern("root:agent")); err != nil {
		t.Fatalf("Failed to set alice:prod for root:agent: %v", err)
	}

	var (
		agentBin     = v23test.BuildGoPkg(sh, "v.io/x/ref/services/cluster/cluster_agentd")
		clientBin    = v23test.BuildGoPkg(sh, "v.io/x/ref/services/cluster/cluster_agent")
		podAgentBin  = v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/pod_agentd")
		principalBin = v23test.BuildGoPkg(sh, "v.io/x/ref/cmd/principal")
	)

	// Start the cluster agent.
	addr := start(t, sh.Cmd(agentBin,
		"--v23.tcp.address=127.0.0.1:0",
		"--v23.permissions.literal={\"Admin\":{\"In\":[\"root:alice:prod\"]}}",
		"--root-dir="+workdir,
	).WithCredentials(agentCreds)).ExpectVar("NAME")

	// Create a new secret.
	secret := strings.TrimSpace(sh.Cmd(clientBin,
		"--agent="+addr,
		"new",
		"foo",
	).WithCredentials(aliceCreds).Stdout())
	secretPath := filepath.Join(workdir, "secret")
	if err := ioutil.WriteFile(secretPath, []byte(secret), 0600); err != nil {
		t.Fatalf("Unexpected WriteFile error: %v", err)
	}

	// Start the pod agent.
	sockPath := filepath.Join(workdir, "agent.sock")
	sh.Cmd(podAgentBin,
		"--agent="+addr,
		"--socket-path="+sockPath,
		"--secret-key-file="+secretPath,
		"--root-blessings="+rootBlessings(t, sh, agentCreds),
	).Start()

	// The principal served by the pod agent should have a blessing name
	// that starts with root:alice:foo:.
	cmd := sh.Cmd(principalBin, "dump", "-s")
	cmd.Vars[ref.EnvAgentPath] = sockPath
	delete(cmd.Vars, ref.EnvCredentials) // set by v23test.Shell.Cmd
	if got, want := cmd.Stdout(), "root:alice:foo:"; !strings.HasPrefix(got, want) {
		t.Errorf("Unexpected output. Got %q, wanted prefix %q", got, want)
	}

	// Bob should not be able to call NewSecret.
	cmd = sh.Cmd(clientBin, "--agent="+addr, "new", "foo").WithCredentials(bobCreds)
	cmd.ExitErrorIsOk = true
	if cmd.Run(); cmd.Err == nil {
		t.Errorf("Unexpected success; Bob should not be able to call NewSecret: %v", cmd.Err)
	}

	// After Alice calls ForgetSecret, SeekBlessings no longer works.
	sh.Cmd(clientBin, "--agent="+addr, "forget", secret).WithCredentials(aliceCreds).Run()
	cmd = sh.Cmd(clientBin, "--agent="+addr, "seekblessings", secret).WithCredentials(aliceCreds)
	cmd.ExitErrorIsOk = true
	if cmd.Run(); cmd.Err == nil {
		t.Errorf("Unexpected success; this secret should not exist anymore: %v", cmd.Err)
	}

	// The pod agent should be unaffected.
	cmd = sh.Cmd(principalBin, "dump", "-s")
	cmd.Vars[ref.EnvAgentPath] = sockPath
	delete(cmd.Vars, ref.EnvCredentials) // set by v23test.Shell.Cmd
	if got, want := cmd.Stdout(), "root:alice:foo:"; !strings.HasPrefix(got, want) {
		t.Errorf("Unexpected output. Got %q, wanted prefix %q", got, want)
	}
}

// Note: This is identical to rootBlessings in
// v.io/x/ref/services/device/claimable/claimable_v23_test.go.
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
