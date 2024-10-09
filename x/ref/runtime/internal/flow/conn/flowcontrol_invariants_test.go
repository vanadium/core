// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"

	"v.io/v23/flow"
)

// allBorrowedLocked returns the total number of borrowed tokens across
// both open and closed flows.
func allBorrowedLocked(c *Conn) (uint64, error) {
	totalBorrowed := uint64(0)
	for _, f := range c.flows {
		borrowing, borrowed := f.flowControl.borrowing, f.flowControl.borrowed
		if !borrowing && borrowed != 0 {
			return 0, fmt.Errorf("borrowed: flow %v: not borrowing, but has non-zero borrowed counters: %v", f.id, borrowed)
		}
		totalBorrowed += borrowed
	}
	outstandingBorrowed := uint64(0)
	for _, v := range c.flowControl.outstandingBorrowed {
		outstandingBorrowed += v
	}
	totalBorrowed += outstandingBorrowed
	return totalBorrowed, nil
}

// flowControlBorrowedClosedInvariant checks the invariant that the sum of all borrowed
// counters is equal to the amount by which the shared pool is decremented. This
// invariant holds when all flow close messages have been processed.
func flowControlBorrowedClosedInvariant(c *Conn) (totalBorrowed, shared uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flowControl.mu.Lock()
	defer c.flowControl.mu.Unlock()

	shared = c.flowControl.shared
	totalBorrowed, err = allBorrowedLocked(c)
	if err != nil {
		return
	}
	if c.flowControl.maxShared-c.flowControl.shared != totalBorrowed {
		err = fmt.Errorf("borrowed: sum of borrowed across all flows %v does not match that taken from the shared pool: %v - %v -> %v != %v", totalBorrowed, c.flowControl.maxShared, c.flowControl.shared, c.flowControl.maxShared-c.flowControl.shared, totalBorrowed)
		return
	}
	if c.flowControl.shared > c.flowControl.maxShared {
		err = fmt.Errorf("shared pool is larger than the max allowed: %v > %v", c.flowControl.shared, c.flowControl.maxShared)
		return
	}
	return
}

// flowControlBorrowedInvariant checks the invariant that the sum of all borrowed
// counters is less than or equal to the number of tokens taken from the shared pool.
// The difference between the two values is due to the asynchronous nature of
// returning outstanding borrowed tokens. When all flows are closed the values
// will be identical as per flowControlBorrowedClosedInvariant.
func flowControlBorrowedInvariant(c *Conn) (totalBorrowed, shared uint64, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flowControl.mu.Lock()
	defer c.flowControl.mu.Unlock()

	shared = c.flowControl.shared
	totalBorrowed, err = allBorrowedLocked(c)
	if err != nil {
		return
	}
	if c.flowControl.maxShared-c.flowControl.shared < totalBorrowed {
		err = fmt.Errorf("borrowed: sum of borrowed across all flows %v is greater than that taken from the shared pool: %v - %v -> %v != %v", totalBorrowed, c.flowControl.maxShared, c.flowControl.shared, c.flowControl.maxShared-c.flowControl.shared, totalBorrowed)
		return
	}
	if c.flowControl.shared > c.flowControl.maxShared {
		err = fmt.Errorf("shared pool is larger than the max allowed: %v > %v", c.flowControl.shared, c.flowControl.maxShared)
		return
	}
	return totalBorrowed, c.flowControl.shared, nil
}

func flowControlBorrowed(c *Conn, fid uint64) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flowControl.mu.Lock()
	defer c.flowControl.mu.Unlock()
	return c.flows[fid].flowControl.borrowed
}

func countRemoteBorrowing(c *Conn) int {
	rb := 0
	for _, f := range c.flows {
		if f.flowControl.remoteBorrowing {
			rb++
		}
	}
	return rb
}

func countToRelease(c *Conn) int {
	nc := len(c.flowControl.toReleaseClosed)
	for _, f := range c.flows {
		if f.flowControl.toRelease > 0 {
			nc++
		}
	}
	return nc
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
//     ie. to be released should not exceed the total available, allowing for
//     borrowing on the dial side.
//  3. the total number of released tokens for each flow on the dial side
//     should never exceed the configured bytesBufferPerFlow.
//  4. the total number of tokens to be released on the accept side per flow
//     should never exceed the configured bytesBufferPerFlow plus the total
//     number of borrowed tokens.
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
		totalReleased += int(f.flowControl.released) //nolint:gosec // disable G115
		// invariant 3.
		if f.flowControl.released > f.flowControl.shared.bytesBufferedPerFlow {
			return fmt.Errorf("invariant 3: dial side flow %v has too many released tokens: %v > %v", f.id, f.flowControl.released, f.flowControl.shared.bytesBufferedPerFlow)
		}
	}
	totalAvail := len(dc.flows) * int(ac.flowControl.bytesBufferedPerFlow) //nolint:gosec // disable G115

	// invariant 1.
	if totalReleased > totalAvail {
		return fmt.Errorf("invariant 1: total number of released counters exceed the capacity of the server: %v > %v (# flows %v * buffered per flow %v)", totalReleased, totalAvail, len(dc.flows), int(ac.flowControl.bytesBufferedPerFlow)) //nolint:gosec // disable G115
	}

	nToRelease := 0
	totalToRelease := 0
	// accept side toRelease.
	for _, r := range ac.flowControl.toReleaseClosed {
		if r.Tokens > (ac.flowControl.bytesBufferedPerFlow + ac.flowControl.toReleaseBorrowed) {
			return fmt.Errorf("invariant 4: accept side closed flow %v has too many released tokens: %v > (%v + %v)", r.FlowID, r.Tokens, ac.flowControl.bytesBufferedPerFlow, ac.flowControl.toReleaseBorrowed)
		}
		totalToRelease += int(r.Tokens) //nolint:gosec // disable G115
		nToRelease++
	}

	for _, f := range ac.flows {
		// invariant 4.
		if f.flowControl.toRelease > (f.flowControl.shared.bytesBufferedPerFlow + ac.flowControl.toReleaseBorrowed) {
			return fmt.Errorf("invariant 4: accept side flow %v has too many tokens to release: %v > (%v + %v)", f.id, f.flowControl.toRelease, f.flowControl.shared.bytesBufferedPerFlow, ac.flowControl.toReleaseBorrowed)
		}
		totalToRelease += int(f.flowControl.toRelease) //nolint:gosec // disable G115
		nToRelease++
	}

	totalAvail = nToRelease * int(ac.flowControl.bytesBufferedPerFlow) //nolint:gosec // disable G115
	totalAvail += int(ac.flowControl.toReleaseBorrowed)                //nolint:gosec // disable G115

	// invariant 2.
	if totalToRelease > totalAvail {
		return fmt.Errorf("invariant 2: total number of toRelease counters exceed the capacity of the server: %v > %v (# flows %v * buffered per flow %v)", totalToRelease, totalAvail, nToRelease, int(ac.flowControl.bytesBufferedPerFlow)) //nolint:gosec // disable G115
	}

	return nil
}

func flowControlReleasedInvariantBidirectional(dc, ac *Conn) error {
	if err := flowControlReleasedInvariant(dc, ac); err != nil {
		return fmt.Errorf("flowControlReleasedInvariant: dial->accept: %v", err)
	}
	if err := flowControlReleasedInvariant(ac, dc); err != nil {
		return fmt.Errorf("flowControlReleasedInvariant: accept->dial: %v", err)
	}
	return nil
}

func flowControlBorrowedInvariantBoth(dc, ac *Conn) error {
	if _, _, err := flowControlBorrowedInvariant(dc); err != nil {
		return fmt.Errorf("flowControlBorrowedInvariant: dial: %v", err)
	}
	if _, _, err := flowControlBorrowedInvariant(ac); err != nil {
		return fmt.Errorf("flowControlBorrowedClosedInvariant: accept: %v", err)
	}
	return nil
}

func flowControlBorrowedClosedInvariantBoth(dc, ac *Conn) error {
	if _, _, err := flowControlBorrowedClosedInvariant(dc); err != nil {
		return fmt.Errorf("flowControlBorrowedClosedInvariant: dial: %v", err)
	}
	if _, _, err := flowControlBorrowedClosedInvariant(ac); err != nil {
		return fmt.Errorf("flowControlBorrowedClosedInvariant: accept: %v", err)
	}
	return nil
}
