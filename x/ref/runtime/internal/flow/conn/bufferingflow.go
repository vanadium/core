// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow"
)

type MTUer interface {
	MTU() uint64
}

// BufferingFlow wraps a Flow and buffers all its writes.  It writes to the
// underlying channel when buffering new data would exceed the MTU of the
// underlying channel or when one of Flush, Close or Note that it will never
// fragment a single payload over multiple writes to that channel.
type BufferingFlow struct {
	flow.Flow
	mtu int

	mu   sync.Mutex
	nBuf netBuf
	buf  []byte
}

// NewBufferingFlow creates a new instance of BufferingFlow.
func NewBufferingFlow(ctx *context.T, f flow.Flow) *BufferingFlow {
	b := &BufferingFlow{
		Flow: f,
		mtu:  DefaultMTU,
	}
	if m, ok := f.Conn().(MTUer); ok {
		b.mtu = int(m.MTU())
	}
	return b
}

// Write buffers data until the underlying channel's MTU is reached at which point
// it will write any buffered data and buffer the newly supplied data.
func (b *BufferingFlow) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.appendLocked(data)
	return n, b.handleError(err)
}

// WriteMsg buffers data until the underlying channel's MTU is reached at which point
// it will write any buffered data and buffer the newly supplied data.
func (b *BufferingFlow) WriteMsg(data ...[]byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	wrote := 0
	for _, d := range data {
		n, err := b.appendLocked(d)
		wrote += n
		if err != nil {
			return wrote, b.handleError(err)
		}
	}
	return wrote, nil
}

func (b *BufferingFlow) appendLocked(data []byte) (int, error) {
	l := len(data)
	if b.buf == nil {
		b.nBuf, b.buf = getNetBuf(b.mtu)
		b.buf = b.buf[:0]
	}
	if len(b.buf)+l < b.mtu {
		b.buf = append(b.buf, data...)
		return l, nil
	}
	_, err := b.Flow.Write(b.buf)
	b.buf = b.buf[:0]
	b.buf = append(b.buf, data...)
	return l, err
}

// Close flushes the already written data and then closes the underlying Flow.
func (b *BufferingFlow) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.Flow.WriteMsgAndClose(b.buf)
	b.reset()
	return err
}

// WriteMsgAndClose writes all buffered data and closes the underlying Flow.
func (b *BufferingFlow) WriteMsgAndClose(data ...[]byte) (int, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	if len(b.buf) > 0 {
		if _, err := b.Flow.Write(b.buf); err != nil {
			return 0, b.handleError(err)
		}
	}
	n, err := b.Flow.WriteMsgAndClose(data...)
	b.reset()
	return n, err
}

// Flush writes all buffered data to the underlying Flow.
func (b *BufferingFlow) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.Flow.Write(b.buf)
	b.buf = b.buf[:0]
	return b.handleError(err)
}

func (b *BufferingFlow) reset() {
	b.nBuf = putNetBuf(b.nBuf)
	b.buf = nil
}

func (b *BufferingFlow) handleError(err error) error {
	if err == nil {
		return nil
	}
	b.reset()
	return err
}
