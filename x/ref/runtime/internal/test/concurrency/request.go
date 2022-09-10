// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"sync"
)

// request is an interface to describe a scheduling request.
type request interface {
	// enabled determines whether the given program transition can be
	// executed without blocking in the given context.
	enabled(ctx *context) bool
	// execute models the effect of advancing the execution of the
	// calling thread.
	execute(ready chan struct{}, e *execution)
	// kind returns the kind of the program transition of the calling
	// thread.
	kind() transitionKind
	// process handles initial processing of an incoming
	// scheduling request, making sure the calling thread is suspended
	// until the user-level scheduler decides to advance its execution.
	process(e *execution)
	// readSet records the identifiers of the abstract resources read by
	// the program transition of the calling thread.
	readSet() resourceSet
	// writeSet records the identifiers of the abstract resources
	// written by the program transition of the calling thread.
	writeSet() resourceSet
}

type defaultRequest struct {
	request
	done chan struct{}
}

func (r defaultRequest) enabled(ctx *context) bool {
	return true
}

func (r defaultRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	close(r.done)
	close(e.done)
}

func (r defaultRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

func (r defaultRequest) kind() transitionKind {
	return tNil
}

func (r defaultRequest) readSet() resourceSet {
	return newResourceSet()
}

func (r defaultRequest) writeSet() resourceSet {
	return newResourceSet()
}

// goRequest is to be called before creating a new goroutine through "go
// fn(tid)" to obtain a thread identifier to supply to the goroutine
// that is about to be created. This request is a part of the
// implementation of the Start() function provided by this package.
type goRequest struct {
	defaultRequest
	reply chan TID
}

func (r goRequest) process(e *execution) {
	e.nthreads++
	tid := e.nextTID()
	thread := e.findThread(e.activeTID)
	newThread := newThread(tid, thread.clock)
	newThread.clock[tid] = 0
	e.threads[tid] = newThread
	r.reply <- tid
	e.nrequests--
	close(r.done)
}

// goParentRequest is to be called right after a new goroutine is created
// through "go fn(tid)" to prevent the race between the parent and the
// child thread. This request is a part of the implementation of the
// Start() function provided by this package.
type goParentRequest struct {
	defaultRequest
}

func (r goParentRequest) kind() transitionKind {
	return tGoParent
}

func (r goParentRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

// goChildRequest is to be called as the first thing inside of a new
// goroutine to prevent the race between the parent and the child
// thread. This request is a part of the implementation of the Start()
// function provided by this package.
type goChildRequest struct {
	defaultRequest
	tid TID
}

func (r goChildRequest) kind() transitionKind {
	return tGoChild
}

func (r goChildRequest) process(e *execution) {
	thread := e.findThread(r.tid)
	thread.clock[r.tid]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

// goExitRequest is to be called as the last thing inside of the body
// of a test and any goroutine that the test spawns to inform the
// testing framework about the termination of a thread. This request
// implements the Exit() function provided by this package.
type goExitRequest struct {
	defaultRequest
}

func (r goExitRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	e.nthreads--
	delete(e.threads, e.activeTID)
	close(r.done)
	close(e.done)
}

func (r goExitRequest) kind() transitionKind {
	return tGoExit
}

func (r goExitRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

// mutexLockRequest is to be called to schedule a mutex lock. This request
// implements the MutexLock() function provided by this package.
type mutexLockRequest struct {
	defaultRequest
	mutex *sync.Mutex
}

func (r mutexLockRequest) enabled(ctx *context) bool {
	m, ok := ctx.mutexes[r.mutex]
	return !ok || m.free()
}

func (r mutexLockRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	thread := e.findThread(e.activeTID)
	m, ok := e.ctx.mutexes[r.mutex]
	if !ok {
		m = newFakeMutex(thread.clock)
		e.ctx.mutexes[r.mutex] = m
	}
	thread.clock.merge(m.clock)
	m.clock.merge(thread.clock)
	m.lock()
	close(r.done)
	close(e.done)
}

func (r mutexLockRequest) kind() transitionKind {
	return tMutexLock
}

func (r mutexLockRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

func (r mutexLockRequest) readSet() resourceSet {
	set := newResourceSet()
	set[r.mutex] = struct{}{}
	return set
}

func (r mutexLockRequest) writeSet() resourceSet {
	set := newResourceSet()
	set[r.mutex] = struct{}{}
	return set
}

// mutexUnlockRequest is to be called to schedule a mutex unlock. This
// request implements the MutexUnlock() function provided by this
// package.
type mutexUnlockRequest struct {
	defaultRequest
	mutex *sync.Mutex
}

func (r mutexUnlockRequest) enabled(ctx *context) bool {
	m, ok := ctx.mutexes[r.mutex]
	if !ok {
		panic("Mutex does not exist.")
	}
	return m.locked()
}

func (r mutexUnlockRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	m, ok := e.ctx.mutexes[r.mutex]
	if !ok {
		panic("Mutex not found.")
	}
	thread := e.findThread(e.activeTID)
	thread.clock.merge(m.clock)
	m.clock.merge(thread.clock)
	m.unlock()
	close(r.done)
	close(e.done)
}

func (r mutexUnlockRequest) kind() transitionKind {
	return tMutexUnlock
}

func (r mutexUnlockRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

func (r mutexUnlockRequest) readSet() resourceSet {
	set := newResourceSet()
	set[r.mutex] = struct{}{}
	return set
}

func (r mutexUnlockRequest) writeSet() resourceSet {
	set := newResourceSet()
	set[r.mutex] = struct{}{}
	return set
}

// rwMutexLockRequest is to be called to schedule a read-write mutex
// lock. This request implements the RWMutexLock() function provided
// by this package.
type rwMutexLockRequest struct {
	defaultRequest
	read    bool
	rwMutex *sync.RWMutex
}

func (r rwMutexLockRequest) enabled(ctx *context) bool {
	rw, ok := ctx.rwMutexes[r.rwMutex]
	if r.read {
		return !ok || rw.free() || rw.shared()
	}
	return !ok || rw.free()
}

func (r rwMutexLockRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	thread := e.findThread(e.activeTID)
	rw, ok := e.ctx.rwMutexes[r.rwMutex]
	if !ok {
		rw = newFakeRWMutex(thread.clock)
		e.ctx.rwMutexes[r.rwMutex] = rw
	}
	thread.clock.merge(rw.clock)
	rw.clock.merge(thread.clock)
	rw.lock(r.read)
	close(r.done)
	close(e.done)
}

func (r rwMutexLockRequest) kind() transitionKind {
	if r.read {
		return tRWMutexRLock
	}
	return tRWMutexLock
}

func (r rwMutexLockRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

func (r rwMutexLockRequest) readSet() resourceSet {
	set := newResourceSet()
	set[r.rwMutex] = struct{}{}
	return set
}

func (r rwMutexLockRequest) writeSet() resourceSet {
	set := newResourceSet()
	set[r.rwMutex] = struct{}{}
	return set
}

// rwMutexUnlockRequest is to be called to schedule a read-write mutex
// unlock. This request implements the RWMutexUnlock() function
// provided by this package.
type rwMutexUnlockRequest struct {
	defaultRequest
	read    bool
	rwMutex *sync.RWMutex
}

func (r rwMutexUnlockRequest) enabled(ctx *context) bool {
	rw, ok := ctx.rwMutexes[r.rwMutex]
	if !ok {
		panic("Read-write mutex does not exist.")
	}
	if r.read {
		return rw.shared()
	}
	return rw.exclusive()
}

func (r rwMutexUnlockRequest) execute(ready chan struct{}, e *execution) {
	<-ready
	rw, ok := e.ctx.rwMutexes[r.rwMutex]
	if !ok {
		panic("Read-write mutex not found.")
	}
	thread := e.findThread(e.activeTID)
	thread.clock.merge(rw.clock)
	rw.clock.merge(thread.clock)
	rw.unlock(r.read)
	close(r.done)
	close(e.done)
}

func (r rwMutexUnlockRequest) kind() transitionKind {
	if r.read {
		return tRWMutexRUnlock
	}
	return tRWMutexUnlock
}

func (r rwMutexUnlockRequest) process(e *execution) {
	thread := e.findThread(e.activeTID)
	thread.clock[e.activeTID]++
	ready := make(chan struct{})
	thread.ready = ready
	thread.request = r
	go r.execute(ready, e)
}

func (r rwMutexUnlockRequest) readSet() resourceSet {
	set := newResourceSet()
	set[r.rwMutex] = struct{}{}
	return set
}

func (r rwMutexUnlockRequest) writeSet() resourceSet {
	set := newResourceSet()
	set[r.rwMutex] = struct{}{}
	return set
}
