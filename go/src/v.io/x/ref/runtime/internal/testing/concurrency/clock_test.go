// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"testing"

	"v.io/x/ref/test/testutil"
)

// TestClone checks the clone() method of a clock.
func TestClone(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	c1 := newClock()
	c1[0] = testutil.RandomIntn(100)
	c2 := c1.clone()
	c1[0]++
	if c2[0] != c1[0]-1 {
		t.Errorf("Unexpected clock value: expected %v, got %v", c1[0]-1, c2[0])
	}
}

// TestEquality checks the equals() method of a clock.
func TestEquality(t *testing.T) {
	c1, c2 := newClock(), newClock()
	for i := TID(0); i < TID(10); i++ {
		c1[i] = testutil.RandomIntn(100)
		c2[i] = c1[i]
	}
	if !c1.equals(c2) {
		t.Errorf("Unexpected inequality between %v and %v", c1, c2)
	}
}

// TestHappensBefore checks the happensBefore() method of a clock.
func TestHappensBefore(t *testing.T) {
	c1, c2, c3 := newClock(), newClock(), newClock()
	for i := TID(0); i < TID(10); i++ {
		c1[i] = testutil.RandomIntn(100)
		if i%2 == 0 {
			c2[i] = c1[i] + 1
			c3[i] = c1[i] + 1
		} else {
			c2[i] = c1[i]
			c3[i] = c2[i] - 1
		}
	}
	if !c1.happensBefore(c1) {
		t.Errorf("Unexpected outcome of %v.happensBefore(%v): expected %v, got %v", c1, c1, true, false)
	}
	if !c1.happensBefore(c2) {
		t.Errorf("Unexpected outcome of %v.happensBefore(%v): expected %v, got %v", c1, c2, true, false)
	}
	if c2.happensBefore(c1) {
		t.Errorf("Unexpected outcome of %v.happensBefore(%v): expected %v, got %v", c2, c1, false, true)
	}
	if c1.happensBefore(c3) {
		t.Errorf("Unexpected outcome of %v.happensBefore(%v): expected %v, got %v", c1, c3, false, true)
	}
	if c3.happensBefore(c1) {
		t.Errorf("Unexpected outcome of %v.happensBefore(%v): expected %v, got %v", c3, c1, false, true)
	}
}

// TestMerge checks the merge() method of a clock.
func TestMerge(t *testing.T) {
	c1, c2 := newClock(), newClock()
	for i := TID(0); i < TID(10); i++ {
		c1[i] = testutil.RandomIntn(100)
		c2[i] = testutil.RandomIntn(100)
	}
	c1.merge(c2)
	for i := TID(0); i < TID(10); i++ {
		if c1[i] < c2[i] {
			t.Errorf("Unexpected order between %v and %v: expected '>=', got '<'", c1[i], c2[i])
		}
	}
}
