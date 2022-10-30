// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"
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
	netBufPoolSizes = []int{1024, 4096, 8192, 16384, 32768, ciphertextBufferSize}
)

func init() {
	netBufPools = make([]sync.Pool, len(netBufPoolSizes))
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
// the buffer is allocated from the heap. Typical usage is:
//
//	netBuf, buf := getNetBuf(size)
//	  ... use buf ...
//	putNetBuf(netBuf)
type netBuf struct {
	buf  []byte
	pool int // a heap allocated buffer has pool set to -1.
}

// getNetBuf returns a new instance of netBuf with a buffer allocated either
// via a sync.Pool or the heap depending on its size. Regardless of whether
// the returned byte slice is extended the returned netBuf must be returned
// using putNetBuf.
func getNetBuf(size int) (*netBuf, []byte) {
	for i, s := range netBufPoolSizes {
		if size <= s {
			nb := netBufPools[i].Get().(*netBuf)
			return nb, nb.buf
		}
	}
	nb := &netBuf{
		buf:  make([]byte, size),
		pool: -1,
	}
	return nb, nb.buf
}

// putNetBuf ensures that buffers obtained from a sync.Pool are returned to
// the appropriate pool. putNetBuf returns a nil netBuf that should be
// assigned to any variable/field containing the netBuf being returned to
// reduce the risk of returning the same buffer more than once. Returning the
// same buffer may result in the same buffer being returned by getNetBuf
// multiple times since sync.Pools have this property.
func putNetBuf(bd *netBuf) *netBuf {
	if bd == nil || bd.pool == -1 {
		return nil
	}
	if bd != nil {
		netBufPools[bd.pool].Put(bd)
	}
	return nil
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
