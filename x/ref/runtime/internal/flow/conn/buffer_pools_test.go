// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import "testing"

func sameBuffer(x, y []byte) bool {
	return &x[0:cap(x)][cap(x)-1] == &y[0:cap(y)][cap(y)-1]
}

func TestBufPools(t *testing.T) {
	extra := make([]byte, ciphertextBufferSize*2)
	for i, tc := range []struct {
		request, cap, pool int
	}{
		{12, 1024, 0},
		{1023, 1024, 0},
		{2035, 4096, 1},
		{4096, 4096, 1},
		{4097, 8192, 2},
		{8193, 16384, 3},
		{16385, 32768, 4},
		{32769, ciphertextBufferSize, 5},
		{ciphertextBufferSize + 1, ciphertextBufferSize + 1, -1},
	} {
		desc, b := getPoolBuf(tc.request)
		if got, want := cap(b), tc.cap; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := desc.pool, tc.pool; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		putPoolBuf(desc)
		desc, nb := getPoolBuf(tc.request)
		if desc.pool != -1 && !sameBuffer(b, nb) {
			t.Errorf("%v: buffers differ", i)
		}
		if desc.pool == -1 && sameBuffer(b, nb) {
			t.Errorf("%v: buffers are the same", i)
		}
		if got, want := desc.pool, tc.pool; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		putPoolBuf(desc)
		ob := b
		b = append(b, extra...)
		if sameBuffer(b, ob) {
			t.Errorf("%v: same buffer", i)
		}
		putPoolBuf(desc) // no effect
	}
}
