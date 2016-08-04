// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"sync"
)

// context stores the abstract state of resources used in an execution
// of a concurrent program.
type context struct {
	// mutexes stores the abstract state of mutexes.
	mutexes map[*sync.Mutex]*fakeMutex
	// rwMutexes stores the abstract state of read-write mutexes.
	rwMutexes map[*sync.RWMutex]*fakeRWMutex
}

// newContext if the context factory.
func newContext() *context {
	return &context{
		mutexes:   make(map[*sync.Mutex]*fakeMutex),
		rwMutexes: make(map[*sync.RWMutex]*fakeRWMutex),
	}
}
