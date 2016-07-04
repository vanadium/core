// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "sync"

// WaitGroup implements a sync.WaitGroup-like structure that does not require
// all calls to Add to be made before Wait, instead calls to Add after Wait
// will fail.
//
// As a result, WaitGroup cannot be "re-used" in the way that sync.WaitGroup
// can. In the following example using sync.WaitGroup, Add, Done and Wait behave
// in the same way in rounds 1 and 2.
//
// var wg sync.WaitGroup
//
// Round #1.
// wg.Add(1)
// go wg.Done()
// wg.Wait()
//
// Round #2.
// wg.Add(1)
// go wg.Done()
// wg.Wait()
//
// However, an equivalent program using WaitGroup would receive an error on the
// second call to TryAdd.
type WaitGroup struct {
	mu      sync.Mutex
	waiting bool
	pending sync.WaitGroup
}

// TryAdd attempts to increment the counter. If Wait has already been called,
// TryAdd fails to increment the counter and returns false.
// If the counter becomes zero, all goroutines blocked on Wait are released.
func (w *WaitGroup) TryAdd() (added bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.waiting {
		return false
	}
	w.pending.Add(1)
	return true
}

// Done decrements the counter. If the counter goes negative, Done panics.
func (w *WaitGroup) Done() {
	w.pending.Done()
}

// Wait blocks until the counter is zero.
func (w *WaitGroup) Wait() {
	w.mu.Lock()
	w.waiting = true
	w.mu.Unlock()
	w.pending.Wait()
}
