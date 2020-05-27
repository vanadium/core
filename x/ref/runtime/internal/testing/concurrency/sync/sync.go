// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync"

	"v.io/x/ref/runtime/internal/testing/concurrency"
)

// Mutex is a wrapper around the Go implementation of Mutex.
type Mutex struct {
	m sync.Mutex
}

// MUTEX INTERFACE IMPLEMENTATION

func (m *Mutex) Lock() {
	if t := concurrency.T(); t != nil {
		t.MutexLock(&m.m)
	} else {
		m.m.Lock()
	}
}

func (m *Mutex) Unlock() {
	if t := concurrency.T(); t != nil {
		t.MutexUnlock(&m.m)
	} else {
		m.m.Unlock()
	}
}

// RWMutex is a wrapper around the Go implementation of RWMutex.
type RWMutex struct {
	m sync.RWMutex
}

// RWMUTEX INTERFACE IMPLEMENTATION

func (m *RWMutex) Lock() {
	if t := concurrency.T(); t != nil {
		t.RWMutexLock(&m.m)
	} else {
		m.m.Lock()
	}
}

func (m *RWMutex) RLock() {
	if t := concurrency.T(); t != nil {
		t.RWMutexRLock(&m.m)
	} else {
		m.m.RLock()
	}
}

func (m *RWMutex) RLocker() sync.Locker {
	if t := concurrency.T(); t != nil {
		return (*rlocker)(m)
	}
	return m.m.RLocker()
}

func (m *RWMutex) RUnlock() {
	if t := concurrency.T(); t != nil {
		t.RWMutexRUnlock(&m.m)
	} else {
		m.m.RUnlock()
	}
}
func (m *RWMutex) Unlock() {
	if t := concurrency.T(); t != nil {
		t.RWMutexUnlock(&m.m)
	} else {
		m.m.Unlock()
	}
}

type rlocker RWMutex

func (r *rlocker) Lock() {
	(*RWMutex)(r).RLock()
}

func (r *rlocker) Unlock() {
	(*RWMutex)(r).RUnlock()
}
