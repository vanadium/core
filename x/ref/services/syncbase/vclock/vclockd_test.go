// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"net"
	"testing"
	"time"

	"v.io/x/ref/services/syncbase/store"
)

const (
	constElapsedTime = 50 * time.Nanosecond
)

func putVClockData(t *testing.T, st store.Store, data *VClockData) {
	if err := store.Put(nil, st, vclockDataKey, data); err != nil {
		t.Fatalf("Writing VClockData failed: %v", err)
	}
}

func newVClockDForTests(vclock *VClock, ntpSource NtpSource) *VClockD {
	return &VClockD{
		vclock:    vclock,
		ntpSource: ntpSource,
		closed:    make(chan struct{}),
	}
}

////////////////////////////////////////////////////////////////////////////////
// Tests for DoLocalUpdate

// No reboot, no drift.
func TestLocalNormal(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	d := newVClockDForTests(NewVClockForTests(sysClock), nil)

	elapsedTime := time.Second
	sysClock.SetNow(sysTs.Add(elapsedTime))
	sysClock.SetElapsedTime(elapsedTime)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs, 0, elapsedTime, time.Time{}, 0, 0))
}

// No reboot, no drift. Values of Skew, LastNtpTs, NumReboots, and NumHops
// should not be touched.
func TestLocalNormalWithOtherValues(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	d := newVClockDForTests(NewVClockForTests(sysClock), nil)

	origData := newVClockData(sysTs, time.Minute, 0, sysTs.Add(-time.Hour), 2, 4)
	putVClockData(t, d.vclock.st, origData)

	elapsedTime := time.Second
	sysClock.SetNow(sysTs.Add(elapsedTime))
	sysClock.SetElapsedTime(elapsedTime)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs, time.Minute, elapsedTime, sysTs.Add(-time.Hour), 2, 4))
}

// System clock elapsedTime < VClockData.ElapsedTimeSinceBoot.
// We should detect a reboot.
func TestLocalReboot(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, time.Hour)

	d := newVClockDForTests(NewVClockForTests(sysClock), nil)

	elapsedTime := time.Minute
	sysClock.SetNow(sysTs.Add(2 * time.Hour))
	sysClock.SetElapsedTime(elapsedTime)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysClock.Now().Add(-elapsedTime), 0, elapsedTime, time.Time{}, 1, 0)) // NumReboots=1
}

// No reboot, small drift.
func TestLocalDriftBelowThreshold(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	d := newVClockDForTests(NewVClockForTests(sysClock), nil)

	elapsedTime := time.Minute
	sysClock.SetNow(sysTs.Add(59 * time.Second))
	sysClock.SetElapsedTime(elapsedTime)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-1*time.Second), 0, elapsedTime, time.Time{}, 0, 0))
}

// No reboot, large drift. We should adjust skew.
func TestLocalDriftAboveThreshold(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	d := newVClockDForTests(NewVClockForTests(sysClock), nil)

	elapsedTime := time.Minute
	sysClock.SetNow(sysTs.Add(55 * time.Second))
	sysClock.SetElapsedTime(elapsedTime)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-5*time.Second), 5*time.Second, elapsedTime, time.Time{}, 0, 0))
}

// Runs DoLocalUpdate with a real system clock and verifies that the behavior is
// as expected.
func TestLocalWithRealSysClock(t *testing.T) {
	d := newVClockDForTests(NewVClockForTests(nil), nil)
	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}

	// Get initial VClockData, written by NewVClockForTests.
	origData := &VClockData{}
	if err := d.vclock.GetVClockData(origData); err != nil {
		t.Errorf("GetVClockData failed: %v", err)
	}

	// Bigger than error margin for the difference between two elapsed time
	// samples, which is one second.
	time.Sleep(2 * time.Second)

	if err := d.DoLocalUpdate(); err != nil {
		t.Errorf("DoLocalUpdate failed: %v", err)
	}

	// Check that the new VClockData has approximately the same SystemTimeAtBoot,
	// a bigger ElapsedTimeSinceBoot, and everything else unchanged.
	newData := &VClockData{}
	if err := d.vclock.GetVClockData(newData); err != nil {
		t.Errorf("GetVClockData failed: %v", err)
	}

	checkEqSubsetOfVClockData(t, *origData, *newData)
	delta := abs(newData.SystemTimeAtBoot.Sub(origData.SystemTimeAtBoot))
	if delta > 1100*time.Millisecond {
		t.Errorf("unexpected difference in SystemTimeAtBoot: %v, %v", origData, newData)
	}
	delta = abs(newData.ElapsedTimeSinceBoot - origData.ElapsedTimeSinceBoot)
	if delta < 900*time.Millisecond || delta > 3100*time.Millisecond {
		t.Errorf("unexpected difference in ElapsedTimeSinceBoot: %v, %v", origData, newData)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Tests for DoNtpUpdate

func TestNtpError(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	ntpSource := newMockNtpSource()
	ntpSource.Err = net.UnknownNetworkError("test")

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	if err := d.DoNtpUpdate(); err == nil {
		t.Errorf("expected error from DoNtpUpdate")
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs, 0, 0, time.Time{}, 0, 0))
}

func TestNtpErrorDoesNotTouchOtherFields(t *testing.T) {
	sysTs := time.Now()
	sysClock := newMockSystemClock(sysTs, 0)

	ntpSource := newMockNtpSource()
	ntpSource.Err = net.UnknownNetworkError("test")

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	origData := newVClockData(sysTs, time.Minute, 2*time.Minute, sysTs.Add(-time.Hour), 2, 4)
	putVClockData(t, d.vclock.st, origData)

	if err := d.DoNtpUpdate(); err == nil {
		t.Errorf("expected error from DoNtpUpdate")
	}
	verifyVClockData(t, d.vclock, origData)
}

func TestNtpSkewBelowThreshold(t *testing.T) {
	sysTs := time.Now()
	elapsedTime := 50 * time.Nanosecond
	sysClock := newMockSystemClock(sysTs, elapsedTime)

	ntpSource := newMockNtpSource()
	skew := 1900 * time.Millisecond // error threshold is 2 seconds
	ntpSource.Data = &NtpData{
		skew:  skew,
		delay: 5 * time.Millisecond,
		ntpTs: sysTs.Add(skew),
	}

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	if err := d.DoNtpUpdate(); err != nil {
		t.Errorf("DoNtpUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-elapsedTime), 0, elapsedTime, ntpSource.Data.ntpTs, 0, 0))
}

func TestNtpSkewBelowThresholdOtherFields(t *testing.T) {
	sysTs := time.Now()
	elapsedTime := 50 * time.Nanosecond
	sysClock := newMockSystemClock(sysTs, elapsedTime)

	ntpSource := newMockNtpSource()
	skew := 1900 * time.Millisecond // error threshold is 2 seconds
	ntpSource.Data = &NtpData{
		skew:  skew,
		delay: 5 * time.Millisecond,
		ntpTs: sysTs.Add(skew),
	}

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	origData := newVClockData(sysTs.Add(-elapsedTime), time.Second, 2*time.Minute, sysTs.Add(-time.Hour), 2, 4)
	putVClockData(t, d.vclock.st, origData)

	if err := d.DoNtpUpdate(); err != nil {
		t.Errorf("DoNtpUpdate failed: %v", err)
	}
	// Skew shouldn't change, but NumReboots and NumHops should be set to 0.
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-elapsedTime), time.Second, elapsedTime, ntpSource.Data.ntpTs, 0, 0))
}

func TestNtpSkewAboveThreshold(t *testing.T) {
	sysTs := time.Now()
	elapsedTime := 10 * time.Minute
	sysClock := newMockSystemClock(sysTs, elapsedTime)

	ntpSource := newMockNtpSource()
	skew := 2100 * time.Millisecond // error threshold is 2 seconds
	ntpSource.Data = &NtpData{
		skew:  skew,
		delay: 5 * time.Millisecond,
		ntpTs: sysTs.Add(skew),
	}

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	if err := d.DoNtpUpdate(); err != nil {
		t.Errorf("DoNtpUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-elapsedTime), skew, elapsedTime, ntpSource.Data.ntpTs, 0, 0))
}

func TestNtpSkewBelowThresholdAndExistingLargeSkew(t *testing.T) {
	sysTs := time.Now()
	elapsedTime := 10 * time.Minute
	sysClock := newMockSystemClock(sysTs, elapsedTime)

	ntpSource := newMockNtpSource()
	skew := 200 * time.Millisecond // error threshold is 2 seconds
	ntpSource.Data = &NtpData{
		skew:  skew,
		delay: 5 * time.Millisecond,
		ntpTs: sysTs.Add(skew),
	}

	d := newVClockDForTests(NewVClockForTests(sysClock), ntpSource)

	// Note, this test also verifies that DoNtpUpdate updates SystemTimeAtBoot and
	// ElapsedTimeSinceBoot.
	origData := &VClockData{Skew: 2300 * time.Millisecond} // large skew
	putVClockData(t, d.vclock.st, origData)

	if err := d.DoNtpUpdate(); err != nil {
		t.Errorf("DoNtpUpdate failed: %v", err)
	}
	verifyVClockData(t, d.vclock, newVClockData(sysTs.Add(-elapsedTime), skew, elapsedTime, ntpSource.Data.ntpTs, 0, 0))
}
