// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
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
	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

func init() {
	library.AllowMultipleInitializations = true
}

const (
	proxyName        = "proxy"     // Name which the proxy mounts itself at
	serverName       = "server"    // Name which the server mounts itself at
	responseVar      = "RESPONSE"  // Name of the variable used by client program to output the first response
	responseVar1     = "RESPONSE1" // Name of the variable used by client program to output the second response
	downloadSize     = 64 * 1024 * 1024
	using2OfMProxies = 2 // Use 2 out of the total available set of proxies when using all proxies.
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
	sh.Cmd(proxyd,
		"--v23.tcp.address=127.0.0.1:0",
		"--name="+proxyName,
		"--access-list", "{\"In\":[\"root:server\"]}",
		"--stats-access-list", "{\"In\":[\"root:server\"]}",
	).WithCredentials(proxydCreds).Start()

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

func TestV23MultipleProxyd(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		numProxies  = 3
	)

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	proxyLogs, proxyStatsAddresses := startProxies(t, sh, numProxies)

	t.Logf("starting servers.....\n")
	// Start the servers that use the first and all proxies once since the
	// set of proxies they connect to is deterministic.
	firstProxyAddress, firstProxyLog, _, err := startServer(t, sh, serverName, 1, runServer, serverCreds)
	if err != nil {
		t.Fatalf("first proxy policy server: %v", err)
	}

	allProxiesAddress, allProxiesLog, _, err := startServer(t, sh, serverName+"-all", using2OfMProxies, runServerAllProxies, serverCreds)
	if err != nil {
		t.Fatalf("all proxy policy server: %v", err)
	}

	// Run all of the clients.
	t.Logf("running clients.....\n")
	for _, fn := range []*gosh.Func{runClient, runClientAllProxiesServer} {
		if err := runSingleClient(sh, fn, clientCreds); err != nil {
			t.Fatal(err)
		}
	}

	t.Logf("gathering statistics.....\n")
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	if err != nil {
		t.Fatal(err)
	}

	// Note, that the routing id is used as the prefix for the stats published
	// by the proxy server.
	firstProxyStatsPrefix, allProxiesStatsPrefix := statsPrefix(firstProxyAddress), statsPrefix(allProxiesAddress)

	proxyRequests, proxiedMessages, proxiedBytes, err := gatherStats(ctx, proxyStatsAddresses, firstProxyStatsPrefix, allProxiesStatsPrefix)
	if err != nil {
		t.Error(err)
	}

	used := proxiesUsedForServer(proxyRequests, firstProxyStatsPrefix)
	t.Logf("first policy server used proxy %v\n", used)
	// This server should always connect to the first proxy.
	if got, want := used, []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if sumFor(proxiedBytes[0], firstProxyStatsPrefix) == 0 {
		t.Errorf("no bytes counted for %v with proxy %v", firstProxyStatsPrefix, 0)
	}

	if sumFor(proxiedMessages[0], firstProxyStatsPrefix) == 0 {
		t.Errorf("no messages counted for %v with proxy %v", firstProxyStatsPrefix, 0)
	}

	// This server should always connect to all proxies.
	used = proxiesUsedForServer(proxyRequests, allProxiesStatsPrefix)
	t.Logf("all policies server used proxies %v\n", used)
	if got, want := len(used), using2OfMProxies; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, i := range used {
		if sumFor(proxiedBytes[i], allProxiesStatsPrefix) == 0 {
			t.Errorf("no bytes counted for %v with proxy %v", firstProxyStatsPrefix, i)
		}
		if sumFor(proxiedMessages[i], allProxiesStatsPrefix) == 0 {
			t.Errorf("no messages counted for %v with proxy %v", firstProxyStatsPrefix, i)
		}
	}

	if t.Failed() {
		for i, log := range proxyLogs {
			t.Logf("------ proxy %v ------\n", i)
			t.Log(log.String())
		}
		t.Logf("------ first proxy server ------\n")
		t.Log(firstProxyLog.String())

		t.Logf("------ all proxies server ------\n")
		t.Log(allProxiesLog.String())
	}
}

func tryRandomProxySelection(t *testing.T, sh *v23test.Shell, proxyStatsAddresses []naming.MountedServer, clientCreds, serverCreds *v23test.Credentials) (bool, *bytes.Buffer) {
	// Make sure that there isn't a server already running and mounted.
	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))
	if _, err := waitForNoMountedServer(t, sh.Ctx, ns, serverName+"-random"); err != nil {
		t.Fatal(err)
	}

	// Run the server.
	t.Logf("starting server.....\n")
	serverAddress, serverLog, serverCmd, err := startServer(t, sh, serverName+"-random", 1, runServerRandomProxy, serverCreds)
	if err != nil {
		t.Fatalf("random proxies policy server: %v", err)
	}
	// Make sure to kill the server on returning from this function.
	defer func() {
		serverCmd.Terminate(os.Interrupt)
	}()

	// Run the client.
	t.Logf("running clients.....\n")
	if err := runSingleClient(sh, runClientRandProxyServer, clientCreds); err != nil {
		t.Fatal(err)
	}

	// Gather stats.
	t.Logf("gathering statistics.....\n")
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	if err != nil {
		t.Fatal(err)
	}
	proxyStatsPrefix := statsPrefix(serverAddress)
	proxyRequests, proxiedMessages, proxiedBytes, err := gatherStats(ctx, proxyStatsAddresses, proxyStatsPrefix)
	if err != nil {
		t.Error(err)
	}

	used := proxiesUsedForServer(proxyRequests, proxyStatsPrefix)
	t.Logf("random used %v\n", used)

	// Make sure there was some data flowing through the chosen proxy.
	for _, p := range used {
		if sumFor(proxiedBytes[p], proxyStatsPrefix) == 0 {
			t.Errorf("no bytes counted for %v with proxy %v", proxyStatsPrefix, 0)
		}

		if sumFor(proxiedMessages[p], proxyStatsPrefix) == 0 {
			t.Errorf("no messages counted for %v with proxy %v", proxyStatsPrefix, 0)
		}
	}

	// Return true if the chosen proxy was not the first one.
	return len(used) == 1 && used[0] != 0, serverLog
}

func TestV23MultipleProxydRandom(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		// A larger # of proxies will reduce the number of attempts before
		// we're likely to randomly select one other than the first.
		numProxies = 6
	)

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))
	proxyLogs, proxyStatsAddresses := startProxies(t, sh, numProxies)

	// The server that makes a random selection will be started multiple times
	// until it connects to a proxy other than the first one.
	// This will either succeed or be timed out!
	serverLogs := []*bytes.Buffer{}
	start := time.Now()
	for {
		ok, serverLog := tryRandomProxySelection(t, sh, proxyStatsAddresses, clientCreds, serverCreds)
		serverLogs = append(serverLogs, serverLog)
		if ok {
			break
		}
		if d := time.Since(start); d > time.Minute {
			t.Errorf("timing out after %v and %v attempts", d, len(serverLogs))
			break
		}
		t.Logf("retrying random proxy policy test: after %v attempts", len(serverLogs))
	}

	if t.Failed() {
		for i, log := range proxyLogs {
			t.Logf("------ proxy %v ------\n", i)
			t.Log(log.String())
		}
		for i, log := range serverLogs {
			t.Logf("------ server instance %v ------\n", i)
			t.Log(log.String())
		}
	}
}

func waitForNMountedServers(t *testing.T, ctx *context.T, ns namespace.T, name string, expected int) ([]naming.MountedServer, error) {
	t.Logf("waiting for %v instances of %q to be available in the mounttable\n", expected, name)
	return waitForFunc(t, ctx, ns, name, func(resolved *naming.MountEntry, err error) (bool, error) {
		if err == nil {
			if resolved != nil {
				t.Logf("%v: found %v/%v instances of %v", time.Now(), len(resolved.Servers), expected, name)
				for i, s := range resolved.Servers {
					t.Logf("%v: %v/%v: %v", name, i, len(resolved.Servers), s.Server)
				}
			}
			return len(resolved.Servers) == expected, nil
		}
		if err != nil && verror.ErrorID(err) != naming.ErrNoSuchName.ID {
			return false, fmt.Errorf("unexpected error waiting for %v: %v", name, err)
		}
		return false, nil
	})
}

func waitForNoMountedServer(t *testing.T, ctx *context.T, ns namespace.T, name string) ([]naming.MountedServer, error) {
	t.Logf("waiting for %v to not exist in the mount table\n", name)
	return waitForFunc(t, ctx, ns, name, func(resolved *naming.MountEntry, err error) (bool, error) {
		if err != nil {
			if verror.ErrorID(err) == naming.ErrNoSuchName.ID {
				t.Logf("%v does not exist in the mount table\n", name)
				return true, nil
			}
			return false, fmt.Errorf("unexpected error waiting for %v: %v", name, err)
		}
		return false, nil
	})
}

func waitForFunc(t *testing.T, ctx *context.T, ns namespace.T, name string,
	isdone func(*naming.MountEntry, error) (bool, error),
) ([]naming.MountedServer, error) {
	start := time.Now()
	for {
		resolved, err := ns.Resolve(ctx, name)
		ok, err := isdone(resolved, err)
		if ok {
			if resolved == nil {
				return nil, nil
			}
			return resolved.Servers, nil
		}
		if err != nil {
			return nil, err
		}
		if time.Since(start) > time.Second*5 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
	return nil, fmt.Errorf("timed out after %v waiting for %v", time.Since(start), name)
}

func startProxies(t *testing.T, sh *v23test.Shell, numProxies int) ([]*bytes.Buffer, []naming.MountedServer) {
	ns := v23.GetNamespace(sh.Ctx)
	proxyd := v23test.BuildGoPkg(sh, "v.io/x/ref/services/xproxy/xproxyd")
	proxyLogs := make([]*bytes.Buffer, numProxies)
	// Start multiple proxies.
	t.Logf("starting proxies.....\n")
	for i := 0; i < numProxies; i++ {
		proxyLogs[i] = &bytes.Buffer{}
		proxydCreds := sh.ForkCredentials("proxyd")
		cmd := sh.Cmd(proxyd,
			"--v23.tcp.address=127.0.0.1:0",
			"--name="+proxyName,
			"--access-list", "{\"In\":[\"root:server\"]}",
			"--stats-access-list", "{\"In\":[\"root:server\"]}",
		).WithCredentials(proxydCreds)
		cmd.AddStdoutWriter(proxyLogs[i])
		cmd.AddStderrWriter(proxyLogs[i])
		cmd.Start()
	}
	// Wait for all proxies to be mounted.
	proxyAddresses, err := waitForNMountedServers(t, sh.Ctx, ns, proxyName, numProxies)
	if err != nil {
		for i, log := range proxyLogs {
			t.Logf("proxy %v", i)
			t.Log(log.String())
		}
		t.Fatal(err)
	}

	proxyStatsAddresses, err := waitForNMountedServers(t, sh.Ctx, ns, proxyName+"-mon", numProxies)
	if err != nil {
		for i, log := range proxyLogs {
			t.Logf("proxy %v", i)
			t.Log(log.String())
		}
		t.Fatal(err)
	}

	// Need to sort the stats endpoints so that they correspond to the proxies
	// that they belong to, thay is proxyStatsAddress[0] corresponds
	// to proxyAddresses[0]. The proxy and its stats interface will share
	// the same host:port but have a different routing id.
	sortedStatsAddresses := []naming.MountedServer{}
	for _, mp := range proxyAddresses {
		ep, _ := naming.ParseEndpoint(mp.Server)
		for _, stats := range proxyStatsAddresses {
			sep, _ := naming.ParseEndpoint(stats.Server)
			if ep.Address == sep.Address {
				sortedStatsAddresses = append(sortedStatsAddresses, stats)
			}
		}
	}
	t.Logf("proxies running.....\n")
	return proxyLogs, sortedStatsAddresses
}

func proxiesUsedForServer(stats []map[string]int64, server string) []int {
	used := []int{}
	for i, proxy := range stats {
		for k := range proxy {
			if k == server {
				used = append(used, i)
			}
		}
	}
	return used
}

func sumFor(stats map[string]int64, server string) int64 {
	var r int64
	for k, v := range stats {
		if k == server {
			r += v
		}
	}
	return r
}

func statsPrefix(address string) string {
	ep, _ := naming.ParseEndpoint(address)
	return ep.RoutingID.String()
}

func startServer(t *testing.T, sh *v23test.Shell, name string, expected int, fn *gosh.Func, creds *v23test.Credentials) (string, *bytes.Buffer, *v23test.Cmd, error) {
	log := &bytes.Buffer{}
	// Run a server that uses all proxies simultaneously.
	cmd := sh.FuncCmd(fn).WithCredentials(creds)
	cmd.AddStdoutWriter(log)
	cmd.AddStderrWriter(log)
	cmd.Start()
	ns := v23.GetNamespace(sh.Ctx)
	server, err := waitForNMountedServers(t, sh.Ctx, ns, name, expected)
	if err != nil {
		t.Logf("failed to start server %v: %v\n", name, err)
		t.Log(log.String())
		return "", nil, nil, err
	}
	return server[0].Server, log, cmd, nil
}

func createProxiedServer(name, proxy string, policy rpc.ProxyPolicy, limit int) func() error {
	return func() error {
		ctx, shutdown := test.V23Init()
		defer func() {
			fmt.Println("calling shutdown")
			shutdown()
			fmt.Println("shutdown done")
		}()
		// Set the listen spec to listen only via the proxy.
		ls := rpc.ListenSpec{Proxy: proxy, ProxyPolicy: policy, ProxyLimit: limit}
		ctx = v23.WithListenSpec(ctx, ls)
		if _, _, err := v23.WithNewServer(ctx, name, service{}, security.AllowEveryone()); err != nil {
			return err
		}
		fmt.Println("waiting for signal")
		<-signals.ShutdownOnSignals(ctx)
		fmt.Println("got signal")
		return nil
	}
}

var runServer = gosh.RegisterFunc(
	"runServer",
	createProxiedServer(serverName, proxyName, rpc.UseFirstProxy, 0),
)
var runServerRandomProxy = gosh.RegisterFunc(
	"runServerRandomProxy",
	createProxiedServer(serverName+"-random", proxyName, rpc.UseRandomProxy, 0),
)
var runServerAllProxies = gosh.RegisterFunc(
	"runServerAllProxies",
	createProxiedServer(serverName+"-all", proxyName, rpc.UseAllProxies, using2OfMProxies),
)

func createClient(serverName string, iterations int) func() error {
	return func() error {
		ctx, shutdown := test.V23Init()
		defer shutdown()
		var response string
		var payload []byte
		for i := 0; i < iterations; i++ {
			if err := v23.GetClient(ctx).Call(ctx, serverName, "Echo", nil, []interface{}{&response}); err != nil {
				return err
			}
			if err := v23.GetClient(ctx).Call(ctx, serverName, "Download", []interface{}{downloadSize}, []interface{}{&response, &payload}); err != nil {
				return err
			}
			if i == 0 {
				fmt.Printf("%v=%v\n", responseVar, response)
				fmt.Printf("%v=%v\n", responseVar1, len(payload))
			}
		}
		return nil
	}
}

func runSingleClient(sh *v23test.Shell, fn *gosh.Func, creds *v23test.Credentials) error {
	cmd := sh.FuncCmd(fn).WithCredentials(creds)
	cmd.AddStderrWriter(os.Stdout)
	cmd.AddStdoutWriter(os.Stdout)
	cmd.Run()
	if got, want := cmd.S.ExpectVar(responseVar), "server [root:server] saw client [root:client]"; got != want {
		return fmt.Errorf("Got %q, want %q", got, want)
	}
	if got, want := cmd.S.ExpectVar(responseVar1), fmt.Sprintf("%v", downloadSize); got != want {
		return fmt.Errorf("Got %q, want %q", got, want)
	}
	return nil
}

var runClient = gosh.RegisterFunc("runClient", createClient(serverName, 1))
var runClientRandProxyServer = gosh.RegisterFunc("runClientRandom", createClient(serverName+"-random", 1))

// Run the 'all proxies' example enough times to ensure that the client uses all of the proxies.
var runClientAllProxiesServer = gosh.RegisterFunc("runClientAll", createClient(serverName+"-all", 20))

func intStatForName(ctx *context.T, server, suffix string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	c := stats.StatsClient(naming.Join(server, suffix))
	value, err := c.Value(ctx)
	if err != nil {
		return 0, err
	}
	vv := vdl.ValueOf(value)
	return vv.Int(), nil
}

func gatherStats(ctx *context.T, proxies []naming.MountedServer, servers ...string) (proxyRequests, proxiedMessages, proxiedBytes []map[string]int64, err error) {
	proxyRequests = make([]map[string]int64, len(proxies))
	proxiedMessages = make([]map[string]int64, len(proxies))
	proxiedBytes = make([]map[string]int64, len(proxies))
	for i, pm := range proxies {
		proxyName := naming.JoinAddressName(pm.Server, "")
		proxyRequests[i], proxiedMessages[i], proxiedBytes[i] = make(map[string]int64), make(map[string]int64), make(map[string]int64)
		for _, server := range servers {
			for _, statvar := range []string{
				naming.Join(server, "/requests"),
				naming.Join(server, "/from/msgs"),
				naming.Join(server, "/to/msgs"),
				naming.Join(server, "/to/bytes"),
				naming.Join(server, "/from/bytes"),
			} {
				var n int64
				n, err = intStatForName(ctx, proxyName, statvar)
				if err != nil {
					if verror.ErrorID(err) == verror.ErrNoExist.ID {
						// ignore non existent variables.
						err = nil
					} else {
						err = fmt.Errorf("failed to get stat for %v: %v", statvar, err)
					}
					continue
				}
				switch {
				case strings.Contains(statvar, "msgs"):
					proxiedMessages[i][server] += n
				case strings.Contains(statvar, "requests"):
					proxyRequests[i][server] += n
				case strings.Contains(statvar, "bytes"):
					proxiedBytes[i][server] += n
				}
			}
		}
	}
	return
}

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
