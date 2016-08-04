// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"sync"
	"time"
)

// globalT stores a pointer to the global instance of the tester.
var (
	globalT *Tester = nil
)

// T returns the global instance of the tester.
func T() *Tester {
	return globalT
}

// Setup sets up a new instance of the tester.
func Init(setup, body, cleanup func()) *Tester {
	tree := newState(nil, 0)
	tree.visited = true
	seeds := &stack{}
	seeds.Push(tree)
	tree.seeded = true
	globalT = &Tester{
		tree:    tree,
		seeds:   seeds,
		setup:   setup,
		body:    body,
		cleanup: cleanup,
	}
	return globalT
}

// Cleanup destroys the existing instance of the tester.
func Finish() {
	globalT = nil
}

// Tester represents an instance of the systematic test.
type Tester struct {
	// enabled records whether the tester is to be used or not.
	enabled bool
	// execution represents the currently explored execution.
	execution *execution
	// tree represents the current state of the exploration of the space
	// of all possible interleavings of concurrent transitions.
	tree *state
	// seeds is records the collection of scheduling alternatives to be
	// explored in the future.
	seeds *stack
	// setup is a function that is executed before an instance of the
	// test is started. It is assumed to always produce the same initial
	// state.
	setup func()
	// body is a function that implements the body of the test.
	body func()
	// cleanup is a function that is executed after an instance of a
	// test instance terminates.
	cleanup func()
}

// Explore explores the space of possible test schedules until the
// state space is fully exhausted.
func (t *Tester) Explore() (int, error) {
	return t.explore(0, 0)
}

// ExploreFor explores the space of possible test schedules until the
// state space is fully exhausted or the given duration elapses,
// whichever occurs first.
func (t *Tester) ExploreFor(d time.Duration) (int, error) {
	return t.explore(0, d)
}

// ExploreN explores the space of possible test schedules until the
// state space is fully exhausted or the given number of schedules is
// explored, whichever occurs first.
func (t *Tester) ExploreN(n int) (int, error) {
	return t.explore(n, 0)
}

// MutexLock implements the logic related to modeling and scheduling
// an execution of "m.Lock()".
func (t *Tester) MutexLock(m *sync.Mutex) {
	if t.enabled {
		done := make(chan struct{})
		request := mutexLockRequest{defaultRequest{done: done}, m}
		t.execution.requests <- request
		<-done
	} else {
		m.Lock()
	}
}

// MutexUnlock implements the logic related to modeling and scheduling
// an execution of "m.Unlock()".
func (t *Tester) MutexUnlock(m *sync.Mutex) {
	if t.enabled {
		done := make(chan struct{})
		request := mutexUnlockRequest{defaultRequest{done: done}, m}
		t.execution.requests <- request
		<-done
	} else {
		m.Unlock()
	}
}

// RWMutexLock implements the logic related to modeling and scheduling
// an execution of "rw.Lock()".
func (t *Tester) RWMutexLock(rw *sync.RWMutex) {
	if t.enabled {
		done := make(chan struct{})
		request := rwMutexLockRequest{defaultRequest{done: done}, false, rw}
		t.execution.requests <- request
		<-done
	} else {
		rw.Lock()
	}
}

// RWMutexRLock implements the logic related to modeling and
// scheduling an execution of "rw.RLock()".
func (t *Tester) RWMutexRLock(rw *sync.RWMutex) {
	if t.enabled {
		done := make(chan struct{})
		request := rwMutexLockRequest{defaultRequest{done: done}, true, rw}
		t.execution.requests <- request
		<-done
	} else {
		rw.RLock()
	}
}

// RWMutexRUnlock implements the logic related to modeling and
// scheduling an execution of "rw.RUnlock()".
func (t *Tester) RWMutexRUnlock(rw *sync.RWMutex) {
	if t.enabled {
		done := make(chan struct{})
		request := rwMutexUnlockRequest{defaultRequest{done: done}, true, rw}
		t.execution.requests <- request
		<-done
	} else {
		rw.RUnlock()
	}
}

// RWMutexUnlock implements the logic related to modeling and
// scheduling an execution of "rw.Unlock()".
func (t *Tester) RWMutexUnlock(rw *sync.RWMutex) {
	if t.enabled {
		done := make(chan struct{})
		request := rwMutexUnlockRequest{defaultRequest{done: done}, false, rw}
		t.execution.requests <- request
		<-done
	} else {
		rw.Unlock()
	}
}

// Start implements the logic related to modeling and scheduling an
// execution of "go fn()".
func Start(fn func()) {
	t := globalT
	if t != nil && t.enabled {
		done1 := make(chan struct{})
		reply := make(chan TID)
		request1 := goRequest{defaultRequest{done: done1}, reply}
		t.execution.requests <- request1
		tid := <-reply
		<-done1
		go t.startHelper(tid, fn)
		done2 := make(chan struct{})
		request2 := goParentRequest{defaultRequest{done: done2}}
		t.execution.requests <- request2
		<-done2
	} else {
		fn()
	}
}

// Exit implements the logic related to modeling and scheduling thread
// termination.
func Exit() {
	t := globalT
	if t != nil && t.enabled {
		done := make(chan struct{})
		request := goExitRequest{defaultRequest{done: done}}
		t.execution.requests <- request
		<-done
	}
}

// startHelper is a wrapper used by the implementation of Start() to
// make sure the child thread is registered with the correct
// identifier.
func (t *Tester) startHelper(tid TID, fn func()) {
	done := make(chan struct{})
	request := goChildRequest{defaultRequest{done: done}, tid}
	t.execution.requests <- request
	<-done
	fn()
}

func (t *Tester) explore(n int, d time.Duration) (int, error) {
	niterations := 0
	start := time.Now()
	for !t.seeds.Empty() &&
		(n == 0 || niterations < n) &&
		(d == 0 || time.Since(start) < d) {
		t.setup()
		seed, err := t.seeds.Pop()
		if err != nil {
			panic("Corrupted stack.\n")
		}
		strategy := seed.generateStrategy()
		t.execution = newExecution(strategy)
		t.enabled = true
		if err := t.tree.addBranch(t.execution.Run(t.body), t.seeds); err != nil {
			t.enabled = false
			return niterations, err
		}
		t.enabled = false
		// Sort the seeds because dynamic partial order reduction might
		// have added elements that violate the depth-first ordering of
		// seeds. The depth-first ordering is used for space-efficient
		// (O(d) where d is the depth of the execution tree) exploration
		// of the execution tree.
		t.seeds.Sort()
		t.cleanup()
		niterations++
	}
	return niterations, nil
}
