// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"math/rand"
	"testing"
	"time"
)

func TestHasSysClockChangedWithRealVClock(t *testing.T) {
	for i := 0; i < 10; i++ {
		sysClock := newRealSystemClock()
		e1, err := sysClock.ElapsedTime()
		t1 := sysClock.Now()
		if err != nil {
			t.Errorf("Found error while fetching e1: %v", err)
		}

		// spend some time.
		d := time.Duration(rand.Int63n(50)) * time.Millisecond
		time.Sleep(d)

		t2 := sysClock.Now()
		e2, err := sysClock.ElapsedTime()
		if err != nil {
			t.Errorf("Found error while fetching e2: %v", err)
		}

		if hasSysClockChanged(t1, t2, e1, e2) {
			t.Errorf("VClock found changed incorrectly. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
		}
	}
}

func TestHasSysClockChangedFakeVClock(t *testing.T) {
	e1 := 2000 * time.Millisecond
	t1 := time.Now()

	// elapsed time diff slightly greater than vclock diff.
	t2 := t1.Add(200 * time.Millisecond)
	e2 := e1 + 300*time.Millisecond

	if hasSysClockChanged(t1, t2, e1, e2) {
		t.Errorf("VClock found changed incorrectly. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
	}

	// elapsed time diff slightly smaller than vclock diff.
	t2 = t1.Add(300 * time.Millisecond)
	e2 = e1 + 200*time.Millisecond

	if hasSysClockChanged(t1, t2, e1, e2) {
		t.Errorf("VClock found changed incorrectly. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
	}

	// elapsed time diff much greater than vclock diff.
	t2 = t1.Add(200 * time.Millisecond)
	e2 = e1 + 3000*time.Millisecond

	if !hasSysClockChanged(t1, t2, e1, e2) {
		t.Errorf("VClock changed but not caught. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
	}

	// elapsed time diff much smaller than vclock diff.
	t2 = t1.Add(4000 * time.Millisecond)
	e2 = e1 + 300*time.Millisecond

	if !hasSysClockChanged(t1, t2, e1, e2) {
		t.Errorf("VClock changed but not caught. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
	}

	// vclock diff is negative
	t2 = t1.Add(-200 * time.Millisecond)
	e2 = e1 + 300*time.Millisecond

	if !hasSysClockChanged(t1, t2, e1, e2) {
		t.Errorf("VClock changed but not caught. e1: %v, t1: %v, t2: %v, e2: %v", e1, t1, t2, e2)
	}
}
