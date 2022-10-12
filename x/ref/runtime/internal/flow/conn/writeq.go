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
// active or queued.
//
// Usage is as follows:
//
//		if err := wait(ctx, &writer, priority); err != nil {
//				// handle context canceled/timeout etc.
//		}
//
//		// access the message pipe
//
//		done(&writer)
//
//	 1. every call to wait, must be paired with a call to done.
//	 2. only a single instance of writer may be in the writeq at any point
//	    in time and it is the caller's responsibility to ensure that.
//	 3. if a writer is to be used concurrently, the it should be locked
//	    outside of the writeq implementation. writeq will never lock
//	    a writer.
//	 4. when wait returns (without an error), it's writer is guaranteed to be
//	     the active one. Another writer cannot get it's turn until
//	    done has been called. If wait returns with an error, it guarantees
//	    that the supplied writer is not the active one. An error is returned
//	    either for context cancelation or attempting to wait on the same
//	    writer more than once.
type writeq struct {
	mu sync.Mutex

	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters [numPriorities]*writer

	// The currently active writer. A writer will be signaled when it
	// becomes active, with the special case, that a writer is added
	// to an empty writeq (via wait) will return immediately since there is
	// no need for it to wait.
	active *writer
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

func (q *writeq) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.stringLocked()
}

func (q *writeq) stringLocked() string {
	out := &strings.Builder{}
	if q.active == nil {
		fmt.Fprintf(out, "writeq(%p): idle\n", q)
	} else {
		fmt.Fprintf(out, "writeq(%p): active: %v\n", q, q.active)
	}
	for p, h := range q.activeWriters {
		if h != nil {
			fmt.Fprintf(out, "\t%v: ", p)
			for w := h; w != nil; w = w.next {
				fmt.Fprintf(out, "%v ", w)
				if w.next == h {
					break
				}
			}
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (q *writeq) rmWriterLocked(w *writer, p int) {
	prv, nxt := w.prev, w.next
	if head := q.activeWriters[p]; head == w {
		if w == nxt {
			q.activeWriters[p] = nil
		} else {
			q.activeWriters[p] = nxt
		}
	}
	if w.next == nil && w.prev == nil {
		// just in case rmWriter is not in the queue.
		return
	}
	w.next.prev = prv
	prv.next = nxt
	w.prev, w.next = nil, nil
}

func (q *writeq) addWriterLocked(w *writer, p int) bool {
	if w.next != nil && w.prev != nil {
		return false
	}
	if head := q.activeWriters[p]; head != nil {
		// Insert at the 'tail' of the list, ie. as the item to be sent last.
		w.prev, w.next = head.prev, head
		head.prev.next = w
		head.prev = w
		return true
	}
	// Insert at the head of the list. w.next == w.prev since
	// it's both the first and last item to be sent.
	w.prev, w.next = w, w
	q.activeWriters[p] = w
	return true
}

// nextLocked returns the next writer in the q by priority and LIFO
// order. If a writer is found it is removed from the queue.
func (q *writeq) nextLocked() (*writer, int) {
	for p, head := range q.activeWriters {
		if head != nil {
			prv, nxt := head.prev, head.next
			if nxt == head {
				q.activeWriters[p] = nil
				head.prev, head.next = nil, nil
				return head, p
			}
			q.activeWriters[p] = head.next
			nxt.prev, prv.next = prv, nxt
			head.prev, head.next = nil, nil
			return head, p
		}
	}
	return nil, -1
}

func (q *writeq) handleCancel(w *writer, p int) {
	q.mu.Lock()
	// The writer could be in the queue or active, but not both.
	if q.active == w {
		q.active = nil
		// Replace the canceled writer with a new one, if there is one.
		if head, _ := q.nextLocked(); head != nil {
			// Make the item removed from the queue the active one and signal it.
			q.active = head
			q.mu.Unlock()
			q.active.notify <- struct{}{}
			return
		}
		q.mu.Unlock()
		return
	}
	// It must be in the queue, so remove it.
	q.rmWriterLocked(w, p)
	q.mu.Unlock()
}

func (q *writeq) signalWait(ctx *context.T, w *writer, p int) error {
	if ctx == nil {
		<-w.notify
		return nil
	}
	select {
	case <-ctx.Done():
		q.handleCancel(w, p)
		return ctx.Err()
	case <-w.notify:
	}
	return nil
}

func (q *writeq) wait(ctx *context.T, w *writer, p int) error {
	q.mu.Lock()
	if q.active != nil {
		if !q.addWriterLocked(w, p) || w == q.active {
			return fmt.Errorf("writer %p, priority %v already exists in the writeq", w, p)
		}
		q.mu.Unlock()
		return q.signalWait(ctx, w, p)
	}

	if head, p := q.nextLocked(); head != nil {
		// Make the item removed from the queue the active one and signal it.
		q.active = head
		q.mu.Unlock()
		head.notify <- struct{}{}
		return q.signalWait(ctx, w, p)
	}
	q.active = w
	// No need to signal the writer, just return immediately.
	q.mu.Unlock()
	return nil

}

func (q *writeq) done(w *writer) {
	q.mu.Lock()
	if q.active == w {
		q.active = nil
		w.next, w.prev = nil, nil
		if head, _ := q.nextLocked(); head != nil {
			// If there is a new active writer, signal it.
			q.active = head
			q.mu.Unlock()
			head.notify <- struct{}{}
			return
		}
	}
	q.mu.Unlock()
}
