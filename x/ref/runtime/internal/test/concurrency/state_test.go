// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"testing"
)

// createsBranch creates a branch of a simple execution tree used for
// testing the state implementation. This branch emulates an execution
// in which two threads compete for a mutex.
func createBranch(i, j TID) []*choice {
	choices := make([]*choice, 0)
	set := resourceSet{"mutex": struct{}{}}

	{
		c := newChoice()
		c.next = i
		ti := &transition{
			tid:      i,
			clock:    newClock(),
			enabled:  true,
			kind:     tMutexLock,
			readSet:  set,
			writeSet: set,
		}
		ti.clock[i] = 1
		c.transitions[i] = ti
		tj := &transition{
			tid:      j,
			clock:    newClock(),
			enabled:  true,
			kind:     tMutexLock,
			readSet:  set,
			writeSet: set,
		}
		tj.clock[j] = 1
		c.transitions[j] = tj
		choices = append(choices, c)
	}

	{
		c := newChoice()
		c.next = i
		ti := &transition{
			tid:      i,
			clock:    newClock(),
			enabled:  true,
			kind:     tMutexUnlock,
			readSet:  set,
			writeSet: set,
		}
		ti.clock[i] = 2
		c.transitions[i] = ti
		tj := &transition{
			tid:      j,
			clock:    newClock(),
			enabled:  false,
			kind:     tMutexLock,
			readSet:  set,
			writeSet: set,
		}
		tj.clock[j] = 1
		c.transitions[j] = tj
		choices = append(choices, c)
	}

	{
		c := newChoice()
		c.next = j
		tj := &transition{
			tid:      j,
			clock:    newClock(),
			enabled:  true,
			kind:     tMutexLock,
			readSet:  set,
			writeSet: set,
		}
		tj.clock[j] = 1
		c.transitions[j] = tj
		choices = append(choices, c)
	}

	{
		c := newChoice()
		c.next = j
		tj := &transition{
			tid:      j,
			clock:    newClock(),
			enabled:  true,
			kind:     tMutexUnlock,
			readSet:  set,
			writeSet: set,
		}
		tj.clock[i] = 2
		tj.clock[j] = 2
		c.transitions[j] = tj
		choices = append(choices, c)
	}

	return choices
}

// TestCommon checks common operation of the state implementation.
func TestCommon(t *testing.T) {
	leftBranch := createBranch(0, 1)
	rightBranch := createBranch(1, 0)
	root := newState(nil, 0)
	seeds := &stack{}
	if err := root.addBranch(leftBranch, seeds); err != nil {
		t.Fatalf("addBranch() failed: %v", err)
	}
	// Check a new exploration seed has been identified.
	if seeds.Length() != 1 {
		t.Fatalf("Unexpected number of seeds: expected %v, got %v", 1, seeds.Length())
	}
	if err := root.addBranch(rightBranch, seeds); err != nil {
		t.Fatalf("addBranch() failed: %v", err)
	}
	// Check no new exploration seeds have been identified.
	if seeds.Length() != 1 {
		t.Fatalf("Unexpected number of seeds: expected %v, got %v", 1, seeds.Length())
	}
	// Check exploration status have been correctly updated.
	if !root.explored {
		t.Fatalf("Unexpected exploration status: expected %v, got %v", true, root.explored)
	}
	// Check compation of explored children subtrees.
	if len(root.children) != 0 {
		t.Fatalf("Unexpected number of children: expected %v, got %v", true, root.explored)
	}
}

// TestDivergence checks the various types of execution divergence.
func TestDivergence(t *testing.T) {
	// Emulate a missing transition.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		delete(rightBranch[0].transitions, 0)
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
	// Emulate an extra transition.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		rightBranch[0].transitions[2] = &transition{}
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
	// Emulate divergent transition enabledness.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		rightBranch[0].transitions[0].enabled = false
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
	// Emulate divergent transition clock.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		rightBranch[0].transitions[1].clock[0]++
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
	// Emulate divergent transition read set.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		delete(rightBranch[0].transitions[1].readSet, "mutex")
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
	// Emulate divergent transition write set.
	{
		leftBranch := createBranch(0, 1)
		rightBranch := createBranch(1, 0)
		root := newState(nil, 0)
		seeds := &stack{}
		if err := root.addBranch(leftBranch, seeds); err != nil {
			t.Fatalf("addBranch() failed: %v", err)
		}
		delete(rightBranch[0].transitions[1].writeSet, "mutex")
		if err := root.addBranch(rightBranch, seeds); err == nil {
			t.Fatalf("addBranch() did not fail")
		}
	}
}
