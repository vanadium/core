// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

import (
	"io"
)

// Reader wraps an io.Reader to provide a buffered Read() operation.
type Reader struct {
	pool        *Pool
	iobuf       *buf
	base, bound uint
	conn        io.Reader
}

// NewReader returns a new io reader.
func NewReader(pool *Pool, conn io.Reader) *Reader {
	iobuf := pool.alloc(0)
	return &Reader{pool: pool, iobuf: iobuf, conn: conn}
}

// Close closes the Reader.  Do not call this concurrently with Read.  Instead,
// close r.conn, wait until Read() has completed, then perform the Close().
func (r *Reader) Close() {
	r.iobuf.release()
	r.pool = nil
	r.iobuf = nil
}

// Fill ensures that the input contains at least <bytes> bytes.  Returns an
// error iff the input is short (even if some input was read).
func (r *Reader) fill(bytes uint) error {
	if r.bound-r.base >= bytes {
		return nil
	}

	// If there is not enough space to read the data contiguously, allocate a
	// new iobuf.
	if uint(len(r.iobuf.Contents))-r.base < bytes {
		iobuf := r.pool.alloc(bytes)
		r.bound = uint(copy(iobuf.Contents, r.iobuf.Contents[r.base:r.bound]))
		r.base = 0
		r.iobuf.release()
		r.iobuf = iobuf
	}

	// Read into the iobuf.
	for r.bound-r.base < bytes {
		amount, err := r.conn.Read(r.iobuf.Contents[r.bound:])
		if amount == 0 && err != nil {
			return err
		}
		r.bound += uint(amount)
	}
	return nil
}

// Read returns the next <n> bytes of input as a Slice.  Returns an error if the
// read was short, even if some input was read.
func (r *Reader) Read(n int) (*Slice, error) {
	bytes := uint(n)
	if err := r.fill(bytes); err != nil {
		return nil, err
	}
	slice := r.iobuf.slice(r.base, r.base, r.base+bytes)
	r.base += bytes
	return slice, nil
}
