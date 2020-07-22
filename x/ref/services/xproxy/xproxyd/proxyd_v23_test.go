// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/stats"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/x/lib/gosh"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

const (
	proxyName    = "proxy"     // Name which the proxy mounts itself at
	serverName   = "server"    // Name which the server mounts itself at
	responseVar  = "RESPONSE"  // Name of the variable used by client program to output the first response
	responseVar1 = "RESPONSE1" // Name of the variable used by client program to output the second response
	downloadSize = 64 * 1024 * 1024
)

func TestV23Proxyd(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		proxydCreds = sh.ForkCredentials("proxyd")
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		proxyd      = v23test.BuildGoPkg(sh, "v.io/x/ref/services/xproxy/xproxyd")
	)

	// Start proxyd.
	sh.Cmd(proxyd, "--v23.tcp.address=127.0.0.1:0", "--name="+proxyName, "--access-list", "{\"In\":[\"root:server\"]}").WithCredentials(proxydCreds).Start()

	// Start the server that only listens via the proxy.
	sh.FuncCmd(runServer).WithCredentials(serverCreds).Start()

	// Run the client.
	cmd := sh.FuncCmd(runClient).WithCredentials(clientCreds)
	cmd.Run()
	if got, want := cmd.S.ExpectVar(responseVar), "server [root:server] saw client [root:client]"; got != want {
		t.Fatalf("Got %q, want %q", got, want)
	}
	if got, want := cmd.S.ExpectVar(responseVar1), fmt.Sprintf("%v", downloadSize); got != want {
		t.Fatalf("Got %q, want %q", got, want)
	}
}

func waitForNMountedServers(ctx *context.T, ns namespace.T, name string, expected int) ([]naming.MountedServer, error) {
	start := time.Now()
	for {
		resolved, err := ns.Resolve(ctx, name)
		if verror.ErrorID(err) == naming.ErrNoSuchName.ID {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("unexpected error waiting for %v servers for %v: %v", expected, name, err)
		}
		if len(resolved.Servers) == expected {
			return resolved.Servers, nil
		}
		if time.Since(start) > time.Second*30 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
	return nil, fmt.Errorf("timed out after %v waiting for %v servers for %v", time.Since(start), expected, name)
}

func TestV23MultipleProxyd(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		proxyd      = v23test.BuildGoPkg(sh, "v.io/x/ref/services/xproxy/xproxyd")
		numProxies  = 2
	)

	proxyLogs := make([]*bytes.Buffer, numProxies)
	serverLog := &bytes.Buffer{}

	// Start multiple proxies.
	for i := 0; i < numProxies; i++ {
		proxyLogs[i] = &bytes.Buffer{}
		proxydCreds := sh.ForkCredentials("proxyd")
		cmd := sh.Cmd(proxyd, "--v23.tcp.address=127.0.0.1:0", "--name="+proxyName, "--access-list", "{\"In\":[\"root:server\"]}").WithCredentials(proxydCreds)
		cmd.AddStdoutWriter(proxyLogs[i])
		cmd.AddStderrWriter(proxyLogs[i])
		cmd.Start()
	}

	// Wait for all proxies to be mounted.
	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))
	if _, err := waitForNMountedServers(sh.Ctx, ns, proxyName, numProxies); err != nil {
		t.Fatal(err)
	}

	proxyStatsAddresses, err := waitForNMountedServers(sh.Ctx, ns, proxyName+"-mon", numProxies)
	if err != nil {
		t.Fatal(err)
	}

	// Start the server that only listens via the proxy.
	cmd := sh.FuncCmd(runServer).WithCredentials(serverCreds)
	cmd.AddStdoutWriter(serverLog)
	cmd.AddStderrWriter(serverLog)
	cmd.Start()

	// Run the client enough times to use multiple proxies.
	for i := 0; i < numProxies*2; i++ {
		cmd = sh.FuncCmd(runClient).WithCredentials(clientCreds)
		cmd.Run()
		if got, want := cmd.S.ExpectVar(responseVar), "server [root:server] saw client [root:client]"; got != want {
			t.Fatalf("Got %q, want %q", got, want)
		}
		if got, want := cmd.S.ExpectVar(responseVar1), fmt.Sprintf("%v", downloadSize); got != want {
			t.Fatalf("Got %q, want %q", got, want)
		}
	}

	// Need the address of the server to determine the stats suffixes to use
	// with each proxy.
	server, err := ns.Resolve(sh.Ctx, "server")
	if err != nil || len(server.Servers) == 0 {
		t.Fatalf("failed to resolve server name")
	}
	serverAddress := server.Servers[0].Server

	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	if err != nil {
		t.Fatal(err)
	}

	proxiedMessages := make([]int64, numProxies)
	proxiedBytes := make([]int64, numProxies)

	for i, pm := range proxyStatsAddresses {
		proxyName := naming.JoinAddressName(pm.Server, "")
		for _, statvar := range []string{
			naming.Join(serverAddress, "/from/msgs"),
			naming.Join(serverAddress, "/to/msgs"),
			naming.Join(serverAddress, "/to/bytes"),
			naming.Join(serverAddress, "/from/bytes"),
		} {
			n, err := intStatForName(ctx, proxyName, statvar)
			if err != nil {
				t.Error(err)
				continue
			}
			if strings.Contains(statvar, "msgs") {
				proxiedMessages[i] += n
			} else {
				proxiedBytes[i] += n
			}
		}
	}

	// Make sure all proxies get at least some calls and bytes.
	for i := 0; i < numProxies; i++ {
		if proxiedBytes[i] == 0 || proxiedMessages[i] == 0 {
			t.Errorf("proxy %v (%v) handled zero messages or bytes (#messages, #bytes) (%v,%v)", i, proxyStatsAddresses[i], proxiedBytes[i], proxiedMessages[i])
		}
	}

	if t.Failed() {
		for i, log := range proxyLogs {
			fmt.Printf("proxy --------------- %v -----------\n", i)
			fmt.Println(log.String())
		}
		fmt.Printf("server --------------------------\n")
		fmt.Println(serverLog.String())
	}
}

func intStatForName(ctx *context.T, server, suffix string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	c := stats.StatsClient(naming.Join(server, suffix))
	value, err := c.Value(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get stat for %v: %v: %v", server, suffix, err)
	}
	vv := vdl.ValueOf(value)
	return vv.Int(), nil
}

var runServer = gosh.RegisterFunc("runServer", func() error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	// Set the listen spec to listen only via the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: proxyName})
	if _, _, err := v23.WithNewServer(ctx, serverName, service{}, security.AllowEveryone()); err != nil {
		return err
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
})

var runClient = gosh.RegisterFunc("runClient", func() error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var response string
	var payload []byte
	if err := v23.GetClient(ctx).Call(ctx, serverName, "Echo", nil, []interface{}{&response}); err != nil {
		return err
	}
	if err := v23.GetClient(ctx).Call(ctx, serverName, "Download", []interface{}{downloadSize}, []interface{}{&response, &payload}); err != nil {
		return err
	}
	fmt.Printf("%v=%v\n", responseVar, response)
	fmt.Printf("%v=%v\n", responseVar1, len(payload))
	return nil
})

type service struct{}

func (service) Echo(ctx *context.T, call rpc.ServerCall) (string, error) {
	client, _ := security.RemoteBlessingNames(ctx, call.Security())
	server := security.LocalBlessingNames(ctx, call.Security())
	return fmt.Sprintf("server %v saw client %v", server, client), nil
}

func (service) Download(ctx *context.T, call rpc.ServerCall, size int) (string, []byte, error) {
	client, _ := security.RemoteBlessingNames(ctx, call.Security())
	server := security.LocalBlessingNames(ctx, call.Security())
	return fmt.Sprintf("server %v saw client %v", server, client), make([]byte, size), nil
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
