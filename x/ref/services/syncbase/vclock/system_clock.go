// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the SystemClock interface as well as two implementations of
// this interface, realSystemClock and fakeSystemClock.

package vclock

import (
	"time"
)

// SystemClock provides access to system clock data.
type SystemClock interface {
	// Now returns the system's notion of current UTC time (which may be off).
	Now() time.Time

	// ElapsedTime returns the time elapsed since this device booted.
	ElapsedTime() (time.Duration, error)
}

////////////////////////////////////////
// realSystemClock

type realSystemClock struct{}

var _ SystemClock = (*realSystemClock)(nil)

func (*realSystemClock) Now() time.Time {
	return time.Now().UTC()
}

func newRealSystemClock() SystemClock {
	return &realSystemClock{}
}

////////////////////////////////////////
// fakeSystemClock

// Note, fakeSystemClock assumes the real system clock time is never changed.
// This assumption is reasonable because fakeSystemClock is meant only for
// testing.

type fakeSystemClock struct {
	realClock   SystemClock
	initRealNow time.Time
	initNow     time.Time
	initElapsed time.Duration
}

var _ SystemClock = (*fakeSystemClock)(nil)

func (c *fakeSystemClock) Now() time.Time {
	return c.initNow.Add(c.realElapsed())
}

func (c *fakeSystemClock) ElapsedTime() (time.Duration, error) {
	return time.Duration(c.initElapsed + c.realElapsed()), nil
}

func (c *fakeSystemClock) realElapsed() time.Duration {
	// See comment above. We assume the real system clock time is never changed.
	return c.realClock.Now().Sub(c.initRealNow)
}

func newFakeSystemClock(now time.Time, elapsed time.Duration) SystemClock {
	realClock := newRealSystemClock()
	return &fakeSystemClock{
		realClock:   realClock,
		initRealNow: realClock.Now(),
		initNow:     now,
		initElapsed: elapsed,
	}
}
