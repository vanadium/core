// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bqueue implements several kinds of buffer queues, as a N-writer,
// 1-reader queue.  By "buffer," we mean iobuf.Slice values.  Each writer has a
// separate bounded queue to which it writes buffers.  The queue also supports
// flow control.
//
// Initialization:
//
//     // Create a new queue using one of the implementations
//     // (currently only bqueue/drrqueue).
//     q := drrqueue.New()
//
// Reader API:
//
//     // Returns the next buffer in the queue, blocking until there is one
//     // available.  Returns with an error if <q> is closed:
//     _, buf, err := q.Get()
//
// Writer API:
//
//     // Allocate a new Writer with the id, priority, and space for N elements.
//     w := q.New(id, priority, N)
//
//     // Add <buf> to the <w>.  Blocks until there is space in the Writer.
//     // Aborts if <cancel> is closed or contains a value.
//     err := w.Put(buf, cancel)
//
//     w.Release(N)  // Make the next N buffers available to q.Get().
//
// The q.Release() method is used for rate limiting.  Buffers can be added with
// q.Put(), but they are not passed to q.Get() until they are released.
package bqueue

import (
	"errors"

	"v.io/x/ref/runtime/internal/lib/iobuf"
)

// Priority is an integer priority.  Smaller is greater priority.
//
// For performance, priorities should be dense and start from 0.  Some
// implementations like drrqueue have use space linear in the max priority.
type Priority uint // TODO(jyh): Change the dense requirement if we need it.

// ID is the type of Writer identifiers.
type ID int64

// FlushFunc is the type of flushing functions.  See T.Get for more info.
type FlushFunc func() error

// T specifies a buffer queue.  The NewWriter method is used to create new
// writer queues, and the Get method returns the next buffer to be served.
type T interface {
	Close()
	String() string

	// Find returns the Writer with the specified ID.  Returns nil if there is
	// no such writer.
	Find(id ID) Writer

	// Get returns the next contents of the queue.  Get returns a Writer and an
	// array of elements dequeued from the Writer.  The number of elements
	// returned depends on the implementation (for example, drrqueue specifies a
	// cap on how many bytes can be dequeued per Writer per round-robin cycle).
	// In addition, multiple elements are returned so that iobuf.Coalesce() can
	// be used to coalesce the contents.
	//
	// Get blocks until at least one element can be returned or the queue is
	// closed.  If non-nil, the <flush> function is called just before Get
	// blocks.
	//
	// If a Writer is closed (the Writer's Close() method was called), then Get
	// returns the Writer with empty contents.  The caller should call
	// writer.Shutdown() to remove the Writer and prevent it from being returned
	// in subsequent calls.
	//
	// It is not safe to call Get() concurrently.
	Get(flush FlushFunc) (Writer, []*iobuf.Slice, error)

	// NewWriter allocates a new Writer.
	NewWriter(id ID, p Priority, n int) (Writer, error)
}

// Writer represents a single writer queue.  Writer queues are served
// according to the policy defined by the container queue T.
type Writer interface {
	ID() ID

	// Close closes the Writer, without discarding the contents.  All Put
	// operations currently running may, or may not, add their values to the
	// Writer.  All Put operations that happen-after the Close will fail.
	Close()

	// Shutdown closes the Writer as in Close and also discards the contents.
	// If removeWriter is true the writer will be removed from the
	// associated T's queue entirely, otherwise the now empty writer will
	// remain and eventually be returned by a T.Get.
	Shutdown(removeWriter bool)

	// IsClosed returns true if the Writer is closed.
	IsClosed() bool

	// IsDrained returns true if the Writer is closed and has no data
	IsDrained() bool

	// Put adds an element to the queue.  Put blocks until there is space in
	// the Writer.  The element is not made available to T.Get until it is
	// released with the Release method.  Returns an error if the queue is
	// closed or the operation is cancelled.
	Put(buf *iobuf.Slice, cancel <-chan struct{}) error

	// TryPut is like Put, but it is nonblocking.
	TryPut(buf *iobuf.Slice) error

	// Release allows the next <n> elements to be removed from the Writer and
	// passed to Get.  If <n> is negative, all messages are released and flow
	// control is no longer used.
	Release(n int) error
}

var (
	ErrBQueueIsClosed        = errors.New("bqueue: queue is closed")
	ErrWriterAlreadyExists   = errors.New("bqueue: writer already exists with this identifier")
	ErrWriterIsClosed        = errors.New("bqueue: writer is closed")
	ErrCantToggleFlowControl = errors.New("bqueue: can't turn on flow control when it is off")
	ErrCancelled             = errors.New("bqueue: operation was canceled")
	ErrTryAgain              = errors.New("bqueue: writer is not ready, try again")
)
