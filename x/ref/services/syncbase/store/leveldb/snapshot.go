// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb

// #include "leveldb/c.h"
import "C"
import (
	"sync"

	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/store"
)

// snapshot is a wrapper around LevelDB snapshot that implements
// the store.Snapshot interface.
type snapshot struct {
	store.SnapshotSpecImpl
	// mu protects the state of the snapshot.
	mu        sync.RWMutex
	node      *store.ResourceNode
	d         *db
	cSnapshot *C.leveldb_snapshot_t
	cOpts     *C.leveldb_readoptions_t
	err       error
}

var _ store.Snapshot = (*snapshot)(nil)

func newSnapshot(d *db, parent *store.ResourceNode) *snapshot {
	cSnapshot := C.leveldb_create_snapshot(d.cDb)
	cOpts := C.leveldb_readoptions_create()
	C.leveldb_readoptions_set_verify_checksums(cOpts, 1)
	C.leveldb_readoptions_set_snapshot(cOpts, cSnapshot)
	s := &snapshot{
		node:      store.NewResourceNode(),
		d:         d,
		cSnapshot: cSnapshot,
		cOpts:     cOpts,
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
	C.leveldb_readoptions_destroy(s.cOpts)
	s.cOpts = nil
	C.leveldb_release_snapshot(s.d.cDb, s.cSnapshot)
	s.cSnapshot = nil
	s.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgAbortedSnapshot)
	return nil
}

// Get implements the store.StoreReader interface.
func (s *snapshot) Get(key, valbuf []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return valbuf, store.ConvertError(s.err)
	}
	return s.d.getWithOpts(key, valbuf, s.cOpts)
}

// Scan implements the store.StoreReader interface.
func (s *snapshot) Scan(start, limit []byte) store.Stream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return &store.InvalidStream{Error: s.err}
	}
	return newStream(s.d, s.node, start, limit, s.cOpts)
}
