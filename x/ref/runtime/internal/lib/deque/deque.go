// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package deque implements a deque using a circular array.
package deque

const (
	initialQueueSize = 4
)

// T is the type of queues.
type T struct {
	contents []interface{}

	// Boundary cases.
	// o If full, size==len and fx==bx
	// o If empty, size==0 and fx==bx
	// o On initialization, contents=nil, size==0, fx==bx.
	size int // Number of elements in the queue.
	fx   int // Index of the first element.
	bx   int // Index one past the last element (the index of the last element is (bx-1)%len).
}

// Size returns the number of items in the queue.
func (q *T) Size() int {
	return q.size
}

// Clear removes all the elements of the queue.
func (q *T) Clear() {
	q.fx = 0
	q.bx = 0
	q.size = 0
	q.contents = nil
}

// PushBack adds an element to the back of the queue.
func (q *T) PushBack(item interface{}) {
	q.reserve()
	q.contents[q.bx] = item
	q.bx = (q.bx + 1) % len(q.contents)
	q.size++
}

// PushFront adds an element to the front of the deque.
func (q *T) PushFront(item interface{}) {
	q.reserve()
	q.fx = (q.fx + len(q.contents) - 1) % len(q.contents)
	q.contents[q.fx] = item
	q.size++
}

// Front returns the first element of the deque, or nil if there is none.
func (q *T) Front() interface{} {
	if q.size == 0 {
		return nil
	}
	return q.contents[q.fx]
}

// Back returns the last element of the deque, or nil if there is none.
func (q *T) Back() interface{} {
	if q.size == 0 {
		return nil
	}
	return q.contents[(q.bx+len(q.contents)-1)%len(q.contents)]
}

// PopFront removes an element from the front of the queue and returns it.
func (q *T) PopFront() interface{} {
	if q.size == 0 {
		return nil
	}
	item := q.contents[q.fx]
	q.contents[q.fx] = nil
	q.fx = (q.fx + 1) % len(q.contents)
	q.size--
	return item
}

// PopBack removes an element from the front of the queue and returns it.
func (q *T) PopBack() interface{} {
	if q.size == 0 {
		return nil
	}
	q.bx = (q.bx + len(q.contents) - 1) % len(q.contents)
	item := q.contents[q.bx]
	q.contents[q.bx] = nil
	q.size--
	return item
}

// Iter iterates over the elements of the deque.  f should return false to
// terminate the iteration early.
func (q *T) Iter(f func(item interface{}) bool) {
	for i := 0; i != q.size; i++ {
		ix := (q.fx + i) % len(q.contents)
		if !f(q.contents[ix]) {
			break
		}
	}
}

// Reserve space for at least one additional element.
func (q *T) reserve() {
	if q.size == len(q.contents) {
		if q.contents == nil {
			q.contents = make([]interface{}, initialQueueSize)
			return
		}
		contents := make([]interface{}, q.size*2)
		i := copy(contents, q.contents[q.fx:])
		copy(contents[i:], q.contents[:q.fx])
		q.contents = contents
		q.fx = 0
		q.bx = q.size
	}
}
