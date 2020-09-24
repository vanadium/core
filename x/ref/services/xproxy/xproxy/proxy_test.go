// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xproxy_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/x/lib/netstate"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/services/xproxy/xproxy"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
)

var randData []byte

type ipv4Only struct{}

func (c *ipv4Only) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	al := netstate.ConvertToAddresses(candidates)
	v4only := al.Filter(netstate.IsUnicastIPv4)
	if len(v4only) > 2 {
		v4only = v4only[:2]
	}
	return v4only.AsNetAddrs(), nil
}

func init() {
	randData = make([]byte, 1<<17)
	if _, err := rand.Read(randData); err != nil {
		panic("Could not read random data.")
	}
}

type testService struct{ m string }

func (t *testService) Echo(ctx *context.T, call rpc.ServerCall, arg string) (string, error) {
	return t.m + "response:" + arg, nil
}

func TestProxyRPC(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Start the proxy.
	pname, stop := startProxy(t, ctx, "proxy", security.AllowEveryone(), "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(sctx, "server", &testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()
	var got string
	if err := v23.GetClient(ctx).Call(ctx, "server", "Echo", []interface{}{"hello"}, []interface{}{&got}); err != nil {
		t.Fatal(err)
	}
	if want := "response:hello"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestServerRestart(t *testing.T) {
	// This test ensures that a server can be restarted even if it reuses
	// the same address:port pair across such a restart. This case looks
	// like a proxy to the client, in that the same address:port that was
	// used before with one RID, is now connected to and presents a new,
	// different RID; and hence could be a proxy. The internal logic in
	// the client RPC code detects such RID mismatches and will attempt
	// to connect as a proxy, if that fails, the client should retry
	// and re-resolve the name to obtain the new RID and then make a normal
	// RPC.
	// This scenarios is racy and this test works around the races as
	// outlined below.
	//
	// The problematic sequence of events is as follows:
	//
	// 1. server uses one, or two or more fixed address:ports - e.g. 127.0.0.1:8888
	//    and <public IP>:8888.
	// 2. the client makes an RPC to the server, creating an entry in its local
	//    namespace for that service.
	// 3. the server is restarted
	// 4. the client makes a call to the restarted server, but it uses the
	//    address in its local namespace, that is, the same address:port but
	//    the RID of the original server.
	// 5. the call made for 4 should timeout, since internally, the rpc
	//    client will attempt to treat it as a proxy and then fail to connect.
	// 6. after this initial failure, subsequent calls should succeed since they
	//    will obtain the new server address (ie same address:port but with the
	//    RID) from the mounttable/namespace.

	testServerRestart(t, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{
			{Protocol: "tcp", Address: "127.0.0.1:0"},
		},
	})
	testServerRestart(t, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{
			{Protocol: "tcp", Address: ":0"},
		},
		AddressChooser: &ipv4Only{},
	})

}
func ls(ctx *context.T, m, s string) []naming.MountedServer {
	_, file, line, _ := runtime.Caller(1)
	loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)
	resolved, err := v23.GetNamespace(ctx).Resolve(ctx, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: %s: --- no entries\n", loc, m)
		return nil
	}
	fmt.Fprintf(os.Stderr, "%v:%s: --- %v\n", loc, m, resolved.Servers)
	return resolved.Servers
}

func serverAddrs(ms []naming.MountedServer) []string {
	r := []string{}
	for _, m := range ms {
		r = append(r, m.Server)
	}
	return r
}

func callClient(ctx *context.T, m string) error {
	var got string
	if err := v23.GetClient(ctx).Call(ctx, "server", "Echo", []interface{}{"hello"}, []interface{}{&got}); err != nil {
		return err
	}
	if want := m + "response:hello"; got != want {
		return fmt.Errorf("got %v, want %v", got, want)
	}
	return nil
}

func testServerRestart(t *testing.T, lspec rpc.ListenSpec) { //nolint:gocyclo
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
	sctx, serverCancel := context.WithCancel(sctx)
	_, server, err := v23.WithNewServer(sctx, "server", &testService{"first:"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	testutil.WaitForServerPublished(server)

	servers := ls(sctx, "server side", "server")
	orig := ls(ctx, "client side", "server")
	if got, want := serverAddrs(servers), serverAddrs(orig); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Obtain the address used for the first server so as to reused it for
	// the restarted one.
	reused := rpc.ListenSpec{}
	for _, ep := range server.Status().Endpoints {
		reused.Addrs = append(reused.Addrs,
			struct {
				Protocol, Address string
			}{
				Protocol: ep.Protocol,
				Address:  ep.Address,
			})
	}
	expectNumAddresses := len(server.Status().Endpoints)

	if err := callClient(ctx, "first:"); err != nil {
		t.Fatal(err)
	}

	serverCancel()
	<-server.Closed()

	if got, want := len(ls(sctx, "stopped server side", "server")), 0; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := len(ls(ctx, "stopped client side", "server")), expectNumAddresses; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Restart the server
	sctx, _, err = v23.WithNewNamespace(ctx, ns.Roots()...)
	if err != nil {
		t.Fatal(err)
	}
	sctx = v23.WithListenSpec(sctx, reused)
	sctx, serverCancel = context.WithCancel(sctx)
	_, server, err = v23.WithNewServer(sctx, "server", &testService{"restarted:"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverCancel()

	testutil.WaitForServerPublished(server)

	if got, want := len(ls(sctx, "restarted server side", "server")), expectNumAddresses; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Get a new client in order to get a new flow manager with an empty
	// connection cache, but keep the namespace.
	ctx, _, err = v23.WithNewClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := serverAddrs(ls(ctx, "restarted client side", "server")), serverAddrs(orig); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Ensure a fast timeout for the first call that we expect to fail.
	fctx, _ := context.WithTimeout(ctx, 10*time.Second)
	start := time.Now()
	if err := callClient(fctx, "restarted:"); !errors.Is(err, verror.ErrTimeout) {
		t.Fatalf("missing or unexpected error: %v", err)
	}

	firstDupCall := time.Now()
	// Expect the second call to succeed.
	if err := callClient(ctx, "restarted:"); err != nil {
		t.Fatal(err)
	}
	secondDupCall := time.Now()

	// Make sure the second call is faster than the first.
	firstInterval := firstDupCall.Sub(start)
	secondInterval := secondDupCall.Sub(firstDupCall)
	if firstInterval < secondInterval {
		t.Errorf("second call should be faster than first: first: %v  second: %v", firstInterval, secondInterval)
	}

}

func TestMultipleProxyRPC(t *testing.T) {
	kp := newKillProtocol()
	flow.RegisterProtocol("kill", kp)
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Start the proxies.
	p1name, stop := startProxy(t, ctx, "p1", security.AllowEveryone(), "", address{"kill", "127.0.0.1:0"})
	defer stop()
	p2name, stop := startProxy(t, ctx, "p2", security.AllowEveryone(), p1name, address{"kill", "127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: p2name})
	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(sctx, "server", &testService{}, nil)
	if err != nil {
		t.Error(err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()

	var got string
	if err := v23.GetClient(ctx).Call(ctx, "server", "Echo", []interface{}{"hello"}, []interface{}{&got}); err != nil {
		t.Error(err)
	}
	if want := "response:hello"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	kp.KillConnections()
	// Killing the connections and trying again should work.
	for {
		if err := v23.GetClient(ctx).Call(ctx, "server", "Echo", []interface{}{"hello"}, []interface{}{&got}); err == nil {
			break
		}
	}
	if want := "response:hello"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProxyNotAuthorized(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Make principals for the proxy, server, and client.
	pctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("proxy"))
	if err != nil {
		t.Fatal(err)
	}
	sctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("server"))
	if err != nil {
		t.Fatal(err)
	}
	cctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("client"))
	if err != nil {
		t.Fatal(err)
	}

	// Have the root bless the client, server, and proxy.
	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	if err := root.Bless(v23.GetPrincipal(pctx), "proxy"); err != nil {
		t.Fatal(err)
	}
	if err := root.Bless(v23.GetPrincipal(cctx), "client"); err != nil {
		t.Fatal(err)
	}
	if err := root.Bless(v23.GetPrincipal(sctx), "server"); err != nil {
		t.Fatal(err)
	}

	// Now the proxy's blessings would fail authorization from the client using the
	// default authorizer.
	pname, stop := startProxy(t, pctx, "proxy", security.AllowEveryone(), "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(sctx)
	_, server, err := v23.WithNewServer(sctx, "server", &testService{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()
	// The call should succeed which means that the client did not try to authorize
	// the proxy's blessings.
	var got string
	if err := v23.GetClient(cctx).Call(cctx, "server", "Echo", []interface{}{"hello"},
		[]interface{}{&got}, options.ServerAuthorizer{Authorizer: rejectProxyAuthorizer{}}); err != nil {
		t.Fatal(err)
	}
	if want := "response:hello"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProxyAuthorizesServer(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Make principals for the proxy and server.
	pctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("proxy"))
	if err != nil {
		t.Fatal(err)
	}
	sctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("server"))
	if err != nil {
		t.Fatal(err)
	}

	// Have the root bless the proxy and server.
	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	if err := root.Bless(v23.GetPrincipal(pctx), "proxy"); err != nil {
		t.Fatal(err)
	}
	if err := root.Bless(v23.GetPrincipal(sctx), "server"); err != nil {
		t.Fatal(err)
	}

	// Start a proxy that accepts connections from everyone and ensure that it does.
	pname, stop := startProxy(t, pctx, "acceptproxy", security.AllowEveryone(), "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(sctx)
	defer cancel()
	_, server, err := v23.WithNewServer(sctx, "", &testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := testutil.WaitForProxyEndpoints(server, pname)
	proxyEP := status.Endpoints[0]

	// A proxy using the default authorizer should not authorize the server.
	pname, stop = startProxy(t, pctx, "denyproxy", nil, "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Proxy: pname})
	_, server, err = v23.WithNewServer(sctx, "", &testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for {
		status := server.Status()
		if err, ok := status.ProxyErrors["denyproxy"]; ok && err == nil {
			t.Errorf("proxy should not have authorized server")
		} else if ok {
			break
		}
		<-status.Dirty
	}

	// Artificially constructing the proxied endpoint to the server should
	// not work. (i.e. a client cannot "trick" a proxy into connecting to a server
	// that the proxy doesn't want to talk to).
	ep, err := setEndpointRoutingID(proxyEP, server.Status().Endpoints[0].RoutingID)
	if err != nil {
		t.Error(err)
	}
	var got string
	if err := v23.GetClient(ctx).Call(ctx, ep.Name(), "Echo", []interface{}{"hello"}, []interface{}{&got}, options.NoRetry{}); err == nil {
		t.Error("proxy should not have authorized server.")
	}
}

func TestBigProxyRPC(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Start the proxy.
	pname, stop := startProxy(t, ctx, "", security.AllowEveryone(), "", address{"tcp", "127.0.0.1:0"})
	defer stop()
	// Start the server listening through the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, server, err := v23.WithNewServer(sctx, "", &testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := testutil.WaitForProxyEndpoints(server, pname)
	name := status.Endpoints[0].Name()
	var got string
	if err := v23.GetClient(ctx).Call(ctx, name, "Echo", []interface{}{string(randData)}, []interface{}{&got}); err != nil {
		t.Fatal(err)
	}
	if want := "response:" + string(randData); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestProxiedServerCachedConnection(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	mu := &sync.Mutex{}
	numConns := 0
	ctx = debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		mu.Lock()
		numConns++
		mu.Unlock()
		return c
	})

	// Make principals for the proxy and server.
	pctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("proxy"))
	if err != nil {
		t.Fatal(err)
	}
	sctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("server"))
	if err != nil {
		t.Fatal(err)
	}

	// Have the root bless the proxy and server.
	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	if err := root.Bless(v23.GetPrincipal(pctx), "proxy"); err != nil {
		t.Fatal(err)
	}
	if err := root.Bless(v23.GetPrincipal(sctx), "server"); err != nil {
		t.Fatal(err)
	}

	// Start the proxy.
	pname, stop := startProxy(t, pctx, "proxy", security.AllowEveryone(), "", address{"debug", "tcp/127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(sctx)
	ctx, server, err := v23.WithNewServer(sctx, "server", &testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()
	var got string
	// Make the server call itself.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Echo", []interface{}{"hello"}, []interface{}{&got}); err != nil {
		t.Fatal(err)
	}
	if want := "response:hello"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	// Ensure that only 1 connection was made. So we check for 2, 1 accepted, and 1 dialed.
	// One connection for the server connecting to the proxy, the proxy should reuse
	// the connection on the second hop back to the server.
	mu.Lock()
	n := numConns
	mu.Unlock()
	if want := 2; n != want {
		t.Errorf("got %v, want %v", n, want)
	}
}

type blockingServer struct {
	start chan struct{} // closed when the
	wait  chan struct{}
}

func (s *blockingServer) BlockingCall(*context.T, rpc.ServerCall) error {
	close(s.start)
	<-s.wait
	return nil
}

func newBlockingServer() *blockingServer {
	return &blockingServer{make(chan struct{}), make(chan struct{})}
}

func TestConcurrentProxyConnections(t *testing.T) {
	// Test that when a client makes a connection to two different proxied servers
	// a call to one server doesn't cause a call to the other to fail.
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Start the proxy.
	pname, stop := startProxy(t, ctx, "proxy", security.AllowEveryone(), "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(ctx)
	defer cancel()
	service := newBlockingServer()
	_, server, err := v23.WithNewServer(sctx, "", service, nil)
	if err != nil {
		t.Error(err)
	}
	status := testutil.WaitForProxyEndpoints(server, pname)
	ep := status.Endpoints[0]

	// Create a nonexistent server.
	badep, err := setEndpointRoutingID(ep, naming.FixedRoutingID(0x666))
	if err != nil {
		t.Error(err)
	}

	// Start a call to the good server.
	call, err := v23.GetClient(ctx).StartCall(ctx, ep.Name(), "BlockingCall", nil)
	if err != nil {
		t.Error(err)
	}
	// wait for the server to get the rpc.
	<-service.start
	// Make a call to the noexistent server.
	if _, err := v23.GetClient(ctx).PinConnection(ctx, badep.Name(), options.NoRetry{}); err == nil {
		t.Errorf("Call should not succeed.")
	}
	// Unblock the first rpc and ensure that it succeeds.
	close(service.wait)
	if err := call.Finish(); err != nil {
		t.Error(err)
	}
}

func TestProxyBlessings(t *testing.T) {
	// Test that the proxy presents the blessings tagged for the server, rather than
	// its default blessings.
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Make principals for the proxy and server.
	pctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("proxy"))
	if err != nil {
		t.Fatal(err)
	}
	sctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("server"))
	if err != nil {
		t.Fatal(err)
	}

	// Have the root bless the proxy and server.
	root := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))
	if err := root.Bless(v23.GetPrincipal(pctx), "proxy"); err != nil {
		t.Fatal(err)
	}
	if err := root.Bless(v23.GetPrincipal(sctx), "server"); err != nil {
		t.Fatal(err)
	}

	// Set the for peer blessings that the server will send to the proxy to be one
	// that the proxy will reject. Additionally ensure that the default blessing is
	// one that the proxy will accept. This ensures that the server is sending the
	// blessings tagged for the proxy.
	serverP := v23.GetPrincipal(sctx)
	def, _ := serverP.BlessingStore().Default()
	pblesser := testutil.IDProviderFromPrincipal(v23.GetPrincipal(pctx))
	if err := pblesser.Bless(serverP, "server"); err != nil {
		t.Fatal(err)
	}
	if _, err := serverP.BlessingStore().Set(def, "..."); err != nil {
		t.Fatal(err)
	}

	// Start the proxy.
	pname, stop := startProxy(t, pctx, "proxy", nil, "", address{"tcp", "127.0.0.1:0"})
	defer stop()

	// Start the server listening through the proxy.
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Proxy: pname})
	sctx, cancel := context.WithCancel(sctx)
	defer cancel()
	_, server, err := v23.WithNewServer(sctx, "", &testService{}, nil)
	if err != nil {
		t.Error(err)
	}

	for {
		status := server.Status()
		if err, ok := status.ProxyErrors["proxy"]; ok && err == nil {
			t.Errorf("proxy should not have authorized server")
		} else if ok {
			break
		}
		<-status.Dirty
	}
}

type rejectProxyAuthorizer struct{}

func (rejectProxyAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	names, _ := security.RemoteBlessingNames(ctx, call)
	for _, n := range names {
		if strings.Contains(n, "proxy") {
			panic("should not call authorizer on proxy")
		}
	}
	return nil
}

type address struct {
	Protocol, Address string
}

func startProxy(t *testing.T, ctx *context.T, name string, auth security.Authorizer, listenOnProxy string, addrs ...address) (string, func()) {
	var ls rpc.ListenSpec
	for _, addr := range addrs {
		ls.Addrs = append(ls.Addrs, addr)
	}
	ls.Proxy = listenOnProxy
	ctx = v23.WithListenSpec(ctx, ls)
	ctx, cancel := context.WithCancel(ctx)
	proxy, err := xproxy.New(ctx, name, auth)
	if err != nil {
		t.Fatal(err)
	}
	stop := func() {
		cancel()
		<-proxy.Closed()
	}
	if len(name) > 0 {
		return name, stop
	}
	peps := proxy.ListeningEndpoints()
	for _, pep := range peps {
		if pep.Addr().Network() == "tcp" || pep.Addr().Network() == "kill" {
			return pep.Name(), stop
		}
	}
	t.Fatal("Proxy not listening on network address.")
	return "", nil
}

type killProtocol struct {
	protocol flow.Protocol
	mu       sync.Mutex
	conns    []flow.Conn
}

type kpListener struct {
	kp *killProtocol
	flow.Listener
}

func (l *kpListener) Accept(ctx *context.T) (flow.Conn, error) {
	c, err := l.Listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	l.kp.mu.Lock()
	l.kp.conns = append(l.kp.conns, c)
	l.kp.mu.Unlock()
	return c, err
}

func newKillProtocol() *killProtocol {
	p, _ := flow.RegisteredProtocol("tcp")
	return &killProtocol{protocol: p}
}

func (p *killProtocol) KillConnections() {
	p.mu.Lock()
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = nil
	p.mu.Unlock()
}

func (p *killProtocol) Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	c, err := p.protocol.Dial(ctx, "tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.conns = append(p.conns, c)
	p.mu.Unlock()
	return c, nil
}

func (p *killProtocol) Listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	l, err := p.protocol.Listen(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return &kpListener{kp: p, Listener: l}, nil
}

func (p *killProtocol) Resolve(ctx *context.T, protocol, address string) (string, []string, error) {
	return p.protocol.Resolve(ctx, "tcp", address)
}

func setEndpointRoutingID(ep naming.Endpoint, rid naming.RoutingID) (naming.Endpoint, error) {
	network, address, _, mountable := getEndpointParts(ep)
	var opts []naming.EndpointOpt
	opts = append(opts, rid)
	opts = append(opts, mountable)
	epString := naming.FormatEndpoint(network, address, opts...)
	return naming.ParseEndpoint(epString)
}

// getEndpointParts returns all the fields of ep.
func getEndpointParts(ep naming.Endpoint) (network string, address string,
	rid naming.RoutingID, mountable naming.EndpointOpt) {
	network, address = ep.Addr().Network(), ep.Addr().String()
	rid = ep.RoutingID
	mountable = naming.ServesMountTable(ep.ServesMountTable)
	return
}
