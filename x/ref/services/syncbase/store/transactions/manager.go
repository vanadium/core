// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
)

// txGcInterval is the interval between transaction garbage collections.
// TODO(nlacasse): Profile and optimize this value.
const txGcInterval = 100 * time.Millisecond

// BatchStore is a CRUD-capable storage engine that supports atomic batch
// writes. BatchStore doesn't support transactions.
// This interface is a Go version of the C++ LevelDB interface. It serves as
// an intermediate interface between store.Store and the LevelDB interface.
type BatchStore interface {
	store.StoreReader

	// WriteBatch atomically writes a list of write operations to the database.
	WriteBatch(batch ...WriteOp) error

	// Close closes the store.
	Close() error

	// NewSnapshot creates a snapshot.
	NewSnapshot() store.Snapshot
}

// manager handles transaction-related operations of the store.
type manager struct {
	BatchStore
	// stopTxGc is used to stop garbage collecting transactions.
	stopTxGc func()
	// mu protects the variables below, and is also held during transaction
	// commits. It must always be acquired before the store-level lock.
	mu sync.Mutex
	// events is a queue of create/commit transaction events.  Events are
	// pushed to the back of the queue, and removed from the front via GC.
	events *list.List
	seq    uint64
	// txTable is a set of keys written by recent transactions. This set
	// includes all write sets of transactions committed after the oldest living
	// (in-flight) transaction.
	txTable *trie
}

// commitedTransaction is only used as an element of manager.events.
type commitedTransaction struct {
	seq   uint64
	batch [][]byte
}

// Wrap wraps the BatchStore with transaction functionality.
func Wrap(bs BatchStore) store.Store {
	mg := &manager{
		BatchStore: bs,
		events:     list.New(),
		txTable:    newTrie(),
	}

	// Start a goroutine that garbage collects transactions every txGcInterval.
	t := time.NewTicker(txGcInterval)
	mg.stopTxGc = t.Stop
	go func() {
		for range t.C {
			mg.gcTransactions()
		}
	}()

	return mg
}

// Close implements the store.Store interface.
func (mg *manager) Close() error {
	mg.stopTxGc()
	mg.mu.Lock()
	if mg.txTable == nil {
		mg.mu.Unlock()
		return verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore)
	}
	mg.BatchStore.Close()
	events := mg.events
	mg.events = nil
	mg.txTable = nil
	// tx.Abort() internally locks mg.mu.
	mg.mu.Unlock()
	for event := events.Front(); event != nil; event = event.Next() {
		if tx, ok := event.Value.(*transaction); ok {
			tx.Abort()
		}
	}
	return nil
}

// NewTransaction implements the store.Store interface.
func (mg *manager) NewTransaction() store.Transaction {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if mg.txTable == nil {
		return &store.InvalidTransaction{
			Error: verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore),
		}
	}
	return newTransaction(mg)
}

// Put implements the store.StoreWriter interface.
func (mg *manager) Put(key, value []byte) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if mg.txTable == nil {
		return verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore)
	}
	write := WriteOp{
		T:     PutOp,
		Key:   key,
		Value: value,
	}
	if err := mg.BatchStore.WriteBatch(write); err != nil {
		return err
	}
	mg.trackBatch(write)
	return nil
}

// Delete implements the store.StoreWriter interface.
func (mg *manager) Delete(key []byte) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if mg.txTable == nil {
		return verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore)
	}
	write := WriteOp{
		T:   DeleteOp,
		Key: key,
	}
	if err := mg.BatchStore.WriteBatch(write); err != nil {
		return err
	}
	mg.trackBatch(write)
	return nil
}

// trackBatch writes the batch to txTable and adds a commit event to
// the events queue.
// Assumes mu is held.
func (mg *manager) trackBatch(batch ...WriteOp) {
	if mg.events.Len() == 0 {
		return
	}
	mg.seq++
	var keys [][]byte
	for _, write := range batch {
		mg.txTable.add(write.Key, mg.seq)
		keys = append(keys, write.Key)
	}
	tx := &commitedTransaction{
		seq:   mg.seq,
		batch: keys,
	}
	mg.events.PushBack(tx)
}

// gcTransactions cleans up all transactions that were commited before the
// first open (non-commited) transaction.  Transactions are cleaned up in the
// order that they were commited.  Note that this function holds mg.mu,
// preventing other transaction operations from occurring while it runs.
//
// TODO(nlacasse): If a transaction never commits or aborts, then we will never
// be able to GC any transaction that occurs after it.  Consider aborting any
// transaction that has been open longer than a maximum amount of time.
func (mg *manager) gcTransactions() {
	mg.mu.Lock()
	defer mg.mu.Unlock()
	if mg.events == nil {
		return
	}
	ev := mg.events.Front()
	for ev != nil {
		switch tx := ev.Value.(type) {
		case *transaction:
			return
		case *commitedTransaction:
			for _, batch := range tx.batch {
				mg.txTable.remove(batch, tx.seq)
			}
			next := ev.Next()
			mg.events.Remove(ev)
			ev = next
		default:
			panic(fmt.Sprintf("unknown event type: %T", tx))
		}
	}
}

//////////////////////////////////////////////////////////////
// Read and Write types used for storing transaction reads
// and uncommitted writes.

type WriteType int

const (
	PutOp WriteType = iota
	DeleteOp
)

type WriteOp struct {
	T     WriteType
	Key   []byte
	Value []byte
}

type scanRange struct {
	Start, Limit []byte
}

type readSet struct {
	Keys   [][]byte
	Ranges []scanRange
}
