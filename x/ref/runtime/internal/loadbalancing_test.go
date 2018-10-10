package internal_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type testService struct{ m string }

func (t *testService) Echo(ctx *context.T, call rpc.ServerCall, arg string) (string, error) {
	return t.m + ": " + arg, nil
}

func callClient(ctx *context.T, m string, n int, callCancel bool) ([]string, error) {
	clt := v23.GetClient(ctx)
	responses := make([]string, n)
	for i := 0; i < n; i++ {
		msg := fmt.Sprintf("%v: %v", m, i)
		cctx, cancel := context.WithTimeout(ctx, time.Second)
		if err := clt.Call(cctx, "server", "Echo", []interface{}{msg}, []interface{}{&responses[i]}); err != nil {
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
	ca = strings.Count(joined, sa)
	cb = strings.Count(joined, sb)
	return
}

func TestApproximateLoadBalancing(t *testing.T) {
	lspec := rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{
			{Protocol: "tcp", Address: "127.0.0.1:0"},
		},
	}
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Disable caching in the namespace to be used by the client.
	ns := v23.GetNamespace(ctx)
	ns.CacheCtl(naming.DisableCache(true))

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
	_, serverA, err := v23.WithNewServer(sctxA, "server", &testService{"__A__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverCancelA()

	// Start a second server
	sctxB, serverCancelB := context.WithCancel(sctx)
	_, serverB, err := v23.WithNewServer(sctxB, "server", &testService{"__B__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if serverCancelB != nil {
			serverCancelB()
		}
	}()

	testutil.WaitForServerPublished(serverA)
	testutil.WaitForServerPublished(serverB)

	resolved, err := v23.GetNamespace(sctx).Resolve(sctx, "server")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(resolved.Servers), 2; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	iterations := 300
	runClient := func(useCancel bool, s1, s2 string) (a, b int) {
		name := "with_cancel"
		if !useCancel {
			name = "without_cancel"
		}
		responses, err := callClient(ctx, name, iterations, useCancel)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(responses), iterations; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		a, b = countServers(responses, s1, s2)
		return
	}

	var ca, cb, nca, ncb int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		ca, cb = runClient(true, "__A__", "__B__")
		wg.Done()
	}()
	go func() {
		nca, ncb = runClient(false, "__A__", "__B__")
		wg.Done()
	}()

	wg.Wait()

	if ca == 0 || cb == 0 {
		t.Errorf("with cancel, no load balancing: %v, %v", ca, cb)
	}
	if got, want := ca+cb, iterations; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if nca == 0 || ncb == 0 {
		t.Errorf("with out cancel, no load balancing: %v, %v", nca, ncb)
	}
	if got, want := nca+ncb, iterations; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Stop one of the servers and restart a third one below.
	serverCancelB()
	serverCancelB = nil
	<-serverB.Closed()

	// Start a third server to replace the one that was stopped
	sctxC, serverCancelC := context.WithCancel(sctx)
	_, serverC, err := v23.WithNewServer(sctxC, "server", &testService{"__C__"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverCancelC()

	testutil.WaitForServerPublished(serverC)

	resolved, err = v23.GetNamespace(sctx).Resolve(sctx, "server")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(resolved.Servers), 2; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	wg.Add(2)
	go func() {
		ca, cb = runClient(true, "__A__", "__C__")
		wg.Done()
	}()
	go func() {
		nca, ncb = runClient(false, "__A__", "__C__")
		wg.Done()
	}()
	wg.Wait()

	if ca == 0 || cb == 0 {
		t.Errorf("with cancel, no load balancing: %v, %v", ca, cb)
	}
	if got, want := ca+cb, iterations; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if nca == 0 || ncb == 0 {
		t.Errorf("without cancel, no load balancing: %v, %v", nca, ncb)
	}
	if got, want := nca+ncb, iterations; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
