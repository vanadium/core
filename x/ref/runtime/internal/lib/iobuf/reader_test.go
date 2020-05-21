// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

import (
	"io"
	"testing"
)

type testReader struct {
	off      int
	isClosed bool
}

func (r *testReader) Read(buf []byte) (int, error) {
	if r.isClosed {
		return 0, io.EOF
	}
	amount := len(buf)
	for i := 0; i != amount; i++ {
		buf[i] = charAt(r.off + i)
	}
	r.off += amount
	return amount, nil
}

func TestReader(t *testing.T) {
	pool := NewPool(iobufSize)
	defer pool.Close()

	var tr testReader
	r := NewReader(pool, &tr)
	defer r.Close()

	const amount = 4
	const loopCount = 1000
	for off := 0; off < loopCount*amount; off += amount {
		s, err := r.Read(amount)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		checkBuf(t, s.Contents, off)
		s.Release()
	}

	tr.isClosed = true
	for off := amount * loopCount; off != tr.off; off++ {
		s, err := r.Read(1)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		checkBuf(t, s.Contents, off)
		s.Release()
	}

	_, err := r.Read(1)
	if err == nil {
		t.Errorf("Expected error")
	}
}
