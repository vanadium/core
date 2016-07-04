// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow"
)

var bufferPool = sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}

type MTUer interface {
	MTU() uint64
}

// BufferingFlow wraps a Flow and buffers all its writes.  It only truly writes to the
// underlying flow when the buffered data exceeds the MTU of the underlying channel, or
// Flush, Close, or WriteMsgAndClose is called.
type BufferingFlow struct {
	flow.Flow
	mtu uint64

	mu  sync.Mutex
	buf *bytes.Buffer // Protected by mu.
}

func NewBufferingFlow(ctx *context.T, flw flow.Flow) *BufferingFlow {
	b := &BufferingFlow{
		Flow: flw,
		buf:  bufferPool.Get().(*bytes.Buffer),
		mtu:  defaultMtu,
	}
	b.buf.Reset()
	if m, ok := flw.Conn().(MTUer); ok {
		b.mtu = m.MTU()
	}
	return b
}

// Write buffers data until the underlying channels MTU is reached at which point
// it calls Write on the wrapped Flow.
func (b *BufferingFlow) Write(data []byte) (int, error) {
	return b.WriteMsg(data)
}

// WriteMsg buffers data until the underlying channels MTU is reached at which point
// it calls WriteMsg on the wrapped Flow.
func (b *BufferingFlow) WriteMsg(data ...[]byte) (int, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	if b.buf == nil {
		return b.Flow.WriteMsg(data...)
	}
	wrote := 0
	for _, d := range data {
		if l := b.buf.Len(); l > 0 && uint64(l+len(d)) > b.mtu {
			if _, err := b.Flow.WriteMsg(b.buf.Bytes()); err != nil {
				return wrote, err
			}
			b.buf.Reset()
		}
		cur, err := b.buf.Write(d)
		wrote += cur
		if err != nil {
			return wrote, err
		}
	}
	return wrote, nil
}

// Close flushes the already written data and then closes the underlying Flow.
func (b *BufferingFlow) Close() error {
	defer b.mu.Unlock()
	b.mu.Lock()
	if b.buf == nil {
		return b.Flow.Close()
	}
	_, err := b.Flow.WriteMsgAndClose(b.buf.Bytes())
	bufferPool.Put(b.buf)
	b.buf = nil
	return err
}

// WriteMsgAndClose writes all buffered data and closes the underlying Flow.
func (b *BufferingFlow) WriteMsgAndClose(data ...[]byte) (int, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	if b.buf == nil {
		return b.Flow.WriteMsgAndClose(data...)
	}
	wrote, err := b.WriteMsg(data...)
	if err != nil {
		return wrote, err
	}
	return wrote, b.Close()
}

// Flush writes all buffered data to the underlying Flow.
func (b *BufferingFlow) Flush() (err error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	if b.buf != nil && b.buf.Len() > 0 {
		byts := b.buf.Bytes()
		_, err = b.Flow.WriteMsg(byts)
		b.buf.Reset()
	}
	return err
}
