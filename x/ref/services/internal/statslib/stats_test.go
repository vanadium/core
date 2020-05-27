// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package statslib_test

import (
	"reflect"
	"sort"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/services/stats"
	"v.io/v23/services/watch"
	"v.io/v23/vdl"
	"v.io/v23/vom"
	libstats "v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/histogram"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/statslib"
	s_stats "v.io/x/ref/services/stats"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type statsDispatcher struct {
}

func (d *statsDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return statslib.NewStatsService(suffix, 100*time.Millisecond), nil, nil
}

func TestStatsImpl(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", &statsDispatcher{})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
		return
	}
	endpoint := server.Status().Endpoints[0].String()

	counter := libstats.NewCounter("testing/foo/bar")
	counter.Incr(10)

	histogram := libstats.NewHistogram("testing/hist/foo", histogram.Options{
		NumBuckets:         5,
		GrowthFactor:       1,
		SmallestBucketSize: 1,
		MinValue:           0,
	})
	for i := 0; i < 10; i++ {
		histogram.Add(int64(i)) //nolint:errcheck
	}

	name := naming.JoinAddressName(endpoint, "")
	c := stats.StatsClient(name)

	// Test Glob()
	{
		results, _, err := testutil.GlobName(ctx, name, "testing/foo/...")
		if err != nil {
			t.Fatalf("testutil.GlobName failed: %v", err)
		}
		expected := []string{
			"testing/foo",
			"testing/foo/bar",
			"testing/foo/bar/delta10m",
			"testing/foo/bar/delta1h",
			"testing/foo/bar/delta1m",
			"testing/foo/bar/rate10m",
			"testing/foo/bar/rate1h",
			"testing/foo/bar/rate1m",
			"testing/foo/bar/timeseries10m",
			"testing/foo/bar/timeseries1h",
			"testing/foo/bar/timeseries1m",
		}
		sort.Strings(results)
		sort.Strings(expected)
		if !reflect.DeepEqual(results, expected) {
			t.Errorf("unexpected result. Got %v, want %v", results, expected)
		}
	}

	// Test WatchGlob()
	{
		var noRM watch.ResumeMarker
		ctx, cancel := context.WithCancel(ctx)
		stream, err := c.WatchGlob(ctx, watch.GlobRequest{Pattern: "testing/foo/bar"})
		if err != nil {
			t.Fatalf("c.WatchGlob failed: %v", err)
		}
		iterator := stream.RecvStream()
		if !iterator.Advance() {
			t.Fatalf("expected more stream values")
		}
		got := vdl.ValueOf(iterator.Value())
		expected := vdl.ValueOf(watch.Change{Name: "testing/foo/bar", Value: vom.RawBytesOf(int64(10)), ResumeMarker: noRM})
		if !vdl.EqualValue(got, expected) {
			t.Errorf("unexpected result. Got %#v, want %#v", got, expected)
		}

		counter.Incr(5)

		if !iterator.Advance() {
			t.Fatalf("expected more stream values")
		}
		got = vdl.ValueOf(iterator.Value())
		expected = vdl.ValueOf(watch.Change{Name: "testing/foo/bar", Value: vom.RawBytesOf(int64(15)), ResumeMarker: noRM})
		if !vdl.EqualValue(got, expected) {
			t.Errorf("unexpected result. Got %#v, want %#v", got, expected)
		}

		counter.Incr(2)

		if !iterator.Advance() {
			t.Fatalf("expected more stream values")
		}
		got = vdl.ValueOf(iterator.Value())
		expected = vdl.ValueOf(watch.Change{Name: "testing/foo/bar", Value: vom.RawBytesOf(int64(17)), ResumeMarker: noRM})
		if !vdl.EqualValue(got, expected) {
			t.Errorf("unexpected result. Got %#v, want %#v", got, expected)
		}
		cancel()

		if iterator.Advance() {
			t.Errorf("expected no more stream values, got: %v", iterator.Value())
		}
	}

	// Test Value()
	{
		c := stats.StatsClient(naming.JoinAddressName(endpoint, "testing/foo/bar"))
		value, err := c.Value(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		vv := vdl.ValueOf(value)
		if want := vdl.IntValue(vdl.Int64Type, 17); !vdl.EqualValue(vv, want) {
			t.Errorf("unexpected result. Got %v, want %v", vv, want)
		}
	}

	// Test Value() with Histogram
	{
		c := stats.StatsClient(naming.JoinAddressName(endpoint, "testing/hist/foo"))
		value, err := c.Value(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		vv := vdl.ValueOf(value)
		want := vdl.ValueOf(s_stats.HistogramValue{
			Count: 10,
			Sum:   45,
			Min:   0,
			Max:   9,
			Buckets: []s_stats.HistogramBucket{
				{LowBound: 0, Count: 1},
				{LowBound: 1, Count: 2},
				{LowBound: 3, Count: 4},
				{LowBound: 7, Count: 3},
				{LowBound: 15, Count: 0},
			},
		})
		if !vdl.EqualValue(vv, want) {
			t.Errorf("unexpected result. Got %v, want %v", vv, want)
		}
	}
}
