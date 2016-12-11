// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"fmt"
	"sort"
)

// execution represents an execution of the test.
type execution struct {
	// strategy describes the initial sequence of scheduling decisions
	// to make.
	strategy []TID
	// nsteps records the number of scheduling decision made.
	nsteps int
	// nthreads records the number of threads in the system.
	nthreads int
	// nrequests records the number of currently pending requests.
	nrequests int
	// requests is a channel on which scheduling requests are received.
	requests chan request
	// done is a channel that the request handlers can use to wake up
	// the main scheduling loop.
	done chan struct{}
	// nextTID is a function that can be used to generate unique thread
	// identifiers.
	nextTID func() TID
	// activeTID records the identifier of the currently active thread.
	activeTID TID
	// ctx stores the abstract state of resources used by the
	// execution.
	ctx *context
	// threads records the abstract state of threads active in the
	// execution.
	threads map[TID]*thread
}

// newExecution is the execution factory.
func newExecution(strategy []TID) *execution {
	execution := &execution{
		strategy: strategy,
		nthreads: 1,
		requests: make(chan request),
		nextTID:  TIDGenerator(),
		ctx:      newContext(),
		threads:  make(map[TID]*thread),
	}
	clock := newClock()
	clock[0] = 0
	execution.threads[0] = newThread(0, clock)
	return execution
}

// Run executes the body of the test, exploring a sequence of
// scheduling decisions, and returns a vector of the scheduling
// decisions it made as well as the alternative scheduling decisions
// it could have made instead.
func (e *execution) Run(testBody func()) []*choice {
	go testBody()
	choices := make([]*choice, 0)
	// Keep scheduling requests until there are threads left in the
	// system.
	for e.nthreads != 0 {
		// Keep receiving scheduling requests until all threads are
		// blocked on a decision.
		for e.nrequests != e.nthreads {
			request, ok := <-e.requests
			if !ok {
				panic("Command channel closed unexpectedly.")
			}
			e.nrequests++
			request.process(e)
		}
		choice := e.generateChoice()
		choices = append(choices, choice)
		e.activeTID = choice.next
		e.schedule(choice.next)
	}
	return choices
}

// findThread uses the given thread identifier to find a thread among
// the known threads.
func (e *execution) findThread(tid TID) *thread {
	thread, ok := e.threads[tid]
	if !ok {
		panic(fmt.Sprintf("Could not find thread %v.", tid))
	}
	return thread
}

// generateChoice describes the scheduling choices available at the
// current abstract program state.
func (e *execution) generateChoice() *choice {
	c := newChoice()
	enabled := make([]TID, 0)
	for tid, thread := range e.threads {
		t := &transition{
			tid:      tid,
			clock:    thread.clock.clone(),
			enabled:  thread.enabled(e.ctx),
			kind:     thread.kind(),
			readSet:  thread.readSet(),
			writeSet: thread.writeSet(),
		}
		c.transitions[tid] = t
		if t.enabled {
			enabled = append(enabled, tid)
		}
	}
	if len(c.transitions) == 0 {
		panic("Encountered a deadlock.")
	}
	if e.nsteps < len(e.strategy) {
		// Follow the scheduling strategy.
		c.next = e.strategy[e.nsteps]
	} else {
		// Schedule an enabled thread using a deterministic round-robin
		// scheduler.
		sort.Sort(IncreasingTID(enabled))
		index := 0
		for ; index < len(enabled) && enabled[index] <= e.activeTID; index++ {
		}
		if index == len(enabled) {
			index = 0
		}
		c.next = enabled[index]
	}
	return c
}

// schedule advances the execution of the given thread.
func (e *execution) schedule(tid TID) {
	e.nrequests--
	e.nsteps++
	thread, ok := e.threads[tid]
	if !ok {
		panic(fmt.Sprintf("Could not find thread %v.\n", tid))
	}
	if !thread.enabled(e.ctx) {
		panic(fmt.Sprintf("Thread %v is about to be scheduled and is not enabled.", tid))
	}
	e.done = make(chan struct{})
	close(thread.ready)
	<-e.done
}
