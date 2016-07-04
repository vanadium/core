// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package memstore provides a simple, in-memory implementation of store.Store.
// Since it's a prototype implementation, it makes no attempt to be performant.
package memstore

import (
	"fmt"
	"sync"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/ptrie"
	"v.io/x/ref/services/syncbase/store/transactions"
)

type memstore struct {
	mu   sync.Mutex
	node *store.ResourceNode
	data *ptrie.T
	err  error
}

// New creates a new memstore.
func New() store.Store {
	return transactions.Wrap(&memstore{
		node: store.NewResourceNode(),
		data: ptrie.New(true),
	})
}

// Close implements the store.Store interface.
func (st *memstore) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return store.ConvertError(st.err)
	}
	st.node.Close()
	st.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgClosedStore)
	return nil
}

// Get implements the store.StoreReader interface.
func (st *memstore) Get(key, valbuf []byte) ([]byte, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return valbuf, store.ConvertError(st.err)
	}
	value := st.data.Get(key)
	if value == nil {
		return valbuf, verror.New(store.ErrUnknownKey, nil, string(key))
	}
	return store.CopyBytes(valbuf, value.([]byte)), nil
}

// Scan implements the store.StoreReader interface.
func (st *memstore) Scan(start, limit []byte) store.Stream {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return &store.InvalidStream{Error: st.err}
	}
	return newStream(st.data.Copy(), st.node, start, limit)
}

// NewSnapshot implements the store.Store interface.
func (st *memstore) NewSnapshot() store.Snapshot {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return &store.InvalidSnapshot{Error: st.err}
	}
	return newSnapshot(st.data.Copy(), st.node)
}

// WriteBatch implements the transactions.BatchStore interface.
func (st *memstore) WriteBatch(batch ...transactions.WriteOp) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.err != nil {
		return store.ConvertError(st.err)
	}
	for _, write := range batch {
		switch write.T {
		case transactions.PutOp:
			st.data.Put(write.Key, store.CopyBytes(nil, write.Value))
		case transactions.DeleteOp:
			st.data.Delete(write.Key)
		default:
			panic(fmt.Sprintf("unknown write operation type: %v", write.T))
		}
	}
	return nil
}
