// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/services/xproxy/xproxy"
	"v.io/x/ref/test"
)

type canceld struct {
	name     string
	child    string
	started  chan struct{}
	canceled chan struct{}
}

func (c *canceld) Run(ctx *context.T, _ rpc.ServerCall) error {
	close(c.started)
	client := v23.GetClient(ctx)
	var done chan struct{}
	if c.child != "" {
		done = make(chan struct{})
		go func() {
			client.Call(ctx, c.child, "Run", nil, nil) //nolint:errcheck
			close(done)
		}()
	}
	<-ctx.Done()
	if done != nil {
		<-done
	}
	close(c.canceled)
	return nil
}

func makeCanceld(ctx *context.T, name, child string) (*canceld, error) {
	c := &canceld{
		name:     name,
		child:    child,
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	_, _, err := v23.WithNewServer(ctx, name, c, security.AllowEveryone())
	if err != nil {
		return nil, err
	}
	return c, nil
}

// TestCancellationPropagation tests that cancellation propagates along an
// RPC call chain without user intervention.
func TestCancellationPropagation(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	c1, err := makeCanceld(ctx, "c1", "c2")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := makeCanceld(ctx, "c2", "")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		v23.GetClient(ctx).Call(ctx, "c1", "Run", nil, nil) //nolint:errcheck
		close(done)
	}()

	<-c1.started
	<-c2.started
	cancel()
	<-c1.canceled
	<-c2.canceled
	<-done
}

type cancelTestServer struct {
	started   chan struct{}
	cancelled chan struct{}
	t         *testing.T
}

func newCancelTestServer(t *testing.T) *cancelTestServer {
	return &cancelTestServer{
		started:   make(chan struct{}),
		cancelled: make(chan struct{}),
		t:         t,
	}
}

func (s *cancelTestServer) CancelStreamReader(ctx *context.T, call rpc.StreamServerCall) error {
	close(s.started)
	var b []byte
	if err := call.Recv(&b); err != io.EOF {
		s.t.Errorf("Got error %v, want io.EOF", err)
	}
	<-ctx.Done()
	close(s.cancelled)
	return nil
}

// CancelStreamIgnorer doesn't read from it's input stream so all it's
// buffers fill.  The intention is to show that call.Done() is closed
// even when the stream is stalled.
func (s *cancelTestServer) CancelStreamIgnorer(ctx *context.T, _ rpc.StreamServerCall) error {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	return nil
}

func waitForCancel(t *testing.T, ts *cancelTestServer, cancel context.CancelFunc) {
	<-ts.started
	cancel()
	<-ts.cancelled
}

// TestCancel tests cancellation while the server is reading from a stream.
func TestCancel(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	var (
		sctx = withPrincipal(t, ctx, "server")
		cctx = withPrincipal(t, ctx, "client")
		ts   = newCancelTestServer(t)
	)
	_, _, err := v23.WithNewServer(sctx, "cancel", ts, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithCancel(cctx)
	done := make(chan struct{})
	//nolint:errcheck
	go func() {
		v23.GetClient(cctx).Call(cctx, "cancel", "CancelStreamReader", nil, nil)
		close(done)
	}()
	waitForCancel(t, ts, cancel)
	<-done
}

// TestCancelWithFullBuffers tests that even if the writer has filled the buffers and
// the server is not reading that the cancel message gets through.
func TestCancelWithFullBuffers(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	var (
		sctx = withPrincipal(t, ctx, "server")
		cctx = withPrincipal(t, ctx, "client")
		ts   = newCancelTestServer(t)
	)
	_, _, err := v23.WithNewServer(sctx, "cancel", ts, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	cctx, cancel := context.WithCancel(cctx)
	call, err := v23.GetClient(cctx).StartCall(cctx, "cancel", "CancelStreamIgnorer", nil)
	if err != nil {
		t.Fatalf("Start call failed: %v", err)
	}

	// Fill up all the write buffers to ensure that cancelling works even when the stream
	// is blocked.
	if err := call.Send(make([]byte, conn.DefaultBytesBufferedPerFlow-2048)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	//nolint:errcheck
	go func() {
		call.Finish()
		close(done)
	}()

	waitForCancel(t, ts, cancel)
	<-done
}

type channelTestServer struct {
	waiting  chan struct{}
	canceled chan struct{}
}

func (s *channelTestServer) Run(ctx *context.T, call rpc.ServerCall, wait time.Duration) error {
	time.Sleep(wait)
	return nil
}

func (s *channelTestServer) WaitForCancel(ctx *context.T, call rpc.ServerCall) error {
	close(s.waiting)
	<-ctx.Done()
	close(s.canceled)
	return nil
}

type disconnect interface {
	stop(read, write bool)
}

//nolint:deadcode,unused
type disConn struct {
	net.Conn
	mu                  sync.Mutex
	stopread, stopwrite bool
}

func (p *disConn) stop(read, write bool) {
	p.mu.Lock()
	p.stopread = read
	p.stopwrite = write
	p.mu.Unlock()
}
func (p *disConn) Write(b []byte) (int, error) {
	p.mu.Lock()
	stopwrite := p.stopwrite
	p.mu.Unlock()
	if stopwrite {
		return len(b), nil
	}
	return p.Conn.Write(b)
}
func (p *disConn) Read(b []byte) (int, error) {
	for {
		n, err := p.Conn.Read(b)
		p.mu.Lock()
		stopread := p.stopread
		p.mu.Unlock()
		if err != nil || !stopread {
			return n, err
		}
	}
}

type flowDisConn struct {
	flow.Conn
	mu                  sync.Mutex
	stopread, stopwrite bool
}

func (p *flowDisConn) stop(read, write bool) {
	p.mu.Lock()
	p.stopread = read
	p.stopwrite = write
	p.mu.Unlock()
}
func (p *flowDisConn) WriteMsg(data ...[]byte) (int, error) {
	p.mu.Lock()
	stopwrite := p.stopwrite
	p.mu.Unlock()
	if stopwrite {
		l := 0
		for _, d := range data {
			l += len(d)
		}
		return l, nil
	}
	return p.Conn.WriteMsg(data...)
}
func (p *flowDisConn) ReadMsg() ([]byte, error) {
	for {
		msg, err := p.Conn.ReadMsg()
		p.mu.Lock()
		stopread := p.stopread
		p.mu.Unlock()
		if err != nil || !stopread {
			return msg, err
		}
	}
}

type flowdis struct {
	base flow.Protocol
}

func (f *flowdis) Dial(ctx *context.T, protocol, address string, timeout time.Duration) (flow.Conn, error) {
	return f.base.Dial(ctx, "tcp", address, timeout)
}
func (f *flowdis) Resolve(ctx *context.T, proctocol, address string) (string, []string, error) {
	return f.base.Resolve(ctx, "tcp", address)
}
func (f *flowdis) Listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	return f.base.Listen(ctx, "tcp", address)
}

func registerDisProtocol(wrap string, conns chan disconnect) {
	// We only register this flow protocol to make the test work in clients mode.
	protocol, _ := flow.RegisteredProtocol("tcp")
	flow.RegisterProtocol("dis", &flowdis{base: protocol})
}

func testChannelTimeout(t *testing.T, ctx *context.T) {
	conns := make(chan disconnect, 1)
	sctx := v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{Protocol: "debug", Address: "tcp/127.0.0.1:0"}},
	})
	ctx = debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		dc := &flowDisConn{Conn: c}
		conns <- dc
		return dc
	})
	_, s, err := v23.WithNewServer(sctx, "", &channelTestServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	ep := s.Status().Endpoints[0]
	registerDisProtocol(ep.Addr().Network(), conns)

	// Long calls don't cause the timeout, the control stream is still operating.
	err = v23.GetClient(ctx).Call(ctx, ep.Name(), "Run", []interface{}{2 * time.Second},
		nil, options.ChannelTimeout(500*time.Millisecond))
	if err != nil {
		t.Errorf("got %v want nil", err)
	}
	(<-conns).stop(true, true)
	err = v23.GetClient(ctx).Call(ctx, ep.Name(), "Run", []interface{}{time.Duration(0)},
		nil, options.ChannelTimeout(100*time.Millisecond))
	if err == nil {
		t.Errorf("wanted non-nil error: %v", err)
	}
}

func TestChannelTimeout(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	testChannelTimeout(t, ctx)
}

func TestChannelTimeout_Proxy(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ls := v23.GetListenSpec(ctx)
	ctx, cancel := context.WithCancel(ctx)
	p, err := xproxy.New(ctx, "", security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	ls.Addrs = nil
	ls.Proxy = p.ListeningEndpoints()[0].Name()
	defer func() {
		cancel()
		<-p.Closed()
	}()
	testChannelTimeout(t, v23.WithListenSpec(ctx, ls))
}

func testChannelTimeOutServer(t *testing.T, ctx *context.T) {
	conns := make(chan disconnect, 1)
	sctx := v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{Protocol: "debug", Address: "tcp/127.0.0.1:0"}},
	})
	ctx = debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		dc := &flowDisConn{Conn: c}
		conns <- dc
		return dc
	})
	cts := &channelTestServer{
		canceled: make(chan struct{}),
		waiting:  make(chan struct{}),
	}
	_, s, err := v23.WithNewServer(sctx, "", cts, security.AllowEveryone(),
		options.ChannelTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	ep := s.Status().Endpoints[0]
	registerDisProtocol(ep.Addr().Network(), conns)

	// Long calls don't cause the timeout, the control stream is still operating.
	err = v23.GetClient(ctx).Call(ctx, ep.Name(), "Run", []interface{}{2 * time.Second},
		nil)
	if err != nil {
		t.Errorf("got %v want nil", err)
	}
	// When the server closes the VC in response to the channel timeout the server
	// call will see a cancellation.  We do a call and wait for that server-side
	// cancellation.  Then we cancel the client call just to clean up.
	cctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	//nolint:errcheck
	go func() {
		v23.GetClient(cctx).Call(cctx, ep.Name(), "WaitForCancel", nil, nil)
		close(done)
	}()
	<-cts.waiting
	(<-conns).stop(true, true)
	<-cts.canceled
	cancel()
	<-done
}

func TestChannelTimeoutServer(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	testChannelTimeOutServer(t, ctx)
}

func TestChannelTimeoutServerProxy(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ls := v23.GetListenSpec(ctx)
	ctx, cancel := context.WithCancel(ctx)
	p, err := xproxy.New(ctx, "", security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	ls.Addrs = nil
	ls.Proxy = p.ListeningEndpoints()[0].Name()
	defer func() {
		cancel()
		<-p.Closed()
	}()
	testChannelTimeOutServer(t, v23.WithListenSpec(ctx, ls))
}
