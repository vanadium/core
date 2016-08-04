// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agentlib_test

import (
	"bufio"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"v.io/v23/security"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/agent"
	"v.io/x/ref/services/agent/agentlib"
	"v.io/x/ref/services/agent/internal/constants"
	"v.io/x/ref/test/v23test"
)

func createFakeV23AgentdDir(t *testing.T, sh *v23test.Shell) string {
	d := sh.MakeTempDir()
	if err := ioutil.WriteFile(filepath.Join(d, "v23agentd"), []byte("junk"), 0700); err != nil {
		t.Fatalf("failed to create fake agent: %v", err)
	}
	return d
}

// TestV23AgentPrincipal tests LoadPrincipal, which spawns an agent process.
func TestV23AgentPrincipal(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	sh := v23test.NewShell(t, nil)
	sh.PropagateChildOutput = true
	// Make sure we enable launching an agent when loading credentials.
	os.Unsetenv(ref.EnvCredentialsNoAgent)
	// This is not strictly needed since we don't create child processes
	// using the shell that need to load credentials using the agent, but
	// it's good practice to use the same setting in the parent and any
	// children.
	delete(sh.Vars, ref.EnvCredentialsNoAgent)
	defer sh.Cleanup()

	agentd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")

	// Create a credentials directory with an unencrypted principal.
	credsDir := sh.MakeTempDir()
	principal, err := vsecurity.CreatePersistentPrincipal(credsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, "happy"); err != nil {
		t.Fatal(err)
	}

	// loadPrincipal uses LoadPrincipal and verifies the blessing is as
	// expected.
	loadPrincipal := func() agent.Principal {
		p, err := agentlib.LoadPrincipal(credsDir)
		if err != nil {
			t.Fatalf("LoadPrincipal(%v) failed: %v", credsDir, err)
		}
		blessings, _ := p.BlessingStore().Default()
		if got, want := security.BlessingNames(p, blessings), []string{"happy"}; !reflect.DeepEqual(got, want) {
			t.Errorf("Got %v want %v", got, want)
		}
		return p
	}

	origPATH := os.Getenv("PATH")
	badAgentDir := createFakeV23AgentdDir(t, sh)
	badAgentPATH := func() {
		// Set up the PATH with an invalid agent.
		if err = os.Setenv("PATH", badAgentDir+":"+origPATH); err != nil {
			t.Fatalf("Setenv failed: %v", err)
		}
	}
	goodAgentPATH := func() {
		// Put v23agentd we built above in the PATH.
		if err = os.Setenv("PATH", filepath.Dir(agentd)+":"+origPATH); err != nil {
			t.Fatalf("Setenv failed: %v", err)
		}
	}

	badAgentPATH()
	// Load the principal locally (since an agent can't be started
	// successfully).
	p1 := loadPrincipal()

	// A subsequent attempt to load the principal should fail since the
	// local loading is exclusive.
	if _, err := agentlib.LoadPrincipal(credsDir); err == nil {
		t.Fatalf("Should have not been able to load the principal.")
	}

	// Even if we put a valid agent in the PATH, it should still not be able
	// to load the principal due to the exclusivity.
	goodAgentPATH()
	if _, err := agentlib.LoadPrincipal(credsDir); err == nil {
		t.Fatalf("Should have not been able to load the principal.")
	}

	// After letting go of the locally loaded principal, a subsequent local
	// load will succeed.
	p1.Close()
	badAgentPATH()
	p1 = loadPrincipal()
	p1.Close()

	// Use the agent now.
	goodAgentPATH()
	p1 = loadPrincipal()
	// Set up a way to communicate with the agent's commands socket.
	cmds, err := net.Dial("unix", filepath.Join(constants.AgentDir(credsDir), "commands"))
	if err != nil {
		t.Fatal(err)
	}
	defer cmds.Close()
	cmdsRead := bufio.NewScanner(cmds)
	sendCmd := func(cmd string) {
		if _, err := cmds.Write([]byte(cmd + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	cmdReply := func() string {
		if !cmdsRead.Scan() {
			t.Fatal("failed to read reply")
		}
		if err := cmdsRead.Err(); err != nil {
			t.Fatal(err)
		}
		return cmdsRead.Text()
	}
	expectReply := func(expected string) {
		if reply := cmdReply(); reply != expected {
			t.Fatalf("expected %v, got %v", expected, reply)
		}
	}

	// There ought to be one client connected to the agent (p1).
	sendCmd("CONNECTIONS")
	expectReply("1")
	// The agent is not idle.
	sendCmd("IDLE")
	expectReply("0.00")

	p2 := loadPrincipal()

	// Now there are two clients (p1 and p2).
	sendCmd("CONNECTIONS")
	expectReply("2")

	// Close p2 and recreate a new principal.
	p2.Close()
	p2 = loadPrincipal()

	// Still two clients.
	sendCmd("CONNECTIONS")
	expectReply("2")

	p1.Close()
	p2.Close()

	// Now the agent should become idle.
	sendCmd("IDLE")
	timeout := time.After(20 * time.Second)
	for cmdReply() == "0.00" {
		sendCmd("IDLE")
		time.Sleep(10 * time.Millisecond)
		select {
		case <-timeout:
			t.Logf("Warning: still waiting to become idle after %v", timeout)
		default:
		}
	}

	// Shorten the grace period to 1 second; the agent should exit.
	sendCmd("GRACE=1")
	expectReply("OK")
	// Scan will block until the agent exits.
	if cmdsRead.Scan() {
		t.Errorf("Agent didn't seem to have exited")
	}

	p1 = loadPrincipal()

	// The agent restarted, so re-establish the commands socket connection.
	cmds, err = net.Dial("unix", filepath.Join(constants.AgentDir(credsDir), "commands"))
	if err != nil {
		t.Fatal(err)
	}
	defer cmds.Close()
	cmdsRead = bufio.NewScanner(cmds)

	// One connection (p1).
	sendCmd("CONNECTIONS")
	expectReply("1")

	// Tell the agent to exit.
	sendCmd("EXIT")
	// Either it replied "OK", or it exited before it was able to reply "OK"
	// (in which case the socket connection would have broken).
	if cmdsRead.Scan() && cmdsRead.Text() != "OK" {
		t.Fatal("failed to OK")
	}
	if err := cmdsRead.Err(); err != nil {
		t.Fatal(err)
	}
}
