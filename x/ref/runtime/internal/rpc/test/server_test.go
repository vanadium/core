// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type noMethodsType struct{ Field string }

type fieldType struct {
	unexported string //nolint:structcheck,unused
}

type noExportedFieldsType struct{}

func (noExportedFieldsType) F(_ *context.T, _ rpc.ServerCall, f fieldType) error { return nil }

type badObjectDispatcher struct{}

func (badObjectDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return noMethodsType{}, nil, nil
}

// TestBadObject ensures that Serve handles bad receiver objects gracefully (in
// particular, it doesn't panic).
func TestBadObject(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")

	if _, _, err := v23.WithNewServer(sctx, "", nil, nil); err == nil {
		t.Fatal("should have failed")
	}
	if _, _, err := v23.WithNewServer(sctx, "", new(noMethodsType), nil); err == nil {
		t.Fatal("should have failed")
	}
	if _, _, err := v23.WithNewServer(sctx, "", new(noExportedFieldsType), nil); err == nil {
		t.Fatal("should have failed")
	}
	if _, _, err := v23.WithNewDispatchingServer(sctx, "", badObjectDispatcher{}); err != nil {
		t.Fatalf("ServeDispatcher failed: %v", err)
	}
	// TODO(mattr): It doesn't necessarily make sense to me that a bad object from
	// the dispatcher results in a retry.
	var cancel context.CancelFunc
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var result string
	if err := v23.GetClient(cctx).Call(cctx, "servername", "SomeMethod", nil, []interface{}{&result}); err == nil {
		// TODO(caprita): Check the error type rather than
		// merely ensuring the test doesn't panic.
		t.Fatalf("Call should have failed")
	}
}

type statusServer struct{ ch chan struct{} }

func (s *statusServer) Hang(ctx *context.T, _ rpc.ServerCall) error {
	s.ch <- struct{}{} // Notify the server has received a call.
	<-s.ch             // Wait for the server to be ready to go.
	return nil
}

func TestServerStatus(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	serverChan := make(chan struct{})
	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(sctx, "test", &statusServer{serverChan}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := server.Status()
	if got, want := status.State, rpc.ServerActive; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}

	progress := make(chan error)
	makeCall := func(ctx *context.T) {
		call, err := v23.GetClient(ctx).StartCall(ctx, "test", "Hang", nil)
		progress <- err
		progress <- call.Finish()
	}
	go makeCall(ctx)

	// Wait for RPC to start and the server has received the call.
	if err := <-progress; err != nil {
		t.Fatal(err)
	}
	<-serverChan

	// Stop server asynchronously
	go func() {
		cancel()
		<-server.Closed()
	}()

	waitForStatus := func(want rpc.ServerState) {
		then := time.Now()
		for {
			status = server.Status()
			if got := status.State; got != want {
				if time.Since(then) > time.Minute {
					t.Fatalf("got %s, want %s", got, want)
				}
			} else {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Server should enter 'ServerStopping' state.
	waitForStatus(rpc.ServerStopping)
	// Server won't stop until the statusServer's hung method completes.
	close(serverChan)
	// Wait for RPC to finish
	if err := <-progress; err != nil {
		t.Fatal(err)
	}
	// Now that the RPC is done, the server should be able to stop.
	waitForStatus(rpc.ServerStopped)
}

func TestPublisherStatus(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{
			{"tcp", "127.0.0.1:0"},
			{"tcp", "127.0.0.1:0"},
		},
	})
	_, server, err := v23.WithNewServer(sctx, "foo", &testServer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := testutil.WaitForServerPublished(server)
	if got, want := len(status.PublisherStatus), 2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	eps := server.Status().Endpoints
	if got, want := len(eps), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	// Add a second name and we should now see 4 mounts, 2 for each name.
	if err := server.AddName("bar"); err != nil {
		t.Fatal(err)
	}
	status = testutil.WaitForServerPublished(server)
	if got, want := len(status.PublisherStatus), 4; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	serversPerName := map[string][]string{}
	for _, ms := range status.PublisherStatus {
		serversPerName[ms.Name] = append(serversPerName[ms.Name], ms.Server)
	}
	if got, want := len(serversPerName), 2; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
	for _, name := range []string{"foo", "bar"} {
		if got, want := len(serversPerName[name]), 2; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
		sort.Strings(serversPerName[name])
		if got, want := serversPerName[name], endpointToStrings(eps); !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestIsLeafServerOption(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	_, _, err := v23.WithNewDispatchingServer(ctx, "leafserver",
		&testServerDisp{&testServer{}}, options.IsLeaf(true))
	if err != nil {
		t.Fatal(err)
	}
	// we have set IsLeaf to true, sending any suffix to leafserver should result
	// in an suffix was not expected error.
	var result string
	callErr := v23.GetClient(ctx).Call(ctx, "leafserver/unwantedSuffix", "Echo", []interface{}{"Mirror on the wall"}, []interface{}{&result})
	if callErr == nil {
		t.Fatalf("Call should have failed with suffix was not expected error")
	}
}

func endpointToStrings(eps []naming.Endpoint) []string {
	r := []string{}
	for _, ep := range eps {
		r = append(r, ep.String())
	}
	sort.Strings(r)
	return r
}

type ldServer struct {
	wait chan struct{}
}

func (s *ldServer) Do(ctx *context.T, call rpc.ServerCall) (bool, error) {
	<-s.wait
	return ctx.Err() != nil, nil
}

func TestLameDuck(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	cases := []struct {
		timeout     time.Duration
		finishError bool
		wasCanceled bool
	}{
		{timeout: time.Minute, wasCanceled: false},
		{timeout: 0, finishError: true},
	}
	for _, c := range cases {
		s := &ldServer{wait: make(chan struct{})}
		sctx, cancel := context.WithCancel(ctx)
		_, server, err := v23.WithNewServer(sctx, "ld", s, nil, options.LameDuckTimeout(c.timeout))
		if err != nil {
			t.Fatal(err)
		}
		call, err := v23.GetClient(ctx).StartCall(ctx, "ld", "Do", nil)
		if err != nil {
			t.Fatal(err)
		}
		// Now cancel the context putting the server into lameduck mode.
		cancel()
		// Now allow the call to complete and see if the context was canceled.
		close(s.wait)

		var wasCanceled bool
		err = call.Finish(&wasCanceled)
		if c.finishError {
			if err == nil {
				t.Errorf("case: %v: Expected error for call but didn't get one", c)
			}
		} else {
			if wasCanceled != c.wasCanceled {
				t.Errorf("case %v: got %v.", c, wasCanceled)
			}
		}
		<-server.Closed()
	}
}

type dummyService struct{}

func (dummyService) Do(ctx *context.T, call rpc.ServerCall) error {
	return nil
}

func mountedBlessings(ctx *context.T, name string) ([]string, error) {
	me, err := v23.GetNamespace(ctx).Resolve(ctx, "server")
	if err != nil {
		return nil, err
	}
	if len(me.Servers) > 0 {
		ms := me.Servers[0]
		ep, err := naming.ParseEndpoint(ms.Server)
		if err != nil {
			return nil, err
		}
		return ep.BlessingNames(), nil
	}
	return nil, nil
}

func TestUpdateServerBlessings(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	v23.GetNamespace(ctx).CacheCtl(naming.DisableCache(true))

	idp := testutil.IDProviderFromPrincipal(v23.GetPrincipal(ctx))

	serverp := testutil.NewPrincipal()
	if err := idp.Bless(serverp, "b1"); err != nil {
		t.Error(err)
	}
	sctx, err := v23.WithPrincipal(ctx, serverp)
	if err != nil {
		t.Error(err)
	}

	if _, _, err = v23.WithNewServer(sctx, "server", dummyService{}, security.AllowEveryone()); err != nil {
		t.Error(err)
	}

	want := "test-blessing:b1"
	for {
		bs, err := mountedBlessings(ctx, "server")
		if err == nil && len(bs) == 1 && bs[0] == want {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	pcon, err := v23.GetClient(ctx).PinConnection(ctx, "server")
	if err != nil {
		t.Error(err)
	}
	defer pcon.Unpin()
	conn := pcon.Conn()
	names, _ := security.RemoteBlessingNames(ctx, security.NewCall(&security.CallParams{
		LocalPrincipal:  v23.GetPrincipal(ctx),
		RemoteBlessings: conn.RemoteBlessings(),
	}))
	if len(names) != 1 || names[0] != want {
		t.Errorf("got %v, wanted %q", names, want)
	}

	// Now we bless the server with a new blessing (which will change its default
	// blessing).  Then we wait for the new value to propagate to the remote end.
	if err := idp.Bless(serverp, "b2"); err != nil {
		t.Error(err)
	}
	want = "test-blessing:b2"
	for {
		bs, err := mountedBlessings(ctx, "server")
		if err == nil && len(bs) == 1 && bs[0] == want {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for {
		names, _ := security.RemoteBlessingNames(ctx, security.NewCall(&security.CallParams{
			LocalPrincipal:  v23.GetPrincipal(ctx),
			RemoteBlessings: conn.RemoteBlessings(),
		}))
		if len(names) == 1 || names[0] == want {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestListenNetworkInterrupted(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	bp := newBrokenNetProtocol()
	sctx, cancel := context.WithCancel(ctx)
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "broken", Address: "127.0.0.1:0"}}})
	// If the network is broken and prevents listening before the server is created,
	// the server should listen and accept rpcs when the network is fixed.
	bp.setBroken(true)
	_, server, err := v23.WithNewServer(sctx, "server", &testServer{}, security.AllowEveryone())
	if err != nil {
		t.Error(err)
	}
	// rpc should fail until the network works.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Closure", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("call should have failed")
	}
	bp.setBroken(false)
	// rpc should succeed now that network is working again.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Closure", nil, nil); err != nil {
		t.Error(err)
	}

	// kill connections to clear the cache.
	bp.killConnections()
	// If the network is broken after the server is created and successfully listened,
	// the server should listen and accept rpcs when the network is fixed.
	bp.setBroken(true)
	// rpc should fail until the network works.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Closure", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("call should have failed")
	}
	bp.setBroken(false)
	// rpc should succeed now that network is working again.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Closure", nil, nil); err != nil {
		t.Error(err)
	}
	if eps := server.Status().Endpoints; len(eps) != 1 {
		t.Errorf("server should only be listening on one endpoint, got %v", eps)
	}
	cancel()
	<-server.Closed()
}

type brokenNetProtocol struct {
	protocol flow.Protocol
	mu       sync.Mutex
	// Unfortunately broken needs to be a channel rather than a bool since we need
	// to interrupt listener.Accept() when the network breaks.
	broken chan struct{}
	conns  []flow.Conn
}

func newBrokenNetProtocol() *brokenNetProtocol {
	p, _ := flow.RegisteredProtocol("tcp")
	bp := &brokenNetProtocol{protocol: p, broken: make(chan struct{})}
	flow.RegisterProtocol("broken", bp)
	return bp
}

// setBroken sets the protocol to disallow all dials and listens if broken is true.
func (p *brokenNetProtocol) setBroken(broken bool) {
	defer p.mu.Unlock()
	p.mu.Lock()
	if broken {
		select {
		case <-p.broken:
		default:
			close(p.broken)
		}
	} else {
		select {
		case <-p.broken:
			p.broken = make(chan struct{})
		default:
		}
	}
}

func (p *brokenNetProtocol) isBroken() bool {
	p.mu.Lock()
	broken := p.broken
	p.mu.Unlock()
	select {
	case <-broken:
		return true
	default:
	}
	return false
}

// killConnections kills all connections to avoid using cached connections.
func (p *brokenNetProtocol) killConnections() {
	defer p.mu.Unlock()
	p.mu.Lock()
	for _, c := range p.conns {
		c.Close()
	}
	p.conns = nil
}

func (p *brokenNetProtocol) Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	// Return an error if the network is broken.
	if p.isBroken() {
		return nil, fmt.Errorf("network is broken")
	}
	c, err := p.protocol.Dial(ctx, "tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.conns = append(p.conns, c)
	p.mu.Unlock()
	return c, nil
}

func (p *brokenNetProtocol) Listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	// Return an error if the network is broken.
	if p.isBroken() {
		return nil, fmt.Errorf("network is broken")
	}
	l, err := p.protocol.Listen(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	return &brokenNetListener{p: p, Listener: l}, nil
}

func (p *brokenNetProtocol) Resolve(ctx *context.T, protocol, address string) (string, []string, error) {
	return p.protocol.Resolve(ctx, "tcp", address)
}

type brokenNetListener struct {
	p *brokenNetProtocol
	flow.Listener
}

type connAndErr struct {
	c   flow.Conn
	err error
}

func (l *brokenNetListener) Accept(ctx *context.T) (flow.Conn, error) {
	ch := make(chan connAndErr)
	go func() {
		c, err := l.Listener.Accept(ctx)
		ch <- connAndErr{c, err}
	}()

	l.p.mu.Lock()
	broken := l.p.broken
	l.p.mu.Unlock()
	select {
	case <-broken:
		cae := <-ch
		c, err := cae.c, cae.err
		if err != nil {
			return nil, err
		}
		c.Close()
		return nil, fmt.Errorf("network is broken")
	case cae := <-ch:
		c, err := cae.c, cae.err
		if err != nil {
			return nil, err
		}
		l.p.mu.Lock()
		l.p.conns = append(l.p.conns, c)
		l.p.mu.Unlock()
		return c, nil
	}
}
