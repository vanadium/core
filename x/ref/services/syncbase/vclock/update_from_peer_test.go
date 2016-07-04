// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"testing"
	"time"
)

// ** Test Setup **
// Local: no NTP info
// Remote: no NTP info
// Result: VClock not synced
func TestVClockCheckLocalVClock1(t *testing.T) {
	localSkew := 3 * time.Second
	data := newSyncVClockData(localSkew, time.Time{}, 0, 0)

	remoteDelta := 5 * time.Second
	psd := newPeerSyncData(remoteDelta, time.Time{}, 0, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

// ** Test Setup **
// Local: no NTP info
// Remote: has NTP info with acceptable {numReboots, numHops}
// Result: VClock synced
func TestVClockCheckLocalVClock2(t *testing.T) {
	localSkew := 3 * time.Second
	data := newSyncVClockData(localSkew, time.Time{}, 0, 0)

	remoteDelta := 5 * time.Second
	remoteNtpTs := time.Now().Add(-30 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 0, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err != nil {
		t.Errorf("expected update, got err: %v", err)
	} else {
		checkEqSubsetOfVClockData(t, *newData, *newSyncVClockData(remoteDelta+localSkew, remoteNtpTs, 0, 1))
	}
}

// ** Test Setup **
// Local: has with NTP info
// Remote: no NTP info
// Result: VClock not synced
func TestVClockCheckLocalVClock3(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 0, 0)

	remoteDelta := 5 * time.Second
	psd := newPeerSyncData(remoteDelta, time.Time{}, 0, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp > RemoteNtp, acceptable values for {reboot,hop}
// Result: VClock not synced
func TestVClockCheckLocalVClock4(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 1, 1)

	remoteDelta := 5 * time.Second
	remoteNtpTs := localNtpTs.Add(-10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 0, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp < RemoteNtp, Remote-numReboots = 0, Remote-numHops = 1
// Result: VClock synced, skew = oldSkew + offset, numReboots = 0, numHops = 2
func TestVClockCheckLocalVClock5(t *testing.T) {
	localSkew := 5 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 0, 0)

	remoteDelta := 8 * time.Second
	remoteNtpTs := localNtpTs.Add(10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 0, 1)

	if newData, err := maybeUpdateFromPeerData(data, psd); err != nil {
		t.Errorf("expected update, got err: %v", err)
	} else {
		checkEqSubsetOfVClockData(t, *newData, *newSyncVClockData(remoteDelta+localSkew, remoteNtpTs, 0, 2))
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp < RemoteNtp, unacceptable value for numHops
// Result: VClock not synced
func TestVClockCheckLocalVClock6(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 1, 1)

	remoteDelta := 5 * time.Second
	remoteNtpTs := localNtpTs.Add(10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 0, 2)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp < RemoteNtp, Remote-numReboots > 0 but Diff between two vclocks < 1
// minute
// Result: VClock synced, skew = oldSkew + offset,
// numReboots = Remote-numReboots+1, numHops = 1
func TestVClockCheckLocalVClock7(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 0, 0)

	remoteDelta := 5 * time.Second
	remoteNtpTs := localNtpTs.Add(10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 3, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err != nil {
		t.Errorf("expected update, got err: %v", err)
	} else {
		checkEqSubsetOfVClockData(t, *newData, *newSyncVClockData(remoteDelta+localSkew, remoteNtpTs, 3, 1))
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp < RemoteNtp, Remote-numReboots > 0 and difference between two
// vclocks > 1 minute
// Result: VClock not synced
func TestVClockCheckLocalVClock8(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 0, 0)

	remoteDelta := 5 * time.Hour
	remoteNtpTs := localNtpTs.Add(10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 3, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

// ** Test Setup **
// Local & Remote have NTP info.
// LocalNtp < RemoteNtp, but the difference between the two vclocks is too small
// Result: VClock not synced
func TestVClockCheckLocalVClock9(t *testing.T) {
	localSkew := 3 * time.Second
	localNtpTs := time.Now().Add(-30 * time.Minute)
	data := newSyncVClockData(localSkew, localNtpTs, 1, 1)

	remoteDelta := 1 * time.Second
	remoteNtpTs := localNtpTs.Add(10 * time.Minute)
	psd := newPeerSyncData(remoteDelta, remoteNtpTs, 0, 0)

	if newData, err := maybeUpdateFromPeerData(data, psd); err == nil {
		t.Errorf("expected no update, got %v", *newData)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Helper functions

func maybeUpdateFromPeerData(data *VClockData, psd *PeerSyncData) (*VClockData, error) {
	return MaybeUpdateFromPeerData(NewVClockForTests(nil), data, psd)
}

func toRemoteTime(t time.Time, remoteDelta time.Duration) time.Time {
	return t.Add(remoteDelta)
}

func toLocalTime(t time.Time, remoteDelta time.Duration) time.Time {
	return t.Add(-remoteDelta)
}

func newPeerSyncData(remoteDelta time.Duration, lastNtpTs time.Time, numReboots, numHops uint16) *PeerSyncData {
	oTs := time.Now()
	rTs := toRemoteTime(oTs.Add(time.Second), remoteDelta)
	sTs := rTs.Add(time.Millisecond)
	clientRecvTs := toLocalTime(sTs, remoteDelta).Add(time.Second)
	return &PeerSyncData{
		MySendTs:   oTs,
		RecvTs:     rTs,
		SendTs:     sTs,
		MyRecvTs:   clientRecvTs,
		LastNtpTs:  lastNtpTs,
		NumReboots: numReboots,
		NumHops:    numHops,
	}
}

func newSyncVClockData(skew time.Duration, lastNtpTs time.Time, numReboots, numHops uint16) *VClockData {
	return newVClockData(time.Time{}, skew, 0, lastNtpTs, numReboots, numHops)
}
