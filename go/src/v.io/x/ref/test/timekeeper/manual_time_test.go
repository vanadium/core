// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package timekeeper

import (
	"testing"
	"time"
)

func checkNotReady(t *testing.T, ch <-chan time.Time) {
	select {
	case <-ch:
		t.Errorf("Channel not supposed to be ready")
	default:
	}
}

func checkReady(t *testing.T, ch <-chan time.Time) {
	select {
	case <-ch:
	default:
		t.Errorf("Channel supposed to be ready")
	}
}

func expectRequest(t *testing.T, ch <-chan time.Duration, expect time.Duration) {
	select {
	case got := <-ch:
		if got != expect {
			t.Errorf("Expected %v, got %v instead", expect, got)
		}
	default:
		t.Errorf("Nothing received on channel")
	}
}

func TestAfter(t *testing.T) {
	mt := NewManualTime()
	ch1 := mt.After(5 * time.Second)
	ch2 := mt.After(3 * time.Second)
	checkNotReady(t, ch1)
	checkNotReady(t, ch2)
	expectRequest(t, mt.Requests(), 5*time.Second)
	expectRequest(t, mt.Requests(), 3*time.Second)

	mt.AdvanceTime(time.Second)
	checkNotReady(t, ch1)
	checkNotReady(t, ch2)
	ch3 := mt.After(2 * time.Second)
	checkNotReady(t, ch3)
	expectRequest(t, mt.Requests(), 2*time.Second)

	mt.AdvanceTime(2 * time.Second)
	checkNotReady(t, ch1)
	checkReady(t, ch2)
	checkReady(t, ch3)

	mt.AdvanceTime(time.Second)
	checkNotReady(t, ch1)
	checkNotReady(t, ch2)
	checkNotReady(t, ch3)

	mt.AdvanceTime(time.Second)
	checkReady(t, ch1)
	checkNotReady(t, ch2)
	checkNotReady(t, ch3)

	ch4 := mt.After(0)
	checkReady(t, ch4)
	expectRequest(t, mt.Requests(), 0)
}

func TestSleep(t *testing.T) {
	mt := NewManualTime()
	c := make(chan time.Time, 1)
	go func() {
		mt.Sleep(5 * time.Second)
		c <- time.Time{}
		mt.Sleep(3 * time.Second)
		c <- time.Time{}
	}()
	if got, expect := <-mt.Requests(), 5*time.Second; got != expect {
		t.Errorf("Expected %v, got %v instead", expect, got)
	}
	checkNotReady(t, c)
	mt.AdvanceTime(5 * time.Second)
	if got, expect := <-mt.Requests(), 3*time.Second; got != expect {
		t.Errorf("Expected %v, got %v instead", expect, got)
	}
	checkReady(t, c)
	mt.AdvanceTime(2 * time.Second)
	checkNotReady(t, c)
	mt.AdvanceTime(1 * time.Second)
	<-c
}

func TestBlocking(t *testing.T) {
	mt := NewManualTime()
	sync := make(chan bool)
	go func() {
		// Simulate blocking on a timer.
		<-mt.After(10 * time.Second)
		<-mt.After(2 * time.Second)
		sync <- true
		<-mt.After(4 * time.Second)
		sync <- true
	}()
	<-mt.Requests() // 10
	mt.AdvanceTime(11 * time.Second)
	<-mt.Requests() // 2
	mt.AdvanceTime(3 * time.Second)
	mt.AdvanceTime(time.Second)
	<-sync
	<-mt.Requests() // 4
	mt.AdvanceTime(5 * time.Second)
	<-sync
}
