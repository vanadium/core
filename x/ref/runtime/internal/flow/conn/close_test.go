// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

type conn struct {
	flow.Conn
	set *set
}

func (c *conn) Close() error {
	c.set.remove(c.Conn)
	return c.Conn.Close()
}

type set struct {
	mu    sync.Mutex
	conns map[flow.Conn]bool
}

func (s *set) add(c flow.Conn) flow.Conn {
	s.mu.Lock()
	s.conns[c] = true
	s.mu.Unlock()
	return &conn{c, s}
}

func (s *set) remove(c flow.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

func (s *set) closeAll() {
	s.mu.Lock()
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
}

func (s *set) open() int {
	s.mu.Lock()
	o := len(s.conns)
	s.mu.Unlock()
	return o
}

func TestRemoteDialerClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	s := set{conns: map[flow.Conn]bool{}}
	ctx = debug.WithFilter(ctx, s.add)
	d, a, derr, aerr := setupConns(t, "debug", "local/", ctx, ctx, nil, nil, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	d.Close(ctx, fmt.Errorf("closing randomly"))
	<-d.Closed()
	<-a.Closed()
	if s.open() != 0 {
		t.Errorf("The connections should be closed")
	}
}

func TestRemoteAcceptorClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	s := set{conns: map[flow.Conn]bool{}}
	ctx = debug.WithFilter(ctx, s.add)
	d, a, derr, aerr := setupConns(t, "debug", "local/", ctx, ctx, nil, nil, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	a.Close(ctx, fmt.Errorf("closing randomly"))
	<-a.Closed()
	<-d.Closed()
	if s.open() != 0 {
		t.Errorf("The connections should be closed")
	}
}

func TestUnderlyingConnectionClosed(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	s := set{conns: map[flow.Conn]bool{}}
	ctx = debug.WithFilter(ctx, s.add)
	d, a, derr, aerr := setupConns(t, "debug", "local/", ctx, ctx, nil, nil, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	s.closeAll()
	<-a.Closed()
	<-d.Closed()
}

func TestDialAfterConnClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	d, a, derr, aerr := setupConns(t, "local", "", ctx, ctx, nil, nil, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}

	d.Close(ctx, fmt.Errorf("closing randomly"))
	<-d.Closed()
	<-a.Closed()
	if _, err := d.Dial(ctx, d.LocalBlessings(), nil, naming.Endpoint{}, 0, false); err == nil {
		t.Errorf("nil error dialing on dialer")
	}
	if _, err := a.Dial(ctx, a.LocalBlessings(), nil, naming.Endpoint{}, 0, false); err == nil {
		t.Errorf("nil error dialing on acceptor")
	}
}

func TestReadWriteAfterConnClose(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	for _, dialerDials := range []bool{true, false} {
		df, flows, cl := setupFlow(t, "local", "", ctx, ctx, dialerDials)
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		af := <-flows
		if got, err := af.ReadMsg(); err != nil {
			t.Fatalf("read failed: %v", err)
		} else if !bytes.Equal(got, []byte("hello")) {
			t.Errorf("got %s want %s", string(got), "hello")
		}
		if _, err := df.WriteMsg([]byte("there")); err != nil {
			t.Fatalf("second write failed: %v", err)
		}
		df.(*flw).conn.Close(ctx, fmt.Errorf("closing randomly"))
		<-af.Conn().Closed()
		if got, err := af.ReadMsg(); err != nil {
			t.Fatalf("read failed: %v", err)
		} else if !bytes.Equal(got, []byte("there")) {
			t.Errorf("got %s want %s", string(got), "there")
		}
		if _, err := df.WriteMsg([]byte("fail")); err == nil {
			t.Errorf("nil error for write after close.")
		}
		if _, err := af.ReadMsg(); err == nil {
			t.Fatalf("nil error for read after close.")
		}
		cl()
	}
}

func TestFlowCancelOnWrite(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	accept := make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConns(t, "local", "", ctx, ctx, nil, accept, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	defer func() {
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
	}()
	dctx, cancel := context.WithCancel(ctx)
	df, err := dc.Dial(dctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			panic("could not write flow: " + err.Error())
		}
		for {
			if _, err := df.WriteMsg([]byte("hello")); err == io.EOF {
				break
			} else if err != nil {
				panic("unexpected error waiting for cancel: " + err.Error())
			}
		}
		close(done)
	}()
	af := <-accept
	cancel()
	<-done
	<-af.Closed()
}

func TestFlowCancelOnRead(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()
	accept := make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConns(t, "local", "", ctx, ctx, nil, accept, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	defer func() {
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
	}()
	dctx, cancel := context.WithCancel(ctx)
	df, err := dc.Dial(dctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		defer close(done)
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			errCh <- fmt.Errorf("could not write flow: %v", err)
			return
		}
		if _, err := df.ReadMsg(); err != io.EOF {
			errCh <- fmt.Errorf("unexpected error waiting for cancel: %v", err)
			return
		}
		errCh <- nil
	}()
	af := <-accept
	cancel()
	<-done
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
	<-af.Closed()
}

func testCounters(t *testing.T, ctx *context.T, count int, dialClose, acceptClose bool) (
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int) {
	accept := make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConns(t, "local", "", ctx, ctx, nil, accept, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatalf("setup: dial err: %v, accept err: %v", derr, aerr)
	}

	errCh := make(chan error, 1)
	go func() {
		for flw := range accept {
			m, err := flw.ReadMsg()
			if err != nil {
				errCh <- err
				return
			}
			if _, err := flw.WriteMsg(m); err != nil {
				errCh <- err
				return
			}
			if acceptClose {
				if err := flw.Close(); err != nil {
					errCh <- err
					return
				}
			}
		}
		errCh <- nil
	}()

	for i := 0; i < count; i++ {
		df, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := df.WriteMsg([]byte("hello")); err != nil {
			t.Fatalf("could not write flow: %v", err)
		}
		if _, err := df.ReadMsg(); err != nil {
			t.Fatalf("unexpected error reading from flow: %v", err)
		}
		if dialClose {
			if err := df.Close(); err != nil {
				t.Fatalf("unexpected error closing flow: %v", err)
			}
		}
	}

	close(accept)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	dc.mu.Lock()
	dialRelease, dialBorrowed = len(dc.toRelease), len(dc.borrowing)
	dc.mu.Unlock()
	ac.mu.Lock()
	acceptRelease, acceptBorrowed = len(ac.toRelease), len(ac.borrowing)
	ac.mu.Unlock()
	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	return
}

func TestCounters(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	check := func(got, want int) {
		if got > want {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line: %v, got %v, want %v", line, got, want)
		}
	}
	count := 1000
	// The actual values should be 1 for the dial side and 2 for the accept side, but
	// we allow a few more than that to avoid racing for the network comms to complete after
	// the flows are closed.
	approx := 3
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed := testCounters(t, ctx, count, true, true)
	check(dialRelease, approx)
	check(dialBorrowed, approx)
	check(acceptRelease, approx)
	check(acceptBorrowed, approx)
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCounters(t, ctx, count, true, false)
	check(dialRelease, approx)
	check(dialBorrowed, approx)
	check(acceptRelease, approx)
	check(acceptBorrowed, approx)
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCounters(t, ctx, count, false, true)
	check(dialRelease, approx)
	check(dialBorrowed, approx)
	check(acceptRelease, approx)
	check(acceptBorrowed, approx)
}
