// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memstore

import (
	"sync"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/ptrie"
)

type snapshot struct {
	store.SnapshotSpecImpl
	mu   sync.Mutex
	node *store.ResourceNode
	data *ptrie.T
	err  error
}

var _ store.Snapshot = (*snapshot)(nil)

// Assumes st lock is held.
func newSnapshot(data *ptrie.T, parent *store.ResourceNode) *snapshot {
	s := &snapshot{
		node: store.NewResourceNode(),
		data: data,
	}
	parent.AddChild(s.node, func() {
		s.Abort()
	})
	return s
}

// Abort implements the store.Snapshot interface.
func (s *snapshot) Abort() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return store.ConvertError(s.err)
	}
	s.node.Close()
	s.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgAbortedSnapshot)
	return nil
}

// Get implements the store.StoreReader interface.
func (s *snapshot) Get(key, valbuf []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return valbuf, store.ConvertError(s.err)
	}
	value := s.data.Get(key)
	if value == nil {
		return valbuf, verror.New(store.ErrUnknownKey, nil, string(key))
	}
	return store.CopyBytes(valbuf, value.([]byte)), nil
}

// Scan implements the store.StoreReader interface.
func (s *snapshot) Scan(start, limit []byte) store.Stream {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return &store.InvalidStream{Error: s.err}
	}
	return newStream(s.data, s.node, start, limit)
}
