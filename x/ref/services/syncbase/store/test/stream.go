// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"testing"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
)

// RunStreamTest verifies store.Stream operations.
func RunStreamTest(t *testing.T, st store.Store) {
	// Test that advancing or canceling a stream that has reached its end
	// doesn't cause a panic.
	s := st.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, nil, nil)
	verifyAdvance(t, s, nil, nil)
	if s.Err() != nil {
		t.Fatalf("unexpected error: %v", s.Err())
	}
	s.Cancel()
	if s.Err() != nil {
		t.Fatalf("unexpected error: %v", s.Err())
	}

	key1, value1 := []byte("key1"), []byte("value1")
	st.Put(key1, value1)
	key2, value2 := []byte("key2"), []byte("value2")
	st.Put(key2, value2)
	key3, value3 := []byte("key3"), []byte("value3")
	st.Put(key3, value3)
	s = st.Scan([]byte("a"), []byte("z"))
	verifyAdvance(t, s, key1, value1)
	if !s.Advance() {
		t.Fatalf("can't advance the stream")
	}
	s.Cancel()
	for i := 0; i < 2; i++ {
		var key, value []byte
		if key = s.Key(key); !bytes.Equal(key, key2) {
			t.Fatalf("unexpected key: got %q, want %q", key, key2)
		}
		if value = s.Value(value); !bytes.Equal(value, value2) {
			t.Fatalf("unexpected value: got %q, want %q", value, value2)
		}
	}
	verifyAdvance(t, s, nil, nil)
	verifyError(t, s.Err(), verror.ErrCanceled.ID, store.ErrMsgCanceledStream)
}
