// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions

import (
	"bytes"
	"fmt"
	"sync"

	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/ptrie"
)

//////////////////////////////////////////////////////////////
// mergedStream implementation of Stream
//
// This implementation of Stream must take into account writes
// which have occurred since the snapshot was taken on the
// transaction.

// Create a new stream which merges a snapshot stream with write operations.
func mergeWritesWithStream(sn store.Snapshot, w *ptrie.T, start, limit []byte) store.Stream {
	m := &mergedStream{
		s:       sn.Scan(start, limit),
		sHasKey: true,
		p:       w.Scan(start, limit),
		pHasKey: true,
	}
	m.advanceS()
	m.advanceP()
	return m
}

type valueSourceType uint32

const (
	notInitialized valueSourceType = iota
	snapshotStream
	ptrieStream
)

type mergedStream struct {
	mu sync.Mutex

	s       store.Stream
	sHasKey bool
	sKey    []byte

	p       *ptrie.Stream
	pHasKey bool
	pKey    []byte
	// value indicates which stream holds the staged key-value pair
	valueSource valueSourceType
	cancelMutex sync.Mutex // protects isCanceled
	isCanceled  bool
}

func (m *mergedStream) advanceS() {
	if m.sHasKey {
		m.sHasKey = m.s.Advance()
	}
	if m.sHasKey {
		m.sKey = m.s.Key(m.sKey)
	}
}

func (m *mergedStream) advanceP() {
	if m.pHasKey {
		m.pHasKey = m.p.Advance()
	}
	if m.pHasKey {
		m.pKey = m.p.Key(m.pKey)
	}
}

func (m *mergedStream) canceled() bool {
	m.cancelMutex.Lock()
	defer m.cancelMutex.Unlock()
	return m.isCanceled
}

// stage stages a key-value pair from either the snapshot or the uncommitted
// writes.
func (m *mergedStream) stage() valueSourceType {
	if m.sHasKey && (!m.pHasKey || bytes.Compare(m.sKey, m.pKey) < 0) {
		return snapshotStream
	}
	if m.sHasKey && bytes.Compare(m.sKey, m.pKey) == 0 {
		m.advanceS()
	}
	switch value := m.p.Value().(type) {
	case isDeleted:
		m.advanceP()
		return notInitialized
	case []byte:
		return ptrieStream
	default:
		panic(fmt.Sprintf("unexpected type %T of value", value))
	}
}

// Advance implements the Stream interface.
func (m *mergedStream) Advance() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Invariant: one of the two streams points to the last staged key-value
	// pair and the other stream points to the key greater than the last staged
	// key with the exception if it is the first call of Advance().
	switch m.valueSource {
	case snapshotStream:
		m.advanceS()
	case ptrieStream:
		m.advanceP()
	}
	m.valueSource = notInitialized
	// Invariant: both streams point to a key-value pairs with keys greater than
	// the last staged key.
	// We need to pick a stream that points to a smaller key. If the picked
	// stream is the ptrie stream and the key-value pair represents a delete
	// operation, we skip the key-value pair and pick a key-value pair again.
	for true {
		if m.canceled() || (!m.sHasKey && !m.pHasKey) {
			return false
		}
		if m.valueSource = m.stage(); m.valueSource != notInitialized {
			return true
		}
	}
	return false // compiler insists on this line
}

// Key implements the Stream interface.
func (m *mergedStream) Key(keybuf []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.valueSource {
	case snapshotStream:
		return store.CopyBytes(keybuf, m.sKey)
	case ptrieStream:
		return store.CopyBytes(keybuf, m.pKey)
	default:
		panic("nothing staged")
	}
}

// Value implements the Stream interface.
func (m *mergedStream) Value(valbuf []byte) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.valueSource {
	case snapshotStream:
		// m.s.Value internally copies bytes to valbuf.
		return m.s.Value(valbuf)
	case ptrieStream:
		return store.CopyBytes(valbuf, m.p.Value().([]byte))
	default:
		panic("nothing staged")
	}
}

// Err implements the Stream interface.
func (m *mergedStream) Err() error {
	return m.s.Err()
}

// Cancel implements the Stream interface.
func (m *mergedStream) Cancel() {
	m.cancelMutex.Lock()
	if !m.isCanceled {
		m.isCanceled = true
		m.s.Cancel()
	}
	m.cancelMutex.Unlock()
}
