// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"io"
	"sync"

	"v.io/v23/context"
)

type readq struct {
	mu     sync.Mutex
	bufs   [][]byte
	b, e   int
	closed bool

	id     uint64
	size   int
	nbufs  int
	notify chan struct{}
	conn   *Conn
}

const initialReadqBufferSize = 10

func newReadQ(conn *Conn, id uint64) *readq {
	return &readq{
		bufs:   make([][]byte, initialReadqBufferSize),
		notify: make(chan struct{}, 1),
		conn:   conn,
		id:     id,
	}
}

func (r *readq) put(ctx *context.T, bufs [][]byte) error {
	l := 0
	for _, b := range bufs {
		l += len(b)
	}
	if l == 0 {
		return nil
	}

	defer r.mu.Unlock()
	r.mu.Lock()
	if r.closed {
		// The flow has already closed.  Simply drop the data.
		return nil
	}
	newSize := l + r.size
	if newSize > DefaultBytesBufferedPerFlow {
		return ErrCounterOverflow.Errorf(ctx, "a remote process has sent more data than allowed")
	}
	newBufs := r.nbufs + len(bufs)
	r.reserveLocked(newBufs)
	for _, b := range bufs {
		r.bufs[r.e] = b
		r.e = (r.e + 1) % len(r.bufs)
	}
	r.nbufs = newBufs
	if r.size == 0 {
		select {
		case r.notify <- struct{}{}:
		default:
		}
	}
	r.size = newSize
	return nil
}

func (r *readq) read(ctx *context.T, data []byte) (n int, err error) {
	r.mu.Lock()
	if err = r.waitLocked(ctx); err == nil {
		err = nil
		buf := r.bufs[r.b]
		n = copy(data, buf)
		buf = buf[n:]
		if len(buf) > 0 {
			r.bufs[r.b] = buf
		} else {
			r.nbufs--
			r.b = (r.b + 1) % len(r.bufs)
		}
		r.size -= n
	}
	r.mu.Unlock()
	if r.conn != nil {
		r.conn.release(ctx, r.id, uint64(n))
	}
	return
}

func (r *readq) get(ctx *context.T) (out []byte, err error) {
	r.mu.Lock()
	if err = r.waitLocked(ctx); err == nil {
		err = nil
		out = r.bufs[r.b]
		r.b = (r.b + 1) % len(r.bufs)
		r.size -= len(out)
		r.nbufs--
	}
	r.mu.Unlock()
	if r.conn != nil {
		r.conn.release(ctx, r.id, uint64(len(out)))
	}
	return
}

func (r *readq) waitLocked(ctx *context.T) (err error) {
	for r.size == 0 && err == nil {
		r.mu.Unlock()
		select {
		case _, ok := <-r.notify:
			if !ok {
				err = io.EOF
			}
		case <-ctx.Done():
			err = io.EOF
		}
		r.mu.Lock()
	}
	// Even if the flow is closed, if we have data already queued
	// we'll let it be read.
	if err == io.EOF && r.nbufs > 0 {
		err = nil
	}
	return err
}

func (r *readq) close(ctx *context.T) bool {
	r.mu.Lock()
	closed := false
	if !r.closed {
		r.closed = true
		closed = true
		close(r.notify)
	}
	r.mu.Unlock()
	return closed
}

func (r *readq) reserveLocked(n int) {
	if n < len(r.bufs) {
		return
	}
	nb := make([][]byte, 2*n)
	copied := 0
	if r.e >= r.b {
		copied = copy(nb, r.bufs[r.b:r.e])
	} else {
		copied = copy(nb, r.bufs[r.b:])
		copied += copy(nb[copied:], r.bufs[:r.e])
	}
	r.bufs, r.b, r.e = nb, 0, copied
}
