// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concurrency

import (
	"reflect"
)

// clock is a type for the vector clock, which is used to keep track
// of logical time in a concurrent program.
type clock map[TID]int

// newClock is the clock factory.
func newClock() clock {
	return make(clock)
}

// clone produces a copy of the clock.
func (c clock) clone() clock {
	clone := newClock()
	for k, v := range c {
		clone[k] = v
	}
	return clone
}

// equals checks if this clock identifies a logical time identical to
// the logical time identified by the given clock.
func (c clock) equals(other clock) bool {
	return reflect.DeepEqual(c, other)
}

// happensBefore checks if this clock identifies a logical time that
// happened before (in the sense of Lamport's happens-before relation)
// the logical time identified by the given clock.
func (c clock) happensBefore(other clock) bool {
	for k, v := range c {
		if value, found := other[k]; !found || v > value {
			return false
		}
	}
	return true
}

// merge merges the value of the given clock with this clock.
func (c clock) merge(other clock) {
	for key, value := range other {
		if v, found := c[key]; !found || v < value {
			c[key] = value
		}
	}
}
