// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

// Utilities for testing vclock.

import (
	"reflect"
	"testing"
	"time"

	"v.io/x/ref/services/syncbase/store/memstore"
)

////////////////////////////////////////
// Mock SystemClock

var _ SystemClock = (*mockSystemClock)(nil)

func newMockSystemClock(now time.Time, elapsedTime time.Duration) *mockSystemClock {
	return &mockSystemClock{
		now:         now,
		elapsedTime: elapsedTime,
	}
}

type mockSystemClock struct {
	now         time.Time
	elapsedTime time.Duration
}

func (sc *mockSystemClock) Now() time.Time {
	return sc.now
}

func (sc *mockSystemClock) SetNow(now time.Time) {
	sc.now = now
}

func (sc *mockSystemClock) ElapsedTime() (time.Duration, error) {
	return sc.elapsedTime, nil
}

func (sc *mockSystemClock) SetElapsedTime(elapsedTime time.Duration) {
	sc.elapsedTime = elapsedTime
}

////////////////////////////////////////
// Mock NtpSource

var _ NtpSource = (*ntpSourceMockImpl)(nil)

func newMockNtpSource() *ntpSourceMockImpl {
	return &ntpSourceMockImpl{}
}

type ntpSourceMockImpl struct {
	Err  error
	Data *NtpData
}

func (ns *ntpSourceMockImpl) NtpSync(sampleCount int) (*NtpData, error) {
	if ns.Err != nil {
		return nil, ns.Err
	}
	return ns.Data, nil
}

// NewVClockForTests returns a *VClock suitable for tests. It mimics "real"
// Syncbase behavior in that it calls InitVClockData to ensure that VClockData
// exists in the store.
func NewVClockForTests(sc SystemClock) *VClock {
	if sc == nil {
		sc = newRealSystemClock()
	}
	c := &VClock{
		st:       memstore.New(),
		sysClock: sc,
	}
	if err := c.InitVClockData(); err != nil {
		panic(err)
	}
	return c
}

////////////////////////////////////////
// Utility functions

// checkEqVClockData checks equality of got and want.
func checkEqVClockData(t *testing.T, got, want VClockData) {
	// Make sure all time.Time location values are UTC so that DeepEqual doesn't
	// trip up over unequal locations. Locations are irrelevant for us: they are
	// not persisted, and they only affect the presentation of time.Time values
	// (e.g. when printing using fmt).
	got.SystemTimeAtBoot = got.SystemTimeAtBoot.UTC()
	got.LastNtpTs = got.LastNtpTs.UTC()
	want.SystemTimeAtBoot = want.SystemTimeAtBoot.UTC()
	want.LastNtpTs = want.LastNtpTs.UTC()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

// checkEqSubsetOfVClockData checks equality of got and want, ignoring
// SystemTimeAtBoot and ElapsedTimeSinceBoot.
func checkEqSubsetOfVClockData(t *testing.T, got, want VClockData) {
	got.SystemTimeAtBoot = time.Time{}
	got.ElapsedTimeSinceBoot = 0
	want.SystemTimeAtBoot = time.Time{}
	want.ElapsedTimeSinceBoot = 0
	checkEqVClockData(t, got, want)
}

func verifyVClockData(t *testing.T, c *VClock, want *VClockData) {
	got := &VClockData{}
	if err := c.GetVClockData(got); err != nil {
		t.Errorf("Error fetching VClockData: %v", err)
		return
	}
	checkEqVClockData(t, *got, *want)
}

func newVClockData(systemTimeAtBoot time.Time, skew, elapsedTimeSinceBoot time.Duration, lastNtpTs time.Time, numReboots, numHops uint16) *VClockData {
	return &VClockData{
		SystemTimeAtBoot:     systemTimeAtBoot,
		Skew:                 skew,
		ElapsedTimeSinceBoot: elapsedTimeSinceBoot,
		LastNtpTs:            lastNtpTs,
		NumReboots:           numReboots,
		NumHops:              numHops,
	}
}
