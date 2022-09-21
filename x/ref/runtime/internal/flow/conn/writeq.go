// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
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
	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters [numPriorities]writer
	writing       writer
}

type writer interface {
	notify()
	priority() int
	neighbors() (prev, next writer)
	setNeighbors(prev, next writer)
}

// activateWriterLocked adds a given writer to the list of active writers.
// The writer will be given a turn when the channel becomes available.
// You should try to only have writers with actual work to do in the
// list of activeWriters because we will switch to that thread to allow it
// to do work, and it will be wasteful if it turns out there is no work to do.
// After calling this you should typically call notifyNextWriterLocked.
func (q *writeq) activateWriterLocked(w writer) {
	priority := w.priority()
	_, wn := w.neighbors()
	head := q.activeWriters[priority]
	if head == w || wn != w {
		// We're already active.
		return
	}
	if head == nil { // We're the head of the list.
		q.activeWriters[priority] = w
	} else { // Insert us before head, which is the end of the list.
		hp, _ := head.neighbors()
		w.setNeighbors(hp, head)
		hp.setNeighbors(nil, w)
		head.setNeighbors(w, nil)
	}
}

// deactivateWriterLocked removes a writer from the active writer list.  After
// this function is called it is certain that the writer will not be given any
// new turns.  If the writer is already in the middle of a turn, that turn is
// not terminated, workers must end their turn explicitly by calling
// notifyNextWriterLocked.
func (q *writeq) deactivateWriterLocked(w writer) {
	priority := w.priority()
	p, n := w.neighbors()
	if head := q.activeWriters[priority]; head == w {
		if w == n { // We're the only one in the list.
			q.activeWriters[priority] = nil
		} else {
			q.activeWriters[priority] = n
		}
	}
	n.setNeighbors(p, nil)
	p.setNeighbors(nil, n)
	w.setNeighbors(w, w)
}

// notifyNextWriterLocked notifies the highest priority activeWriter to take
// a turn writing.  If w is the active writer give up w's claim and choose
// the next writer.  If there is already an active writer != w, this function does
// nothing.
func (q *writeq) notifyNextWriterLocked(w writer) {
	if q.writing == w {
		q.writing = nil
	}
	if q.writing == nil {
		for p, head := range q.activeWriters {
			if head != nil {
				_, q.activeWriters[p] = head.neighbors()
				q.writing = head
				head.notify()
				return
			}
		}
	}
}

type writerList struct {
	// next and prev are protected by c.mu
	next, prev writer
}

func (s *writerList) neighbors() (prev, next writer) { return s.prev, s.next }
func (s *writerList) setNeighbors(prev, next writer) {
	if prev != nil {
		s.prev = prev
	}
	if next != nil {
		s.next = next
	}
}

// singleMessageWriter is used to send a single message with a given priority.
type singleMessageWriter struct {
	writeCh chan struct{}
	p       int
	writerList
}

func (s *singleMessageWriter) notify()       { close(s.writeCh) }
func (s *singleMessageWriter) priority() int { return s.p }

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
	s := singleMessageWriter{writeCh: make(chan struct{}), p: priority}
	s.next, s.prev = &s, &s
	c.writers.activateWriterLocked(&s)
	c.writers.notifyNextWriterLocked(&s)
	c.mu.Unlock()
	// wait for my turn.
	if cancelWithContext {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-s.writeCh:
		}
	} else {
		<-s.writeCh
	}
	// send the actual message.
	if err == nil {
		err = c.mp.writeMsg(ctx, m)
	}
	c.mu.Lock()
	c.writers.deactivateWriterLocked(&s)
	c.writers.notifyNextWriterLocked(&s)
	return err
}
