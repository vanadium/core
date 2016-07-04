// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package upcqueue implements an unbounded producer/consumer queue.  An
// unbounded producer/consumer queue is a concurrent buffer supporting multiple
// concurrent producers and consumers, with timeouts.  The queue can be closed
// from either end, by the producer and/or the consumer.  When closed, the
// contents are discarded, and subsequent operations return an error.
//
// Note: the main reason to use a producer/consumer queue instead of a channel
// is to allow the consumer to close the channel.  This queue can be used for
// many-to-many communication with multiple producers and/or multiple consumers.
// Any of the producers and any of the consumers are allowed to close the
// queue.
package upcqueue

import (
	"errors"
	"sync"

	"v.io/x/ref/runtime/internal/lib/deque"
	vsync "v.io/x/ref/runtime/internal/lib/sync"
)

var (
	ErrQueueIsClosed = errors.New("queue is closed")
)

// T is an unbounded producer/consumer queue.  It fulfills the same purpose as a
// Go channel, the main advantage is that the Put() operation does not panic,
// even after the queue is closed.  The main disadvantage is that the T can't be
// used in a select operation.
type T struct {
	mutex    sync.Mutex
	contents deque.T // GUARDED_BY(mutex)
	isClosed bool    // GUARDED_BY(mutex)

	// Number of available elements.
	avail *vsync.Semaphore
}

// New returns a producer/consumer queue.
func New() *T {
	return &T{avail: vsync.NewSemaphore()}
}

// Put adds an item to the queue, or returns an error if the queue is closed.
func (q *T) Put(item interface{}) error {
	q.mutex.Lock()
	if q.isClosed {
		q.mutex.Unlock()
		return ErrQueueIsClosed
	}
	q.contents.PushBack(item)
	q.mutex.Unlock()
	q.avail.Inc()
	return nil
}

// Get returns the next item from the queue, or an error if the queue is closed
// or the operation is cancelled.
func (q *T) Get(cancel <-chan struct{}) (interface{}, error) {
	err := q.avail.Dec(cancel)
	if err != nil {
		if err == vsync.ErrClosed {
			err = ErrQueueIsClosed
		}
		return nil, err
	}
	return q.getContents()
}

// TryGet returns the next item from the queue if there is one.
func (q *T) TryGet() (interface{}, error) {
	err := q.avail.TryDec()
	if err != nil {
		if err == vsync.ErrClosed {
			err = ErrQueueIsClosed
		}
		return nil, err
	}
	return q.getContents()
}

func (q *T) getContents() (interface{}, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.contents.Size() == 0 {
		if q.isClosed {
			return nil, ErrQueueIsClosed
		}
		return nil, nil
	}
	item := q.contents.PopFront()
	return item, nil
}

// Close closes the queue, without discarding the contents.  All Put* operations
// currently running may, or may not, add their values to the queue.  All Put*
// operations that happen-after the Close will fail.
func (q *T) Close() {
	q.mutex.Lock()
	q.isClosed = true
	q.mutex.Unlock()
	q.avail.Close()
}

// Shutdown closes the queue and returns the contents.  Any concurrent Get
// and Put operations might exchange values, but all operations that
// happen-after the Shutdown will fail.
func (q *T) Shutdown() []interface{} {
	q.mutex.Lock()
	q.isClosed = true
	var contents []interface{}
	for {
		item := q.contents.PopFront()
		if item == nil {
			break
		}
		contents = append(contents, item)
	}
	q.mutex.Unlock()
	q.avail.Close()
	return contents
}

// IsClosed returns whether the queue has been closed (with Close or Shutdown).
func (q *T) IsClosed() bool {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.isClosed
}
