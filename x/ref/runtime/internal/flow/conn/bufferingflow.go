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

const bufferingFlowInternalArraySize = 4096

// BufferingFlow wraps a Flow and buffers all its writes.  It writes to the
// underlying channel when buffering new data would exceed the MTU of the
// underlying channel or when one of Flush, Close are called.
// Note that it will never fragment a single payload over multiple writes
// to the underlying channel but the underlying channel will of course
// fragment to keep within its mtu boundaries.
type BufferingFlow struct {
	flow.Flow
	mtu int

	mu sync.Mutex
	// Use a small internal buffer to absorb the common case of a
	// lot of small writes followed by a flush as issued by RPC clients
	// for example.
	internal [bufferingFlowInternalArraySize]byte
	nBuf     *netBuf
	buf      []byte
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
	b.buf = b.internal[:0]
	return b
}

// flushAndWriteLocked is called when data is larger than the mtu size
// and hence should be written immediately, possibly flushing any
// existing buffered data.
func (b *BufferingFlow) flushAndWriteLocked(data []byte) (int, error) {
	if len(b.buf) > 0 {
		if _, err := b.Flow.Write(b.buf); err != nil {
			return 0, b.handleError(err)
		}
		b.buf = b.buf[:0]
	}
	n, err := b.Flow.Write(data)
	b.buf = b.buf[:0]
	return n, b.handleError(err)
}

// appendLocked is called when data is smaller or equal to the mtu size
// and hence there is the possibility of buffering it. If the size
// of any buffered data plus the new data exceeds the mtu then
// the prior data will be written and the new data buffered.
func (b *BufferingFlow) appendLocked(data []byte) (int, error) {
	l := len(data)
	if need := len(b.buf) + l; need < b.mtu {
		if need > cap(b.buf) {
			// We've exceeded the capacity of the intitially assigned
			// buffer so obtain a new one, that's of mtu size and
			// use that going forward.
			newNetBuf, newBuf := getNetBuf(b.mtu)
			newBuf = append(newBuf[:0], b.buf...)
			b.nBuf = putNetBuf(b.nBuf)
			b.nBuf = newNetBuf
			b.buf = newBuf
		}
		b.buf = append(b.buf, data...)
		return l, nil
	}
	_, err := b.Flow.Write(b.buf)
	b.buf = b.buf[:0]
	b.buf = append(b.buf, data...)
	return l, err
}

// Write buffers data until the underlying channel's MTU is reached at which point
// it will write any buffered data and buffer the newly supplied data.
func (b *BufferingFlow) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) > b.mtu {
		return b.flushAndWriteLocked(data)
	}
	n, err := b.appendLocked(data)
	return n, b.handleError(err)
}

// WriteMsg buffers data until the underlying channel's MTU is reached at which point
// it will write any buffered data and buffer the newly supplied data.
func (b *BufferingFlow) WriteMsg(data ...[]byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	wrote := 0
	var n int
	var err error
	for _, d := range data {
		if len(d) > b.mtu {
			n, err = b.flushAndWriteLocked(d)
		} else {
			n, err = b.appendLocked(d)
		}
		wrote += n
		if err != nil {
			return wrote, b.handleError(err)
		}
	}
	return wrote, nil
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
	b.nBuf = putNetBuf(b.nBuf)
	b.buf = nil
	return err
}
