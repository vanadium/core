// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace

import (
	"encoding/binary"
	"reflect"
	"sort"
	"testing"

	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"

	"v.io/x/ref/lib/flags"
)

var nextid = uint64(1)

func id() uniqueid.Id {
	var out uniqueid.Id
	binary.BigEndian.PutUint64(out[8:], nextid)
	nextid++
	return out
}

func makeTraces(n int, st *Store) []uniqueid.Id {
	traces := make([]uniqueid.Id, n)
	for i := range traces {
		curid := id()
		traces[i] = curid
		st.ForceCollect(curid, 0)
	}
	return traces
}

func recordids(records ...vtrace.TraceRecord) map[uniqueid.Id]bool {
	out := make(map[uniqueid.Id]bool)
	for _, trace := range records {
		out[trace.Id] = true
	}
	return out
}

func traceids(traces ...uniqueid.Id) map[uniqueid.Id]bool {
	out := make(map[uniqueid.Id]bool)
	for _, trace := range traces {
		out[trace] = true
	}
	return out
}

func pretty(in map[uniqueid.Id]bool) []int {
	out := make([]int, 0, len(in))
	for k := range in {
		out = append(out, int(k[15]))
	}
	sort.Ints(out)
	return out
}

func compare(t *testing.T, want map[uniqueid.Id]bool, records []vtrace.TraceRecord) {
	got := recordids(records...)
	if !reflect.DeepEqual(want, got) {
		t.Errorf("Got wrong traces.  Got %v, want %v.", pretty(got), pretty(want))
	}
}

func TestTrimming(t *testing.T) {
	st, err := NewStore(flags.VtraceFlags{CacheSize: 5})
	if err != nil {
		t.Fatalf("Could not create store: %v", err)
	}
	traces := makeTraces(10, st)

	compare(t, traceids(traces[5:]...), st.TraceRecords())

	traces = append(traces, id(), id(), id())

	// Starting a span on an existing trace brings it to the front of the queue
	// and prevent it from being removed when a new trace begins.
	st.start(&span{trace: traces[5], id: id()})
	st.ForceCollect(traces[10], 0)
	compare(t, traceids(traces[10], traces[5], traces[7], traces[8], traces[9]), st.TraceRecords())

	// Finishing a span on one of the traces should bring it back into the stored set.
	st.finish(&span{trace: traces[7], id: id()})
	st.ForceCollect(traces[11], 0)
	compare(t, traceids(traces[10], traces[11], traces[5], traces[7], traces[9]), st.TraceRecords())

	// Annotating a span on one of the traces should bring it back into the stored set.
	st.annotate(&span{trace: traces[9], id: id()}, "hello")
	st.ForceCollect(traces[12], 0)
	compare(t, traceids(traces[10], traces[11], traces[12], traces[7], traces[9]), st.TraceRecords())
}

func TestRegexp(t *testing.T) {
	traces := []uniqueid.Id{id(), id(), id()}

	type testcase struct {
		pattern string
		results []uniqueid.Id
	}
	tests := []testcase{
		{".*", traces},
		{"foo.*", traces},
		{".*bar", traces[1:2]},
		{".*bang", traces[2:3]},
	}

	for _, test := range tests {
		st, err := NewStore(flags.VtraceFlags{
			CacheSize:     10,
			CollectRegexp: test.pattern,
		})
		if err != nil {
			t.Fatalf("Could not create store: %v", err)
		}

		_, err = newSpan(traces[0], "foo", traces[0], st)
		if err != nil {
			t.Fatal(err)
		}
		_, err = newSpan(traces[1], "foobar", traces[1], st)
		if err != nil {
			t.Fatal(err)
		}
		sp, err := newSpan(traces[2], "baz", traces[2], st)
		if err != nil {
			t.Fatal(err)
		}
		sp.Annotate("foobang")

		compare(t, traceids(test.results...), st.TraceRecords())
	}
}
