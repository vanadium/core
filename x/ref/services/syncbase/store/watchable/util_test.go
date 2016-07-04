// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"testing"

	"v.io/x/ref/services/syncbase/vclock"
)

// TestGetNextLogSeq tests that the getNextLogSeq helper works on range 0..10.
func TestGetNextLogSeq(t *testing.T) {
	st, destroy := createStore()
	defer destroy()
	st, err := Wrap(st, vclock.NewVClockForTests(nil), &Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := uint64(0); i <= uint64(10); i++ {
		seq, err := getNextLogSeq(st)
		if err != nil {
			t.Fatalf("failed to get log seq: %v", err)
		}
		if got, want := seq, i; got != want {
			t.Fatalf("unexpected log seq: got %v, want %v", got, want)
		}
		st.Put([]byte(logEntryKey(i)), nil)
	}
}
