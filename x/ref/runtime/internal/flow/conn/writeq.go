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

// writeq implements a set of LIFOs queues used to order and prioritize
// writing to a connections underlying message pipe. The intention is for
// writers to block until their turn to transmit comes up. This approach,
// as opposed to adding buffers to a queue, can minimize data copying
// simplify data management.
//
// Usage is as follows:
//
//	activateAndNotifyWriter(&writer, priority)
//	<- writer.notify
//	// access the message pipe
//	deactivateAndNotifyWriter(&writer)
//
// The invariant that must be maintained is that the total number of
// additions to writeq must be matched by total number of notifications
// issued to ensure that every blocked writer is guaranteed to be woken
// up.
type writeq struct {
	mu sync.Mutex

	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters [numPriorities]*writer

	active *writer

	trace callerCircularList
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

	stack []uintptr
}

// initWriter initializes the supplied writer, in particular creating a channel
// for notificaions.
func initWriter(w *writer, chanSize int) {
	w.prev, w.next = nil, nil
	w.notify = make(chan struct{}, chanSize)
	w.stack = callers(0, 15)
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
		fmt.Fprintf(&out, "writeq(%p): active: %p\n", q, q.active)

	}
	for p, h := range q.activeWriters {
		if h != nil {
			fmt.Fprintf(&out, "\t%v: ", p)
			for w := h; w != nil; w = w.next {
				fmt.Fprintf(&out, "%p ", w)
				if w.next == h {
					break
				}
			}
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (q *writeq) addWriterLocked(head *writer, w *writer, p int) {
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

func (q *writeq) wait(ctx *context.T, w *writer, p int) error {
	q.mu.Lock()
	if q.active != nil {
		//		fmt.Printf("%p: active: non idle %p\n", q, q.active)
		q.addWriterLocked(q.activeWriters[p], w, p)
		q.mu.Unlock()
	} else {
		//		fmt.Printf("%p: active: idle\n", q)
		q.active = w
		head := q.nextLocked()
		if head == nil || head == w {
			// Either the q is empty or w is the only item in the q.
			q.mu.Unlock()
			return nil
		}
		q.mu.Unlock()
		// notify the next item and then wait to be notified below.
		head.notify <- struct{}{}
	}

	if ctx == nil {
		<-w.notify
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.notify:
		}
	}
	return nil
}

func (q *writeq) done(w *writer) {
	q.mu.Lock()
	if q.active == w {
		q.active = nil
	}
	head := q.nextLocked()
	q.mu.Unlock()
	if head != nil {
		head.notify <- struct{}{}
	}
}

func (q *writeq) notifyNext() {
	q.mu.Lock()
	head := q.nextLocked()
	q.mu.Unlock()
	if head == nil {
		return
	}
	head.notify <- struct{}{}
}

// nextLocked returns the next writer in the q by priority and LIFO
// order. If a writer is found it is removed from the queue.
func (q *writeq) nextLocked() *writer {
	for p, head := range q.activeWriters {
		if head != nil {
			prv, nxt := head.prev, head.next
			if nxt == head {
				q.activeWriters[p] = nil
				return head
			}
			q.activeWriters[p] = head.next
			nxt.prev = prv
			prv.next = nxt
			head.prev, head.next = nil, nil
			return head
		}
	}
	return nil
}
