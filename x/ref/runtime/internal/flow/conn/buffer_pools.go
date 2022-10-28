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
	bufPools     []sync.Pool
	bufPoolSizes = []int{1024, 4096, 8192, 16384, 32768, ciphertextBufferSize}
	bufMaxPool   = 0
)

func init() {
	bufPools = make([]sync.Pool, len(bufPoolSizes))
	for i, size := range bufPoolSizes {
		size := size
		bufPools[i].New = func() interface{} {
			b := make([]byte, size)
			return &b
		}
	}
	bufMaxPool = len(bufPoolSizes)
}

type poolBuf struct {
	rbuf *[]byte
	pool int
}

func getPoolBuf(size int) (poolBuf, []byte) {
	for i, s := range bufPoolSizes {
		if size <= s {
			b := bufPools[i].Get().(*[]byte)
			bd := poolBuf{
				rbuf: b,
				pool: i,
			}
			return bd, *b
		}
	}
	buf := make([]byte, size)
	bd := poolBuf{
		rbuf: &buf,
		pool: -1,
	}
	return bd, buf
}

func putPoolBuf(bd poolBuf) {
	if bd.pool == -1 {
		bd.rbuf = nil
		return
	}
	bufPools[bd.pool].Put(bd.rbuf)
}

func (pb poolBuf) bytes() []byte {
	return *pb.rbuf
}
