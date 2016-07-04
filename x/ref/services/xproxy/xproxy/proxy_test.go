// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xproxy_test

import (
	"crypto/rand"
	"strings"
	"sync"
	"testing"
	"time"

	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/services/xproxy/xproxy"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
	"v.io/x/ref/test/testutil"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

const leakWaitTime = 250 * time.Millisecond

var randData []byte

func init() {
	randData = make([]byte, 1<<17)
	if _, err := rand.Read(randData); err != nil {
		panic("Could not read random data.")
	}
}

type testService struct{}

func (t *testService) Echo(ctx *context.T, call rpc.ServerCall, arg string) (string, error) {
	return "response:" + arg, nil
}

func TestProxyRPC(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
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

func TestMultipleProxyRPC(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
		[]interface{}{&got}, options.ServerAuthorizer{rejectProxyAuthorizer{}}); err != nil {
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
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
	serverP.BlessingStore().Set(def, "...")

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
