// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package iobuf performs explicit memory management for data buffers used
// to perform network IO.  The intent is that it is more efficient to perform
// manual allocation than to rely on the Go garbage collector to manage large
// chunks of frequently recycled memory.
//
// In this model, a Pool is a collection of contiguous memory area (of type
// <buf>) used for memory allocation.  The bufs are subdivided into slices of
// type Slice.
//
//     Pool: a source of memory areas.
//     Slice: a contiguous region of allocated memory.
//     Allocator: a Slice allocator.
//     Reader: an IO reader that reads into Slices.
//
// There is an analogy with sbrk/malloc: the Pool is the source of memory (like
// sbrk), and the Allocator hands out small areas (like malloc).  Allocations
// are mostly sequential within a buf, allowing sequentially-allocated Slices to
// be coalesced at some later point.
//
// For efficiency, Slice values hold reference counts to the underlying buf.
// When all references are to a buf released, the buf is recycled into its Pool.
// This does not happen automatically.  The caller is responsible for calling
// slice.Release() when finished using a slice.
package iobuf

import (
	"sync"
	"sync/atomic"

	"v.io/x/ref/internal/logger"
)

// A iobuf is a storage space for memory read from the network.  The data should
// be read into the Contents field, then sliced up into Slice slices that
// correspond to header, payload, etc.
//
// iobufs are reference counted.  The count includes one reference for the iobuf
// itself, plus one for each Slice.
type buf struct {
	refcount int32 // Use atomic operations.
	Contents []byte
	pool     *Pool
}

// Pool manages a pool of iobufs.  The size of the pool is not fixed,
// it can grow without bound.
//
// The implementation here allocates a new iobuf whenever there is an allocation
// request and the pool is empty.  For iobufs to be recycled, explicit Release()
// calls are required.  However, if these Release() calls are missing, the
// program will continue to function, recycling the buffers through the gc.
// Therefore, if you forget Release() calls, you will be putting pressure on gc
// to recycle the iobufs.  You can examine the <allocated> field to check how
// many iobufs have been allocated during the lifetime of the Pool.
type Pool struct {
	minSize   uint
	mutex     sync.Mutex
	freelist  []*buf
	allocated uint64 // Total number of iobufs allocated.
}

const defaultMinSize = 1 << 12

// NewPool creates a new pool. The pool will allocate iobufs in multiples of minSize.
// If minSize is zero, the default value (4K) will be used.
func NewPool(minSize uint) *Pool {
	if minSize == 0 {
		minSize = defaultMinSize
	}
	return &Pool{minSize: minSize, freelist: []*buf{}}
}

// Close shuts down the Pool.
func (pool *Pool) Close() {
	pool.mutex.Lock()
	pool.freelist = nil
	pool.mutex.Unlock()
}

// alloc allocates a new iobuf.  The returned iobuf has at least <size> bytes of free space.
func (pool *Pool) alloc(size uint) *buf {
	if size == 0 {
		size = pool.minSize
	} else if r := size % pool.minSize; r > 0 {
		size += pool.minSize - r
	}

	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	if pool.freelist == nil {
		logger.Global().Info("iobuf.Pool is closed")
		return nil
	}

	// Search for an iobuf that is large enough.
	for i := len(pool.freelist) - 1; i >= 0; i-- {
		iobuf := pool.freelist[i]
		if uint(len(iobuf.Contents)) >= size {
			pool.freelist[i] = pool.freelist[len(pool.freelist)-1]
			pool.freelist = pool.freelist[:len(pool.freelist)-1]
			atomic.AddInt32(&iobuf.refcount, 1)
			return iobuf
		}
	}

	// All the free buffers are too small.  Allocate a fresh one.
	pool.allocated++
	iobuf := &buf{refcount: 1, Contents: make([]byte, size), pool: pool}
	return iobuf
}

// release recycles an iobuf that has a zero refcount.
func (pool *Pool) release(iobuf *buf) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()
	// TODO(jyh): Ideally we would like to overwrite the iobuf so that if there
	// are slices still referring to it (due to a double-Release), it will be
	// more likely that the problem is noticed.  Implement this if we define a
	// "debug mode."
	if pool.freelist != nil {
		pool.freelist = append(pool.freelist, iobuf)
	}
}

// release decrements the iobuf's refcount, recycling the iobuf when the count
// reaches zero.
func (iobuf *buf) release() {
	refcount := atomic.AddInt32(&iobuf.refcount, -1)
	if refcount < 0 {
		logger.Global().Infof("Refcount is negative: %d.  This is a bug in the program.", refcount)
	}
	if refcount == 0 {
		iobuf.pool.release(iobuf)
	}
}

// slice creates an Slice that refers to a slice of the iobuf contents.
func (iobuf *buf) slice(free, base, bound uint) *Slice {
	atomic.AddInt32(&iobuf.refcount, 1)
	return &Slice{iobuf: iobuf, free: free, base: base, Contents: iobuf.Contents[base:bound]}
}
