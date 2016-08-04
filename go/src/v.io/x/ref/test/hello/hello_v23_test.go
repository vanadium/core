// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hello_test

import (
	"testing"

	"v.io/x/ref"
	"v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

func init() {
	ref.EnvClearCredentials()
}

func withCreds(dir string, c *v23test.Cmd) *v23test.Cmd {
	c.Vars[ref.EnvCredentials] = dir
	return c
}

func withCredsSock(sock string, c *v23test.Cmd) *v23test.Cmd {
	c.Vars[ref.EnvAgentPath] = sock
	return c
}

// setupCreds makes a bunch of credentials directories.
// We do this ourselves instead of using v23test's credentials APIs because we
// want to use the actual v23agentd binary, so that for regression tests we can
// test against old agents.
func setupCreds(sh *v23test.Shell, names ...string) (map[string]string, error) {
	idp := testutil.NewIDProvider("root")
	out := make(map[string]string, len(names))
	for _, name := range names {
		dir := sh.MakeTempDir()
		p, err := security.CreatePersistentPrincipal(dir, nil)
		if err != nil {
			return nil, err
		}
		if err := idp.Bless(p, name); err != nil {
			return nil, err
		}
		out[name] = dir
	}
	return out, nil
}

func startAgent(t *testing.T, sh *v23test.Shell, agentdbin, creds string) string {
	agent := sh.Cmd(agentdbin, "--daemon=false", "--credentials="+creds)
	agent.Start()
	agentSock := agent.S.ExpectVar(ref.EnvAgentPath)
	if agent.S.Failed() {
		t.Fatalf("Could not get %s: %v", ref.EnvAgentPath, agent.S.Error())
	}
	return agentSock
}

func withAgentCreds(t *testing.T, sh *v23test.Shell, agentdbin, creds string, c *v23test.Cmd) *v23test.Cmd {
	return withCredsSock(startAgent(t, sh, agentdbin, creds), c)
}

func TestV23HelloDirect(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	creds, err := setupCreds(sh, "helloclient", "helloserver")
	if err != nil {
		t.Fatalf("Could not create credentials: %v", err)
	}
	clientbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloclient")
	serverbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloserver")

	server := withCreds(creds["helloserver"], sh.Cmd(serverbin))
	server.Start()
	name := server.S.ExpectVar("SERVER_NAME")
	if server.S.Failed() {
		t.Fatalf("Could not get SERVER_NAME: %v", server.S.Error())
	}
	withCreds(creds["helloclient"], sh.Cmd(clientbin, "--name", name)).Run()
}

func TestV23HelloAgentd(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	creds, err := setupCreds(sh, "helloclient", "helloserver")
	if err != nil {
		t.Fatalf("Could not create credentials: %v", err)
	}
	agentdbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")
	serverbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloserver")
	clientbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloclient")

	server := withCreds(creds["helloserver"], sh.Cmd(serverbin))
	server.Start()
	name := server.S.ExpectVar("SERVER_NAME")
	if server.S.Failed() {
		t.Fatalf("Could not get SERVER_NAME: %v", server.S.Error())
	}
	withAgentCreds(t, sh, agentdbin, creds["helloclient"], sh.Cmd(clientbin, "--name", name)).Run()
}

func TestV23HelloMounttabled(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	creds, err := setupCreds(sh, "helloclient", "helloserver", "mounttabled")
	if err != nil {
		t.Fatalf("Could not create credentials: %v", err)
	}
	agentdbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")
	mounttabledbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/mounttable/mounttabled")
	serverbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloserver")
	clientbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloclient")

	name := "hello"
	mounttabled := withAgentCreds(t, sh, agentdbin, creds["mounttabled"], sh.Cmd(mounttabledbin, "--v23.tcp.address", "127.0.0.1:0"))
	mounttabled.Start()
	mtname := mounttabled.S.ExpectVar("NAME")
	if mounttabled.S.Failed() {
		t.Fatalf("Could not get NAME: %v", mounttabled.S.Error())
	}
	withAgentCreds(t, sh, agentdbin, creds["helloserver"], sh.Cmd(serverbin, "--name", name, "--v23.namespace.root", mtname)).Start()
	withAgentCreds(t, sh, agentdbin, creds["helloclient"], sh.Cmd(clientbin, "--name", name, "--v23.namespace.root", mtname)).Run()
}

func TestV23HelloProxy(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	creds, err := setupCreds(sh, "helloclient", "helloserver", "mounttabled", "proxyd", "xproxyd")
	if err != nil {
		t.Fatalf("Could not create credentials: %v", err)
	}
	agentdbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/agent/v23agentd")
	mounttabledbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/mounttable/mounttabled")
	xproxydbin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/xproxy/xproxyd")
	serverbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloserver")
	clientbin := v23test.BuildGoPkg(sh, "v.io/x/ref/test/hello/helloclient")

	name := "hello"
	mounttabled := withAgentCreds(t, sh, agentdbin, creds["mounttabled"], sh.Cmd(mounttabledbin, "--v23.tcp.address", "127.0.0.1:0"))
	mounttabled.Start()
	mtname := mounttabled.S.ExpectVar("NAME")
	if mounttabled.S.Failed() {
		t.Fatalf("Could not get NAME: %v", mounttabled.S.Error())
	}
	proxyname := "proxy"
	withAgentCreds(t, sh, agentdbin, creds["xproxyd"], sh.Cmd(xproxydbin, "--name", proxyname, "--v23.tcp.address", "127.0.0.1:0", "--v23.namespace.root", mtname, "--access-list", "{\"In\":[\"root\"]}")).Start()
	withAgentCreds(t, sh, agentdbin, creds["helloserver"], sh.Cmd(serverbin, "--name", name, "--v23.proxy", proxyname, "--v23.tcp.address", "", "--v23.namespace.root", mtname)).Start()
	withAgentCreds(t, sh, agentdbin, creds["helloclient"], sh.Cmd(clientbin, "--name", name, "--v23.proxy", proxyname, "--v23.tcp.address", "", "--v23.namespace.root", mtname)).Run()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
