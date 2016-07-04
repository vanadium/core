// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"time"

	"v.io/x/lib/vlog"
)

// PeerSyncData contains data collected during a GetTime RPC to a peer.
type PeerSyncData struct {
	MySendTs time.Time // when we sent request (our vclock time)
	RecvTs   time.Time // when peer received request (their vclock time)
	SendTs   time.Time // when peer sent response (their vclock time)
	MyRecvTs time.Time // when we received response (our vclock time)
	// Select fields from peer's VClockData.
	LastNtpTs  time.Time
	NumReboots uint16
	NumHops    uint16
}

// NoUpdateErr is an error indicating that no update should be performed to
// VClockData. We define this error type explicitly so that the calling code
// (syncWithPeer) knows not to log the error with vlog.Errorf.
type NoUpdateErr struct {
	msg string
}

func (e *NoUpdateErr) Error() string {
	return e.msg
}

func nue(msg string) error {
	return &NoUpdateErr{msg: msg}
}

// MaybeUpdateFromPeerData updates data (the local VClockData) based on the
// given PeerSyncData. Returns nil if data has been updated; otherwise, the
// returned error specifies why the data was not updated.
// TODO(sadovsky): This design assumes trust across syncgroups, which is
// generally not desirable. Eventually we may need to perform
// MaybeUpdateFromPeerData separately for each db or syncgroup, or something
// along these lines.
func MaybeUpdateFromPeerData(c *VClock, data *VClockData, psd *PeerSyncData) (*VClockData, error) {
	// Same skew calculation as in NTP.
	skew := (psd.RecvTs.Sub(psd.MySendTs) + psd.SendTs.Sub(psd.MyRecvTs)) / 2
	vlog.VI(2).Infof("vclock: MaybeUpdateFromPeerData: skew: %v", skew)
	if abs(skew) < PeerSyncSkewThreshold {
		// Avoid constant tweaking of the clock.
		return nil, nue("abs(skew) is less than PeerSyncSkewThreshold")
	}
	if psd.LastNtpTs.IsZero() {
		return nil, nue("peer's clock has not synced to NTP")
	}
	if psd.LastNtpTs.Before(data.LastNtpTs) {
		return nil, nue("peer's NTP is less recent than local")
	}
	// TODO(sadovsky): Do we really need the abs(skew) > RebootSkewThreshold part?
	if psd.NumReboots > 0 && abs(skew) > RebootSkewThreshold {
		return nil, nue("peer's vclock exceeds reboot tolerance")
	}
	if psd.NumHops+1 > MaxNumHops {
		return nil, nue("peer's NTP is from too many hops away")
	}

	// All checks have passed. Update VClockData based on PeerSyncData.
	now, elapsedTime, err := c.SysClockVals()
	if err != nil {
		vlog.Errorf("vclock: MaybeUpdateFromPeerData: SysClockVals failed: %v", err)
		return nil, err
	}

	return &VClockData{
		SystemTimeAtBoot:     now.Add(-elapsedTime),
		Skew:                 data.Skew + skew,
		ElapsedTimeSinceBoot: elapsedTime,
		LastNtpTs:            psd.LastNtpTs,
		NumReboots:           psd.NumReboots,
		NumHops:              psd.NumHops + 1,
	}, nil
}
