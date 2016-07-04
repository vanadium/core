// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions_test

import (
	"bytes"
	"testing"

	"v.io/x/ref/services/syncbase/store/memstore"
)

// TestMergedStream checks correctness of the mergedStream.
func TestMergedStream(t *testing.T) {
	// Here is the test structure: we take N keys and run through all possible
	// states of store with 3 keys and all possible transactions that
	// involve 3 keys. For store key there are two states: exists and doesn't
	// exist. For a transaction key there are three states: not in transaction,
	// put and delete.
	keys := []string{"a", "b", "c", "d"}
	N := len(keys)
	n2, n3 := 1, 1 // 2**N and 3**N
	for i := 0; i < N; i++ {
		n2, n3 = n2*2, n3*3
	}
	for storeState := 0; storeState < n2; storeState++ {
		for txState := 0; txState < n3; txState++ {
			var expected []string
			st := memstore.New()
			ss, ts := storeState, txState
			// Populate the store and the expected result.
			for i := 0; i < N; i++ {
				if ss%2 == 1 {
					st.Put([]byte(keys[i]), []byte(keys[i]))
				}
				if (ss%2 == 1 || ts%3 == 1) && (ts%3 != 2) {
					expected = append(expected, keys[i])
				}
				ss /= 2
				ts /= 3
			}
			// Create a transaction.
			tx := st.NewTransaction()
			ts = txState
			for i := 0; i < N; i++ {
				if ts%3 == 1 {
					tx.Put([]byte(keys[i]), []byte(keys[i]))
				} else if ts%3 == 2 {
					tx.Delete([]byte(keys[i]))
				}
				ts /= 3
			}
			s := tx.Scan(nil, nil)
			for i := 0; i < len(expected); i++ {
				if !s.Advance() {
					t.Fatal("the stream didn't advance")
				}
				if got, want := s.Key(nil), []byte(expected[i]); !bytes.Equal(got, want) {
					t.Fatalf("unexpected key: got %q, want %q", got, want)
				}
				if got, want := s.Value(nil), []byte(expected[i]); !bytes.Equal(got, want) {
					t.Fatalf("unexpected value: got %q, want %q", got, want)
				}
			}
			if s.Advance() {
				t.Fatal("the stream advanced")
			}
			if err := s.Err(); err != nil {
				t.Fatalf("unexpected stream error: %v", err)
			}
		}
	}
}
