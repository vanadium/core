// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/x/ref/test"
)

func flowEcho(wg *sync.WaitGroup, f flow.Flow, msgCh chan<- []byte, errCh chan<- error) {
	defer wg.Done()
	for {
		m, err := f.ReadMsg()
		if err != nil {
			if err != io.EOF {
				errCh <- err
			}
			return
		}
		msgCh <- m
	}
}

func flowWrite(f flow.Flow, errCh chan<- error, msgs ...string) {
	for i, m := range msgs {
		n, err := f.Write([]byte(m))
		if err != nil {
			errCh <- err
			return
		}
		if got, want := n, len(m); got != want {
			errCh <- fmt.Errorf("%v: got %v, want %v (%s)", i, got, want, m)
		}
	}
}

func flowWriteThenClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	flowWrite(f, errCh, msgs...)
	f.Close()
}

func flowWriteAndClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	end := len(msgs) - 1
	flowWrite(f, errCh, msgs[:end]...)
	f.WriteMsgAndClose([]byte(msgs[end]))
}

func flowWriteFlushClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	for _, msg := range msgs {
		flowWrite(f, errCh, msg)
		f.(*BufferingFlow).Flush()
	}
	f.Close()
}

func flowWriteMsgAndClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	data := make([][]byte, len(msgs))
	for i := range msgs {
		data[i] = []byte(msgs[i])
	}
	f.WriteMsgAndClose(data...)
}

func expectedMessages(t *testing.T, msgCh <-chan []byte, expected ...string) {
	_, _, line, _ := runtime.Caller(1)
	i := 0
	for msg := range msgCh {
		if i >= len(expected) {
			t.Fatalf("line %v: %v: got unexpected msg: %v", line, i, string(msg))
		}
		if got, want := string(msg), expected[i]; got != want {
			t.Fatalf("line %v: %v: got %v, want %v", line, i, got, want)
		}
		i++
	}
	if got, want := i, len(expected); got != want {
		t.Fatalf("line %v: %v: got %v, want %v", line, i, got, want)
	}
}

func assertNoErrors(t *testing.T, depth int, errCh <-chan error) {
	_, _, line, _ := runtime.Caller(depth + 1)
	for err := range errCh {
		if err != nil {
			t.Fatalf("line: %v, err: %v", line, err)
		}
	}
}

func bufferingFlowTestRunner(t *testing.T, ctx *context.T, f flow.Flow, accept <-chan flow.Flow, writer func(flow.Flow, chan<- error, ...string), msgs ...string) <-chan []byte {
	errCh := make(chan error, 2)
	msgCh := make(chan []byte, 1000)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		writer(NewBufferingFlow(ctx, f), errCh, msgs...)
		wg.Done()
	}()
	go flowEcho(&wg, <-accept, msgCh, errCh)

	wg.Wait()
	close(errCh)
	assertNoErrors(t, 1, errCh)

	close(msgCh)
	return msgCh
}

func TestBufferingFlow(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, 6, Opts{MTU: 16})

	// Series of writes followed by a close.
	msgCh := bufferingFlowTestRunner(t, ctx, flows[0], accept, flowWriteThenClose,
		"hello", " there", " world", " some more that ...should cause a flush")
	expectedMessages(t, msgCh, "hello there", " world", " some more that ", "...should cause ", "a flush")

	msgCh = bufferingFlowTestRunner(t, ctx, flows[1], accept, flowWriteThenClose,
		strings.Repeat("ab", 10), strings.Repeat("cd", 10))
	expectedMessages(t, msgCh, strings.Repeat("ab", 8), strings.Repeat("ab", 2),
		strings.Repeat("cd", 8), strings.Repeat("cd", 2))

	// Series of writes+flush followed by a close.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[2], accept, flowWriteFlushClose,
		strings.Repeat("ab", 10), strings.Repeat("cd", 10))
	expectedMessages(t, msgCh, strings.Repeat("ab", 8), strings.Repeat("ab", 2),
		strings.Repeat("cd", 8), strings.Repeat("cd", 2))

	msgCh = bufferingFlowTestRunner(t, ctx, flows[3], accept, flowWriteFlushClose,
		strings.Repeat("ab", 6), strings.Repeat("cd", 6))
	expectedMessages(t, msgCh, strings.Repeat("ab", 6), strings.Repeat("cd", 6))

	// A series of writes followed by a writeAndClose.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[4], accept, flowWriteAndClose,
		strings.Repeat("ab", 6), strings.Repeat("cd", 6), strings.Repeat("ef", 6))
	expectedMessages(t, msgCh, strings.Repeat("ab", 6), strings.Repeat("cd", 6), strings.Repeat("ef", 6))

	// A single call to WriteMsgAndClose.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[5], accept, flowWriteMsgAndClose,
		strings.Repeat("ab", 6), strings.Repeat("cd", 6), strings.Repeat("ef", 6))
	expectedMessages(t, msgCh, strings.Repeat("ab", 6)+strings.Repeat("cd", 2),
		strings.Repeat("cd", 4)+strings.Repeat("ef", 4), strings.Repeat("ef", 2))

	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	<-ac.Closed()
	<-dc.Closed()
}
