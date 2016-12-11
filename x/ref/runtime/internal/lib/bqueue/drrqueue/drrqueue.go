// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package drrqueue implements a deficit round-robin buffer queue.
//
//     Efficient Fair Queueing Using Deficit Round-Robin
//     M. Shreedhar and George Varghese
//     IEEE/ACM Transactions on Networking, Vol. 4, No. 3, June 1996
//
// The queue supports N-writers and 1-reader queue.  By "buffer," we mean []byte
// blocks.
//
// Writers have a priority that takes precedence over the deficit.  Writers
// with greater priority are served first.  Deficits are not even updated for
// lower priorities when higher priority Writers are being served.
package drrqueue

// LOCKING DISCIPLINE:
//
// Each Writer has a lock, and so does T.  Locks are always taken in order:
// Writer.mutex first, then T.mutex.  Never take the locks in the opposite
// order.

import (
	"fmt"
	"io"
	"sync"

	"v.io/x/ref/runtime/internal/lib/bqueue"
	"v.io/x/ref/runtime/internal/lib/deque"
	"v.io/x/ref/runtime/internal/lib/iobuf"
	vsync "v.io/x/ref/runtime/internal/lib/sync"
)

// T defines the type of round-robin buffer queues.  The queue has multiple
// input Writer queues that are served according to the deficit round-robin
// policy.
type T struct {
	mutex sync.Mutex
	cond  *sync.Cond

	// active contains an array of active Writers, indexed by Priority.
	active [][]*writer

	// writers contains all of the Writers.
	writers map[bqueue.ID]*writer

	// quantum is the amount of data that each Writer can send per round-robin cycle.
	quantum int

	isClosed bool
}

// Writer is a single bounded input queue supporting a Put operation.
type writer struct {
	id       bqueue.ID
	q        *T
	priority bqueue.Priority

	// free contains the number of free bytes in the writer queue.
	//
	// INVARIANT: free + size == size of the queue (a constant).  This can't be
	// computed, because <free> is a semaphore, but it is true nonetheless.
	free *vsync.Semaphore

	// The following are all protected by the mutex.
	mutex    sync.Mutex
	isClosed bool
	contents deque.T
	size     int // Total number of bytes in the queue.
	released int // Number of bytes that can be dequeued (negative for unlimited).

	// The following are protected by q.mutex.  mutex is not required.
	isActive activeMode
	deficit  int
}

// activeMode has three states:
//   busy: The Writer is being updated by Get.  It is not in the active list.
//   idle: The Writer is inactive.  It is not in the active list.
//   active: The Writer is in the active list.
type activeMode int

const (
	busy activeMode = iota
	idle
	active
)

// ID returns the numeric identifier for the queue.
func (w *writer) ID() bqueue.ID {
	return w.id
}

// Close closes the writer, without discarding the contents.  All Put operations
// currently running may, or may not, add their values to the queue.  All Put
// operations that happen-after the Close will fail.
func (w *writer) Close() {
	w.mutex.Lock()
	w.isClosed = true
	w.updateStateLocked(false, 0)
	w.mutex.Unlock()
	w.free.Close()
}

// Shutdown closes the writer as in Close and also discards the contents.
// If removeWriter is true the writer will be removed from the
// associated T's queue entirely, otherwise the now empty writer will
// remain and eventually be returned by a T.Get.
func (w *writer) Shutdown(removeWriter bool) {
	w.mutex.Lock()

	w.isClosed = true
	if !removeWriter {
		w.contents.Clear()
		w.size = 0
		w.updateStateLocked(false, 0)
	}

	w.mutex.Unlock()

	if removeWriter {
		w.q.removeWriter(w)
	}
	w.free.Close()
}

// IsClosed returns true iff the Writer is closed.
func (w *writer) IsClosed() bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.isClosed
}

// IsDrained returns true iff the Writer is closed and empty.
func (w *writer) IsDrained() bool {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.isClosed && w.size == 0
}

// Put adds an element to the queue.  Put blocks until there is space in the
// Writer.  The element is not made available to T.Get until it is released with
// the Release method.  Returns an error if the queue is closed or the operation
// is cancelled.
func (w *writer) Put(buf *iobuf.Slice, cancel <-chan struct{}) error {
	// Block until there is space in the Writer.
	if err := w.free.DecN(uint(buf.Size()), cancel); err != nil {
		return err
	}
	return w.putContents(buf)
}

// TryPut is like Put, but it is nonblocking.
func (w *writer) TryPut(buf *iobuf.Slice) error {
	if err := w.free.TryDecN(uint(buf.Size())); err != nil {
		return err
	}
	return w.putContents(buf)
}

func (w *writer) putContents(buf *iobuf.Slice) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.isClosed {
		return bqueue.ErrWriterIsClosed
	}
	w.contents.PushBack(buf)
	w.size += buf.Size()
	w.updateStateLocked(false, 0)
	return nil
}

// Release allows the next <bytes> to be removed from the queue and passed to
// Get.  If <bytes> is negative, all messages are released and flow control is
// no longer used.
func (w *writer) Release(bytes int) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.released < 0 && bytes >= 0 {
		return bqueue.ErrCantToggleFlowControl
	}
	if bytes < 0 {
		w.released = -1
	} else {
		w.released += bytes
	}
	w.updateStateLocked(false, 0)
	return nil
}

// getContents returns as much data as possible, up to the deficit, and then
// updates the state.
func (w *writer) getContents(deficit int) ([]*iobuf.Slice, bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Collect the contents into bufs
	if w.released >= 0 && deficit > w.released {
		deficit = w.released
	}

	// Writer is closed.
	if w.size == 0 && w.isClosed {
		return nil, true
	}

	var consumed int
	var bufs []*iobuf.Slice
	for w.contents.Size() != 0 {
		b := w.contents.Front().(*iobuf.Slice)
		size := consumed + b.Size()
		if size > deficit {
			break
		}
		consumed = size
		bufs = append(bufs, b)
		w.contents.PopFront()
	}

	// Update counters by number of bytes consumed.
	w.size -= consumed
	// Decrement released, but only if it is nonnegative.
	if w.released >= 0 {
		w.released -= consumed
		if w.released < 0 {
			panic("released is negative")
		}
	}
	w.updateStateLocked(true, consumed)

	return bufs, bufs != nil
}

// updateStateLocked updates the ready state of the Writer.
//
// REQUIRES: w.mutex is locked.
func (w *writer) updateStateLocked(overrideBusy bool, consumed int) {
	w.free.IncN(uint(consumed))

	// The w.isActive state does not depend on the deficit.
	isActive := (w.size == 0 && w.isClosed) ||
		(w.size != 0 && (w.released < 0 || w.contents.Front().(*iobuf.Slice).Size() <= w.released))
	w.q.updateWriterState(w, overrideBusy, isActive, consumed)
}

// New returns a new T.  Each writer is allowed to send quantum bytes per round-robin cycle.
func New(quantum int) bqueue.T {
	q := &T{writers: make(map[bqueue.ID]*writer), quantum: quantum}
	q.cond = sync.NewCond(&q.mutex)
	return q
}

// Close closes the queue.
func (q *T) Close() {
	q.mutex.Lock()
	writers := q.writers
	q.isClosed = true
	q.writers = make(map[bqueue.ID]*writer)
	for i := 0; i != len(q.active); i++ {
		q.active[i] = nil
	}
	q.cond.Signal()
	q.mutex.Unlock()

	// Close the queues outside the q.mutex lock to preserve lock order.
	for _, w := range writers {
		w.Shutdown(true)
	}
}

// NewWriter allocates a new Writer.
func (q *T) NewWriter(id bqueue.ID, p bqueue.Priority, bytes int) (bqueue.Writer, error) {
	w := &writer{
		id:       id,
		priority: p,
		q:        q,
		free:     vsync.NewSemaphore(),
		isActive: idle,
	}
	w.free.IncN(uint(bytes))

	q.mutex.Lock()
	defer q.mutex.Unlock()
	if q.isClosed {
		return nil, bqueue.ErrBQueueIsClosed
	}
	q.addPriorityLocked(p)
	if _, ok := q.writers[w.id]; ok {
		return nil, bqueue.ErrWriterAlreadyExists
	}
	q.writers[w.id] = w
	return w, nil
}

// String provides a string representation of the queue.
func (q *T) String() string {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	s := "q{"
	for _, w := range q.writers {
		s += fmt.Sprintf("Writer{id: %d, size: %d, released: %d}, ", w.id, w.size, w.released)
	}
	s += "}"
	return s
}

// Find returns the queue with the specified ID.
func (q *T) Find(id bqueue.ID) bqueue.Writer {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	w, ok := q.writers[id]
	if !ok {
		// Don't return w; that would return a non-nil Writer interface
		// containing nil.
		return nil
	}
	return w
}

// Get returns the next element from a queue.  Get blocks until a buffer is
// available or the queue is closed.
func (q *T) Get(flush bqueue.FlushFunc) (bqueue.Writer, []*iobuf.Slice, error) {
	for {
		w, deficit, err := q.nextWriter(flush)
		if w == nil {
			return nil, nil, err
		}
		bufs, ok := w.getContents(deficit)
		if ok {
			return w, bufs, nil
		}
	}
}

// nextWriter walks through the pending buffers and returns the first active
// Writer.  The writer is removed from the active queue and made 'busy' so that
// it will not be re-added to the active queue.
func (q *T) nextWriter(flush bqueue.FlushFunc) (*writer, int, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	for {
		if q.isClosed {
			return nil, 0, io.EOF
		}
		for p, writers := range q.active {
			if len(writers) != 0 {
				w := writers[0]
				w.isActive = busy
				w.deficit += q.quantum
				q.active[p] = writers[1:]
				return w, w.deficit, nil
			}
		}
		if flush != nil {
			flush()
		}
		q.cond.Wait()
	}
}

// addPriorityLocked adds a ready queue with the specified priority level.
//
// REQUIRES: q.mutex is locked.
func (q *T) addPriorityLocked(p bqueue.Priority) {
	if int(p) >= len(q.active) {
		newActive := make([][]*writer, int(p)+1)
		copy(newActive, q.active)
		q.active = newActive
	}
}

// removeWriter removes the queue from the q.
//
// NOTE: does not require that w.mutex is locked.
func (q *T) removeWriter(w *writer) {
	q.mutex.Lock()
	if w.isActive == active {
		// Remove the writer from the active queue.
		active := q.active[w.priority]
		for i, w2 := range active {
			if w2 == w {
				copy(active[i:], active[i+1:])
				q.active[w.priority] = active[:len(active)-1]
				break
			}
		}
	}
	w.isActive = idle
	delete(q.writers, w.id)
	q.mutex.Unlock()
}

// updateWriterState updates the active state of the queue.
//
// REQUIRES: w.mutex is locked.
func (q *T) updateWriterState(w *writer, overrideBusy bool, isActive bool, consumed int) {
	q.mutex.Lock()
	if isActive {
		if w.isActive == idle || w.isActive == busy && overrideBusy {
			q.active[w.priority] = append(q.active[w.priority], w)
			w.isActive = active
			w.deficit -= consumed
			if w.deficit < 0 {
				panic("deficit is negative")
			}
			q.cond.Signal()
		}
	} else {
		if w.isActive == active {
			panic("Writer is active when it should not be")
		}
		if overrideBusy {
			w.isActive = idle
		}
		w.deficit = 0
	}
	q.mutex.Unlock()
}
