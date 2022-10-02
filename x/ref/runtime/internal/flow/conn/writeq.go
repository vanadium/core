// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"

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
// as opposed to adding buffers to a queue, can minimize data copying and
// simplify data management.
//
// Usage is as follows:
//
//	wait(ctx, &writer, priority)
//	// access the message pipe
//	done(&writer)
//
// The invariant that must be maintained is that the total number of
// additions to writeq must be matched by the total number of notifications
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

	nextID uint64

	bypass, in, out, cdone uint64
	nwaits, nsignals       uint64
	active                 *writer
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

	mu     sync.Mutex
	bypass bool
	notify chan struct{}

	id uint64

	msg string
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

func (w *writer) String() string {
	return fmt.Sprintf("%p: cid: %v :: %s", w, w.id, w.msg)
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

func (q *writeq) stats(msg string, w *writer) string {
	return fmt.Sprintf("writeq:%p: cid: %2v, %s in:out: %2v:%2v - bypass %2v, done: %2v, waits:signals: %2v:%2v:\t%v", q, atomic.LoadUint64(&q.nextID), msg, atomic.LoadUint64(&q.in), atomic.LoadUint64(&q.out), atomic.LoadUint64(&q.bypass), atomic.LoadUint64(&q.cdone), atomic.LoadUint64(&q.nwaits), atomic.LoadUint64(&q.nsignals), w)
}

func (q *writeq) wait(ctx *context.T, w *writer, p int) error {

	q.mu.Lock()
	w.id = atomic.AddUint64(&q.nextID, 1)
	w.bypass = false

	dbqf("%s\n", q.stats("wait: entry", w))
	defer func() {
		dbqf("%s\n", q.stats("wait: exit ", w))
	}()

	atomic.AddUint64(&q.in, 1)
	defer atomic.AddUint64(&q.out, 1)

	if q.active != nil {
		dbqf("writeq:%p: cid: %2v priority %v: active: non idle %v: (%v)\n", q, q.nextID, p, q.active, w)
		dbqf("writeq:%p: cid: %2v %s\n", q, q.nextID, q.stringLocked())
		if !q.addWriterLocked(q.activeWriters[p], w, p) {
			dbqf("writeq:%p: cid: %2v state: %s\n", q, q.nextID, q.stringLocked())
			q.mu.Unlock()
			return fmt.Errorf("writer %p, priority %v already exists in the writeq", w, p)
		}

		if !q.active.bypass {
			atomic.AddUint64(&q.nsignals, 1)
			q.active.notify <- struct{}{}
			q.active.bypass = true
		}
		/*		select {
				case q.active.notify <- struct{}{}:
				default:
				}*/
		// how do we know if it's waiting? If it is, signal it, otherwise
		// wait for it to be done.

	} else {
		dbqf("writeq:%p: cid: %2v priority %v: active: idle: (%v)\n", q, q.nextID, p, w)
		dbqf("writeq:%p: cid: %2v state: %s\n", q, q.nextID, q.stringLocked())
		q.active = w
		head := q.nextLocked()
		if head == nil || head == w {
			fmt.Printf("writeq:%p: cid: %2v priority %v: active: idle: (%v): done bypass\n", q, q.nextID, p, w)
			// Either the q is empty or w is the only item in the q.
			w.bypass = true
			q.mu.Unlock()
			atomic.AddUint64(&q.bypass, 1)
			return nil
		}
		// notify the next item and then wait to be notified below.
		dbqf("writeq:%p: cid: %2v wait.signal: (%v)\n", q, q.nextID, head)
		atomic.AddUint64(&q.nsignals, 1)
		head.notify <- struct{}{}
	}
	dbqf("writeq:%p: cid: %2v wait.wait: ctx %v (%v): waiting\n", q, q.nextID, ctx != nil, w)
	atomic.AddUint64(&q.nwaits, 1)
	q.mu.Unlock()
	if ctx == nil {
		<-w.notify
	} else {
		select {
		case <-ctx.Done():
			dbqf("writeq:%p: cid: %2v wait.wait: ctx %v (%v): err: %v\n", q, q.nextID, ctx != nil, w, ctx.Err())
			return ctx.Err()
		case <-w.notify:
		}
	}
	dbqf("writeq:%p: cid: %2v wait.wait: ctx %v (%v): done\n", q, q.nextID, ctx != nil, w)
	return nil
}

func (q *writeq) done(w *writer) {
	atomic.AddUint64(&q.cdone, 1)
	q.mu.Lock()
	if q.active == w {
		q.active = nil
	}
	head := q.nextLocked()
	if head != nil {
		dbqf("writeq:%p: cid: %2v done.signal: %v\n", q, q.nextID, head)
		atomic.AddUint64(&q.nsignals, 1)
		head.notify <- struct{}{}
	}
	q.mu.Unlock()
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
