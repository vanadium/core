// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow/message"
)

const (
	expressPriority = iota
	flowPriority
	tearDownPriority

	// Must be last.
	numPriorities
)

type writeq struct {
	mu sync.Mutex

	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters [numPriorities]*writer

	writing *writer
}

type writer struct {
	// Maintain a doubly linked list whereby next and prev are unconventially
	// used to create two circular lists in opposite directions to create
	// a LIFO list. That is, 'next' always refers to the next writer to be sent,
	// ie. the 'last' one added, whereas 'prev' refers to the most recently
	// added.
	//
	//  head ->  e1    e2    e3
	//    next     --->  --->
	//             <---------
	//    prev     ---------->
	//             <---  <---  prev
	//
	// The invariant is therefore that head.next points to the next item
	// to be sent and head.prev points to the last item to be sent.
	prev, next *writer
	notify     chan struct{}
}

func (q *writeq) activateWriterLocked(w *writer, p int) {
	q.activateWriter(w, p)
}

// activateWriter adds a given writer to the list of active writers.
// The writer will be given a turn when the channel becomes available.
// You should try to only have writers with actual work to do in the
// list of activeWriters because we will switch to that thread to allow it
// to do work, and it will be wasteful if it turns out there is no work to do.
// After calling this you should typically call notifyNextWriterLocked.
func (q *writeq) activateWriter(w *writer, p int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	head := q.activeWriters[p]
	if head == w || w.prev != nil || w.next != nil {
		// We're already active, ie. in a list.
		return
	}
	if head == nil {
		// Insert at the head of the list. w.next == w.prev since
		// it's both the first and last item to be sent.
		w.prev, w.next = w, w
		q.activeWriters[p] = w
	} else {
		// Insert at the 'tail' of the list, ie. as the item to be sent last.
		w.prev, w.next = head.prev, head
		head.prev.next = w
		head.prev = w
	}
}

func (q *writeq) deactivateWriterLocked(w *writer, p int) {
	q.deactivateWriter(w, p)
}

// deactivateWriterLocked removes a writer from the active writer list.  After
// this function is called it is certain that the writer will not be given any
// new turns.  If the writer is already in the middle of a turn, that turn is
// not terminated, workers must end their turn explicitly by calling
// notifyNextWriterLocked.
func (q *writeq) deactivateWriter(w *writer, p int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	prv, nxt := w.prev, w.next
	if head := q.activeWriters[p]; head == w {
		if w == nxt { // We're the only one in the list.
			q.activeWriters[p] = nil
		} else {
			q.activeWriters[p] = nxt
		}
	}
	w.next.prev = prv
	prv.next = nxt
	w.prev, w.next = nil, nil
}

// notifyNextWriterLocked notifies the highest priority activeWriter to take
// a turn writing.  If w is the active writer give up w's claim and choose
// the next writer.  If there is already an active writer != w, this function does
// nothing.
func (q *writeq) notifyNextWriterLocked(w *writer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.writing == w {
		// give up w's turn.
		fmt.Printf("notify... %p - give up our turn\n", w)
		q.writing = nil
	} else {
		fmt.Printf("notify... %p - maybe there's someone else?\n", w)
	}
	if q.writing != nil {
		fmt.Printf("notify... %p - there's someone else: %p\n", w, q.writing)
		return
	}
	for p, head := range q.activeWriters {
		if head != nil {
			q.activeWriters[p] = head.next
			q.writing = head
			fmt.Printf("notify... %p\n", w)
			head.notify <- struct{}{}
			return
		}
	}
}

// sendMessageLocked sends a single message on the conn with the given priority.
// if cancelWithContext is true, then this write attempt will fail when the context
// is canceled.  Otherwise context cancellation will have no effect and this call
// will block until the message is sent.
// NOTE: The mutex is not held for the entirety of this call,
// therefore this call will interrupt your critical section. This
// should be called only at the end of a mutex protected region.
func (c *Conn) sendMessageLocked(
	ctx *context.T,
	cancelWithContext bool,
	priority int,
	m message.Message) (err error) {
	c.writers.activateWriterLocked(&c.writer, priority)
	c.writers.notifyNextWriterLocked(&c.writer)
	c.mu.Unlock()
	// wait for my turn.
	if cancelWithContext {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-c.writer.notify:
		}
	} else {
		<-c.writer.notify
	}
	// send the actual message.
	if err == nil {
		err = c.mp.writeMsg(ctx, m)
	}
	c.mu.Lock()
	c.writers.deactivateWriterLocked(&c.writer, priority)
	c.writers.notifyNextWriterLocked(&c.writer)
	return err
}
