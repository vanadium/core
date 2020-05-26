// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uniqueid

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestNewID(t *testing.T) {
	g := RandomGenerator{}
	numHitMaxCount := 2
	maxCount := 0x7fff
	firstRandomValue := []byte{}
	var atLeastOneDiffersFromFirst bool
	for i := 0; i < numHitMaxCount*maxCount; i++ {
		id, err := g.NewID()
		if err != nil {
			t.Fatal("Error generating new id: ", err)
		}

		if firstRandomValue == nil {
			firstRandomValue = id[:14]
		}
		if !bytes.Equal(firstRandomValue, id[:14]) {
			atLeastOneDiffersFromFirst = true
		}

		if id[14]&0x80 != 0x80 {
			t.Errorf("Expected high bit to be 1, but containing byte was: %x", id[14])
		}
		if binary.BigEndian.Uint16(id[14:])&0x7fff != uint16(i)&0x7fff {
			t.Errorf("Counts don't match. Got: %d, Expected: %d", binary.BigEndian.Uint16(id[14:])&0x7fff, i&0x7fff)
		}
	}
	if !atLeastOneDiffersFromFirst {
		t.Errorf("Expected at least two of the randomly generated numbers to be different")
	}
}

func TestString(t *testing.T) {
	id := Id{8, 9, 10, 11, 12, 13, 14, 15, 0, 1, 2, 3, 4, 5, 6, 7}
	if got, want := fmt.Sprintf("%v", id), "0x08090a0b0c0d0e0f0001020304050607"; got != want {
		t.Errorf("Got %q, want %q", got, want)
	}
}

func BenchmarkNewIDParallel(b *testing.B) {
	g := RandomGenerator{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := g.NewID(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
