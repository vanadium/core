// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
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

// flowEcho echos all messages received on a flow.
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

// flowWrite uses the Write method to write msgs to a flow. It does not
// call Close.
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

func asByteSlice(msgs []string) [][]byte {
	data := make([][]byte, len(msgs))
	for i := range msgs {
		data[i] = []byte(msgs[i])
	}
	return data
}

// flowWriteMsgThenClose uses the WriteMsg method to write msgs to a flow.
// It then calls Close.
func flowWriteMsgThenClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	bs := asByteSlice(msgs)
	s := totalSize(bs)
	n, err := f.WriteMsg(bs...)
	if err != nil {
		errCh <- err
		return
	}
	if got, want := n, s; got != want {
		errCh <- fmt.Errorf("got %v, want %v", got, want)
	}
	f.Close()
}

// flowWriteThenClose uses flowWrite and then calls Close.
func flowWriteThenClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	flowWrite(f, errCh, msgs...)
	f.Close()
}

// flowWriteAndClose uses flowWrite and the calls WriteMsgAndClose with
// the final msg.
func flowWriteAndClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	end := len(msgs) - 1
	flowWrite(f, errCh, msgs[:end]...)
	f.WriteMsgAndClose([]byte(msgs[end]))
}

// flowWriteFlushClose uses flowWrite for each msg followed by a Flush
// and finally Close.
func flowWriteFlushClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	for _, msg := range msgs {
		flowWrite(f, errCh, msg)
		f.(*BufferingFlow).Flush()
	}
	f.Close()
}

// flowWriteMsgAndClose uses the WriteMsgAndClose method to write all
// msgs in one operation.
func flowWriteMsgAndClose(f flow.Flow, errCh chan<- error, msgs ...string) {
	f.WriteMsgAndClose(asByteSlice(msgs)...)
}

func expectedMessages(t *testing.T, msgCh <-chan []byte, expected ...string) {
	_, _, line, _ := runtime.Caller(1)
	i := 0
	for msg := range msgCh {
		if i >= len(expected) {
			t.Fatalf("line %v: %v: got unexpected msg: %v", line, i, string(msg))
		}
		if got, want := string(msg), expected[i]; got != want {
			t.Fatalf("line %v: %v: got (%v):%v, want (%v):%v", line, i, len(got), got, len(want), want)
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

func assertBufferReset(depth int, bf *BufferingFlow) error {
	_, _, line, _ := runtime.Caller(depth + 1)
	if bf.buf != nil {
		return fmt.Errorf("line: %v, buffer not reset", line)
	}
	if bf.nBuf != nil {
		return fmt.Errorf("line: %v, netBuf not reset", line)
	}
	return nil
}

func bufferingFlowTestRunner(t *testing.T, ctx *context.T, f flow.Flow, accept <-chan flow.Flow, writer func(flow.Flow, chan<- error, ...string), msgs ...string) <-chan []byte {
	errCh := make(chan error, 4)
	msgCh := make(chan []byte, 1000)

	var wg sync.WaitGroup
	wg.Add(2)

	bf := NewBufferingFlow(ctx, f)
	go func() {
		writer(bf, errCh, msgs...)
		wg.Done()
	}()
	go flowEcho(&wg, <-accept, msgCh, errCh)

	wg.Wait()
	close(errCh)
	assertNoErrors(t, 1, errCh)
	if err := assertBufferReset(1, bf); err != nil {
		t.Fatal(err)
	}
	close(msgCh)
	return msgCh
}

func TestBufferingFlow(t *testing.T) {
	netbufsFreed(t)
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, 8, Opts{MTU: 16})

	// Series of writes followed by a close.
	msgCh := bufferingFlowTestRunner(t, ctx, flows[0], accept, flowWriteThenClose,
		"hello",
		" there",
		" world",
		" some more that ...should cause a flush")
	expectedMessages(t, msgCh,
		"hello there",
		" world",
		" some more that ",
		"...should cause ",
		"a flush")

	msgCh = bufferingFlowTestRunner(t, ctx, flows[1], accept, flowWriteThenClose,
		strings.Repeat("ab", 10),
		strings.Repeat("cd", 10))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 8),
		strings.Repeat("ab", 2),
		strings.Repeat("cd", 8),
		strings.Repeat("cd", 2))

	// Series of writes+flush followed by a close.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[2], accept, flowWriteFlushClose,
		strings.Repeat("ab", 10),
		strings.Repeat("cd", 10))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 8),
		strings.Repeat("ab", 2),
		strings.Repeat("cd", 8),
		strings.Repeat("cd", 2))

	msgCh = bufferingFlowTestRunner(t, ctx, flows[3], accept, flowWriteFlushClose,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6))

	// A series of writes followed by a writeAndClose.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[4], accept, flowWriteAndClose,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6),
		strings.Repeat("ef", 6))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6),
		strings.Repeat("ef", 6))

	// A single call to WriteMsgAndClose.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[5], accept, flowWriteMsgAndClose,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6),
		strings.Repeat("ef", 6))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 6)+strings.Repeat("cd", 2),
		strings.Repeat("cd", 4)+strings.Repeat("ef", 4),
		strings.Repeat("ef", 2))

	// A call to WriteMsg and then a call to Close.
	msgCh = bufferingFlowTestRunner(t, ctx, flows[6], accept, flowWriteMsgThenClose,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6),
		strings.Repeat("ef", 6))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 6),
		strings.Repeat("cd", 6),
		strings.Repeat("ef", 6))

	msgCh = bufferingFlowTestRunner(t, ctx, flows[7], accept, flowWriteMsgThenClose,
		strings.Repeat("ab", 10),
		strings.Repeat("cd", 10),
		strings.Repeat("ef", 10))
	expectedMessages(t, msgCh,
		strings.Repeat("ab", 8),
		strings.Repeat("ab", 2),
		strings.Repeat("cd", 8),
		strings.Repeat("cd", 2),
		strings.Repeat("ef", 8),
		strings.Repeat("ef", 2))

	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	<-ac.Closed()
	<-dc.Closed()
}

func TestBufferingFlowLarge(t *testing.T) {
	defer netbufsFreed(t)

	ctx, shutdown := test.V23Init()
	defer shutdown()

	mtu := bufferingFlowInternalArraySize * 2
	flows, accept, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, 2, Opts{MTU: uint64(mtu)})

	// Series of writes followed by a close but using buffers larger than the internal
	// array of 4k to test assigning a new netBuf to replace that array.
	msgCh := bufferingFlowTestRunner(t, ctx, flows[0], accept, flowWriteThenClose,
		strings.Repeat("a", bufferingFlowInternalArraySize-1),
		strings.Repeat("b", 10),
		strings.Repeat("c", bufferingFlowInternalArraySize+1),
		strings.Repeat("d", mtu+1))
	expectedMessages(t, msgCh,
		strings.Repeat("a", bufferingFlowInternalArraySize-1)+strings.Repeat("b", 10),
		strings.Repeat("c", bufferingFlowInternalArraySize+1),
		strings.Repeat("d", mtu),
		strings.Repeat("d", 1))

	msgCh = bufferingFlowTestRunner(t, ctx, flows[1], accept, flowWriteMsgThenClose,
		strings.Repeat("a", bufferingFlowInternalArraySize-1),
		strings.Repeat("b", 10),
		strings.Repeat("c", bufferingFlowInternalArraySize+1),
		strings.Repeat("d", mtu+1))
	expectedMessages(t, msgCh,
		strings.Repeat("a", bufferingFlowInternalArraySize-1)+strings.Repeat("b", 10),
		strings.Repeat("c", bufferingFlowInternalArraySize+1),
		strings.Repeat("d", mtu),
		strings.Repeat("d", 1))

	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	<-ac.Closed()
	<-dc.Closed()

}

type writeErrorFlow struct {
	*flw
}

func (ef *writeErrorFlow) Write([]byte) (int, error) {
	return 0, fmt.Errorf("write")
}

func (ef *writeErrorFlow) WriteMsg(...[]byte) (int, error) {
	return 0, fmt.Errorf("writeMsg")
}

func (ef *writeErrorFlow) WriteMsgAndClose(...[]byte) (int, error) {
	return 0, fmt.Errorf("writeMsgAndClose")
}

func (ef *writeErrorFlow) Flush() error {
	return fmt.Errorf("flush")
}

func TestBufferingFlowErrors(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	defer netbufsFreed(t)

	flows, _, dc, ac := setupFlowsOpts(t, "local", "", ctx, ctx, true, 1, Opts{MTU: 2})

	var err error
	var bf *BufferingFlow

	assertReset := func(errmsg string) {
		_, _, line, _ := runtime.Caller(1)
		if nerr := assertBufferReset(1, bf); err == nil || err.Error() != errmsg || nerr != nil {
			t.Fatalf("line: %v, missing/incorrect error: (%q), or buffer not reset correctly on error: %v", line, err, nerr)
		}
	}

	wf := &writeErrorFlow{flw: flows[0].(*flw)}
	bf = NewBufferingFlow(ctx, wf)
	_, err = bf.Write([]byte{'1', '2'})
	assertReset("write")

	bf = NewBufferingFlow(ctx, wf)
	_, err = bf.WriteMsg([]byte{'1', '2'})
	assertReset("write")

	bf = NewBufferingFlow(ctx, wf)
	bf.Write([]byte{'1'}) // make sure there is some buffered data before the larger write.
	_, err = bf.Write(bytes.Repeat([]byte{'b'}, bufferingFlowInternalArraySize+1))
	assertReset("write")

	bf = NewBufferingFlow(ctx, wf)
	_, err = bf.WriteMsgAndClose([]byte{'1', '2'})
	assertReset("writeMsgAndClose")

	bf = NewBufferingFlow(ctx, wf)
	if _, err := bf.Write([]byte{'x'}); err != nil {
		t.Fatal(err)
	}
	_, err = bf.WriteMsgAndClose([]byte{'2'})
	assertReset("write")

	bf = NewBufferingFlow(ctx, wf)
	if _, err := bf.Write([]byte{'x'}); err != nil {
		t.Fatal(err)
	}
	err = bf.Flush()
	assertReset("write")

	ac.Close(ctx, nil)
	dc.Close(ctx, nil)
	<-ac.Closed()
	<-dc.Closed()
}
