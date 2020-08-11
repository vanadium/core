// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type testService struct {
	msg string
}

func (t *testService) Echo(ctx *context.T, call rpc.ServerCall, arg string) (string, error) {
	fmt.Fprintf(os.Stderr, "Echo: %v: %v\n", t.msg, arg)
	return t.msg + ": " + arg, nil
}

func callClient(ctx *context.T, expected string, n int, callCancel bool) ([]string, error) {
	clt := v23.GetClient(ctx)
	responses := make([]string, n)
	for i := 0; i < n; i++ {
		msg := fmt.Sprintf("%v: %v", expected, i)
		cctx, cancel := context.WithTimeout(ctx, time.Second)
		if err := clt.Call(cctx, "lb_server", "Echo", []interface{}{msg}, []interface{}{&responses[i]}); err != nil {
			return nil, err
		}
		if got, want := responses[i], msg; !strings.Contains(got, want) {
			return nil, fmt.Errorf("got %v does not contain %v", got, want)
		}
		if callCancel {
			cancel()
		}
	}
	return responses, nil
}

func countServers(responses []string, sa, sb string) (ca, cb int) {
	joined := strings.Join(responses, "\n")
	return strings.Count(joined, sa), strings.Count(joined, sb)
}

func TestApproximateLoadBalancing(t *testing.T) { //nolint:gocyclo
	lspec := rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{
			{Protocol: "tcp", Address: "127.0.0.1:0"},
		},
	}
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Enable caching in the namespace to be used by the client.
	ns := v23.GetNamespace(ctx)
	ns.CacheCtl(naming.DisableCache(false))

	// Create a new context, with a new namespace for servers to use, this
	// replicates the practical situation with client and servers being
	// in different address spaces. If the namespace is shared then the
	// server will clean up the client entries when the server is stopped.
	sctx, _, err := v23.WithNewNamespace(ctx, ns.Roots()...)
	if err != nil {
		t.Fatal(err)
	}

	sctx = v23.WithListenSpec(sctx, lspec)
	// Start a server
	sctxA, serverCancelA := context.WithCancel(sctx)
	_, serverA, err := v23.WithNewServer(sctxA, "lb_server", &testService{"__A__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverCancelA()
	// Start a second server
	sctxB, serverCancelB := context.WithCancel(sctx)
	_, serverB, err := v23.WithNewServer(sctxB, "lb_server", &testService{"__B__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if serverCancelB != nil {
			serverCancelB()
		}
	}()

	status := testutil.WaitForServerPublished(serverA)
	t.Logf("server status: A: %v", status)
	status = testutil.WaitForServerPublished(serverB)
	t.Logf("server status: B: %v", status)

	resolved, err := v23.GetNamespace(sctx).Resolve(sctx, "lb_server")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(resolved.Servers), 2; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	t.Logf("Resolved A & B: %v", resolved)

	iterations := 30
	runAndTestClient := func(useCancel bool, s1, s2 string) {
		expected := "with_cancel"
		if !useCancel {
			expected = "without_cancel"
		}
		responses, err := callClient(ctx, expected, iterations, useCancel)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(responses), iterations; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		a, b := countServers(responses, s1, s2)
		if a == 0 || b == 0 {
			t.Errorf("%v: no load balancing: %v, %v", expected, a, b)
		}
		if got, want := a+b, iterations; got != want {
			t.Errorf("%v: got %v, want %v", expected, got, want)
		}
		t.Logf("%v: %v %v, %v %v", expected, s1, a, s2, b)
	}

	runAndTestClient(true, "__A__", "__B__")
	runAndTestClient(false, "__A__", "__B__")

	// Stop one of the servers and restart a third one below.
	serverCancelB()
	serverCancelB = nil
	<-serverB.Closed()

	resolved, err = v23.GetNamespace(sctx).Resolve(sctx, "lb_server")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(resolved.Servers), 1; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	t.Logf("Resolved A: %v", resolved)

	// Flush the local cache.
	ns.CacheCtl(naming.DisableCache(true))
	ns.CacheCtl(naming.DisableCache(false))

	// Start a third server to replace the one that was stopped
	sctxC, serverCancelC := context.WithCancel(sctx)
	_, serverC, err := v23.WithNewServer(sctxC, "lb_server", &testService{"__C__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverCancelC()

	status = testutil.WaitForServerPublished(serverC)
	t.Logf("server status: C: %v", status)

	resolved, err = v23.GetNamespace(sctx).Resolve(sctx, "lb_server")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(resolved.Servers), 2; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	t.Logf("Resolved A & C: %v", resolved)

	runAndTestClient(true, "__A__", "__C__")
	runAndTestClient(false, "__A__", "__C__")

}
