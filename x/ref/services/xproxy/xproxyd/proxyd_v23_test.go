// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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
	proxyName        = "proxy"         // Name which the proxy mounts itself at
	proxyNameStats   = "proxy-mon"     // Name which the proxy mounts its stats at
	serverName       = "server"        // Name which the server mounts itself at
	serverNameRandom = "server-random" // Name which the server using a random proxy mounts itself at
	serverNameAll    = "server-all"    // Name which the server using all proxies mounts itself at

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
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
	)

	// Start proxyd.
	startProxy(t, sh)

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
		err         error
	)

	assert := func(msg string, logs ...*bytes.Buffer) {
		assertWithLog(t, err, msg, logs)
	}

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	proxies, _, proxyStatsAddresses := startInitialSetOfProxies(t, sh, numProxies)
	assert("start proxies", logsForProxies(proxies)...)

	// Start the servers that use the first and all proxies policy whose
	// proxy selections are deterministic.
	firstProxyAddress, firstProxyLog, _, err := startServer(t, sh, serverName, 1, runServer, serverCreds)
	assert("first proxy policy server", firstProxyLog)

	allProxiesAddress, allProxiesLog, _, err := startServer(t, sh, serverNameAll, using2OfMProxies, runServerAllProxies, serverCreds)
	assert("all proxies policy server", allProxiesLog)

	// Run all of the clients.
	for _, fn := range []*gosh.Func{runClient, runClientAllProxiesServer} {
		if err := runSingleClient(sh, fn, clientCreds); err != nil {
			t.Fatal(err)
		}
	}

	// Gather statistics.
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	assert("withPrincipal")
	proxyRequests, proxiedMessages, proxiedBytes, err := gatherStats(ctx, proxyStatsAddresses, firstProxyAddress, allProxiesAddress)
	assert("gatherStats")

	used := proxiesUsedForServer(proxyRequests, firstProxyAddress)
	t.Logf("first policy server used proxy %v\n", used)

	// This server should always connect to the first proxy.
	if got, want := used, []int{0}; !reflect.DeepEqual(got, want) {
		t.Log("log from server using the first proxy policy")
		t.Log(firstProxyLog.String())
		t.Fatalf("got %v, want %v", got, want)
	}

	if sumFor(proxiedBytes[0], firstProxyAddress) == 0 {
		t.Errorf("no bytes counted for %v with proxy %v", firstProxyAddress, 0)
	}

	if sumFor(proxiedMessages[0], firstProxyAddress) == 0 {
		t.Errorf("no messages counted for %v with proxy %v", firstProxyAddress, 0)
	}

	// This server should always connect to all proxies.
	used = proxiesUsedForServer(proxyRequests, allProxiesAddress)
	t.Logf("all policies server used proxies %v\n", used)

	if got, want := len(used), using2OfMProxies; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, i := range used {
		if sumFor(proxiedBytes[i], allProxiesAddress) == 0 {
			t.Errorf("no bytes counted for %v with proxy %v", allProxiesAddress, i)
		}
		if sumFor(proxiedMessages[i], allProxiesAddress) == 0 {
			t.Errorf("no messages counted for %v with proxy %v", allProxiesAddress, i)
		}
	}
}

func tryRandomProxySelection(t *testing.T, sh *v23test.Shell, proxyStatsAddresses []naming.MountedServer, clientCreds, serverCreds *v23test.Credentials) (int, *bytes.Buffer) {
	// Make sure that there isn't a server already running and mounted.
	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	var err error
	assert := func(msg string, logs ...*bytes.Buffer) {
		assertWithLog(t, err, msg, logs)
	}
	err = waitForNoMountedServer(t, sh.Ctx, ns, serverNameRandom)
	assert("waiting for server")

	// Run the server.
	serverAddress, serverLog, serverCmd, err := startServer(t, sh, serverNameRandom, 1, runServerRandomProxy, serverCreds)
	assert("random proxies policy server", serverLog)

	// Make sure to kill the server on returning from this function.
	defer func() {
		serverCmd.Terminate(os.Interrupt)
	}()

	// Run the client.
	err = runSingleClient(sh, runClientRandProxyServer, clientCreds)
	assert("client")

	// Gather stats.
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	assert("withPrincipal")

	proxyRequests, proxiedMessages, proxiedBytes, err := gatherStats(ctx, proxyStatsAddresses, serverAddress)
	assert("gatherStats")

	used := proxiesUsedForServer(proxyRequests, serverAddress)
	t.Logf("random used %v\n", used)

	// Make sure there was some data flowing through the chosen proxy.
	for _, p := range used {
		if sumFor(proxiedBytes[p], serverAddress) == 0 {
			t.Errorf("no bytes counted for %v with proxy %v", serverAddress, 0)
		}

		if sumFor(proxiedMessages[p], serverAddress) == 0 {
			t.Errorf("no messages counted for %v with proxy %v", serverAddress, 0)
		}
	}

	if len(used) > 0 {
		return used[0], serverLog
	}
	return -1, serverLog
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
	_, _, proxyStatsAddresses := startInitialSetOfProxies(t, sh, numProxies)

	// The server that makes a random selection will be started multiple times
	// until it connects to a proxy other than the first one.
	// This will either succeed or be timed out!
	serverLogs := []*bytes.Buffer{}
	start := time.Now()
	prev := -1
	for {
		used, serverLog := tryRandomProxySelection(t, sh, proxyStatsAddresses, clientCreds, serverCreds)
		if used < 0 {
			t.Errorf("failed to select a proxy")
			continue
		}
		t.Logf("Random proxy policy selected proxy: %v", used)
		serverLogs = append(serverLogs, serverLog)
		if prev >= 0 && prev != used {
			break
		}
		prev = used
		if d := time.Since(start); d > time.Minute {
			t.Errorf("timing out after %v and %v attempts", d, len(serverLogs))
			break
		}
		t.Logf("retrying random proxy policy test: after %v attempts", len(serverLogs))
	}
}

func TestV23MultiProxyResilience(t *testing.T) {

	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		err         error
	)

	assert := func(msg string, logs ...*bytes.Buffer) {
		assertWithLog(t, err, msg, logs)
	}

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	// Start two proxies.
	first2, _, first2StatsAddrs := startInitialSetOfProxies(t, sh, 2)
	assert("first two proxies", logsForProxies(first2)...)

	// Start the server.
	serverAddress, serverLog, _, err := startServer(t, sh, serverNameAll, using2OfMProxies, runServerAllProxies, serverCreds)
	assert("server", serverLog)

	// Run the client.
	err = runSingleClient(sh, runClientAllProxiesServer, clientCreds)
	assert("client")

	// Gather stats and make sure the the server is using both proxies.
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	assert("withPrincipal")

	requests, _, _, err := gatherStats(ctx, first2StatsAddrs, serverAddress)
	assert("gatherStats")

	used := proxiesUsedForServer(requests, serverAddress)
	if got, want := used, []int{0, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Kill one of the two proxies and wait for the server to update the mounttable.
	first2[1].cmd.Terminate(os.Interrupt)
	_, err = waitForNMountedServers(t, sh.Ctx, ns, serverNameAll, 1)
	assert("single server addess", serverLog)

	_, proxyStatsAddresses, err := waitForNProxies(t, sh.Ctx, ns, 1)
	assert("single proxy in mounttable")

	// Run the client.
	err = runSingleClient(sh, runClientAllProxiesServer, clientCreds)
	assert("client with one proxy only")

	// Gather stats and make sure the server is using only one proxy.
	requests, _, _, err = gatherStats(ctx, proxyStatsAddresses, serverAddress)
	assert("gatherStats")

	used = proxiesUsedForServer(requests, serverAddress)
	if got, want := len(used), 1; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Start two more proxies and wait for the server to notice them,
	// this should take about a second.
	third := startProxy(t, sh)
	fourth := startProxy(t, sh)
	_, err = waitForNMountedServers(t, sh.Ctx, ns, serverNameAll, 2)
	assert("third and fourth proxies and server log", third.log, fourth.log, serverLog)

	_, proxyStatsAddresses, err = waitForNProxies(t, sh.Ctx, ns, 3)
	assert("wait for all four proxies to be in the mounttable")

	// Run the client.
	err = runSingleClient(sh, runClientAllProxiesServer, clientCreds)
	assert("client with two proxies again")

	requests, _, _, err = gatherStats(ctx, proxyStatsAddresses, serverAddress)
	assert("gatherStats")

	used = proxiesUsedForServer(requests, serverAddress)
	if got, want := len(used), 2; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

}

func TestV23SingleProxyResilience(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	var (
		serverCreds = sh.ForkCredentials("server")
		clientCreds = sh.ForkCredentials("client")
		err         error
	)

	assert := func(msg string, logs ...*bytes.Buffer) {
		assertWithLog(t, err, msg, logs)
	}

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	// Start two proxies.
	first2, _, first2StatsAddrs := startInitialSetOfProxies(t, sh, 2)
	assert("first two proxies", logsForProxies(first2)...)

	// Start a server that will use the first proxy.
	firstProxyAddress, firstProxyServerLog, _, err := startServer(t, sh, serverName, 1, runServer, serverCreds)
	assert("first proxy policy server", firstProxyServerLog)

	// Start a server that will use a random proxy.
	randomProxyAddress, randomProxyServerLog, _, err := startServer(t, sh, serverName, 1, runServerRandomProxy, serverCreds)
	assert("random proxy policy server", randomProxyServerLog)

	// Run clients for each of the above.
	for _, fn := range []*gosh.Func{runClient, runClientRandProxyServer} {
		if err := runSingleClient(sh, fn, clientCreds); err != nil {
			t.Fatal(err)
		}
	}

	// Gather statistics.
	ctx, err := v23.WithPrincipal(sh.Ctx, serverCreds.Principal)
	assert("withPrincipal")
	proxyRequests, _, _, err := gatherStats(ctx, first2StatsAddrs, firstProxyAddress, randomProxyAddress)
	assert("gatherStats")

	used := proxiesUsedForServer(proxyRequests, firstProxyAddress)
	if got, want := used, []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	used = proxiesUsedForServer(proxyRequests, randomProxyAddress)
	if got, want := len(used), 1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Kill the proxies.
	first2[0].cmd.Terminate(os.Interrupt)
	first2[1].cmd.Terminate(os.Interrupt)

	err = waitForNoMountedServer(t, sh.Ctx, ns, proxyName)
	assert("wait for no mounted proxies")

	err = waitForNoMountedServer(t, sh.Ctx, ns, serverName)
	assert("wait for no mounted first proxy policy server")

	err = waitForNoMountedServer(t, sh.Ctx, ns, serverNameRandom)
	assert("wait for no mounted random proxy policy server")

	// Start a new proxy.
	startProxy(t, sh)
	third, err := waitForNMountedServers(t, sh.Ctx, ns, proxyNameStats, 1)
	assert("wait for proxy")
	_, err = waitForNMountedServers(t, sh.Ctx, ns, serverName, 1)
	assert("wait for first proxy policy server", firstProxyServerLog)
	_, err = waitForNMountedServers(t, sh.Ctx, ns, serverNameRandom, 1)
	assert("wait for random proxy policy server", firstProxyServerLog, randomProxyServerLog)

	// Run the clients.
	for _, fn := range []*gosh.Func{runClient, runClientRandProxyServer} {
		if err := runSingleClient(sh, fn, clientCreds); err != nil {
			t.Fatal(err)
		}
	}

	// Gather stats and make sure the new proxy is being used.
	proxyRequests, _, _, err = gatherStats(ctx, third, randomProxyAddress)
	assert("gatherStats")

	used = proxiesUsedForServer(proxyRequests, firstProxyAddress)
	if got, want := used, []int{0}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	used = proxiesUsedForServer(proxyRequests, randomProxyAddress)
	if got, want := len(used), 1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func waitForNProxies(t *testing.T, ctx *context.T, ns namespace.T, expected int) (proxy, stats []naming.MountedServer, err error) {
	proxy, err = waitForNMountedServers(t, ctx, ns, proxyName, expected)
	if err != nil {
		return
	}
	stats, err = waitForNMountedServers(t, ctx, ns, proxyNameStats, expected)
	return
}

// assertWithLog is intended to be called by a function defined within each test.
func assertWithLog(t *testing.T, err error, msg string, logs []*bytes.Buffer) {
	if err != nil {
		_, file, line, _ := runtime.Caller(2)
		t.Logf("FAILED AT: %v %v: %v", filepath.Base(file), line, err)
		for i, log := range logs {
			t.Logf("%v: -- %v/%v --", msg, i, len(logs))
			t.Log(log.String())
		}
		t.Fatal(err)
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
		if err != nil && !errors.Is(err, naming.ErrNoSuchName) {
			return false, fmt.Errorf("unexpected error waiting for %v: %v", name, err)
		}
		return false, nil
	})
}

func waitForNoMountedServer(t *testing.T, ctx *context.T, ns namespace.T, name string) error {
	t.Logf("waiting for %v to not exist in the mount table\n", name)
	_, err := waitForFunc(t, ctx, ns, name, func(resolved *naming.MountEntry, err error) (bool, error) {
		if err != nil {
			if errors.Is(err, naming.ErrNoSuchName) {
				t.Logf("%v does not exist in the mount table\n", name)
				return true, nil
			}
			return false, fmt.Errorf("unexpected error waiting for %v: %v", name, err)
		}
		return false, nil
	})
	return err
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

// proxyInfo contains the address of the proxy and its associated stats server.
type proxyInfo struct {
	cmd            *v23test.Cmd
	log            *bytes.Buffer
	proxyName      string // the name of the proxy, ie. /address.
	proxyStatsName string // the name of the proxy's stats interface.
}

func logsForProxies(proxies []*proxyInfo) []*bytes.Buffer {
	r := make([]*bytes.Buffer, len(proxies))
	for i, p := range proxies {
		r[i] = p.log
	}
	return r
}

func startProxy(t *testing.T, sh *v23test.Shell) *proxyInfo {
	var (
		proxyd      = v23test.BuildGoPkg(sh, "v.io/x/ref/services/xproxy/xproxyd")
		log         = &bytes.Buffer{}
		proxydCreds = sh.ForkCredentials("proxyd")
	)
	cmd := sh.Cmd(proxyd,
		"--v23.tcp.address=127.0.0.1:0",
		"--name="+proxyName,
		"--access-list", "{\"In\":[\"root:server\"]}",
		"--stats-access-list", "{\"In\":[\"root:server\"]}",
	).WithCredentials(proxydCreds)
	cmd.AddStdoutWriter(log)
	cmd.AddStderrWriter(log)
	cmd.Start()
	return &proxyInfo{
		cmd:            cmd,
		log:            log,
		proxyName:      cmd.S.ExpectVar("NAME"),
		proxyStatsName: cmd.S.ExpectVar("STATS"),
	}
}

// startInitialSetOfProxies starts the initial set of proxies and returns
// slices of their metadata sorted by their order in the mounttable.
func startInitialSetOfProxies(t *testing.T, sh *v23test.Shell, numProxies int) (sortedProxies []*proxyInfo, proxyAddrs, proxyStatsAddrs []naming.MountedServer) {
	ns := v23.GetNamespace(sh.Ctx)

	// Start multiple proxies.
	pairs := make([]*proxyInfo, numProxies)
	for i := 0; i < numProxies; i++ {
		pairs[i] = startProxy(t, sh)
	}

	var err error
	assert := func() {
		if err != nil {
			for i, p := range pairs {
				t.Logf("proxy %v", i)
				if p.log != nil {
					t.Log(p.log.String())
				}
				t.Log(strings.Repeat("-", 30))
			}
			t.Fatal(err)
		}
	}

	// Wait for all proxies to be mounted.
	proxies, err := waitForNMountedServers(t, sh.Ctx, ns, proxyName, numProxies)
	assert()

	// Wait for all proxy stats servers to be mounted.
	stats, err := waitForNMountedServers(t, sh.Ctx, ns, proxyNameStats, numProxies)
	assert()

	sortedProxies, proxyAddrs, proxyStatsAddrs = sortProxyAddresses(t, proxies, stats, pairs)
	return
}

// sortProxyAddresses sorts the names/addresses of multiple proxy's main and stats
// servers into the same order as they appear in the mounttable. This is
// important for tests that care about the particular proxy selected. Its inputs
// are the entries returned by the mounttable for the proxy serving and stats
// addresses and the proxyInfo name pairs that associate serving and stats
// interfaces.
func sortProxyAddresses(t *testing.T, serving, stats []naming.MountedServer, pairs []*proxyInfo) (sortedProxyInfo []*proxyInfo, sortedServing, sortedStats []naming.MountedServer) {
	n := len(serving)
	if n != len(stats) || n != len(pairs) {
		t.Fatalf("mismatched #s of serving, stats and/or proxyInfo slices")
	}
	pairFor := map[string]*proxyInfo{}
	for _, v := range pairs {
		pairFor[v.proxyName] = v
	}

	sortedProxyInfo = make([]*proxyInfo, n)
	sortedServing = make([]naming.MountedServer, n)
	sortedStats = make([]naming.MountedServer, n)

	for i, mounted := range serving {
		sortedServing[i] = mounted
		info := pairFor[naming.JoinAddressName(mounted.Server, "")]
		if info == nil {
			t.Log("Pairs....")
			for i, p := range pairs {
				t.Logf("%v: proxy: %v, stats: %v", i, p.proxyName, p.proxyStatsName)
			}
			t.Log("Pairs for....")
			for k, v := range pairFor {
				t.Logf("%v -> %v %v", k, v.proxyName, v.proxyStatsName)
			}
			t.Log("Proxy servers/stats")
			for i, p := range serving {
				t.Logf("%v: proxy: %v, stats: %v", i, p.Server, stats[i].Server)
			}
			t.Fatalf("no proxyInfo for %v", naming.JoinAddressName(mounted.Server, ""))
		}
		found := false
		for _, p := range stats {
			if naming.JoinAddressName(p.Server, "") == info.proxyStatsName {
				sortedStats[i] = p
				sortedProxyInfo[i] = info
				found = true
				break
			}
		}
		if !found {
			t.Logf("Proxies....")
			t.Logf("Pair for %v is %v", info, naming.JoinAddressName(mounted.Server, ""))
			for _, p := range pairs {
				t.Logf("proxy: %v, stats: %v", p.proxyName, p.proxyStatsName)
			}
			t.Fatalf("failed to find stats address for %v\n", mounted.Server)
		}
	}
	return
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
	createProxiedServer(serverNameRandom, proxyName, rpc.UseRandomProxy, 0),
)
var runServerAllProxies = gosh.RegisterFunc(
	"runServerAllProxies",
	createProxiedServer(serverNameAll, proxyName, rpc.UseAllProxies, using2OfMProxies),
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
var runClientRandProxyServer = gosh.RegisterFunc("runClientRandom", createClient(serverNameRandom, 1))

// Run the 'all proxies' example enough times to ensure that the client uses all of the proxies.
var runClientAllProxiesServer = gosh.RegisterFunc("runClientAll", createClient(serverNameAll, 20))

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
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	prefixes := make([]string, len(servers))
	for i, s := range servers {
		prefixes[i] = statsPrefix(s)
	}
	for i, pm := range proxies {
		proxyName := naming.JoinAddressName(pm.Server, "")
		proxyRequests[i], proxiedMessages[i], proxiedBytes[i] = make(map[string]int64), make(map[string]int64), make(map[string]int64)
		for _, server := range prefixes {
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
					if errors.Is(err, verror.ErrNoExist) {
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

func proxiesUsedForServer(stats []map[string]int64, server string) []int {
	used := []int{}
	server = statsPrefix(server)
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
	server = statsPrefix(server)
	var r int64
	for k, v := range stats {
		if k == server {
			r += v
		}
	}
	return r
}

// Note, that the routing id is used as the prefix for the stats published
// by the proxy server.
func statsPrefix(address string) string {
	ep, _ := naming.ParseEndpoint(address)
	return ep.RoutingID.String()
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
