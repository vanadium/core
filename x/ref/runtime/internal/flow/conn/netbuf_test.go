// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestNetBufs(t *testing.T) {
	large := make([]byte, ciphertextBufferSize*2)
	for i, tc := range []struct {
		request, cap, pool int
	}{
		{12, 128, 0},
		{1023, 1024, 1},
		{2035, 2048, 2},
		{4096, 4096, 3},
		{4097, 16384, 4},
		{8193, 16384, 4},
		{16385, ciphertextBufferSize, 5},
		{32769, ciphertextBufferSize, 5},
		{ciphertextBufferSize + 1, ciphertextBufferSize + 1, -1},
	} {
		nb, b := getNetBuf(tc.request)
		if got, want := cap(b), tc.cap; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := nb.pool, tc.pool; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if differentUnderlyingStorage(nb.buf, b) {
			t.Errorf("%v: should share the same underlying storage", i)
		}
		b = append(b, large...)
		if sameUnderlyingStorage(nb.buf, b) {
			t.Errorf("%v: should not share the same underlying storage", i)
		}
		nb = putNetBuf(nb)
		putNetBuf(nb) // safe to call putNetBuf on nb if nb is nil.
	}
}

func netbufsFreed(t testing.TB) {
	for i := 0; i < len(netBufPoolSizes); i++ {
		nGet, nPut := atomic.LoadInt64(&netBufGet[i]), atomic.LoadInt64(&netBufPut[i])
		if nGet != nPut {
			_, file, line, _ := runtime.Caller(1)
			file = filepath.Base(file)
			t.Errorf("%v:%v: netBuf storage leak for pool: %v, %v != %#v\n", file, line, i, nGet, nPut)
		}
	}
}
