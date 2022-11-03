// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/rand"
	"io"
	"reflect"
	"runtime"
	"testing"
)

func TestPopFrontN(t *testing.T) {
	input := [][]byte{
		make([]byte, 64),
		make([]byte, 32),
		make([]byte, 33),
		make([]byte, 96),
	}
	for i := range input {
		io.ReadFull(rand.Reader, input[i])
	}

	copyInput := func() [][]byte {
		r := make([][]byte, len(input))
		copy(r, input)
		return r
	}
	_ = copyInput

	var send []byte
	var nextSlice, nextOffset, size int
	assert := func(wSend []byte, wNextSlice, wNextOffset, wSize int) {
		_, _, line, _ := runtime.Caller(1)
		if got, want := nextSlice, wNextSlice; got != want {
			t.Errorf("line: %v, next slice: got %v, want %v", line, got, want)
		}
		if got, want := nextOffset, wNextOffset; got != want {
			t.Errorf("line: %v, nextOffset: got %v, want %v", line, got, want)
		}
		if got, want := size, wSize; got != want {
			t.Errorf("line: %v, size: got %v, want %v", line, got, want)
		}
		if got, want := size, len(wSend); got != want {
			t.Errorf("line: %v, size: got %v, want %v", line, got, want)
		}
		if got, want := send, wSend; !reflect.DeepEqual(got, want) {
			t.Errorf("line: %v, send: got % 02x, want % 02x", line, got, want)
		}
	}

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, 1)
	assert(input[0][0:1], 0, 1, 1)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(input[0][1:], 1, 0, len(input[0])-1)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, len(input[0]))
	assert(input[0], 1, 0, len(input[0]))

	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, len(input[0]))
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, len(input[0])+100)
	assert(input[0], 1, 0, len(input[0]))
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 1)
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input[:1], 0, 0, 33)
	assert(input[0][:33], 0, 33, 33)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 1)
	assert(input[0][33:34], 0, 34, 1)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(input[0][34:], 1, 0, 30)
	send, nextSlice, nextOffset, size = readAtMost(input[:1], nextSlice, nextOffset, 100)
	assert(nil, 0, 0, 0)

	partial := len(input[1]) / 3
	atMost := len(input[0]) + partial
	send, nextSlice, nextOffset, size = readAtMost(input, 0, 0, atMost)
	assert(append(input[0], input[1][:partial]...), 1, partial, atMost)

	prevOffset := partial
	partial = len(input[1]) - len(input[1])/3
	atMost = partial + len(input[2])
	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, atMost)
	assert(append(input[1][prevOffset:], input[2]...), 3, 0, atMost)

	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, 1000)
	assert(input[3], 4, 0, len(input[3]))

	send, nextSlice, nextOffset, size = readAtMost(input, nextSlice, nextOffset, 1000)
	assert(nil, 0, 0, 0)

	send, nextSlice, nextOffset, size = readAtMost(input, 0, 0, 1000)
	tmpOut := input[0]
	tmpOut = append(tmpOut, input[1]...)
	tmpOut = append(tmpOut, input[2]...)
	tmpOut = append(tmpOut, input[3]...)
	assert(tmpOut, 4, 0, totalSize(input))
}
