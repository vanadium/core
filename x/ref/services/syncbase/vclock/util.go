// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

// This file defines utility functions for the vclock package.

import (
	"time"

	"v.io/x/lib/vlog"
)

// Note: Individual elapsed time samples have an error margin of 0.5 seconds, so
// the error margin for the difference between two elapsed time samples is 1
// second.
var errorThreshold = 2 * time.Second

// hasSysClockChanged returns true if the system clock has changed between two
// system clock samples of {Now, ElapsedTime}. {t1, e1} is the first sample;
// {t2, e2} is the second. To avoid races, e1 must be sampled before t1, and e2
// must be sampled after t2.
func hasSysClockChanged(t1, t2 time.Time, e1, e2 time.Duration) bool {
	if t2.Before(t1) {
		vlog.VI(2).Infof("vclock: HasSysClockChanged: t2 < t1, returning true")
		return true
	}
	return abs((e2-e1)-t2.Sub(t1)) > errorThreshold
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
