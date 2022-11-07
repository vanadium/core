// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"
	"sync/atomic"
)

const (
	// Estimate of the overhead of the message header fields other
	// than the payloads.
	estimatedMessageOverhead = 256

	// max of gcmTagSize (16) and box.Overhead (16)
	maxCipherOverhead = 16

	// The plaintext message pipe buffer needs to allow for the overhead
	// of the message's header fields as well as its payload.
	// Note that if a connection uses a larger MTU than the default (since
	// it may specified/negoatiated) then extra allocations will take place.
	plaintextBufferSize = DefaultMTU + estimatedMessageOverhead

	// The ciphertext buffer needs to allow for the cipher overhead also.
	ciphertextBufferSize = plaintextBufferSize + maxCipherOverhead
)

var (
	netBufPools     []sync.Pool
	netBufPoolSizes = []int{128, 1024, 2048, 4096, 16384, ciphertextBufferSize}
	netBufGet       []int64
	netBufPut       []int64
)

func init() {
	netBufPools = make([]sync.Pool, len(netBufPoolSizes))
	netBufGet = make([]int64, len(netBufPoolSizes))
	netBufPut = make([]int64, len(netBufPoolSizes))
	for pool, size := range netBufPoolSizes {
		pool := pool
		size := size
		netBufPools[pool].New = func() interface{} {
			return &netBuf{
				buf:  make([]byte, size),
				pool: pool,
			}
		}
	}
}

// netBuf represents a buffer (byte slice) obtained from one of a set of sync.Pools
// each configured to manage buffers of a fixed size or from the heap.
// See netBufPoolSizes for the currently configured set of sizes. If a size
// larger than that suppored by all of the sync.Pools is requested then
// the buffer is allocated from the heap. It is also possible to create
// netBufs that are intentionally backed by heap storage. This allows for
// a uniform UI for managing sync.Pool and heap backed buffers.
// Typical usage is:
//
//	netBuf, buf := getNetBuf(size)
//	  ... use buf ...
//	putNetBuf(netBuf)
type netBuf struct {
	buf  []byte
	pool int // a heap allocated buffer has pool set to -1.
}

// newNetBuf returns a netBuf with storage allocated from the heap rather than
// a sync.Pool.
func newNetBuf(size int) (*netBuf, []byte) {
	nb := &netBuf{
		buf:  make([]byte, size),
		pool: -1,
	}
	return nb, nb.buf
}

// newNetBufPayload returns a netBuf backed by the supplied buffer which
// must have been heap allocated.
func newNetBufPayload(data []byte) (*netBuf, []byte) {
	nb := &netBuf{
		buf:  data,
		pool: -1,
	}
	return nb, nb.buf
}

// getNetBuf returns a new instance of netBuf with a buffer allocated either
// via a sync.Pool or the heap depending on its size. Regardless of whether
// the returned byte slice is extended the returned netBuf must be returned
// using putNetBuf.
func getNetBuf(size int) (*netBuf, []byte) {
	for i, s := range netBufPoolSizes {
		if size <= s {
			nb := netBufPools[i].Get().(*netBuf)
			atomic.AddInt64(&netBufGet[i], 1)
			return nb, nb.buf
		}
	}
	return newNetBuf(size)
}

// putNetBuf ensures that buffers obtained from a sync.Pool are returned to
// the appropriate pool. putNetBuf returns a nil netBuf that should be
// assigned to any variable/field containing the netBuf being returned to
// reduce the risk of returning the same buffer more than once. Returning the
// same buffer may result in the same buffer being returned by getNetBuf
// multiple times since sync.Pools have this property.
func putNetBuf(nb *netBuf) *netBuf {
	if nb == nil || nb.pool == -1 {
		return nil
	}
	if nb != nil {
		netBufPools[nb.pool].Put(nb)
		atomic.AddInt64(&netBufPut[nb.pool], 1)
	}
	return nil
}

// copyIfNeeded determies if the supplied byte slice needs to be copied
// in order to be used after the netBuf is released.
func copyIfNeeded(bd *netBuf, b []byte) []byte {
	if bd.pool == -1 && sameUnderlyingStorage(b, bd.buf) {
		return b
	}
	n := make([]byte, len(b))
	copy(n, b)
	return n
}

// sameUnderlyingStorage returns true if x and y share the same underlying
// storage.
func sameUnderlyingStorage(x, y []byte) bool {
	return &x[0:cap(x)][cap(x)-1] == &y[0:cap(y)][cap(y)-1]
}

// differentUnderlyingStorage returns true if x and y do not share the same
// underlying storage.
func differentUnderlyingStorage(x, y []byte) bool {
	return &x[0:cap(x)][cap(x)-1] != &y[0:cap(y)][cap(y)-1]
}
