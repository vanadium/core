// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bufio"
	"bytes"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/agent/internal/constants"
	"v.io/x/ref/test/v23test"
)

func upComesAgentd(t *testing.T, sh *v23test.Shell, credsDir, password string) {
	agentd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")

	agentC := sh.Cmd(agentd, "--daemon=false", "--credentials="+credsDir)
	if len(password) > 0 {
		agentC.SetStdinReader(strings.NewReader(password))
	}
	agentRead, agentWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	agentC.ExtraFiles = append(agentC.ExtraFiles, agentWrite)
	pipeFD := 3 + len(agentC.ExtraFiles) - 1
	agentC.Vars[constants.EnvAgentParentPipeFD] = strconv.Itoa(pipeFD)
	agentC.Vars["PATH"] = os.Getenv("PATH")
	agentC.Start()
	agentWrite.Close()
	scanner := bufio.NewScanner(agentRead)
	if !scanner.Scan() || scanner.Text() != constants.ServingMsg {
		t.Fatalf("Failed to receive \"%s\" from agent", constants.ServingMsg)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading standard input: %v", err)
	}
	agentRead.Close()
}

func TestV23UnencryptedPrincipal(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	sh.PropagateChildOutput = true
	defer sh.Cleanup()

	testbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/internal/test_principal")

	credsDir := sh.MakeTempDir()
	principal, err := vsecurity.CreatePersistentPrincipal(credsDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, "happy"); err != nil {
		t.Fatal(err)
	}

	const noPassword = ""
	upComesAgentd(t, sh, credsDir, noPassword)

	testC := sh.Cmd(testbin, "--expect-blessing=happy")
	testC.Vars[ref.EnvAgentPath] = constants.SocketPath(credsDir)
	testC.Run()
}

func TestV23EncryptedPrincipal(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	sh.PropagateChildOutput = true
	defer sh.Cleanup()

	testbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/internal/test_principal")

	credsDir := sh.MakeTempDir()
	principal, err := vsecurity.CreatePersistentPrincipal(credsDir, []byte("PASSWORD"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, "sneezy"); err != nil {
		t.Fatal(err)
	}

	upComesAgentd(t, sh, credsDir, "PASSWORD")

	testC := sh.Cmd(testbin, "--expect-blessing=sneezy")
	testC.Vars[ref.EnvAgentPath] = constants.SocketPath(credsDir)
	testC.Run()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}

func upComesAgentdDaemon(t *testing.T, sh *v23test.Shell, credsDir, password string) {
	agentd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")

	agentC := sh.Cmd(agentd, "--credentials="+credsDir)
	if len(password) > 0 {
		agentC.SetStdinReader(strings.NewReader(password))
	}
	agentC.Run()
}

func downComesAgentd(t *testing.T, sh *v23test.Shell, credsDir string) {
	agentd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")

	agentC := sh.Cmd(agentd, "stop", "--credentials="+credsDir)
	agentC.Run()
}

func TestV23Daemon(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	sh.PropagateChildOutput = true
	defer sh.Cleanup()

	testbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/internal/test_principal")

	credsDir := sh.MakeTempDir()
	principal, err := vsecurity.CreatePersistentPrincipal(credsDir, []byte("P@SsW0rd"))
	if err != nil {
		t.Fatal(err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, "grumpy"); err != nil {
		t.Fatal(err)
	}

	upComesAgentdDaemon(t, sh, credsDir, "P@SsW0rd")

	testC := sh.Cmd(testbin, "--expect-blessing=grumpy")
	testC.Vars[ref.EnvAgentPath] = constants.SocketPath(credsDir)
	testC.Run()

	// Since we started the agent as a daemon, we need to explicitly stop it
	// (since the Shell won't do it for us).
	downComesAgentd(t, sh, credsDir)
}

func TestV23PingPong(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	// Setup two principals: One for the agent that runs the pingpong
	// server, one for the client.  Since the server uses the default
	// authorization policy, the client must have a blessing delegated from
	// the server.
	var (
		clientDir, serverDir = createClientAndServerCredentials(t, sh)
		pingpong             = v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/internal/pingpong")
	)

	upComesAgentd(t, sh, serverDir, "")
	server := sh.Cmd(pingpong)
	server.Vars[ref.EnvAgentPath] = constants.SocketPath(serverDir)
	server.Start()
	serverName := server.S.ExpectVar("NAME")

	// Run the client via an agent once.
	upComesAgentd(t, sh, clientDir, "")
	client := sh.Cmd(pingpong, serverName)
	client.Vars[ref.EnvAgentPath] = constants.SocketPath(clientDir)
	client.Start()
	client.S.Expect("Pinging...")
	client.S.Expect("pong (client:[pingpongd:client] server:[pingpongd])")
	client.Wait()

	// Run it through a shell to test that the agent can pass credentials
	// to subprocess of a shell (making things like "agentd bash" provide
	// useful terminals).
	// This only works with shells that propagate open file descriptors to
	// children. POSIX-compliant shells do this as to many other commonly
	// used ones like bash.
	script := filepath.Join(sh.MakeTempDir(), "test.sh")
	if err := writeScript(
		script,
		`#!/bin/bash
echo "Running client"
{{.Bin}} {{.Server}} || exit 101
echo "Running client again"
{{.Bin}} {{.Server}} || exit 102
`,
		struct{ Bin, Server string }{pingpong, serverName}); err != nil {
		t.Fatal(err)
	}
	client = sh.Cmd("bash", script)
	client.Vars[ref.EnvAgentPath] = constants.SocketPath(clientDir)
	client.Start()
	client.S.Expect("Running client")
	client.S.Expect("Pinging...")
	client.S.Expect("pong (client:[pingpongd:client] server:[pingpongd])")
	client.S.Expect("Running client again")
	client.S.Expect("Pinging...")
	client.S.Expect("pong (client:[pingpongd:client] server:[pingpongd])")
	client.Wait()
	downComesAgentd(t, sh, clientDir)
	server.Terminate(syscall.SIGINT)
	downComesAgentd(t, sh, serverDir)
}

func writeScript(dstfile, tmpl string, args interface{}) error {
	t, err := template.New(dstfile).Parse(tmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, args); err != nil {
		return err
	}
	if err := ioutil.WriteFile(dstfile, buf.Bytes(), 0700); err != nil {
		return err
	}
	return nil
}

// createClientAndServerCredentials creates two principals, sets up their
// blessings and returns the created credentials directories.
//
// The server will have a single blessing "pingpongd".
// The client will have a single blessing "pingpongd:client", blessed by the
// server.
func createClientAndServerCredentials(t *testing.T, sh *v23test.Shell) (clientDir, serverDir string) {
	clientDir = sh.MakeTempDir()
	serverDir = sh.MakeTempDir()

	pserver, err := vsecurity.CreatePersistentPrincipal(serverDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	pclient, err := vsecurity.CreatePersistentPrincipal(clientDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Server will only serve, not make any client requests, so only needs a
	// default blessing.
	bserver, err := pserver.BlessSelf("pingpongd")
	if err != nil {
		t.Fatal(err)
	}
	if err := pserver.BlessingStore().SetDefault(bserver); err != nil {
		t.Fatal(err)
	}
	// Clients need not have a default blessing as they will only make a request
	// to the server.
	bclient, err := pserver.Bless(pclient.PublicKey(), bserver, "client", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pclient.BlessingStore().Set(bclient, "pingpongd"); err != nil {
		t.Fatal(err)
	}
	// The client and server must both recognize bserver and its delegates.
	if err := security.AddToRoots(pserver, bserver); err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(pclient, bserver); err != nil {
		t.Fatal(err)
	}
	return
}
