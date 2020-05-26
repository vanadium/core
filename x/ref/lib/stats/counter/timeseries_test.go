// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"reflect"
	"testing"
	"time"
)

func TestTimeSeries(t *testing.T) { //nolint:gocyclo
	now := time.Unix(1, 0)
	ts := newTimeSeries(now, 5*time.Second, time.Second)

	// Time 1
	ts.advanceTime(now)
	ts.set(123)
	if expected, got := int64(123), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	ts.set(234)
	if expected, got := int64(234), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(234), ts.min(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(234), ts.max(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}

	// Time 2
	now = now.Add(time.Second)
	ts.advanceTime(now)
	ts.set(345)
	if expected, got := int64(345), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(0), ts.tailValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(234), ts.min(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(345), ts.max(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}

	// Time 4
	now = now.Add(2 * time.Second)
	ts.advanceTime(now)
	ts.set(111)
	if expected, got := int64(111), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(0), ts.tailValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(111), ts.min(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(345), ts.max(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}

	// Time 7
	now = now.Add(3 * time.Second)
	ts.advanceTime(now)
	if expected, got := int64(111), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(345), ts.tailValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(111), ts.min(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(345), ts.max(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}

	// Time 8
	now = now.Add(time.Second)
	ts.advanceTime(now)
	if expected, got := int64(111), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(345), ts.tailValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(111), ts.min(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(111), ts.max(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}

	// Time 27
	now = now.Add(20 * time.Second)
	ts.advanceTime(now)
	if expected, got := int64(111), ts.headValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(111), ts.tailValue(); got != expected {
		t.Errorf("unexpected value. Got %v, want %v", got, expected)
	}
}

func TestTimeSeriesRate(t *testing.T) {
	now := time.Unix(1, 0)
	ts := newTimeSeries(now, 60*time.Second, time.Second)
	// Increment by 5 every 4 seconds. The rate is 1.25.
	for i := 0; i < 10; i++ {
		now = now.Add(4 * time.Second)
		ts.advanceTime(now)
		ts.incr(5)
	}
	if expected, got := float64(1.25), ts.rate(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}

}

func TestTimeSeriesValues(t *testing.T) {
	now := time.Unix(1, 0)
	// 6 time slots.
	ts := newTimeSeries(now, 5*time.Second, time.Second)
	// Add 3 values.
	// slots: [0, 1, 2, 3, x, x]
	// values: [0, 1, 2, 3]
	now = addValue(1, 3, ts, now)
	if expected, got := []int64{0, 1, 2, 3}, ts.values(); !reflect.DeepEqual(got, expected) {
		t.Errorf("unexpected values. Got %v, want %v", got, expected)
	}
	// Add 2 more values.
	// slots: [0, 1, 2, 3, 4, 5]
	// values: [0, 1, 2, 3, 4, 5]
	now = addValue(1, 2, ts, now)
	if expected, got := []int64{0, 1, 2, 3, 4, 5}, ts.values(); !reflect.DeepEqual(got, expected) {
		t.Errorf("unexpected values. Got %v, want %v", got, expected)
	}
	// Add 3 more values.
	// slots: [6, 7, 8, 3, 4, 5]
	// values: [3, 4, 5, 6, 7, 8]
	addValue(1, 3, ts, now)
	if expected, got := []int64{3, 4, 5, 6, 7, 8}, ts.values(); !reflect.DeepEqual(got, expected) {
		t.Errorf("unexpected values. Got %v, want %v", got, expected)
	}
}

func addValue(increment int64, count int, ts *timeseries, now time.Time) time.Time {
	for i := 0; i < count; i++ {
		now = now.Add(time.Second)
		ts.advanceTime(now)
		ts.incr(increment)
	}
	return now
}
