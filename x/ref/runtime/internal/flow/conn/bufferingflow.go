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
	lf  *flw
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
	b.nBuf, b.buf = getNetBuf(b.mtu)
	b.buf = b.buf[:0]
	if lf, ok := f.(*flw); ok {
		b.lf = lf
	}
	return b
}

// Write buffers data until the underlying channels MTU is reached at which point
// it calls Write on the wrapped Flow.
func (b *BufferingFlow) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.appendLocked(data)
}

func (b *BufferingFlow) writeLocked(data []byte) (int, error) {
	if b.lf != nil {
		return b.lf.writeMsg(false, [][]byte{b.buf})
	}
	return b.Flow.Write(b.buf)
}

func (b *BufferingFlow) appendLocked(data []byte) (int, error) {
	l := len(data)
	if len(b.buf)+l < b.mtu {
		b.buf = append(b.buf, data...)
		return l, nil
	}
	_, err := b.writeLocked(b.buf)
	b.buf = b.buf[:0]
	b.buf = append(b.buf, data...)
	return l, err
}

// WriteMsg buffers data until the underlying channels MTU is reached at which point
// it calls WriteMsg on the wrapped Flow.
func (b *BufferingFlow) WriteMsg(data ...[]byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	wrote := 0
	for _, d := range data {
		n, err := b.appendLocked(d)
		wrote += n
		if err != nil {
			return wrote, err
		}
	}
	return wrote, nil
}

// Close flushes the already written data and then closes the underlying Flow.
func (b *BufferingFlow) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var err error
	if b.lf != nil {
		_, err = b.lf.writeMsg(true, [][]byte{b.buf})
	} else {
		_, err = b.Flow.WriteMsgAndClose(b.buf)
	}
	b.nBuf = putNetBuf(b.nBuf)
	b.buf = nil
	return err
}

// WriteMsgAndClose writes all buffered data and closes the underlying Flow.
func (b *BufferingFlow) WriteMsgAndClose(data ...[]byte) (int, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	if b.lf != nil {
		return b.lf.writeMsg(true, [][]byte{b.buf})
	}
	return b.Flow.WriteMsgAndClose(data...)
}

// Flush writes all buffered data to the underlying Flow.
func (b *BufferingFlow) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.writeLocked(b.buf)
	b.buf = b.buf[:0]
	return err
}
