// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"
)

const (
	// estimate of how the overhead of the message header fields other
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
	bufMaxPool = len(bufPoolSizes) - 1
}

/*
//const bufPoolEmbedSize = 1024
type flexBuf struct {
	rbuf  *[]byte
	pool  int
	embed [bufPoolEmbedSize]byte
}

func (fb *flexBuf) get(size int) []byte {
	if size < bufPoolEmbedSize {
		fb.rbuf = nil
		return fb.embed[:]
	}
	for i, s := range bufPoolSizes {
		if size <= s {
			fb.rbuf = bufPools[i].Get().(*[]byte)
			fb.pool = i
			return *fb.rbuf
		}
	}
	fb.rbuf = bufPools[bufMaxPool].Get().(*[]byte)
	fb.pool = bufMaxPool
	return *fb.rbuf
}

func (fb *flexBuf) put() {
	if fb.rbuf == nil {
		return
	}
	bufPools[fb.pool].Put(fb.rbuf)
	fb.rbuf = nil
}
*/

type bufDesc struct {
	rbuf *[]byte
	pool int
}

func getPoolBuf(size int) (bufDesc, []byte) {
	for i, s := range bufPoolSizes {
		if size <= s {
			bd := bufDesc{
				rbuf: bufPools[i].Get().(*[]byte),
				pool: i,
			}
			return bd, *bd.rbuf
		}
	}
	buf := make([]byte, size)
	bd := bufDesc{
		rbuf: &buf,
		pool: bufMaxPool,
	}
	return bd, buf
}

func putPoolBuf(bd bufDesc) {
	if bd.pool == -1 {
		return
	}
	bufPools[bd.pool].Put(bd.rbuf)
}
