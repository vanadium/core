// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"testing"
	"time"

	"v.io/v23/services/application"
)

// TestRestartPolicy verifies that the daemon mode restart policy operates
// as intended.
func TestRestartPolicy(t *testing.T) {
	nbr := newBasicRestartPolicy()

	type tV struct {
		envelope *application.Envelope
		info     *instanceInfo
		wantInfo *instanceInfo
		decision bool
	}

	testNow := time.Now()

	testVectors := []tV{
		// -1 means always restart.
		{
			&application.Envelope{
				Restarts: -1,
			},
			&instanceInfo{
				Restarts: 0,
			},
			&instanceInfo{
				Restarts: 0,
			},
			true,
		},
		// 0 means restart exactly 0 times.
		{
			&application.Envelope{
				Restarts: 0,
			},
			&instanceInfo{
				Restarts: 0,
			},
			&instanceInfo{
				Restarts: 0,
			},
			false,
		},
		// 1 means restart once (2 invocations total)
		{
			&application.Envelope{
				Restarts:          1,
				RestartTimeWindow: time.Hour,
			},
			&instanceInfo{
				Restarts: 0,
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: time.Now(),
			},
			true,
		},
		// but only ever once.
		{
			&application.Envelope{
				Restarts:          1,
				RestartTimeWindow: time.Hour,
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: testNow,
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: testNow,
			},
			false,
		},
		// after time window, restart count is reset.
		{
			&application.Envelope{
				Restarts:          1,
				RestartTimeWindow: time.Minute,
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: time.Now().Add(-time.Hour),
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: time.Now(),
			},
			true,
		},
		// Every time a restart happens, the beginning of the window
		// should be reset.
		{
			&application.Envelope{
				Restarts:          2,
				RestartTimeWindow: time.Minute,
			},
			&instanceInfo{
				Restarts:           1,
				RestartWindowBegan: time.Now().Add(-10 * time.Second),
			},
			&instanceInfo{
				Restarts:           2,
				RestartWindowBegan: time.Now(),
			},
			true,
		},
	}

	for ti, tv := range testVectors {
		if got, want := nbr.decide(tv.envelope, tv.info), tv.decision; got != want {
			t.Errorf("Test case #%d: basicDecisionPolicy decide: got %v, want %v", ti, got, want)
		}

		if got, want := tv.info.Restarts, tv.wantInfo.Restarts; got != want {
			t.Errorf("basicDecisionPolicy instanceInfo Restarts update got %v, want %v", got, want)
		}

		// Times should be "nearly" same.
		if got, want := tv.info.RestartWindowBegan, tv.wantInfo.RestartWindowBegan; !((got.Sub(want) < time.Second) && (got.Sub(want) >= 0)) {
			t.Errorf("Test case #%d: basicDecisionPolicy instanceInfo RestartTimeBegan got %v, want %v", ti, got, want)
		}
	}
}
