// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter_test

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"v.io/x/ref/lib/stats/counter"
)

var trackerTests = []struct {
	after      time.Duration
	push       int64   // 0 = none, -1 = reset
	mins, maxs []int64 // min, 10min, hour, all time
}{
	{ // T0
		after: 0,
		push:  0,
		mins:  []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64},
		maxs:  []int64{math.MinInt64, math.MinInt64, math.MinInt64, math.MinInt64},
	},
	{ // T1
		after: 1 * time.Second,
		push:  5,
		mins:  []int64{5, 5, 5, 5},
		maxs:  []int64{5, 5, 5, 5},
	},
	{ // T2
		after: 1 * time.Second,
		push:  10,
		mins:  []int64{5, 5, 5, 5},
		maxs:  []int64{10, 10, 10, 10},
	},
	{ // T3
		after: 1 * time.Second,
		push:  1,
		mins:  []int64{1, 1, 1, 1},
		maxs:  []int64{10, 10, 10, 10},
	},
	{ // T4
		after: 1 * time.Minute,
		push:  0,
		mins:  []int64{math.MaxInt64, 1, 1, 1},
		maxs:  []int64{math.MinInt64, 10, 10, 10},
	},
	{ // T5
		after: 10 * time.Minute,
		push:  0,
		mins:  []int64{math.MaxInt64, math.MaxInt64, 1, 1},
		maxs:  []int64{math.MinInt64, math.MinInt64, 10, 10},
	},
	{ // T6
		after: 1 * time.Hour,
		push:  0,
		mins:  []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64, 1},
		maxs:  []int64{math.MinInt64, math.MinInt64, math.MinInt64, 10},
	},
	{ // T7
		after: 1 * time.Second,
		push:  5,
		mins:  []int64{5, 5, 5, 1},
		maxs:  []int64{5, 5, 5, 10},
	},
	{ // T8
		after: 1 * time.Minute,
		push:  20,
		mins:  []int64{20, 5, 5, 1},
		maxs:  []int64{20, 20, 20, 20},
	},
	{ // T9
		after: 10 * time.Minute,
		push:  15,
		mins:  []int64{15, 15, 5, 1},
		maxs:  []int64{15, 15, 20, 20},
	},
	{ // T10
		after: 1 * time.Hour,
		push:  10,
		mins:  []int64{10, 10, 10, 1},
		maxs:  []int64{10, 10, 10, 20},
	},
	{ // T11
		after: 1 * time.Second,
		push:  -1,
		mins:  []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64},
		maxs:  []int64{math.MinInt64, math.MinInt64, math.MinInt64, math.MinInt64},
	},
	{ // T12
		after: 1 * time.Second,
		push:  5,
		mins:  []int64{5, 5, 5, 5},
		maxs:  []int64{5, 5, 5, 5},
	},
}

func TestTracker(t *testing.T) {
	now := time.Unix(1, 0)
	counter.TimeNow = func() time.Time { return now }

	tracker := counter.NewTracker()
	for i, tt := range trackerTests {
		now = now.Add(tt.after)
		name := fmt.Sprintf("[T%d] %s:", i, now.Format("15:04:05"))
		switch {
		case tt.push > 0:
			tracker.Push(tt.push)
			t.Logf("%s pushed %d\n", name, tt.push)
		case tt.push < 0:
			tracker.Reset()
			t.Log(name, "reset")
		default:
			t.Log(name, "none")
		}

		if expected, got := tt.mins[0], tracker.Min1m(); got != expected {
			t.Errorf("%s Min1m returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.maxs[0], tracker.Max1m(); got != expected {
			t.Errorf("%s Max1m returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.mins[1], tracker.Min10m(); got != expected {
			t.Errorf("%s Min10m returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.maxs[1], tracker.Max10m(); got != expected {
			t.Errorf("%s Max10m returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.mins[2], tracker.Min1h(); got != expected {
			t.Errorf("%s Min1h returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.maxs[2], tracker.Max1h(); got != expected {
			t.Errorf("%s Max1h returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.mins[3], tracker.Min(); got != expected {
			t.Errorf("%s Min returned %d, want %v", name, got, expected)
		}
		if expected, got := tt.maxs[3], tracker.Max(); got != expected {
			t.Errorf("%s Max returned %d, want %v", name, got, expected)
		}
	}
}

func min(a, b int64) int64 {
	if a > b {
		return b
	}
	return a
}

func max(a, b int64) int64 {
	if a < b {
		return b
	}
	return a
}

func TestTrackerConcurrent(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	const numGoRoutines = 100
	const numPushPerGoRoutine = 100000
	tracker := counter.NewTracker()

	var mins, maxs [numGoRoutines]int64
	var wg sync.WaitGroup
	wg.Add(numGoRoutines)
	for i := 0; i < numGoRoutines; i++ {
		go func(i int) {
			var localMin, localMax int64 = math.MaxInt64, math.MinInt64
			for j := 0; j < numPushPerGoRoutine; j++ {
				v := rand.Int63()
				tracker.Push(v)
				localMin, localMax = min(localMin, v), max(localMax, v)
			}

			mins[i], maxs[i] = localMin, localMax
			wg.Done()
		}(i)
	}
	wg.Wait()

	var expectedMin, expectedMax int64 = math.MaxInt64, math.MinInt64
	for _, v := range mins {
		expectedMin = min(expectedMin, v)
	}
	for _, v := range maxs {
		expectedMax = max(expectedMax, v)
	}

	if got := tracker.Min(); got != expectedMin {
		t.Errorf("Min returned %d, want %v", got, expectedMin)
	}
	if got := tracker.Max(); got != expectedMax {
		t.Errorf("Max returned %d, want %v", got, expectedMax)
	}
}

func BenchmarkTrackerPush(b *testing.B) {
	const numVals = 10000
	vals := rand.Perm(numVals)
	tracker := counter.NewTracker()

	b.SetParallelism(100)
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			tracker.Push(int64(vals[i%numVals]))
		}
	})
}
