// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A test for crc64window.
package crc64window_test

import "hash/crc64"
import "math/rand"
import "testing"

import "v.io/x/ref/services/syncbase/localblobstore/crc64window"

// A test for the example given in the package's specification.
func TestCRC64WindowExample(t *testing.T) {
	w := crc64window.New(crc64.ECMA, 3)
	w.Advance(0x17)
	w.Advance(0x92)
	w.Advance(0x04)
	rolling := w.Advance(0x28) // Rolls 0x17 out, and 0x28 in.
	nonRolling := crc64.Update(0, crc64.MakeTable(crc64.ECMA), []byte{0x92, 0x04, 0x28})
	if rolling != nonRolling {
		t.Errorf("crc64window: rolling(0x92, 0x04, 0x28)==%x nonRolling(0x92, 0x04, 0x28)==%x\n", rolling, nonRolling)
	}
}

func TestCRC64Window(t *testing.T) {
	winSize := 16
	iterations := 1000

	w := crc64window.New(crc64.ECMA, winSize)

	table := crc64.MakeTable(crc64.ECMA)
	block := make([]byte, winSize-1+iterations)

	for i := 0; i != len(block); i++ {
		block[i] = byte(rand.Int31n(256))
	}

	i := 0
	for ; i != winSize-1; i++ {
		w.Advance(block[i])
	}
	for ; i != len(block); i++ {
		expect := crc64.Update(0, table, block[i+1-winSize:i+1])
		got := w.Advance(block[i])
		if expect != got {
			t.Errorf("crc64window: i %d   winSize %d  got %x, expect %x\n", i, winSize, got, expect)
		}
	}
}
