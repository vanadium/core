// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"errors"
	"sync"
)

// Semaphore is an implementation of unbounded semaphores.  Abstractly, a
// semaphore holds a nonnegative integer value, and supports operations to
// increment and decrement the value.  The semaphore value is not allowed to be
// negative; decrement operations block until the semaphore value is positive.
// http://en.wikipedia.org/wiki/Semaphore_%28programming%29
//
// The standard suggestion for implementing semaphores in Go is to use a
// buffered channel, where the number of elements in the channel is the max
// value of the semaphore.  However, what we implement here is _unbounded_
// semaphores (up to a max value of 2^31-1).
//
// A mutex and integer is used to keep track of the numerical value of the
// and a channel is used for notification of changes.
// When decrementing, the value of the semaphore is decremented and if not
// sufficient, DecN will block until it can subtract more.
//
// Because of this looping, the semaphore is not fair.
// The reason for using a channel for notifications is for cancellation.
// The Dec(cancel <-chan struct{}) method takes a cancelation channel, so we use a
// "select" operation to determine whether to perform a semaphore operation or
// abort because the semaphore is close or the operation was canceled.
//
// NOTE: when the Semaphore is closed, the Dec (or DecN) operations are
// unblocked, returning an error (ErrClosed) if the semaphore value is 0 (or
// less than the DecN value), respectively.  However, even with the Semaphore
// closed, if the semaphore value is non-zero (or sufficient to satisfy the DecN
// value), Dec (or DecN) performs the decrement successfully and returns without
// an error.  Regardless of whether the Semaphore is closed or not, Inc/IncN
// succeed in incrementing the semaphore value.
type Semaphore struct {
	mu    sync.Mutex
	value uint // GUARDED_BY(mu)

	notify chan bool

	isClosed bool
	closed   chan struct{}
}

var (
	ErrClosed   = errors.New("semaphore is closed")
	ErrCanceled = errors.New("semaphore operation was canceled")
	ErrTryAgain = errors.New("semaphore operation failed, try again")
)

// NewSemaphore allocates a semaphore with an initial value.
func NewSemaphore() *Semaphore {
	return &Semaphore{notify: make(chan bool, 1), closed: make(chan struct{})}
}

// Close closes the semaphore.  Subsequent operations do not block.
func (s *Semaphore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isClosed {
		s.isClosed = true
		close(s.closed)
	}
}

// DecN decrements the semaphore.  Blocks until the final value of the semaphore
// is nonnegative, or the <cancel> channel is closed (or has a value).
func (s *Semaphore) DecN(n uint, cancel <-chan struct{}) error {
	s.mu.Lock()
	if s.value >= n {
		s.value -= n
		s.mu.Unlock()
		return nil
	}
	taken := s.value
	s.value = 0
	s.mu.Unlock()

	for taken < n {
		needed := n - taken
		select {
		case <-s.notify:
			notify := true
			s.mu.Lock()
			if s.value <= needed {
				needed = s.value
				notify = false
			}
			taken += needed
			s.value -= needed
			s.mu.Unlock()
			if notify {
				select {
				case s.notify <- true:
				default:
				}
			}
		case <-s.closed:
			// Close does not zero out the semaphore, so check to
			// see if the DecN can be satisfied.
			s.mu.Lock()
			if s.value >= needed {
				s.value -= needed
				taken += needed
			}
			s.mu.Unlock()
			if taken < n {
				return ErrClosed
			}
		case <-cancel:
			s.IncN(taken)
			return ErrCanceled
		}
	}
	return nil
}

// TryDecN tries to decrement the semaphore.
func (s *Semaphore) TryDecN(n uint) error {
	if n == 0 {
		return nil
	}
	s.mu.Lock()
	if s.value >= n {
		s.value -= n
		s.mu.Unlock()
		return nil
	}
	closed := s.isClosed
	s.mu.Unlock()

	if closed {
		return ErrClosed
	}
	return ErrTryAgain
}

// IncN increments the semaphore.  Wakes any potential waiters.
func (s *Semaphore) IncN(n uint) {
	if n == 0 {
		// Avoid notifications when there is no change to the value.
		return
	}

	s.mu.Lock()
	s.value += n
	s.mu.Unlock()

	select {
	case s.notify <- true:
	default:
	}
}

// Dec decrements the semaphore by 1.
func (s *Semaphore) Dec(cancel <-chan struct{}) error {
	return s.DecN(1, cancel)
}

// TryDec tries to decrement the semaphore by 1.
func (s *Semaphore) TryDec() error {
	return s.TryDecN(1)
}

// Inc increments the semaphore by 1.
func (s *Semaphore) Inc() {
	s.IncN(1)
}
