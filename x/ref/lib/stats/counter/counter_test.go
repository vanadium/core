// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter_test

import (
	"sync"
	"testing"
	"time"

	"v.io/x/ref/lib/stats/counter"
)

func TestCounter(t *testing.T) {
	now := time.Unix(1, 0)
	counter.TimeNow = func() time.Time { return now }
	c := counter.New()

	// Time 1, value=1
	c.Incr(1)

	// Time 2, value=2
	now = now.Add(time.Second)
	c.Incr(1)

	if expected, got := int64(2), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	// One second later.
	now = now.Add(time.Second)
	if expected, got := int64(2), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	now = time.Unix(1, 0)
	c.Reset()

	// Time 1, value=1
	c.Incr(1)
	// Time 2, value=2
	now = now.Add(time.Second)
	c.Incr(1)
	// Time 3, value=3
	now = now.Add(time.Second)
	c.Incr(1)
	// Time 4, value=4
	now = now.Add(time.Second)
	c.Incr(1)
	// Time 5, value=5
	now = now.Add(time.Second)
	c.Incr(1)
	// Expect current value=5 and delta1m=(5-0)=5
	if expected, got := int64(5), c.Value(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(5), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	now = time.Unix(0, 0)
	c.Reset()

	// ...
	// Time 64, value=64
	// Time 65, value=65
	// Time 66, value=66
	// Time 67, value=67
	// Time 68, value=68
	// Time 69, value=69
	// Time 70, value=70
	// Expect current value=70 and delta1m=(70-10)=60
	for sec := 1; sec <= 70; sec++ {
		now = now.Add(time.Second)
		c.Incr(1)
	}
	if expected, got := int64(70), c.Value(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(60), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}

	// Time is now 80, value is still 70, delta1m is (70-20)=50
	now = time.Unix(80, 0)
	if expected, got := int64(70), c.Value(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(50), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	c.Reset()

	// Increment by 1 every second for 90 minutes.
	now = time.Unix(59, 0)
	for i := 0; i < 60*90; i++ {
		now = now.Add(time.Second)
		c.Incr(1)
	}
	if expected, got := int64(5400), c.Value(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(60), c.Delta1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(600), c.Delta10m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	if expected, got := int64(3600), c.Delta1h(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
}

func TestCounterRate(t *testing.T) {
	now := time.Unix(1, 0)
	counter.TimeNow = func() time.Time { return now }
	c := counter.New()

	// No data, rate is 0.
	if expected, got := float64(0), c.Rate1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}

	// Increment by 1 every second. The rate is 1, even though the counter
	// doesn't have data for the whole period.
	for i := 0; i < 10; i++ {
		now = now.Add(time.Second)
		c.Incr(1)
	}
	if expected, got := float64(1), c.Rate1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	c.Reset()

	// Increment by 3 every 2 seconds. The rate is 1.5.
	for i := 0; i < 10; i++ {
		now = now.Add(2 * time.Second)
		c.Incr(3)
	}
	if expected, got := float64(1.5), c.Rate1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	c.Reset()

	// Increment by 5 every 4 seconds. The rate is 1.25.
	for i := 0; i < 100; i++ {
		now = now.Add(4 * time.Second)
		c.Incr(5)
	}
	if expected, got := float64(1.25), c.Rate1m(); got != expected {
		t.Errorf("Unexpected value. Got %v, want %v", got, expected)
	}
	c.Reset()
}

func TestConcurrent(t *testing.T) {
	const numGoRoutines = 100
	const numIncrPerGoRoutine = 100000
	c := counter.New()
	var wg sync.WaitGroup
	wg.Add(numGoRoutines)
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			for x := 0; x < numIncrPerGoRoutine; x++ {
				c.Incr(1)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if expected, got := int64(numGoRoutines*numIncrPerGoRoutine), c.Value(); got != expected {
		t.Errorf("unexpected result. Got %v, want %v", got, expected)
	}
}

func BenchmarkCounterIncr(b *testing.B) {
	c := counter.New()
	b.SetParallelism(100)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Incr(1)
		}
	})
}
