// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package timekeeper

import (
	"testing"
	"time"
)

func TestAfter(t *testing.T) {
	tk := RealTime()
	before := time.Now()
	timeToSleep := 500000000 * time.Nanosecond // Half a second.
	<-tk.After(timeToSleep)
	after := time.Now()
	if after.Before(before.Add(timeToSleep / 2)) {
		t.Errorf("Too short: %s", after.Sub(before))
	}
	if after.After(before.Add(timeToSleep * 2)) {
		t.Errorf("Too long: %s", after.Sub(before))
	}
}

func TestSleep(t *testing.T) {
	tk := RealTime()
	before := time.Now()
	timeToSleep := 500000000 * time.Nanosecond // Half a second.
	tk.Sleep(timeToSleep)
	after := time.Now()
	if after.Before(before.Add(timeToSleep / 2)) {
		t.Errorf("Too short: %s", after.Sub(before))
	}
	if after.After(before.Add(timeToSleep * 2)) {
		t.Errorf("Too long: %s", after.Sub(before))
	}
}
