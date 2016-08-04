// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// A producer/consumer queue is a concurrent bounded buffer supporting
// multiple concurrent producers and consumers, with timeouts.  The queue can be
// closed from either end, by the producer and/or the consumer.  When closed,
// the contents are discarded, and subsequent operations return an error.
//
// Note: the main reason to use a producer/consumer queue instead of a channel
// is to allow the consumer to close the channel.  This queue can be used for
// many-to-many communication with multiple producers and/or multiple consumers.
// Any of the producers and any of the consumers are allowed to close the
// queue.
package pcqueue

import (
	"errors"
	"sync"
)

var (
	ErrQueueIsClosed = errors.New("queue is closed")
	ErrCancelled     = errors.New("operation was canceled")
	ErrTryAgain      = errors.New("operation failed, try again")
)

// T is a producer/consumer queue.  It fulfills the same purpose as a Go
// channel, the main advantage is that the Put() operation does not panic, even
// after the queue is closed.  The main disadvantage is that the T can't
// be used in a select operation.
type T struct {
	// The mutex R/W mode depends only on whether the immediate struct fields
	// are being read or modified.  It isn't related to whether the channel
	// operations are mutating.  For example, the Put() method takes a read lock
	// because it reads the contents and isClosed fields.  It mutates the
	// contents channel, but that doesn't matter.
	mutex    sync.RWMutex
	contents chan interface{} // GUARDED_BY(mutex)
	isClosed bool             // GUARDED_BY(mutex), true iff <closed> is closed.

	closed chan struct{}
}

// New(size) returns a producer/consumer queue with maximum
// <size> elements.
func New(maxSize int) *T {
	return &T{
		contents: make(chan interface{}, maxSize),
		closed:   make(chan struct{})}
}

// Put(item, cancel) adds an item to the queue, or returns an error if the queue
// is closed or the operation is cancelled.  The <cancel> channel may be nil, in
// which case the operation can't be cancelled.
func (q *T) Put(item interface{}, cancel <-chan struct{}) error {
	contents := q.putChannel()
	select {
	case contents <- item:
	case <-q.closed:
		return ErrQueueIsClosed
	case <-cancel:
		return ErrCancelled
	}
	return nil
}

// Get(cancel) returns the next item from the queue, or an error if
// the queue is closed or the operation is cancelled.
func (q *T) Get(cancel <-chan struct{}) (interface{}, error) {
	contents := q.getChannel()
	select {
	case v := <-contents:
		return v, nil
	case <-q.closed:
		return q.drain()
	case <-cancel:
		return nil, ErrCancelled
	}
}

// TryPut attempts to add an item to the queue.  If the queue is full,
// ErrTryAgain is returned immediately, without blocking.  If the queue is
// closed, ErrQueueIsClosed is returned.
func (q *T) TryPut(item interface{}) error {
	contents := q.putChannel()
	select {
	case contents <- item:
		return nil
	default:
	}
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	if q.isClosed {
		return ErrQueueIsClosed
	}
	return ErrTryAgain
}

// Close() closes the queue, without discarding the contents.  All Put*() operations
// currently running may, or may not, add their values to the queue.  All Put*()
// operations that happen-after the Close() will fail.
func (q *T) Close() {
	q.mutex.Lock()
	if !q.isClosed {
		q.isClosed = true
		close(q.closed)
	}
	q.mutex.Unlock()
}

// Shutdown() closes the queue and discards all contents.  Any concurrent Get()
// and Put() operations might exchange values, but all operations that
// happen-after the Shutdown() will fail.
func (q *T) Shutdown() {
	q.mutex.Lock()
	if !q.isClosed {
		q.isClosed = true
		close(q.closed)
	}
	q.contents = nil
	q.mutex.Unlock()
}

// putChannel() returns a channel for inserting new values.  Returns nil if
// the queue has been closed.
func (q *T) putChannel() chan interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	if q.isClosed {
		return nil
	}
	return q.contents
}

// getChannel() returns the <contents> channel.
func (q *T) getChannel() chan interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return q.contents
}

// drain() returns any queued elements.  Once the queue is empty, all subsequent
// values are discarded.
func (q *T) drain() (interface{}, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	select {
	case v := <-q.contents:
		return v, nil
	default:
		q.contents = nil
		return nil, ErrQueueIsClosed
	}
}
