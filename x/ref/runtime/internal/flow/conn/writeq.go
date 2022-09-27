// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
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
// TODO(cnicolaou): explore using buffer hand off rather than the blocking
// model.
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

	mu     sync.Mutex
	notify chan struct{}
}

func (w *writer) lock() {
	w.mu.Lock()
}

func (w *writer) unlock() {
	w.mu.Unlock()
}

// initWriter initializes the supplied writer, in particular creating a channel
// for notificaions.
func initWriter(w *writer, chanSize int) {
	//	_, file, line, _ := runtime.Caller(1)
	//	file = filepath.Base(file)
	w.prev, w.next = nil, nil
	w.notify = make(chan struct{}, chanSize)
	// fmt.Fprintf(os.Stderr, "initWriter: %p ch: %v @ %v:%v\n", w, w.notify, file, line)
}

func (q *writeq) String() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.stringLocked()
}

func (q *writeq) stringLocked() string {
	out := strings.Builder{}
	fmt.Fprintf(&out, "writeq(%p): active: %p\n", q, q.writing)
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
	q.addWriterLocked(head, w, p)
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

// notifyNextWriter notifies the highest priority activeWriter to take
// a turn writing.  If w is the active writer give up w's claim and choose
// the next writer (which maybe w itself if there are no other writers ahead
// of it in the queue).  If there is already an active
// writer != w, this function does nothing.
func (q *writeq) notifyNextWriter(w *writer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	/*	if w == nil {
		fmt.Printf("notifyNextWriter nil: before: %s\n", q.stringLocked())
	}*/
	q.notifyNextWriterLocked(w)
	/*
		if w == nil {
			fmt.Printf("notifyNextWriter nil: after: %s\n", q.stringLocked())
		}
	*/
}

func (q *writeq) notifyNextWriterLocked(w *writer) {
	if q.writing == w {
		// give up w's turn.
		q.writing = nil
	}
	if q.writing != nil {
		return
	}
	for p, head := range q.activeWriters {
		if head != nil {
			q.activeWriters[p] = head.next
			q.writing = head
			select {
			case head.notify <- struct{}{}:
				//fmt.Fprintf(os.Stderr, "%p: notify: %p - %v\n", w, head, head.notify)
			default:
				fmt.Fprintf(os.Stderr, "would block: %p: (%v: %v) %s\n", w, len(head.notify), cap(head.notify), q.stringLocked())
				buf := make([]byte, 1<<16)
				n := runtime.Stack(buf, true)
				fmt.Fprintf(os.Stderr, "%s\n\n", buf[:n])
				panic("would block")
			}
			return
		}
	}
}

// deactivateAndNotify is equivalent to calling activateWriter followed
// by notifyNextWriter.
func (q *writeq) activateAndNotify(w *writer, p int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	head := q.activeWriters[p]
	if head != w && w != nil && w.prev == nil && w.next == nil {
		// w is not in any queue, so add it.
		q.addWriterLocked(head, w, p)
	}
	q.notifyNextWriterLocked(w)
}

// deactivateWriterLocked removes a writer from the active writer list.  After
// this function is called it is certain that the writer will not be given any
// new turns.  If the writer is already in the middle of a turn, that turn is
// not terminated, workers must end their turn explicitly by calling
// notifyNextWriterLocked.
func (q *writeq) deactivateWriter(w *writer, p int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if w.next == nil && w.prev == nil {
		return
	}
	q.rmWriter(w, p)
}

func (q *writeq) rmWriter(w *writer, p int) {
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

// deactivateAndNotify is equivalent to calling deactivateWriter followed
// by notifyNextWriter.
func (q *writeq) deactivateAndNotify(w *writer, p int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if w.next != nil && w.prev != nil {
		q.rmWriter(w, p)
	}
	q.notifyNextWriterLocked(w)
}
