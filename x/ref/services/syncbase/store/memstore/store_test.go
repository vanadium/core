// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memstore

import (
	"runtime"
	"testing"

	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/test"
)

func init() {
	runtime.GOMAXPROCS(10)
}

func TestStream(t *testing.T) {
	runTest(t, test.RunStreamTest)
}

func TestSnapshot(t *testing.T) {
	runTest(t, test.RunSnapshotTest)
}

func TestStoreState(t *testing.T) {
	runTest(t, test.RunStoreStateTest)
}

func TestClose(t *testing.T) {
	runTest(t, test.RunCloseTest)
}

func TestReadWriteBasic(t *testing.T) {
	runTest(t, test.RunReadWriteBasicTest)
}

func TestReadWriteRandom(t *testing.T) {
	runTest(t, test.RunReadWriteRandomTest)
}

func TestConcurrentTransactions(t *testing.T) {
	runTest(t, test.RunConcurrentTransactionsTest)
}

func TestTransactionState(t *testing.T) {
	runTest(t, test.RunTransactionStateTest)
}

func TestTransactionsWithGet(t *testing.T) {
	runTest(t, test.RunTransactionsWithGetTest)
}

func runTest(t *testing.T, f func(t *testing.T, st store.Store)) {
	st := New()
	defer st.Close()
	f(t, st)
}
