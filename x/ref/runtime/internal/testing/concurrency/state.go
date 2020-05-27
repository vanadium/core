// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"fmt"
	"reflect"
)

// state records a state of the exploration of the space of all
// possible program states a concurrent test can encounter. The states
// of the exploration are organized using a tree data structure
// referred to as the "execution tree". Nodes of the execution tree
// represent different abstract program states and edges represent the
// different scheduling decisions that can be made at those states. A
// path from the root to a leaf thus uniquely represents an execution
// as a serialization of the scheduling decisions along the execution.
type state struct {
	// depth identifies the depth of this node in the execution tree.
	depth int
	// parent records the pointer to the parent abstract state.
	parent *state
	// children maps threads to the abstract states that correspond to
	// executing the next transitions of that thread.
	children map[TID]*state
	// explored records whether the subtree rooted at this state has
	// been fully explored.
	explored bool
	// seeded records whether the state has been used to seed an
	// exploration strategy.
	seeded bool
	// visited records whether the state has been visited
	visited bool
	// tid records the thread identifier of the thread whose transition
	// lead to this state.
	tid TID
	// kind records the type of the transition that lead to this state.
	kind transitionKind
	// clock records the logical time of this state.
	clock clock
	// enabled records whether the transition that leads to this state
	// is enabled.
	enabled bool
	// readSet records the identifiers of the abstract resources read by
	// the transition that lead to this state.
	readSet resourceSet
	// writeSet records the identifiers of the abstract resources
	// written by the transition that lead to this state.
	writeSet resourceSet
}

// newState is the state factory.
func newState(parent *state, tid TID) *state {
	depth := 0
	if parent != nil {
		depth = parent.depth + 1
	}
	return &state{
		depth:    depth,
		children: make(map[TID]*state),
		parent:   parent,
		tid:      tid,
		readSet:  newResourceSet(),
		writeSet: newResourceSet(),
		clock:    newClock(),
	}
}

// addBranch adds a branch described through the given sequence of
// choices to the execution tree rooted at this state.
func (s *state) addBranch(branch []*choice, seeds *stack) error {
	if len(branch) == 0 {
		s.explored = true
		return nil
	}
	choice := branch[0]
	if len(s.children) != 0 {
		// Check for the absence of divergence.
		if err := s.checkDivergence(choice.transitions); err != nil {
			return err
		}
	} else {
		// Add new children.
		for _, transition := range choice.transitions {
			child := newState(s, transition.tid)
			child.clock = transition.clock
			child.enabled = transition.enabled
			child.kind = transition.kind
			child.readSet = transition.readSet
			child.writeSet = transition.writeSet
			s.children[child.tid] = child
		}
	}
	next, found := s.children[choice.next]
	if !found {
		return fmt.Errorf("invalid choice (no transition fo thread %d)", choice.next)
	}
	next.visited = true
	branch = branch[1:]
	if err := next.addBranch(branch, seeds); err != nil {
		return err
	}
	s.collectSeeds(choice.next, seeds)
	s.compactTree()
	return nil
}

// approach identifies and enforces exploration of transition(s)
// originating from this state that near execution of the transition
// leading to the given state.
func (s *state) approach(other *state, seeds *stack) {
	candidates := make([]TID, 0)
	for tid, child := range s.children {
		if child.enabled && child.happensBefore(other) {
			candidates = append(candidates, tid)
		}
	}
	if len(candidates) == 0 {
		for _, child := range s.children {
			if child.enabled && !child.seeded && !child.visited {
				seeds.Push(child)
				child.seeded = true
			}
		}
	} else {
		for _, tid := range candidates {
			child := s.children[tid]
			if child.seeded || child.visited {
				return
			}
		}
		tid := candidates[0]
		child := s.children[tid]
		seeds.Push(child)
		child.seeded = true
	}
}

// checkDivergence checks whether the given transition matches the
// transition identified by the given thread identifier.
func (s *state) checkDivergence(transitions map[TID]*transition) error {
	if len(s.children) != len(transitions) {
		return fmt.Errorf("divergence encountered (expected %v, got %v)", len(s.children), len(transitions))
	}
	for tid, t := range transitions {
		child, found := s.children[tid]
		if !found {
			return fmt.Errorf("divergence encountered (no transition for thread %d)", tid)
		}
		if child.enabled != t.enabled {
			return fmt.Errorf("divergence encountered (expected %v, got %v)", child.enabled, t.enabled)
		}
		if !child.clock.equals(t.clock) {
			return fmt.Errorf("divergence encountered (expected %v, got %v)", child.clock, t.clock)
		}
		if !reflect.DeepEqual(child.readSet, t.readSet) {
			return fmt.Errorf("divergence encountered (expected %v, got %v)", child.readSet, t.readSet)
		}
		if !reflect.DeepEqual(child.writeSet, t.writeSet) {
			return fmt.Errorf("divergence encountered (expected %v, got %v)", child.writeSet, t.writeSet)
		}
	}
	return nil
}

// collectSeeds uses dynamic partial order reduction
// (http://users.soe.ucsc.edu/~cormac/papers/popl05.pdf) to identify
// which alternative scheduling choices should be explored.
func (s *state) collectSeeds(next TID, seeds *stack) {
	for tid, child := range s.children {
		if tid != next && !s.mayInterfereWith(child) && !s.happensBefore(child) {
			continue
		}
		for handle := s; handle.parent != nil; handle = handle.parent {
			if handle.mayInterfereWith(child) && !handle.happensBefore(child) {
				handle.parent.approach(child, seeds)
			}
		}
	}
}

// compactTree updates the exploration status of this state and
// deallocates its children map if all of its children are explored.
func (s *state) compactTree() {
	for _, child := range s.children {
		if (child.seeded || child.visited) && !child.explored {
			return
		}
	}
	s.children = nil
	s.explored = true
}

// generateStrategy generates a sequence of scheduling decisions that
// will steer an execution towards the state of the execution tree.
func (s *state) generateStrategy() []TID {
	if s.parent == nil {
		return make([]TID, 0)
	}
	return append(s.parent.generateStrategy(), s.tid)
}

// happensBefore checks if the logical time of the transition that
// leads to this state happened before (in the sense of Lamport's
// happens-before relation) the logical time of the transition that
// leads to the given state.
func (s *state) happensBefore(other *state) bool {
	return s.clock.happensBefore(other.clock)
}

// mayInterfereWith checks if the execution of the transition that
// leads to this state may interfere with the execution of the
// transition that leads to the given state.
func (s *state) mayInterfereWith(other *state) bool { //nolint:gocyclo
	if (s.kind == tMutexLock && other.kind == tMutexUnlock) ||
		(s.kind == tMutexUnlock && other.kind == tMutexLock) {
		return false
	}
	if (s.kind == tRWMutexLock && other.kind == tRWMutexUnlock) ||
		(s.kind == tRWMutexUnlock && other.kind == tRWMutexLock) {
		return false
	}
	if (s.kind == tRWMutexRLock && other.kind == tRWMutexUnlock) ||
		(s.kind == tRWMutexRUnlock && other.kind == tRWMutexLock) {
		return false
	}
	for k := range s.readSet {
		if _, found := other.writeSet[k]; found {
			return true
		}
	}
	for k := range s.writeSet {
		if _, found := other.readSet[k]; found {
			return true
		}
	}
	for k := range s.writeSet {
		if _, found := other.writeSet[k]; found {
			return true
		}
	}
	return false
}
