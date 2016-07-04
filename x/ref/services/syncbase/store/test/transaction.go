// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
)

// RunTransactionStateTest verifies operations that modify the state of a
// store.Transaction.
func RunTransactionStateTest(t *testing.T, st store.Store) {
	finalizeFns := []func(t *testing.T, tx store.Transaction) (verror.ID, string){
		func(t *testing.T, tx store.Transaction) (verror.ID, string) {
			if err := tx.Abort(); err != nil {
				Fatalf(t, "can't abort the transaction: %v", err)
			}
			return verror.ErrCanceled.ID, store.ErrMsgAbortedTxn
		},
		func(t *testing.T, tx store.Transaction) (verror.ID, string) {
			if err := tx.Commit(); err != nil {
				Fatalf(t, "can't commit the transaction: %v", err)
			}
			return verror.ErrBadState.ID, store.ErrMsgCommittedTxn
		},
	}
	for _, fn := range finalizeFns {
		key1, value1 := []byte("key1"), []byte("value1")
		st.Put(key1, value1)
		key2 := []byte("key2")
		tx := st.NewTransaction()

		// Test Get and Scan.
		verifyGet(t, tx, key1, value1)
		verifyGet(t, tx, key2, nil)
		s := tx.Scan([]byte("a"), []byte("z"))
		verifyAdvance(t, s, key1, value1)
		verifyAdvance(t, s, nil, nil)

		// Test Put then Get & Scan inside the transaction.
		key3, value3 := []byte("key3"), []byte("value3")
		tx.Put(key3, value3)
		verifyGet(t, tx, key3, value3)
		s = tx.Scan([]byte("a"), []byte("z"))
		verifyAdvance(t, s, key1, value1)
		verifyAdvance(t, s, key3, value3)
		verifyAdvance(t, s, nil, nil)

		// Test Delete of old key then Get inside the transaction.
		tx.Delete(key1)
		verifyGet(t, tx, key1, nil)

		// Test Delete of new key then Get inside the transaction.
		tx.Delete(key3)
		verifyGet(t, tx, key3, nil)

		// Test functions after finalize.
		expectedID, expectedErrMsg := fn(t, tx)
		verifyError(t, tx.Abort(), expectedID, expectedErrMsg)
		verifyError(t, tx.Commit(), expectedID, expectedErrMsg)

		s = tx.Scan([]byte("a"), []byte("z"))
		verifyAdvance(t, s, nil, nil)
		verifyError(t, s.Err(), expectedID, expectedErrMsg)

		_, err := tx.Get(key1, nil)
		verifyError(t, err, expectedID, expectedErrMsg)
		verifyError(t, tx.Put(key1, value1), expectedID, expectedErrMsg)
		verifyError(t, tx.Delete(key1), expectedID, expectedErrMsg)
	}
}

// RunConcurrentTransactionsTest verifies that concurrent transactions
// invalidate each other as expected.
func RunConcurrentTransactionsTest(t *testing.T, st store.Store) {
	st.Put([]byte("a"), []byte("0"))
	st.Put([]byte("b"), []byte("0"))
	st.Put([]byte("c"), []byte("0"))
	// Test Get fails.
	txA := st.NewTransaction()
	txB := st.NewTransaction()
	txA.Get([]byte("a"), nil)
	txB.Get([]byte("a"), nil)
	txA.Put([]byte("a"), []byte("a"))
	txB.Put([]byte("a"), []byte("b"))
	if err := txA.Commit(); err != nil {
		t.Fatalf("can't commit the transaction: %v", err)
	}
	if err := txB.Commit(); verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
		t.Fatalf("unexpected commit error: %v", err)
	}
	if value, _ := st.Get([]byte("a"), nil); !bytes.Equal(value, []byte("a")) {
		t.Fatalf("unexpected value: got %q, want %q", value, "a")
	}
	// Test Scan fails.
	txA = st.NewTransaction()
	txB = st.NewTransaction()
	txA.Scan([]byte("a"), []byte("z"))
	txB.Scan([]byte("a"), []byte("z"))
	txA.Put([]byte("aa"), []byte("a"))
	txB.Put([]byte("bb"), []byte("b"))
	if err := txA.Commit(); err != nil {
		t.Fatalf("can't commit the transaction: %v", err)
	}
	if err := txB.Commit(); verror.ErrorID(err) != store.ErrConcurrentTransaction.ID {
		t.Fatalf("unexpected commit error: %v", err)
	}
	if value, _ := st.Get([]byte("aa"), nil); !bytes.Equal(value, []byte("a")) {
		t.Fatalf("unexpected value: got %q, want %q", value, "a")
	}
	// Test Get and Scan OK.
	txA = st.NewTransaction()
	txB = st.NewTransaction()
	txA.Scan([]byte("a"), []byte("b"))
	txB.Scan([]byte("b"), []byte("c"))
	txA.Get([]byte("c"), nil)
	txB.Get([]byte("c"), nil)
	txA.Put([]byte("a"), []byte("a"))
	txB.Put([]byte("b"), []byte("b"))
	if err := txA.Commit(); err != nil {
		t.Fatalf("can't commit the transaction: %v", err)
	}
	if err := txB.Commit(); err != nil {
		t.Fatalf("can't commit the transaction: %v", err)
	}
}

// RunTransactionsWithGetTest tests transactions that use Put and Get
// operations.
// NOTE: consider setting GOMAXPROCS to something greater than 1.
func RunTransactionsWithGetTest(t *testing.T, st store.Store) {
	// Invariant: value mapped to n is sum of values of 0..n-1.
	// Each of k transactions takes m distinct random values from 0..n-1, adds 1
	// to each and m to value mapped to n.
	// The correctness of sums is checked after all transactions have been
	// committed.
	n, m, k := 10, 3, 100
	for i := 0; i <= n; i++ {
		if err := st.Put([]byte(fmt.Sprintf("%05d", i)), []byte{'0'}); err != nil {
			t.Fatalf("can't write to database")
		}
	}
	var wg sync.WaitGroup
	wg.Add(k)
	for i := 0; i < k; i++ {
		go func(idx int) {
			rnd := rand.New(rand.NewSource(239017 * int64(idx)))
			perm := rnd.Perm(n)
			if err := store.RunInTransaction(st, func(tx store.Transaction) error {
				for j := 0; j <= m; j++ {
					var keystr string
					if j < m {
						keystr = fmt.Sprintf("%05d", perm[j])
					} else {
						keystr = fmt.Sprintf("%05d", n)
					}
					key := []byte(keystr)
					val, err := tx.Get(key, nil)
					if err != nil {
						return fmt.Errorf("can't get key %q: %v", key, err)
					}
					intValue, err := strconv.ParseInt(string(val), 10, 64)
					if err != nil {
						return fmt.Errorf("can't parse int from %q: %v", val, err)
					}
					var newValue int64
					if j < m {
						newValue = intValue + 1
					} else {
						newValue = intValue + int64(m)
					}
					if err := tx.Put(key, []byte(fmt.Sprintf("%d", newValue))); err != nil {
						return fmt.Errorf("can't put {%q: %v}: %v", key, newValue, err)
					}
				}
				return nil
			}); err != nil {
				panic(fmt.Errorf("can't commit transaction: %v", err))
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	var sum int64
	for j := 0; j <= n; j++ {
		keystr := fmt.Sprintf("%05d", j)
		key := []byte(keystr)
		val, err := st.Get(key, nil)
		if err != nil {
			t.Fatalf("can't get key %q: %v", key, err)
		}
		intValue, err := strconv.ParseInt(string(val), 10, 64)
		if err != nil {
			t.Fatalf("can't parse int from %q: %v", val, err)
		}
		if j < n {
			sum += intValue
		} else {
			if intValue != int64(m*k) {
				t.Fatalf("invalid sum value in the database: got %d, want %d", intValue, m*k)
			}
		}
	}
	if sum != int64(m*k) {
		t.Fatalf("invalid sum of values in the database: got %d, want %d", sum, m*k)
	}
}
