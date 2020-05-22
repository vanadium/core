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
)

func block(c *Conn, p int) chan struct{} {
	w := &singleMessageWriter{writeCh: make(chan struct{}), p: p}
	w.next, w.prev = w, w
	ready, unblock := make(chan struct{}), make(chan struct{})
	c.mu.Lock()
	c.activateWriterLocked(w)
	c.notifyNextWriterLocked(w)
	c.mu.Unlock()
	go func() {
		<-w.writeCh
		close(ready)
		<-unblock
		c.mu.Lock()
		c.deactivateWriterLocked(w)
		c.notifyNextWriterLocked(w)
		c.mu.Unlock()
	}()
	<-ready
	return unblock
}

func waitFor(f func() bool) {
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		if f() {
			return
		}
	}
}

func waitForWriters(ctx *context.T, conn *Conn, num int) {
	waitFor(func() bool {
		conn.mu.Lock()
		count := 0
		for _, w := range conn.activeWriters {
			if w != nil {
				count++
				for _, n := w.neighbors(); n != w; _, n = n.neighbors() {
					count++
				}
			}
		}
		conn.mu.Unlock()
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

func TestOrdering(t *testing.T) {
	// We will send nmessages*mtu bytes on each flow.
	// For the test to work properly we should send < defaultBufferSize(2^20) bytes.
	// If we don't then it's possible for the teardown message to be sent before some
	// flows finish writing because the flows' writes will become blocked from
	// counter exhaustion.
	// 4*4*2^16 == 2^20 so it's the perfect number.
	const nflows = 4
	const nmessages = 4

	ctx, shutdown := test.V23Init()
	defer shutdown()

	ch := make(chan message.Message, 100)
	fctx := debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		return &readConn{c, ch, ctx}
	})
	flows, accept, dc, ac := setupFlows(t, "debug", "local/", ctx, fctx, true, nflows)

	unblock := block(dc, 0)
	var wg sync.WaitGroup
	wg.Add(2 * nflows)
	errCh := make(chan error, len(flows)*2)
	defer func() {
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				t.Fatal(err)
			}
		}
	}()
	for _, f := range flows {
		go func(fl flow.Flow) {
			defer wg.Done()
			if _, err := fl.WriteMsg(randData[:defaultMtu*nmessages]); err != nil {
				errCh <- err
				return
			}
			errCh <- nil
		}(f)
		go func() {
			defer wg.Done()
			fl := <-accept
			buf := make([]byte, defaultMtu*nmessages)
			if _, err := io.ReadFull(fl, buf); err != nil {
				errCh <- err
				return
			}
			if !bytes.Equal(buf, randData[:defaultMtu*nmessages]) {
				errCh <- fmt.Errorf("unequal data")
				return
			}
			errCh <- nil
		}()
	}
	waitForWriters(ctx, dc, nflows+1)
	// Now close the conn which will send a teardown message, but only after
	// the other flows finish their current write.
	go dc.Close(ctx, nil)
	defer func() { <-dc.Closed(); <-ac.Closed() }()

	close(unblock)

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
			}
		}
		if len(found) != nflows {
			t.Fatalf("Did not receive a message from each flow in round %d: %v", i, found)
		}
	}
}
