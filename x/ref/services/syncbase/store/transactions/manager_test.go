// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions

import (
	"container/list"
	"testing"

	"v.io/x/ref/services/syncbase/store"
)

// package syncbase/store/memstore imports syncbase/transactions, so we can't
// use memstore as the underlying BatchStore.  Instead, we use a mockBatchStore
// with all methods stubbed out.
type mockBatchStore struct{}

func (mbs mockBatchStore) Get(key, valbuf []byte) ([]byte, error) {
	return nil, nil
}

func (mbs mockBatchStore) Scan(start, limit []byte) store.Stream {
	return nil
}

func (mbs mockBatchStore) WriteBatch(batch ...WriteOp) error {
	return nil
}

func (mbs mockBatchStore) Close() error {
	return nil
}

func (mbs mockBatchStore) NewSnapshot() store.Snapshot {
	return mockSnapshot{}
}

type mockSnapshot struct {
	*store.SnapshotSpecImpl
}

func (ms mockSnapshot) Get(key, valbuf []byte) ([]byte, error) {
	return nil, nil
}

func (ms mockSnapshot) Scan(start, limit []byte) store.Stream {
	return nil
}

func (ms mockSnapshot) Abort() error {
	return nil
}

// TestGcTransactions tests that calling gcTransactions() garbage collects all
// committed transactions.
func TestGcTransactions(t *testing.T) {
	mg := &manager{
		BatchStore: mockBatchStore{},
		events:     list.New(),
		txTable:    newTrie(),
	}

	// Make a Put to the DB.  Since there are no open transactions this should
	// not add to the events queue.
	if err := mg.Put([]byte("foo"), []byte("foo")); err != nil {
		t.Fatal(err)
	}
	if want, got := 0, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Start a new transaction and leave it open.  This will add a single event
	// to the queue.
	tx1 := mg.NewTransaction()
	if want, got := 1, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Make two more Puts to the DB, which will add a two events to the queue.
	if err := mg.Put([]byte("bar"), []byte("bar")); err != nil {
		t.Fatal(err)
	}
	if err := mg.Put([]byte("baz"), []byte("baz")); err != nil {
		t.Fatal(err)
	}
	if want, got := 3, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Garbage collection should not remove anything, since tx1 is still open.
	mg.gcTransactions()
	if want, got := 3, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Start a second transaction and leave it open.  This will add a single event
	// to the queue.
	tx2 := mg.NewTransaction()
	if want, got := 4, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Do a Put and Commit on tx1.  tx1 will be removed from the queue, but a
	// commit event added to the end, so the length is not changed.
	if err := tx1.Put([]byte("qux"), []byte("qux")); err != nil {
		t.Fatal(err)
	}
	if err := tx1.Commit(); err != nil {
		t.Fatal(err)
	}
	if want, got := 4, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Garbage collection should remove the two recent Puts.
	mg.gcTransactions()
	if want, got := 2, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// Abort tx2, which will remove it from the queue.
	if err := tx2.Abort(); err != nil {
		t.Fatal(err)
	}
	if want, got := 1, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}

	// A final garbage collection will remove the commit event for tx1.
	mg.gcTransactions()
	if want, got := 0, mg.events.Len(); want != got {
		t.Errorf("wanted mg.events.Len() to be %v but got %v", want, got)
	}
}
