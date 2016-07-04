// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"testing"
	"time"

	"v.io/x/ref/services/syncbase/store"
)

func TestVClockBasic(t *testing.T) {
	sysTs := time.Now()
	c := NewVClockForTests(newMockSystemClock(sysTs, 0))
	if ts, err := c.Now(); err != nil {
		t.Errorf("c.Now failed: %v", err)
	} else if ts != sysTs {
		t.Errorf("got %v, want %v", ts, sysTs)
	}
}

func TestVClockWithSkew(t *testing.T) {
	// test with positive skew
	checkSkew(t, 5)
	// test with negative skew
	checkSkew(t, -5)
}

func checkSkew(t *testing.T, skew time.Duration) {
	sysTs := time.Now()
	c := NewVClockForTests(newMockSystemClock(sysTs, 0))

	ntpTs := sysTs.Add(time.Duration(-20))
	elapsedTime := 100 * time.Nanosecond
	bootTime := sysTs.Add(-elapsedTime)

	if err := store.Put(nil, c.st, vclockDataKey, newVClockData(bootTime, skew, elapsedTime, ntpTs, 1, 1)); err != nil {
		t.Errorf("Writing VClockData failed: %v", err)
	}

	if ts, err := c.Now(); err != nil {
		t.Errorf("c.Now failed: %v", err)
	} else if ts != sysTs.Add(skew) {
		t.Errorf("expected ts == sysTs + skew: %v, %v, %v", ts, sysTs, skew)
	}
}

func TestApplySkew(t *testing.T) {
	c := NewVClockForTests(nil)
	data := newVClockData(time.Time{}, time.Minute, 0, time.Time{}, 0, 0)
	sysTs := time.Now()
	adjTs := c.ApplySkew(sysTs, data)
	if adjTs.Sub(sysTs) != time.Minute {
		t.Errorf("Unexpected diff: sysTs %v, adjTs %v", sysTs, adjTs)
	}
}
