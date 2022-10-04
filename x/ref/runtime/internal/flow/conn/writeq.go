// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"strings"
	"sync"

	"v.io/v23/context"
)

const (
	expressPriority = iota
	flowPriority
	tearDownPriority

	// Must be last.
	numPriorities
)

// writeq implements a set of LIFO queues used to order and prioritize
// writing to a connection's underlying message pipe. The intention is for
// writers to block until their turn to transmit comes up. This approach,
// as opposed to adding buffers to a queue, can minimize data copying and
// simplify data management. A 'bypass' optimization is implemented whereby
// a call to wait will return immediately if there are no other writers
// active or queued. A number of invariants are required for correct usage
// of a writeq.
//
// Usage is as follows:
//
//		wait(ctx, &writer, priority)
//		// access the message pipe
//		done(&writer)
//
//	 1. every call to wait, must be paired with a call to done.
//	 2. only a single instance of writer may be in the writeq at any point
//	    in time.
//	 3. if a writer is to be used concurrently, the it should be locked
//	    outside of the writeq implementation. writeq will never lock
//	    a writer.
type writeq struct {
	mu sync.Mutex

	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters [numPriorities]*writer

	active *writer
	// Keep track of whether the active writer needs to be signaled.
	activeNeedsSignal bool
}

type writer struct {
	// Maintain a doubly linked list whereby next and prev are used to create
	// two circular lists in opposite directions to create a LIFO list. That is,
	// 'next' always refers to the next writer to be sent, ie. the 'last' one
	// added, whereas 'prev' refers to the most recently
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

	notify chan struct{}
}

// initWriters initializes the supplied writers, in particular creating a channel
// for notificaions.
func initWriters(writers []writer, chanSize int) {
	for i := range writers {
		initWriter(&writers[i], chanSize)
	}
}

func initWriter(w *writer, chanSize int) {
	w.prev, w.next = nil, nil
	w.notify = make(chan struct{}, chanSize)
}

func (q *writeq) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.stringLocked()
}

func (q *writeq) stringLocked() string {
	out := strings.Builder{}
	if q.active == nil {
		fmt.Fprintf(&out, "writeq(%p): idle\n", q)
	} else {
		fmt.Fprintf(&out, "writeq(%p): active: %v\n", q, q.active)
	}
	for p, h := range q.activeWriters {
		if h != nil {
			fmt.Fprintf(&out, "\t%v: ", p)
			for w := h; w != nil; w = w.next {
				fmt.Fprintf(&out, "%v ", w)
				if w.next == h {
					break
				}
			}
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (q *writeq) isPresentLocked(w *writer) bool {
	for _, h := range q.activeWriters {
		if h != nil {
			for c := h; c != nil; c = c.next {
				if c.next == h {
					break
				}
				if c == w {
					return true
				}
			}
		}
	}
	return false
}

func (q *writeq) addWriterLocked(head *writer, w *writer, p int) bool {
	if w.next != nil && w.prev != nil {
		return false
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
	return true
}

// nextLocked returns the next writer in the q by priority and LIFO
// order. If a writer is found it is removed from the queue.
func (q *writeq) nextLocked() *writer {
	for p, head := range q.activeWriters {
		if head != nil {
			prv, nxt := head.prev, head.next
			if nxt == head {
				q.activeWriters[p] = nil
				head.prev, head.next = nil, nil
				return head
			}
			q.activeWriters[p] = head.next
			nxt.prev, prv.next = prv, nxt
			head.prev, head.next = nil, nil
			return head
		}
	}
	return nil
}

func (q *writeq) signalActiveLocked() {
	if q.activeNeedsSignal {
		q.active.notify <- struct{}{}
		q.activeNeedsSignal = false
	}
}

func (q *writeq) waitLocked(ctx *context.T, w *writer) error {
	if ctx == nil {
		<-w.notify
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.notify:
	}
	return nil
}

func (q *writeq) wait(ctx *context.T, w *writer, p int) error {
	q.mu.Lock()
	if q.active != nil {
		q.signalActiveLocked()
		if !q.addWriterLocked(q.activeWriters[p], w, p) {
			return fmt.Errorf("writer %p, priority %v already exists in the writeq", w, p)
		}
		q.mu.Unlock()
		return q.waitLocked(ctx, w)
	}

	head := q.nextLocked()
	if head == nil {
		// The q is empty, so return immediately and bypass any synchronisation.
		q.active = w
		q.activeNeedsSignal = false
		q.mu.Unlock()
		return nil
	}
	q.setActiveLocked(head)
	q.mu.Unlock()
	return q.waitLocked(ctx, w)
}

func (q *writeq) setActiveLocked(head *writer) {
	if head == nil {
		return
	}
	// Make the item removed from the Q the active one and signal it.
	q.active = head
	q.active.notify <- struct{}{}
	q.activeNeedsSignal = false
}

func (q *writeq) done(w *writer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active == w {
		q.active = nil
		w.next, w.prev = nil, nil
	}
	if q.active != nil {
		q.signalActiveLocked()
		return
	}
	q.setActiveLocked(q.nextLocked())
}
