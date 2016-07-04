// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"fmt"
)

// TID is the thread identifier type.
type TID int

// Increasing is used to sort thread identifiers in an increasing order.
type IncreasingTID []TID

// SORT INTERFACE IMPLEMENTATION

func (tids IncreasingTID) Len() int {
	return len(tids)
}
func (tids IncreasingTID) Less(i, j int) bool {
	return tids[i] < tids[j]
}
func (tids IncreasingTID) Swap(i, j int) {
	tids[i], tids[j] = tids[j], tids[i]
}

// TIDGenerator is used for generating unique thread identifiers.
func TIDGenerator() func() TID {
	var n int = 0
	return func() TID {
		n++
		return TID(n)
	}
}

// thread records the abstract state of a thread during an execution
// of the test.
type thread struct {
	// tid is the thread identifier.
	tid TID
	// clock is a vector clock that keeps track of the logical time of
	// this thread.
	clock clock
	// ready is a channel that can be used to schedule execution of the
	// thread.
	ready chan struct{}
	// req holds the current scheduling request of the thread.
	request request
}

// newThread is the thread factory.
func newThread(tid TID, clock clock) *thread {
	return &thread{
		tid:   tid,
		clock: clock.clone(),
	}
}

// enabled checks if the thread can be scheduled given the current
// execution context.
func (t *thread) enabled(ctx *context) bool {
	if t.request == nil {
		panic(fmt.Sprintf("Thread %v has no request.", t.tid))
	}
	return t.request.enabled(ctx)
}

// kind returns the kind of the thread transition.
func (t *thread) kind() transitionKind {
	if t.request == nil {
		panic(fmt.Sprintf("Thread %v has no request.", t.tid))
	}
	return t.request.kind()
}

// readSet returns the set of abstract resources read by the thread.
func (t *thread) readSet() resourceSet {
	if t.request == nil {
		panic(fmt.Sprintf("Thread %v has no request.", t.tid))
	}
	return t.request.readSet()
}

// writeSet returns the set of abstract resources written by the
// thread.
func (t *thread) writeSet() resourceSet {
	if t.request == nil {
		panic(fmt.Sprintf("Thread %v has no request.", t.tid))
	}
	return t.request.writeSet()
}
