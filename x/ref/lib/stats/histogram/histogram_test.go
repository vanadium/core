// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package histogram_test

import (
	"testing"

	"v.io/x/ref/lib/stats/histogram"
)

func TestHistogram(t *testing.T) {
	// This creates a histogram with the following buckets:
	//  [1, 2)
	//  [2, 4)
	//  [4, 8)
	//  [8, 16)
	//  [16, Inf)
	opts := histogram.Options{
		NumBuckets:         5,
		GrowthFactor:       1.0,
		SmallestBucketSize: 1.0,
		MinValue:           1.0,
	}
	h := histogram.New(opts)
	// Trying to add a value that's less than MinValue, should return an error.
	if err := h.Add(0); err == nil {
		t.Errorf("unexpected return value for Add(0.0). Want != nil, got nil")
	}
	// Adding good values. Expect no errors.
	for i := 1; i <= 50; i++ {
		if err := h.Add(int64(i)); err != nil {
			t.Errorf("unexpected return value for Add(%d). Want nil, got %v", i, err)
		}
	}
	expectedCount := []int64{1, 2, 4, 8, 35}
	buckets := h.Value().Buckets
	for i := 0; i < opts.NumBuckets; i++ {
		if buckets[i].Count != expectedCount[i] {
			t.Errorf("unexpected count for bucket[%d]. Want %d, got %v", i, expectedCount[i], buckets[i].Count)
		}
	}

	v := h.Value()
	if expected, got := int64(50), v.Count; got != expected {
		t.Errorf("unexpected count in histogram value. Want %d, got %v", expected, got)
	}
	if expected, got := int64(50*(1+50)/2), v.Sum; got != expected {
		t.Errorf("unexpected sum in histogram value. Want %d, got %v", expected, got)
	}
	if expected, got := int64(1), v.Min; got != expected {
		t.Errorf("unexpected min in histogram value. Want %d, got %v", expected, got)
	}
	if expected, got := int64(50), v.Max; got != expected {
		t.Errorf("unexpected max in histogram value. Want %d, got %v", expected, got)
	}
}

func BenchmarkHistogram(b *testing.B) {
	opts := histogram.Options{
		NumBuckets:         30,
		GrowthFactor:       1.0,
		SmallestBucketSize: 1.0,
		MinValue:           1.0,
	}
	h := histogram.New(opts)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Add(int64(i)) //nolint:errcheck
	}
}
