// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"bytes"
	"compress/flate"
	"testing"
)

func compress(id []byte, level int) (len int) {
	var z bytes.Buffer
	f, _ := flate.NewWriter(&z, level)
	f.Write(id) //nolint:errcheck
	f.Close()
	return z.Len()
}

// A simple sanity test to ensure that a new RoutingID at the very least is
// not so buggy as to be returning some easy to compress pattern or
// value. This is not a true test for randomness; just a sanity test.
func TestNew(t *testing.T) {
	rid, _ := NewRoutingID()
	// Determine the framing overhead for flate.
	framing := compress(rid.value[:], 0) - len(rid.value)
	n := 1000
	for i := 0; i < n; i++ {
		rid, _ := NewRoutingID()
		length := compress(rid.value[:], 9)
		if length < (len(rid.value)+framing)*3/4 {
			t.Errorf("Shouldn't be able to compress a random byte array this well (%d bytes compressed to %d)\n", len(rid.value), length-framing)
		}
	}
}
