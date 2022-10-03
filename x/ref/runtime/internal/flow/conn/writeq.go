// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/x/ref/runtime/internal/flow/conn/debug"
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
	// Keep track of whether the active writer was able to bypass waiting for a signal
	// in order to avoid signaling a writer that will never read the signal.
	activeBypass bool

	debug.WriteqStats
	debug.CallerCircularList
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

	debug.WriterStats
}

const dbq = false

func dbqf(format string, args ...interface{}) {
	if !dbq {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
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

func (w *writer) setComment(comment string) {
	w.SetComment(comment)
}

func (w *writer) String() string {
	return fmt.Sprintf("%p: %v", w, &w.WriterStats)
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

func (q *writeq) addWriterLocked(head *writer, w *writer, p int) bool {
	if w.next != nil && w.prev != nil {
		for _, h := range q.activeWriters {
			if h != nil {
				for c := h; c != nil; c = c.next {
					if c.next == h {
						break
					}
					if c == w {
						fmt.Printf("Actually in the queueuueue\n")
					}
				}
			}
		}
		return false
	}
	q.Add()
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
				q.Rm()
				return head
			}
			q.activeWriters[p] = head.next
			nxt.prev, prv.next = prv, nxt
			head.prev, head.next = nil, nil
			q.Rm()
			return head
		}
	}
	return nil
}

func (q *writeq) wait(ctx *context.T, w *writer, p int) error {

	q.WaitCall()
	defer q.WaitReturn()

	q.mu.Lock()

	q.Next(&w.WriterStats)

	debug.Writeq("%p: writeq: wait:entry:\t%v\n", q, &q.WriteqStats)
	defer debug.Writeq("%p: writeq: wait:exit:\t%v\n", q, &q.WriteqStats)

	if q.active != nil {
		debug.Writeq("%p: writeq: active: %v\n", q, q.active)
		if !q.addWriterLocked(q.activeWriters[p], w, p) {
			q.mu.Unlock()
			q.DumpAndExit(fmt.Sprintf("writer %v, priority %v already exists in the writeq\n", w, p))
			return fmt.Errorf("writer %p, priority %v already exists in the writeq", w, p)
		}
		if !q.activeBypass {
			q.Append(fmt.Sprintf("%p: signaling: active (no bypass): %v\n", q, q.active))
			q.Send()
			select {
			case q.active.notify <- struct{}{}:
			default:
			}
		}
		q.mu.Unlock()
	} else {
		q.active = w
		q.activeBypass = false
		head := q.nextLocked()
		if head == nil {
			debug.Writeq("%p: writeq: idle: bypassing wait for: %v\n", q, w)
			// Either the q is empty or w was the only item in the q, either
			// way we return immediately and bypass any synchronisation.
			q.activeBypass = true
			q.mu.Unlock()
			q.Bypassed()
			q.Append(fmt.Sprintf("%p: bypass: %v", q, w))
			return nil
		}

		// notify the next item and then wait to be notified below.
		debug.Writeq("%p: writeq: idle: signal: %v, wait for: %v\n", q, head, w)
		q.Send()
		q.Append(fmt.Sprintf("%p: signaling: %v", q, head))
		head.notify <- struct{}{}
		q.mu.Unlock()
	}
	q.Receive()
	q.Append(fmt.Sprintf("%p: waiting: %v", q, w))
	if ctx == nil {
		select {
		case <-w.notify:
		case <-time.After(5 * time.Second):
			fmt.Printf("writeq stats %s\n", &q.WriteqStats)
			q.DumpAndExit(fmt.Sprintf("timeout: %v", w))
		}

	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.notify:
		case <-time.After(5 * time.Second):
			fmt.Printf("writeq stats %s\n", &q.WriteqStats)
			q.DumpAndExit(fmt.Sprintf("timeout: %v", w))
		}
	}
	return nil
}

func (q *writeq) done(w *writer) {
	q.DoneCall()
	defer q.DoneReturn()
	q.mu.Lock()

	if q.active == w {
		q.active = nil
	}
	if q.active != nil {
		if !q.activeBypass {
			q.active.notify <- struct{}{}
			q.activeBypass = true
		}
		q.mu.Unlock()
		return
	}
	head := q.nextLocked()
	q.mu.Unlock()
	if head != nil {
		q.Send()
		q.Append(fmt.Sprintf("%p: signaling: %v", q, w))
		head.notify <- struct{}{}
	}
}

func (q *writeq) tryNotify() {
	return
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active == nil || q.activeBypass {
		return
	}
	select {
	case q.active.notify <- struct{}{}:
	default:
	}
}
