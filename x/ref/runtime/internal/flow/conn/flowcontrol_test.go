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
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

func waitFor(delay time.Duration, f func() error) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(delay)
	for range ticker.C {
		err := f()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
	}
	return fmt.Errorf("unreachable")
}

func block(t *testing.T, ctx *context.T, c *Conn, p int) chan struct{} {
	w := writer{notify: make(chan struct{}, 1)}
	ready, unblock := make(chan struct{}), make(chan struct{})
	go func() {
		if err := waitForWriters(ctx, c, 0); err != nil {
			t.Logf("waitForWriters: %v", err)
		}
		c.writeq.wait(nil, &w, p)
		close(ready)
		<-unblock
		c.writeq.done(&w)
	}()
	<-ready
	return unblock
}

func waitForWriters(ctx *context.T, conn *Conn, num int) error {
	return waitFor(time.Minute, func() error {
		conn.writeq.mu.Lock()
		count := 0
		if conn.writeq.active != nil {
			count++
		}
		for _, w := range conn.writeq.activeWriters {
			if w != nil {
				count++
				for n := w.next; n != w; n = n.next {
					count++
				}
			}
		}
		conn.writeq.mu.Unlock()
		if count >= num {
			return nil
		}
		return fmt.Errorf("#writers %v < %v", count, num)
	})
}

type readConn struct {
	flow.Conn
	ch  chan message.Message
	ctx *context.T
}

func (r *readConn) ReadMsg() ([]byte, error) {
	b, err := r.Conn.ReadMsg()
	if len(b) > 0 {
		m, _ := message.Read(r.ctx, b)
		switch msg := m.(type) {
		case message.OpenFlow:
			if msg.ID > 1 { // Ignore the blessings flow.
				r.ch <- m
			}
		case message.Data:
			if msg.ID > 1 { // Ignore the blessings flow.
				r.ch <- m
			}
		}
	}
	return b, err
}

// must also implement ReadMsg2 since it's now required (and used internally).
func (r *readConn) ReadMsg2([]byte) ([]byte, error) {
	return r.ReadMsg()
}

func TestFlowMessageOrdering(t *testing.T) {
	// We will send nmessages*mtu bytes on each flow.
	// For the test to work properly we should send < defaultBufferSize(2^20) bytes.
	// If we don't then it's possible for the teardown message to be sent before some
	// flows finish writing because the flows' writes will become blocked from
	// counter exhaustion.
	// 4*4*2^16 == 2^20 so it's the perfect number.
	const nflows = 2
	const nmessages = 4

	ctx, shutdown := test.V23Init()
	defer shutdown()

	ch := make(chan message.Message, 100)
	fctx := debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		return &readConn{c, ch, ctx}
	})
	flows, accept, dc, ac := setupFlows(t, "debug", "local/", ctx, fctx, true, nflows)

	var wg sync.WaitGroup
	wg.Add(2 * nflows)
	errCh := make(chan error, 2*nflows)
	unblock := block(t, ctx, dc, flowPriority)

	for _, f := range flows {
		go func(fl flow.Flow) {
			defer wg.Done()
			if _, err := fl.WriteMsg(randData[:DefaultMTU*nmessages]); err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}(f)
		go func() {
			defer wg.Done()
			fl := <-accept
			buf := make([]byte, DefaultMTU*nmessages)
			if _, err := io.ReadFull(fl, buf); err != nil {
				errCh <- err
				return
			}
			if !bytes.Equal(buf, randData[:DefaultMTU*nmessages]) {
				errCh <- fmt.Errorf("unequal data")
				return
			}
			errCh <- nil
		}()
	}

	waitForWriters(ctx, dc, nflows+1)
	close(unblock)
	wg.Wait()

	// Now close the conn which will send a teardown message, but only after
	// the other flows finish their current write.
	go dc.Close(ctx, nil)
	defer func() { <-dc.Closed(); <-ac.Closed() }()

	// OK now we expect all the flows to write interleaved messages.
	for i := 0; i < nmessages; i++ {
		found := map[uint64]bool{}
		for j := 0; j < nflows; j++ {
			m := <-ch
			switch msg := m.(type) {
			case message.OpenFlow:
				found[msg.ID] = true
			case message.Data:
				found[msg.ID] = true
			case message.TearDown:
			}
		}
		if len(found) != nflows {
			t.Fatalf("Did not receive a message from each flow in round %d: %v", i, found)
		}
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestFlowControl(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	for _, nflows := range []int{1, 2, 20, 100} {
		for _, bytesBuffered := range []uint64{DefaultMTU, DefaultBytesBuffered} {
			for _, mtu := range []uint64{1024, DefaultMTU} {
				t.Logf("starting: #%v flows, buffered %#v bytes\n", nflows, bytesBuffered)
				dfs, flows, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, nflows, Opts{
					MTU:           mtu,
					BytesBuffered: bytesBuffered})

				defer func() {
					dc.Close(ctx, nil)
					ac.Close(ctx, nil)
					<-dc.Closed()
					<-ac.Closed()
				}()

				errs := make(chan error, nflows*4*2)
				var wg sync.WaitGroup
				wg.Add(nflows * 4)

				for i := 0; i < nflows; i++ {
					go func(i int) {
						err := doWrite(dfs[i], randData)
						if err != nil {
							fmt.Printf("dial: doWrite: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- flowControlBorrowedClosedInvariantBoth(dc, ac)
						wg.Done()
					}(i)
					go func(i int) {
						err := doRead(dfs[i], randData, nil)
						if err != nil {
							fmt.Printf("dial: doRead: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- flowControlBorrowedClosedInvariantBoth(dc, ac)
						wg.Done()
					}(i)
				}
				for i := 0; i < nflows; i++ {
					af := <-flows
					go func(i int) {
						err := doRead(af, randData, nil)
						if err != nil {
							fmt.Printf("accept: doRead: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- flowControlBorrowedClosedInvariantBoth(dc, ac)
						wg.Done()
					}(i)
					go func(i int) {
						err := doWrite(af, randData)
						if err != nil {
							fmt.Printf("accept: doWrite: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- flowControlBorrowedClosedInvariantBoth(dc, ac)
						wg.Done()
					}(i)
				}

				wg.Wait()
				for i := 0; i < nflows; i++ {
					if err := dfs[i].Close(); err != nil {
						t.Error(err)
					}
				}
				close(errs)

				if err := flowControlBorrowedClosedInvariantBoth(dc, ac); err != nil {
					t.Error(err)
				}

				for err := range errs {
					if err != nil {
						t.Error(err)
					}
				}
				t.Logf("done: #%v flows, buffered %#v bytes\n", nflows, bytesBuffered)
			}
		}
	}
}

func readFromFlows(accept <-chan flow.Flow, sent, need int) (int, error) {
	trx := 0
	for {
		af := <-accept
		rx := 0
		for {
			buf, err := af.ReadMsg()
			if err != nil {
				if err != io.EOF {
					return rx, err
				}
				return trx, nil
			}
			rx += len(buf)
			if rx >= sent {
				break
			}
		}
		trx += rx
		if trx >= need {
			return trx, nil
		}
	}
}

func TestFlowControlBorrowing(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	borrowedInvariant := func(c *Conn) (totalBorrowed, shared uint64) {
		totalBorrowed, shared, err := flowControlBorrowedClosedInvariant(c)
		if err != nil {
			t.Fatal(err)
		}
		return
	}

	nflows := 100
	bytesBuffered := uint64(10000)
	mtu := uint64(1024)

	for _, numFlowsBeforeStarvation := range []int{2, 3, 7} {
		flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, nflows, Opts{
			MTU:           mtu,
			BytesBuffered: bytesBuffered})

		perFlowBufSize := (bytesBuffered / uint64(numFlowsBeforeStarvation)) //nolint:gosec // disable G115
		written := 0
		read := 0
		for i := 0; i < nflows; i++ {
			f := flows[i]

			_, err := f.WriteMsg(randData[:perFlowBufSize])
			if err != nil {
				t.Fatal(err)
			}
			written += int(perFlowBufSize) //nolint:gosec // disable G115

			totalBorrowed, shared := borrowedInvariant(dc)

			borrowed := uint64((i%numFlowsBeforeStarvation)+1) * perFlowBufSize //nolint:gosec // disable G115

			flowControlReleasedInvariantBidirectional(dc, ac)

			if got, want := totalBorrowed, borrowed; got != want {
				t.Errorf("%v: %v: got %v, want %v", i, numFlowsBeforeStarvation, got, want)
			}

			if got, want := flowControlBorrowed(dc, flowID(f)), perFlowBufSize; got != want {
				t.Errorf("%v: %v: got %v, want %v", i, numFlowsBeforeStarvation, got, want)
			}

			if shared < mtu {
				// At this point the available borrowed/shared tokens are exhausted.
				// The following loop will read enough data for the accept side
				// to send a release message to the dialer to free up some
				// borrowed tokens for more flows to be opened.
				need := written - read
				n, err := readFromFlows(accept, int(perFlowBufSize), need) //nolint:gosec // disable G115
				if err != nil {
					t.Fatal(err)
				}
				read += n
				totalBorrowed, _ = borrowedInvariant(dc)
				if got, want := int(totalBorrowed), (numFlowsBeforeStarvation * int(perFlowBufSize)); got != 0 && got != want { //nolint:gosec // disable G115
					t.Errorf("%v: %v: got %v, want either 0 or %v", i, numFlowsBeforeStarvation, got, want)
				}

				flowControlReleasedInvariantBidirectional(dc, ac)

			}

			totalBorrowed, _ = borrowedInvariant(ac)
			if got, want := int(totalBorrowed), 0; got != want { //nolint:gosec // disable G115
				t.Errorf("%v: %v: got %v, want either 0 or %v", i, numFlowsBeforeStarvation, got, want)
			}

			flowControlReleasedInvariantBidirectional(dc, ac)

		}
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
		<-dc.Closed()
		<-ac.Closed()
	}

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

func dialWriteRead(t *testing.T, ctx *context.T, dc *Conn, writeBuf []byte) (flow.Flow, error) {
	df, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	// WriteMsg wil fragment messages larger than its default buffer size.
	if _, err := df.WriteMsg(writeBuf); err != nil {
		return nil, fmt.Errorf("could not write flow: %v", err)
	}
	readBuf := make([]byte, len(writeBuf))
	// It's essential to use ReadFull rather than ReadMsg since WriteMsg
	// will fragment a message larger than a default size into multiple
	// messages.
	n, err := io.ReadFull(df, readBuf)
	if err != nil {
		return nil, fmt.Errorf("unexpected error reading from flow: %v", err)
	}
	if got, want := n, len(writeBuf); got != want {
		return nil, fmt.Errorf("got %v, want %v", got, want)
	}
	if !bytes.Equal(writeBuf, readBuf) {
		return nil, fmt.Errorf("data corruption: %v %v", writeBuf[:10], readBuf[:10])
	}
	return df, nil
}

func testCountersOpts(t *testing.T, ctx *context.T, count int, dialClose, acceptClose bool, size int, opts Opts) (
	dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int) {

	_, _, line1, _ := runtime.Caller(1)
	_, _, line2, _ := runtime.Caller(2)

	t.Logf("runAndTest: lines: %v:%v, count: %v, size: %v, dial close: %v, accept close: %v", line2, line1, count, size, dialClose, acceptClose)

	acceptCh := make(chan flow.Flow, 1)
	dc, ac, derr, aerr := setupConnsOpts(t, "local", "", ctx, ctx, nil, acceptCh, nil, nil, opts)
	if derr != nil || aerr != nil {
		t.Fatalf("lines: %v:%v: setup: dial err: %v, accept err: %v", line2, line1, derr, aerr)
	}

	errCh := make(chan error, 1)
	go acceptor(errCh, acceptCh, size, acceptClose)

	writeBuf := make([]byte, size)
	if n, err := io.ReadFull(rand.Reader, writeBuf); n != size || err != nil {
		t.Fatalf("lines: %v:%v: failed to write random bytes: %v %v", line2, line1, n, err)
	}

	for i := 0; i < count; i++ {
		df, err := dialWriteRead(t, ctx, dc, writeBuf)
		if err != nil {
			t.Fatalf("lines: %v:%v: %v", line2, line1, err)
		}
		if dialClose {
			if err := df.Close(); err != nil {
				t.Fatalf("lines: %v:%v: unexpected error closing flow: %v", line2, line1, err)
			}
		}
		if err := flowControlBorrowedInvariantBoth(dc, ac); err != nil {
			t.Fatalf("lines: %v:%v: %v", line2, line1, err)
		}
		if err := flowControlReleasedInvariantBidirectional(dc, ac); err != nil {
			t.Fatalf("lines: %v:%v: flowControlReleasedInvariant: %v", line2, line1, err)
		}
	}

	close(acceptCh)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	err := waitFor(time.Minute, func() error {
		_, _, err := flowControlBorrowedClosedInvariant(dc)
		return err
	})
	if err != nil {
		t.Errorf("dial: %v", err)
	}

	err = waitFor(time.Minute, func() error {
		_, _, err := flowControlBorrowedClosedInvariant(ac)
		return err
	})
	if err != nil {
		t.Errorf("dial: %v", err)
	}

	dc.mu.Lock()
	dc.flowControl.mu.Lock()
	dialRelease = countToRelease(dc)
	dialBorrowed = countRemoteBorrowing(dc)
	dc.flowControl.mu.Unlock()
	dc.mu.Unlock()

	ac.mu.Lock()
	ac.flowControl.mu.Lock()
	acceptRelease = countToRelease(ac)
	acceptBorrowed = countRemoteBorrowing(ac)
	ac.flowControl.mu.Unlock()
	ac.mu.Unlock()
	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	return
}

func TestFlowControlCounters(t *testing.T) {
	defer goroutines.NoLeaks(t, leakWaitTime)()

	ctx, shutdown := test.V23Init()
	defer shutdown()

	var dialRelease, dialBorrowed, acceptRelease, acceptBorrowed int

	assert := func(dialApprox, acceptApprox int) {
		compare := func(msg string, got, want int) {
			if got > want {
				_, _, l1, _ := runtime.Caller(3)
				_, _, l2, _ := runtime.Caller(2)
				t.Errorf("line: %v:%v:%v: got %v, want %v", l1, l2, msg, got, want)
			}
		}
		compare("dialRelease", dialRelease, dialApprox)
		compare("dialBorrowed", dialBorrowed, dialApprox)
		compare("acceptRelease", acceptRelease, acceptApprox)
		compare("acceptBorrowed", acceptBorrowed, acceptApprox)
	}

	runAndTest := func(count, size, dialApprox, acceptApprox int, opts Opts) {
		t.Logf("runAndTest: count: %v, size: %v, dialApprox: %v, acceptApprox: %v", count, size, dialApprox, acceptApprox)
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCountersOpts(t, ctx, count, true, false, size, opts)
		assert(dialApprox, acceptApprox)
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCountersOpts(t, ctx, count, false, true, size, opts)
		assert(dialApprox, acceptApprox)
		dialRelease, dialBorrowed, acceptRelease, acceptBorrowed = testCountersOpts(t, ctx, count, true, true, size, opts)
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
	runAndTest(500, 10, 3, 502, Opts{})
	// Smaller MTU, BytesBuffered and small messages ensure that the
	// release message will need to be fragmented.
	runAndTest(5000, 1, 3, 1000, Opts{BytesBuffered: 4096, MTU: 2048})
	// For larger packets, the connections end up using flow control
	// tokens and hence not using 'borrowed' tokens.
	runAndTest(100, 1024*100, 3, 5, Opts{})
}
