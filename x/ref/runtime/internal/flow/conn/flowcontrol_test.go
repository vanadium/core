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

	for _, nflows := range []int{1, 2, 40, 200} {
		for _, bytesBuffered := range []uint64{DefaultMTU, DefaultBytesBuffered} {
			t.Logf("starting: #%v flows, buffered %#v bytes\n", nflows, bytesBuffered)
			dfs, flows, ac, dc := setupFlowsOpts(t, "local", "", ctx, ctx, true, nflows, Opts{BytesBuffered: bytesBuffered})

			defer func() {
				dc.Close(ctx, nil)
				ac.Close(ctx, nil)
			}()

			errs := make(chan error, nflows*4)
			var wg sync.WaitGroup
			wg.Add(nflows * 4)

			for i := 0; i < nflows; i++ {
				go func(i int) {
					err := doWrite(dfs[i], randData)
					if err != nil {
						fmt.Printf("unexpected error: %v\n", err)
					}
					errs <- err
					wg.Done()
				}(i)
				go func(i int) {
					err := doRead(dfs[i], randData, nil)
					if err != nil {
						fmt.Printf("unexpected error: %v\n", err)
					}
					errs <- err
					wg.Done()
				}(i)
			}
			for i := 0; i < nflows; i++ {
				af := <-flows
				go func() {
					err := doRead(af, randData, nil)
					if err != nil {
						fmt.Printf("unexpected error: %v\n", err)
					}
					errs <- err
					wg.Done()
				}()
				go func() {
					err := doWrite(af, randData)
					if err != nil {
						fmt.Printf("unexpected error: %v\n", err)
					}
					errs <- err
					wg.Done()
				}()
			}

			wg.Wait()
			close(errs)

			for err := range errs {
				if err != nil {
					t.Error(err)
				}
			}
			t.Logf("done: #%v flows, buffered %#v bytes\n", nflows, bytesBuffered)
		}
	}
}
