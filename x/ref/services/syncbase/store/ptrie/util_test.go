// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ptrie

import (
	"bytes"
	"testing"
)

// TestGetBit verifies the getBit() function.
func TestGetBit(t *testing.T) {
	x := []byte{0x1b, 0x1b, 0x1b}
	xbits := []byte{
		0, 0, 0, 1, 1, 0, 1, 1,
		0, 0, 0, 1, 1, 0, 1, 1,
		0, 0, 0, 1, 1, 0, 1, 1,
	}
	for i := 0; i < 24; i++ {
		if got, want := getBit(x, uint32(i)), xbits[i]; got != want {
			t.Fatalf("unexpected bit %d: got %d, want %d", i, got, want)
		}
	}
}

// TestGetBit verifies the appendBits() function.
func TestAppendBits(t *testing.T) {
	a := []byte{0x99, 0x99}
	b := []byte{0xff, 0xff}
	if got, want := appendBits(a, 8, b), []byte{0x99, 0xff, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
	if got, want := appendBits(a, 9, b), []byte{0x99, 0xff, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
	if got, want := appendBits(a, 10, b), []byte{0x99, 0xbf, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
	if got, want := appendBits(a, 11, b), []byte{0x99, 0x9f, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
	if got, want := appendBits(a, 14, b), []byte{0x99, 0x9b, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
	if got, want := appendBits(a, 16, b), []byte{0x99, 0x99, 0xff, 0xff}; !bytes.Equal(got, want) {
		t.Fatalf("unexpected appendBits result: got %v, want %v", got, want)
	}
}

// TestSliceBitLCP verifies the sliceBitLCP() function.
func TestSliceBitLCP(t *testing.T) {
	for i := 0; i < 8; i++ {
		if got, want := sliceBitLCP([]byte{0x99, 0x99}, []byte{0x99, 0x99}, uint32(i)), uint32(16-i); got != want {
			t.Fatalf("unexpected sliceBitLCP result: got %d, want %d", got, want)
		}
	}
	for i := 0; i < 8; i++ {
		if got, want := sliceBitLCP([]byte{0x99, 0x99}, []byte{0x99, 0xf9}, uint32(i)), uint32(9-i); got != want {
			t.Fatalf("unexpected sliceBitLCP result: got %d, want %d", got, want)
		}
	}

	for i := 0; i < 1; i++ {
		if got, want := sliceBitLCP([]byte{0x99, 0x99}, []byte{0xd9, 0xf9}, uint32(i)), uint32(1-i); got != want {
			t.Fatalf("unexpected sliceBitLCP result: got %d, want %d", got, want)
		}
	}
	for i := 2; i < 8; i++ {
		if got, want := sliceBitLCP([]byte{0x99, 0x99}, []byte{0xd9, 0xf9}, uint32(i)), uint32(9-i); got != want {
			t.Fatalf("unexpected sliceBitLCP result: got %d, want %d", got, want)
		}
	}
}
