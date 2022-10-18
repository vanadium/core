// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/test"
	"v.io/x/ref/test/goroutines"
)

func waitFor(f func() bool) {
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		if f() {
			return
		}
	}
}

func block(ctx *context.T, c *Conn, p int) chan struct{} {
	w := writer{notify: make(chan struct{}, 1)}
	ready, unblock := make(chan struct{}), make(chan struct{})
	go func() {
		waitForWriters(ctx, c, 0)
		c.writeq.wait(nil, &w, p)
		close(ready)
		<-unblock
		c.writeq.done(&w)
	}()
	<-ready
	return unblock
}

func waitForWriters(ctx *context.T, conn *Conn, num int) {
	waitFor(func() bool {
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
		return count >= num
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
		case *message.OpenFlow:
			if msg.ID > 1 { // Ignore the blessings flow.
				r.ch <- m
			}
		case *message.Data:
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

func TestOrdering(t *testing.T) {
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
	unblock := block(ctx, dc, flowPriority)

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
			case *message.OpenFlow:
				found[msg.ID] = true
			case *message.Data:
				found[msg.ID] = true
			case *message.TearDown:
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

				testInvariants := func() error {
					if _, _, err := flowControlBorrowedInvariant(dc); err != nil {
						return fmt.Errorf("dial: flowcontrol invariant: %v", err)
					}
					if _, _, err := flowControlBorrowedInvariant(ac); err != nil {
						return fmt.Errorf("accept: flowcontrol invariant: %v", err)
					}
					return nil
				}

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
						errs <- testInvariants()
						wg.Done()
					}(i)
					go func(i int) {
						err := doRead(dfs[i], randData, nil)
						if err != nil {
							fmt.Printf("dial: doRead: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- testInvariants()
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
						errs <- testInvariants()
						wg.Done()
					}(i)
					go func(i int) {
						err := doWrite(af, randData)
						if err != nil {
							fmt.Printf("accept: doWrite: flow: %v/%v, mtu: %v, buffered: %v, unexpected error: %v\n", i, nflows, mtu, bytesBuffered, err)
						}
						errs <- err
						errs <- testInvariants()
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

				if err := testInvariants(); err != nil {
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
			if rx >= int(sent) {
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
		totalBorrowed, shared, err := flowControlBorrowedInvariant(c)
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	releasedInvariant := func(a, b *Conn) {
		if err := flowControlReleasedInvariant(a, b); err != nil {
			t.Fatal(err)
		}
		if err := flowControlReleasedInvariant(b, a); err != nil {
			t.Fatal(err)
		}
	}

	nflows := 100
	bytesBuffered := uint64(10000)
	mtu := uint64(1024)

	for _, numFlowsBeforeStarvation := range []int{2, 3, 7} {
		flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, nflows, Opts{
			MTU:           mtu,
			BytesBuffered: bytesBuffered})

		perFlowBufSize := (bytesBuffered / uint64(numFlowsBeforeStarvation))
		written := 0
		read := 0
		for i := 0; i < nflows; i++ {
			f := flows[i]

			_, err := f.WriteMsg(randData[:perFlowBufSize])
			if err != nil {
				t.Fatal(err)
			}
			written += int(perFlowBufSize)

			totalBorrowed, shared := borrowedInvariant(dc)

			borrowed := uint64((i%numFlowsBeforeStarvation)+1) * perFlowBufSize

			releasedInvariant(dc, ac)

			if got, want := totalBorrowed, borrowed; got != want {
				t.Errorf("%v: %v: got %v, want %v", i, numFlowsBeforeStarvation, got, want)
			}

			if got, want := flowControlBorrowed(dc)[flowID(f)], perFlowBufSize; got != want {
				t.Errorf("%v: %v: got %v, want %v", i, numFlowsBeforeStarvation, got, want)
			}

			if shared < mtu {
				// At this point the available borrowed/shared tokens are exhausted.
				// The following loop will read enough data for the accept side
				// to send a release message to the dialer to free up some
				// borrowed tokens for more flows to be opened.
				need := written - read
				n, err := readFromFlows(accept, int(perFlowBufSize), need)
				if err != nil {
					t.Fatal(err)
				}
				read += n
				totalBorrowed, _ = borrowedInvariant(dc)
				if got, want := int(totalBorrowed), (numFlowsBeforeStarvation * int(perFlowBufSize)); got != 0 && got != want {
					t.Errorf("%v: %v: got %v, want either 0 or %v", i, numFlowsBeforeStarvation, got, want)
				}

				releasedInvariant(dc, ac)

			}

			totalBorrowed, _ = borrowedInvariant(ac)
			if got, want := int(totalBorrowed), 0; got != want {
				t.Errorf("%v: %v: got %v, want either 0 or %v", i, numFlowsBeforeStarvation, got, want)
			}

			releasedInvariant(dc, ac)

		}
		dc.Close(ctx, nil)
		ac.Close(ctx, nil)
		<-dc.Closed()
		<-ac.Closed()
	}

}
