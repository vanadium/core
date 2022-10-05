// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"crypto/rand"
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

func acceptor(errCh chan error, acceptCh chan flow.Flow, size int, close bool) {
	for flw := range acceptCh {
		buf := make([]byte, size)
		// It's essential to use ReadFull rather than ReadMsg since WriteMsg
		// will fragment a message larger than a default size into multiple
		// messages.
		n, err := io.ReadFull(flw, buf)
		if err != nil {
			errCh <- err
			return
		}
		if n != size {
			errCh <- fmt.Errorf("short read: %v != %v", n, size)
		}
		if got, want := n, size; got != want {
			errCh <- fmt.Errorf("got %v, want %v", got, want)
		}
		if _, err := flw.WriteMsg(buf); err != nil {
			errCh <- err
			return
		}

		if close {
			if err := flw.Close(); err != nil {
				errCh <- err
				return
			}
		}
	}
	errCh <- nil
}

func testCounters(t *testing.T, ctx *context.T, count int, dialClose, acceptClose bool, size int) (
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int) {
	return testCountersBytesBuffered(t, ctx, count, dialClose, acceptClose, size, DefaultBytesBufferedPerFlow())
}

func testCountersBytesBuffered(t *testing.T, ctx *context.T, count int, dialClose, acceptClose bool, size int, bytesBuffered uint64) (
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int) {

	acceptCh := make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConnsBytesBuffered(t, "local", "", ctx, ctx, nil, acceptCh, nil, nil, bytesBuffered)
	if derr != nil || aerr != nil {
		t.Fatalf("setup: dial err: %v, accept err: %v", derr, aerr)
	}

	errCh := make(chan error, 1)
	go acceptor(errCh, acceptCh, size, acceptClose)

	writeBuf := make([]byte, size)
	if n, err := io.ReadFull(rand.Reader, writeBuf); n != size || err != nil {
		t.Fatalf("failed to write random bytes: %v %v", n, err)
	}

	for i := 0; i < count; i++ {
		df, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		// WriteMsg wil fragment messages larger than its default buffer size.
		if _, err := df.WriteMsg(writeBuf); err != nil {
			t.Fatalf("could not write flow: %v", err)
		}
		readBuf := make([]byte, size)
		// It's essential to use ReadFull rather than ReadMsg since WriteMsg
		// will fragment a message larger than a default size into multiple
		// messages.
		n, err := io.ReadFull(df, readBuf)
		if err != nil {
			t.Fatalf("unexpected error reading from flow: %v", err)
		}
		if got, want := n, size; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
		if !bytes.Equal(writeBuf, readBuf) {
			t.Fatalf("data corruption: %v %v", writeBuf[:10], readBuf[:10])
		}
		if dialClose {
			if err := df.Close(); err != nil {
				t.Fatalf("unexpected error closing flow: %v", err)
			}
		}
	}

	close(acceptCh)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	dc.mu.Lock()
	dialRelease, dialBorrowed = len(dc.flowControl.toRelease), len(dc.flowControl.borrowing)
	dc.mu.Unlock()
	ac.mu.Lock()
	acceptRelease, acceptBorrowed = len(ac.flowControl.toRelease), len(ac.flowControl.borrowing)
	ac.mu.Unlock()
	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	return
}

func TestCounters(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	var dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int

	assert := func(dialApprox, acceptApprox int) {
		compare := func(got, want int) {
			if got > want {
				_, _, l1, _ := runtime.Caller(3)
				_, _, l2, _ := runtime.Caller(2)
				t.Errorf("line: %v:%v, got %v, want %v", l1, l2, got, want)
			}
		}
		compare(dialRelease, dialApprox)
		compare(dialBorrowed, dialApprox)
		compare(acceptRelease, acceptApprox)
		compare(acceptBorrowed, acceptApprox)
	}

	runAndTest := func(count, size, dialApprox, acceptApprox int) {
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCounters(t, ctx, count, true, false, size)
		assert(dialApprox, acceptApprox)
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCounters(t, ctx, count, false, true, size)
		assert(dialApprox, acceptApprox)
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCounters(t, ctx, count, true, true, size)
		assert(dialApprox, acceptApprox)
	}

	// The actual values should be 1 for the dial side but we allow a few more
	// than that to avoid racing for the network comms to complete after
	// the flows are closed. On the accept side, the number of currently in
	// use toRelease entries depends on the size of the data buffers used.
	// The number is determined by the number of flows that have outstanding
	// Release messages to send to the dialer once count iterations are complete.
	// Increasing size decreases the number of flows with outstanding Release
	// messages since the shared counter is burned through faster when using
	// a larger size.

	// For small packets, all connections end up being 'borrowed' and hence
	// their counters are kept around.
	runAndTest(500, 10, 3, 502)
	// 60K connection setups/teardowns will ensure that the release message
	// is fragmented.
	runAndTest(60000, 10, 3, 10000)
	// For larger packets, the connections end up using flow control
	// tokens and hence not using 'borrowed' tokens.
	runAndTest(100, 1024*100, 3, 5)
}

func TestCountersFragmentation(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testCountersBytesBuffered(t, ctx, 10000, true, true, 1, 4096)
	// This test just needs to complete since all we really care about is
	// avoiding flow control deadlock.
}
