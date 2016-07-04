// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"math"
	"sync"

	"v.io/v23/context"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/store"
)

type Transaction struct {
	itx store.Transaction
	St  *Store
	mu  sync.Mutex // protects the fields below
	err error
	ops []interface{}
	// fromSync is true when a transaction is created by sync.  This causes
	// the log entries written at commit time to have their "FromSync" field
	// set to true.  That in turn causes the sync watcher to filter out such
	// updates since sync already knows about them (echo suppression).
	fromSync bool
}

var _ store.Transaction = (*Transaction)(nil)

func newTransaction(st *Store) *Transaction {
	return &Transaction{
		itx: st.ist.NewTransaction(),
		St:  st,
	}
}

// Get implements the store.StoreReader interface.
func (tx *Transaction) Get(key, valbuf []byte) ([]byte, error) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return valbuf, convertError(tx.err)
	}
	var err error
	if !tx.St.managesKey(key) {
		valbuf, err = tx.itx.Get(key, valbuf)
	} else {
		valbuf, err = getVersioned(tx.itx, key, valbuf)
		tx.ops = append(tx.ops, &GetOp{Key: append([]byte{}, key...)})
	}
	return valbuf, err
}

// Scan implements the store.StoreReader interface.
func (tx *Transaction) Scan(start, limit []byte) store.Stream {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return &store.InvalidStream{Error: tx.err}
	}
	var it store.Stream
	if !tx.St.managesRange(start, limit) {
		it = tx.itx.Scan(start, limit)
	} else {
		it = newStreamVersioned(tx.itx, start, limit)
		tx.ops = append(tx.ops, &ScanOp{
			Start: append([]byte{}, start...),
			Limit: append([]byte{}, limit...),
		})
	}
	return it
}

// Put implements the store.StoreWriter interface.
func (tx *Transaction) Put(key, value []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}
	if !tx.St.managesKey(key) {
		return tx.itx.Put(key, value)
	}
	version, err := putVersioned(tx.itx, key, value)
	if err != nil {
		return err
	}
	tx.ops = append(tx.ops, &PutOp{Key: append([]byte{}, key...), Version: version})
	return nil
}

// Delete implements the store.StoreWriter interface.
func (tx *Transaction) Delete(key []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}
	var err error
	if !tx.St.managesKey(key) {
		return tx.itx.Delete(key)
	}
	err = deleteVersioned(tx.itx, key)
	if err != nil {
		return err
	}
	tx.ops = append(tx.ops, &DeleteOp{Key: append([]byte{}, key...)})
	return nil
}

// Commit implements the store.Transaction interface.
func (tx *Transaction) Commit() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}
	// Note: ErrMsgCommittedTxn is used to prevent clients from calling Commit
	// more than once on a given transaction.
	tx.err = verror.New(verror.ErrBadState, nil, store.ErrMsgCommittedTxn)
	tx.St.mu.Lock()
	defer tx.St.mu.Unlock()
	// NOTE(sadovsky): It's not clear whether we should call Now() under the
	// tx.st.mu lock (there are pros and cons). However, we plan to add a caching
	// layer to VClock, at which point Now() will be near-instant.
	now, err := tx.St.Clock.Now()
	if err != nil {
		return convertError(err)
	}
	// Check if there is enough space left in the sequence number.
	if (math.MaxUint64 - tx.St.seq) < uint64(len(tx.ops)) {
		return verror.New(verror.ErrInternal, nil, "seq maxed out")
	}
	// Write LogEntry records.
	seq := tx.St.seq
	for i, op := range tx.ops {
		key := logEntryKey(seq)
		value := &LogEntry{
			Op:              vom.RawBytesOf(op),
			CommitTimestamp: now.UnixNano(),
			FromSync:        tx.fromSync,
			Continued:       i < len(tx.ops)-1,
		}
		if err := store.Put(nil, tx.itx, key, value); err != nil {
			return err
		}
		seq++
	}
	if err := tx.itx.Commit(); err != nil {
		return err
	}
	tx.St.seq = seq
	tx.St.watcher.broadcastUpdates()
	return nil
}

// Abort implements the store.Transaction interface.
func (tx *Transaction) Abort() error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}
	tx.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgAbortedTxn)
	return tx.itx.Abort()
}

// SetTransactionFromSync marks this transaction as created by sync as opposed
// to one created by an application.  The net effect is that, at commit time,
// the log entries written are marked as made by sync.  This allows the sync
// Watcher to ignore them (echo suppression) because it made these updates.
// Note: this is an internal function used by sync, not part of the interface.
// TODO(rdaoud): support a generic echo-suppression mechanism for apps as well
// maybe by having a creator ID in the transaction and log entries.
// TODO(rdaoud): fold this flag (or creator ID) into Tx options when available.
// TODO(razvanm): move to syncbase side by using a generic annotation mechanism.
func SetTransactionFromSync(tx *Transaction) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	tx.fromSync = true
}

// GetVersion returns the current version of a managed key. This method is used
// by the Sync module when the initiator is attempting to add new versions of
// objects. Reading the version key is used for optimistic concurrency
// control. At minimum, an object implementing the StoreReader interface is
// required since this is a Get operation.
// TODO(razvanm): find a way to get rid of the type switch.
func GetVersion(ctx *context.T, st store.StoreReader, key []byte) ([]byte, error) {
	switch w := st.(type) {
	case *snapshot:
		return getVersion(w.isn, key)
	case *Transaction:
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.err != nil {
			return nil, convertError(w.err)
		}
		return getVersion(w.itx, key)
	case *Store:
		return getVersion(w.ist, key)
	}
	return nil, verror.New(verror.ErrInternal, ctx, "unsupported store type")
}

// GetAtVersion returns the value of a managed key at the requested
// version. This method is used by the Sync module when the responder needs to
// send objects over the wire. At minimum, an object implementing the
// StoreReader interface is required since this is a Get operation.
// TODO(razvanm): find a way to get rid of the type switch.
func GetAtVersion(ctx *context.T, st store.StoreReader, key, valbuf, version []byte) ([]byte, error) {
	switch w := st.(type) {
	case *snapshot:
		return getAtVersion(w.isn, key, valbuf, version)
	case *Transaction:
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.err != nil {
			return valbuf, convertError(w.err)
		}
		return getAtVersion(w.itx, key, valbuf, version)
	case *Store:
		return getAtVersion(w.ist, key, valbuf, version)
	}
	return nil, verror.New(verror.ErrInternal, ctx, "unsupported store type")
}

// PutAtVersion puts a value for the managed key at the requested version. This
// method is used by the Sync module exclusively when the initiator adds objects
// with versions created on other Syncbases. At minimum, an object implementing
// the Transaction interface is required since this is a Put operation.
func PutAtVersion(ctx *context.T, tx *Transaction, key, valbuf, version []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}

	// Note that we do not enqueue a PutOp in the log since this Put is not
	// updating the current version of a key.
	return tx.itx.Put(makeAtVersionKey(key, version), valbuf)
}

// PutVersion updates the version of a managed key to the requested
// version. This method is used by the Sync module exclusively when the
// initiator selects which of the already stored versions (via PutAtVersion
// calls) becomes the current version. At minimum, an object implementing
// the Transaction interface is required since this is a Put operation.
func PutVersion(ctx *context.T, tx *Transaction, key, version []byte) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}

	if err := tx.itx.Put(makeVersionKey(key), version); err != nil {
		return err
	}
	tx.ops = append(tx.ops, &PutOp{
		Key:     append([]byte{}, key...),
		Version: append([]byte{}, version...),
	})
	return nil
}

// ManagesKey returns true if the store used by a transaction manages a
// particular key.
func ManagesKey(tx *Transaction, key []byte) bool {
	return tx.St.managesKey(key)
}

// AddOp provides a generic way to add an arbitrary op to the log. If precond is
// not nil it will be run with the locks held and the append will only happen if
// the precond returns an error.
func AddOp(tx *Transaction, op interface{}, precond func() error) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.err != nil {
		return convertError(tx.err)
	}
	if precond != nil {
		if err := precond(); err != nil {
			return err
		}
	}
	tx.ops = append(tx.ops, op)
	return nil
}
