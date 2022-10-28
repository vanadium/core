// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import "testing"

func TestBufPools(t *testing.T) {
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
	}
}
