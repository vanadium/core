// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package watchable

import (
	"sync"

	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/store"
)

// stream streams keys and values for versioned records.
type stream struct {
	iit      store.Stream
	sntx     store.SnapshotOrTransaction
	mu       sync.Mutex
	err      error
	hasValue bool
	key      []byte
	value    []byte
}

var _ store.Stream = (*stream)(nil)

// newStreamVersioned creates a new stream. It assumes all records in range
// [start, limit) are managed, i.e. versioned.
func newStreamVersioned(sntx store.SnapshotOrTransaction, start, limit []byte) *stream {
	return &stream{
		iit:  sntx.Scan(makeVersionKey(start), makeVersionKey(limit)),
		sntx: sntx,
	}
}

// Advance implements the store.Stream interface.
func (s *stream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasValue = false
	if s.err != nil {
		return false
	}
	if advanced := s.iit.Advance(); !advanced {
		return false
	}
	versionKey, version := s.iit.Key(nil), s.iit.Value(nil)
	s.key = []byte(common.StripFirstKeyPartOrDie(string(versionKey))) // drop "$version" prefix
	s.value, s.err = s.sntx.Get(makeAtVersionKey(s.key, version), nil)
	if s.err != nil {
		return false
	}
	s.hasValue = true
	return true
}

// Key implements the store.Stream interface.
func (s *stream) Key(keybuf []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasValue {
		panic("nothing staged")
	}
	return store.CopyBytes(keybuf, s.key)
}

// Value implements the store.Stream interface.
func (s *stream) Value(valbuf []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasValue {
		panic("nothing staged")
	}
	return store.CopyBytes(valbuf, s.value)
}

// Err implements the store.Stream interface.
func (s *stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return convertError(s.err)
	}
	return s.iit.Err()
}

// Cancel implements the store.Stream interface.
func (s *stream) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	s.iit.Cancel()
}
