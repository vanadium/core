// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions

import (
	"container/list"
	"fmt"
	"sync"

	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/ptrie"
)

type isDeleted struct{}

// transaction is a wrapper on top of a BatchWriter and a store.Snapshot that
// implements the store.Transaction interface.
type transaction struct {
	// mu protects the state of the transaction.
	mu       sync.Mutex
	mg       *manager
	seq      uint64
	event    *list.Element // pointer to element of mg.events
	snapshot store.Snapshot
	reads    readSet
	// writes holds in-flight mutations of the transaction.
	// writes holds key-value pairs where the value type can be:
	//   isDeleted: the last modification of the row was Delete;
	//   []byte: the last modification of the row was Put, value holds
	//           the actual value that was put.
	writes *ptrie.T
	err    error
}

var _ store.Transaction = (*transaction)(nil)

// newTransaction creates a new transaction and adds it to the mg.events queue.
// Assumes mg.mu is held.
func newTransaction(mg *manager) *transaction {
	tx := &transaction{
		mg:       mg,
		snapshot: mg.BatchStore.NewSnapshot(),
		seq:      mg.seq,
		writes:   ptrie.New(true),
	}
	tx.event = mg.events.PushBack(tx)
	return tx
}

// removeEvent removes this transaction from the mg.events queue.
// Assumes mu and mg.mu are held.
func (tx *transaction) removeEvent() {
	if tx.event != nil {
		tx.mg.events.Remove(tx.event)
		tx.event = nil
	}
}

// Get implements the store.StoreReader interface.
func (tx *transaction) Get(key, valbuf []byte) ([]byte, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return valbuf, store.ConvertError(tx.err)
	}
	key = store.CopyBytes(nil, key)
	tx.reads.Keys = append(tx.reads.Keys, key)
	if value := tx.writes.Get(key); value != nil {
		switch bytes := value.(type) {
		case []byte:
			return store.CopyBytes(valbuf, bytes), nil
		case isDeleted:
			return valbuf, verror.New(store.ErrUnknownKey, nil, string(key))
		default:
			panic(fmt.Sprintf("unexpected type %T of value", bytes))
		}
	}
	return tx.snapshot.Get(key, valbuf)
}

// Scan implements the store.StoreReader interface.
func (tx *transaction) Scan(start, limit []byte) store.Stream {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return &store.InvalidStream{Error: tx.err}
	}
	start, limit = store.CopyBytes(nil, start), store.CopyBytes(nil, limit)
	tx.reads.Ranges = append(tx.reads.Ranges, scanRange{
		Start: start,
		Limit: limit,
	})
	// Return a stream which merges the snaphot stream with the uncommitted changes.
	return mergeWritesWithStream(tx.snapshot, tx.writes.Copy(), start, limit)
}

// Put implements the store.StoreWriter interface.
func (tx *transaction) Put(key, value []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return store.ConvertError(tx.err)
	}
	tx.writes.Put(key, value)
	return nil
}

// Delete implements the store.StoreWriter interface.
func (tx *transaction) Delete(key []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return store.ConvertError(tx.err)
	}
	tx.writes.Put(key, isDeleted{})
	return nil
}

// validateReadSet returns true iff the read set of this transaction has not
// been invalidated by other transactions.
// Assumes tx.mg.mu is held.
func (tx *transaction) validateReadSet() bool {
	for _, key := range tx.reads.Keys {
		if tx.mg.txTable.get(key) > tx.seq {
			vlog.VI(3).Infof("key conflict: %q", key)
			return false
		}
	}
	for _, r := range tx.reads.Ranges {
		if tx.mg.txTable.rangeMax(r.Start, r.Limit) > tx.seq {
			vlog.VI(3).Infof("range conflict: {%q, %q}", r.Start, r.Limit)
			return false
		}

	}
	return true
}

// Commit implements the store.Transaction interface.
func (tx *transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return store.ConvertError(tx.err)
	}
	tx.err = verror.New(verror.ErrBadState, nil, store.ErrMsgCommittedTxn)
	tx.snapshot.Abort()
	tx.mg.mu.Lock()
	defer tx.mg.mu.Unlock()
	if tx.mg.txTable == nil {
		return verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore)
	}
	// Explicitly remove this transaction from the event queue. If this was the
	// only active transaction, the event queue becomes empty and trackBatch will
	// not add this transaction's write set to txTable.
	tx.removeEvent()
	if !tx.validateReadSet() {
		return store.NewErrConcurrentTransaction(nil)
	}
	var batch []WriteOp
	s := tx.writes.Scan(nil, nil)
	for s.Advance() {
		switch bytes := s.Value().(type) {
		case []byte:
			batch = append(batch, WriteOp{T: PutOp, Key: s.Key(nil), Value: bytes})
		case isDeleted:
			batch = append(batch, WriteOp{T: DeleteOp, Key: s.Key(nil)})
		default:
			panic(fmt.Sprintf("unexpected type %T of value", bytes))
		}
	}
	if err := tx.mg.BatchStore.WriteBatch(batch...); err != nil {
		return err
	}
	tx.mg.trackBatch(batch...)
	return nil
}

// Abort implements the store.Transaction interface.
func (tx *transaction) Abort() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return store.ConvertError(tx.err)
	}
	tx.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgAbortedTxn)
	tx.snapshot.Abort()
	tx.mg.mu.Lock()
	tx.removeEvent()
	tx.mg.mu.Unlock()
	return nil
}
