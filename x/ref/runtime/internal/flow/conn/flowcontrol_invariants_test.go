// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"

	"v.io/v23/flow"
)

// flowControlBorrowedInvariant checks the invariant that the sum of all borrowed
// counters is equal to the amount by which the shared pool is decremented.
func flowControlBorrowedInvariant(c *Conn) (totalBorrowed, shared uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flowControl.mu.Lock()
	defer c.flowControl.mu.Unlock()

	for _, f := range c.flows {
		borrowing, borrowed := f.flowControl.borrowing, f.flowControl.borrowed
		if !borrowing && borrowed != 0 {
			return 0, 0, fmt.Errorf("borrowed: flow %v: not borrowing, but has non-zero borrowed counters: %v", f.id, borrowed)
		}
		totalBorrowed += borrowed
	}
	if c.flowControl.bytesBufferedPerFlow-c.flowControl.lshared != totalBorrowed {
		return totalBorrowed, c.flowControl.lshared, fmt.Errorf("borrowed: sum of borrowed across all flows %v does not match that taken from the shared pool: %v - %v -> %v != %v", totalBorrowed, c.flowControl.bytesBufferedPerFlow, c.flowControl.lshared, c.flowControl.bytesBufferedPerFlow-c.flowControl.lshared, totalBorrowed)
	}
	return totalBorrowed, c.flowControl.lshared, nil
}

func flowControlBorrowed(c *Conn) map[uint64]uint64 {
	borrowed := map[uint64]uint64{}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flowControl.mu.Lock()
	defer c.flowControl.mu.Unlock()
	for _, f := range c.flows {
		borrowed[f.id] = f.flowControl.borrowed
	}
	return borrowed
}

func flowID(f flow.Flow) uint64 {
	t := f.(*flw)
	return t.id
}

// flowControlReleasedInvariant checks the invariant for released/toRelease
// counters. In particular:
//  1. the total number of released counters on the dial side cannot exceed
//     the total capacity of counters that can be released by the acceptor.
//  2. similarly for the accept side, the total number of counters available,
//     ie. to be released should not exceed the total available.
func flowControlReleasedInvariant(dc, ac *Conn) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	ac.mu.Lock()
	defer ac.mu.Unlock()
	dc.flowControl.mu.Lock()
	defer dc.flowControl.mu.Unlock()
	ac.flowControl.mu.Lock()
	defer ac.flowControl.mu.Unlock()

	// dial side released
	totalReleased := 0
	for _, f := range dc.flows {
		totalReleased += int(f.flowControl.released)
	}

	// invariant 1.
	if totalReleased > len(dc.flows)*int(ac.flowControl.bytesBufferedPerFlow) {
		return fmt.Errorf("total number of released counters exceed the capacity of the server: %v > %v (# flows %v, * buffered per flow:%v)", totalReleased, len(dc.flows)*int(ac.flowControl.bytesBufferedPerFlow), totalReleased, len(dc.flows)*int(ac.flowControl.bytesBufferedPerFlow))
	}

	totalToRelease := 0
	// accept side toRelease.
	for _, r := range ac.flowControl.toRelease {
		totalToRelease += int(r)
	}

	// invariant 2.
	if totalToRelease > len(ac.flowControl.toRelease)*int(ac.flowControl.bytesBufferedPerFlow) {
		return fmt.Errorf("total number of toRelease counters exceed the capacity of the server: %v > %v (# flows %v, * buffered per flow:%v)", totalToRelease, len(ac.flowControl.toRelease)*int(ac.flowControl.bytesBufferedPerFlow), totalReleased, len(ac.flowControl.toRelease)*int(ac.flowControl.bytesBufferedPerFlow))
	}

	return nil
}
