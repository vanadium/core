// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"v.io/x/ref/services/syncbase/store"
)

type snapshot struct {
	store.SnapshotSpecImpl
	isn store.Snapshot
	st  *Store
}

var _ store.Snapshot = (*snapshot)(nil)

func newSnapshot(st *Store) *snapshot {
	return &snapshot{
		isn: st.ist.NewSnapshot(),
		st:  st,
	}
}

// Abort implements the store.Snapshot interface.
func (s *snapshot) Abort() error {
	return s.isn.Abort()
}

// Get implements the store.StoreReader interface.
func (s *snapshot) Get(key, valbuf []byte) ([]byte, error) {
	if !s.st.managesKey(key) {
		return s.isn.Get(key, valbuf)
	}
	return getVersioned(s.isn, key, valbuf)
}

// Scan implements the store.StoreReader interface.
func (s *snapshot) Scan(start, limit []byte) store.Stream {
	if !s.st.managesRange(start, limit) {
		return s.isn.Scan(start, limit)
	}
	return newStreamVersioned(s.isn, start, limit)
}
