// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vclock implements the Syncbase virtual clock, or vclock for short.
// The Syncbase vclock can be thought of as a better version of the system
// clock, in that it occasionally consults NTP as well as peers to attempt to
// improve the accuracy of time estimates coming from the system clock.
//
// The vclock implementation consists of several components:
// - SystemClock: a simple API for accessing system clock data, namely the
//   current time and the time elapsed since boot.
// - VClock struct: encapsulates persisted VClockData and provides methods for
//   obtaining Syncbase's notion of current time.
// - VClockD: daemon (goroutine) that periodically consults NTP as well as the
//   current system clock to update the persisted VClockData.
// - Peer clock sync: the sync subsystem obtains VClockData from peers as part
//   of the sync procedure; if this data is deemed more accurate than our local
//   VClockData, we update our local VClockData accordingly.
//
// For more information, see the design doc:
// TODO(jlodhia): Add link to design doc on v.io.
package vclock
