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

type stream struct {
	mu      sync.Mutex
	node    *store.ResourceNode
	pstream *ptrie.Stream
	err     error
	done    bool
}

var _ store.Stream = (*stream)(nil)

func newStream(data *ptrie.T, parent *store.ResourceNode, start, limit []byte) *stream {
	s := &stream{
		node:    store.NewResourceNode(),
		pstream: data.Scan(start, limit),
	}
	parent.AddChild(s.node, func() {
		s.Cancel()
	})
	return s
}

// Advance implements the store.Stream interface.
func (s *stream) Advance() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done {
		return false
	}
	if s.done = !s.pstream.Advance(); s.done {
		s.node.Close()
	}
	return !s.done
}

// Key implements the store.Stream interface.
func (s *stream) Key(keybuf []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pstream.Key(keybuf)
}

// Value implements the store.Stream interface.
func (s *stream) Value(valbuf []byte) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return store.CopyBytes(valbuf, s.pstream.Value().([]byte))
}

// Err implements the store.Stream interface.
func (s *stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return store.ConvertError(s.err)
}

// Cancel implements the store.Stream interface.
func (s *stream) Cancel() {
	s.mu.Lock()
	if !s.done {
		s.done = true
		s.node.Close()
		s.err = verror.New(verror.ErrCanceled, nil, store.ErrMsgCanceledStream)
	}
	s.mu.Unlock()
}
