// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"io"
	"sync"

	"v.io/v23/context"
)

// readq implements a producer/consumer queue for network buffers with a
// callback to facilate flow control external to the readq. It also
// allows for context cancelation.
//
// NOTE that buffers are shared between the reader/writer and hence
// care must be taken to make a copy of the buffers prior to calling put
// if the underlying storage is to be reused. Also note that the read
// implementation will necessarily copy the data once more leading to
// potentially two copies - one before calling put and one by read.
//
// readq uses a circular buffer that is resized as needed but with a builtin
// array to handle the common case for when the circular buffer is small.
// The circular buffer will be small except when concurrency is limited or
// network latency is very high, the initial size is chosen to handle the
// most strenuous cases (eg. lots of connections with low concurrency).
type readq struct {
	bytesBufferedPerFlow uint64

	// Called whenever data is read/removed from the queue with the number of
	// bytes retured. It is intended to be used by flow control mechanisms
	// that need to keep track of the amount of data read.
	readCallback func(ctx *context.T, n int)

	mu sync.Mutex
	// circular buffer of added buffers
	bufsBuiltin [initialReadqBufferSize]readqEntry
	bufs        []readqEntry
	b, e        int // begin and end indices of data in those circular buffers

	closed bool // set when closed.

	size   uint64        // total amount of buffered data
	nbufs  int           // number of buffers
	notify chan struct{} // used to notify any listeners when an empty readq has had data added to it.

}

type readqEntry struct {
	buf  []byte
	nBuf *netBuf
}

const initialReadqBufferSize = 40

func newReadQ(bytesBufferedPerFlow uint64, readCallback func(ctx *context.T, n int)) *readq {
	rq := &readq{
		bytesBufferedPerFlow: bytesBufferedPerFlow,
		notify:               make(chan struct{}, 1),
		readCallback:         readCallback,
	}
	rq.bufs = rq.bufsBuiltin[:]
	return rq
}

func (r *readq) put(ctx *context.T, buf []byte, nBuf *netBuf) error {
	l := len(buf)
	if l == 0 {
		putNetBuf(nBuf)
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		putNetBuf(nBuf)
		// The flow has already closed.  Simply drop the data.
		return nil
	}

	newSize := uint64(l) + r.size
	if newSize > r.bytesBufferedPerFlow {
		return ErrCounterOverflow.Errorf(ctx, "a remote process has sent more data than allowed: max bytes buffered is %v, current buffered is %v + received: %v", r.bytesBufferedPerFlow, r.size, l)
	}
	newBufs := r.nbufs + 1
	r.reserveLocked(newBufs)
	r.bufs[r.e] = readqEntry{buf: buf, nBuf: nBuf}
	r.e = (r.e + 1) % len(r.bufs)
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

func (r *readq) reserveLocked(n int) {
	if n < len(r.bufsBuiltin) && len(r.bufs) > len(r.bufsBuiltin) {
		r.moveqLocked(r.bufsBuiltin[:])
		return
	}
	if n < len(r.bufs) {
		return
	}
	r.moveqLocked(make([]readqEntry, 2*n))
}

func (r *readq) moveqLocked(to []readqEntry) {
	copied := 0
	if r.e >= r.b {
		copied = copy(to, r.bufs[r.b:r.e])
	} else {
		copied = copy(to, r.bufs[r.b:])
		copied += copy(to[copied:], r.bufs[:r.e])
	}
	r.bufs, r.b, r.e = to, 0, copied
}

func (r *readq) read(ctx *context.T, data []byte) (n int, err error) {
	r.mu.Lock()
	if err = r.waitLocked(ctx); err == nil {
		err = nil
		entry := r.bufs[r.b]
		n = copy(data, entry.buf)
		entry.buf = entry.buf[n:]
		if len(entry.buf) > 0 {
			r.bufs[r.b] = entry
		} else {
			r.nbufs--
			putNetBuf(entry.nBuf)
			r.bufs[r.b] = readqEntry{}
			r.b = (r.b + 1) % len(r.bufs)
		}
		r.size -= uint64(n)
	}
	r.mu.Unlock()
	r.readCallback(ctx, n)
	return
}

func (r *readq) get(ctx *context.T) (out []byte, err error) {
	r.mu.Lock()
	if err = r.waitLocked(ctx); err == nil {
		entry := r.bufs[r.b]
		out = copyIfNeeded(entry.nBuf, entry.buf)
		putNetBuf(entry.nBuf)
		r.bufs[r.b] = readqEntry{}
		r.b = (r.b + 1) % len(r.bufs)
		r.size -= uint64(len(out))
		r.nbufs--
	}
	r.mu.Unlock()
	r.readCallback(ctx, len(out))
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
			if ctx.Err() == context.DeadlineExceeded {
				err = context.DeadlineExceeded
			}
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
	// Make sure to release currently held netBuf's that may be backed by
	// sync.Pool and storage and replace them with storage allocated from the
	// heap via a heap backed netBuf.
	for i := r.b; i != r.e; i = (i + 1) % len(r.bufs) {
		entry := r.bufs[i]
		if entry.nBuf != nil {
			out := copyIfNeeded(entry.nBuf, entry.buf)
			putNetBuf(entry.nBuf)
			nb, b := newNetBufPayload(out)
			r.bufs[i] = readqEntry{buf: b, nBuf: nb}
		}
	}
	closed := false
	if !r.closed {
		r.closed = true
		closed = true
		close(r.notify)
	}
	r.mu.Unlock()
	return closed
}
