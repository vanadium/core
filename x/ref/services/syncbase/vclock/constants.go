// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

import (
	"time"
)

const (
	// Used by VClockD.doNtpUpdate.
	NtpSampleCount        = 15
	NtpSkewDeltaThreshold = 2 * time.Second
	// Used by VClockD.doLocalUpdate.
	SystemClockDriftThreshold = time.Second
	// Used by MaybeUpdateFromPeerData.
	PeerSyncSkewThreshold = NtpSkewDeltaThreshold
	RebootSkewThreshold   = time.Minute
	MaxNumHops            = 2
)
