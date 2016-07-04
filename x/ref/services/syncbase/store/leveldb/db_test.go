// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb

import (
	"fmt"
	"io/ioutil"
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

func TestOpenOptions(t *testing.T) {
	path, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		t.Fatalf("can't create temp dir: %v", err)
	}
	// DB is missing => call should fail.
	st, err := Open(path, OpenOptions{CreateIfMissing: false, ErrorIfExists: false})
	if err == nil {
		t.Fatalf("open should've failed")
	}
	// DB is missing => call should succeed.
	st, err = Open(path, OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	st.Close()
	// DB exists => call should succeed.
	st, err = Open(path, OpenOptions{CreateIfMissing: false, ErrorIfExists: false})
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	st.Close()
	// DB exists => call should fail.
	st, err = Open(path, OpenOptions{CreateIfMissing: false, ErrorIfExists: true})
	if err == nil {
		t.Fatalf("open should've failed")
	}
	// DB exists => call should fail.
	st, err = Open(path, OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err == nil {
		t.Fatalf("open should've failed")
	}
	// DB exists => call should succeed.
	st, err = Open(path, OpenOptions{CreateIfMissing: true, ErrorIfExists: false})
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	st.Close()
	if err := Destroy(path); err != nil {
		t.Fatalf("destroy failed: %v", err)
	}
}

func runTest(t *testing.T, f func(t *testing.T, st store.Store)) {
	st, dbPath := newDB()
	defer destroyDB(st, dbPath)
	f(t, st)
}

func newDB() (store.Store, string) {
	path, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		panic(fmt.Sprintf("can't create temp dir: %v", err))
	}
	st, err := Open(path, OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err != nil {
		panic(fmt.Sprintf("can't open db at %v: %v", path, err))
	}
	return st, path
}

func destroyDB(st store.Store, path string) {
	st.Close()
	if err := Destroy(path); err != nil {
		panic(fmt.Sprintf("can't destroy db at %v: %v", path, err))
	}
}
