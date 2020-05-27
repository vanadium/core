// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"errors"
	"sort"
)

// stack is an implementation of the stack data structure.
type stack struct {
	contents []*state
}

// IncreasingDepth is used to sort states in the increasing order of
// their depth.
type IncreasingDepth []*state

// SORT INTERFACE IMPLEMENTATION

func (states IncreasingDepth) Len() int {
	return len(states)
}
func (states IncreasingDepth) Less(i, j int) bool {
	return states[i].depth < states[j].depth
}
func (states IncreasingDepth) Swap(i, j int) {
	states[i], states[j] = states[j], states[i]
}

// Empty checks if the stack is empty.
func (s *stack) Empty() bool {
	return len(s.contents) == 0
}

// Length returns the length of the stack.
func (s *stack) Length() int {
	return len(s.contents)
}

// Pop removes and returns the top element of the stack. If the stack
// is empty, an error is returned.
func (s *stack) Pop() (*state, error) {
	l := len(s.contents)
	if l > 0 {
		x := s.contents[l-1]
		s.contents = s.contents[:l-1]
		return x, nil
	}
	return nil, errors.New("stack is empty")
}

// Push adds a new element to the top of the stack.
func (s *stack) Push(value *state) {
	s.contents = append(s.contents, value)
}

// Sort sorts the elements of the stack in the decreasing order of
// their depth.
func (s *stack) Sort() {
	sort.Sort(IncreasingDepth(s.contents))
}
