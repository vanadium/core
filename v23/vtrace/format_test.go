// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"
)

var nextid = uint64(1)

func id() uniqueid.Id {
	var out uniqueid.Id
	binary.BigEndian.PutUint64(out[8:], nextid)
	nextid++
	return out
}

func TestFormat(t *testing.T) {
	trid := id()
	trstart := time.Date(2014, 11, 6, 13, 1, 22, 400000000, time.UTC)
	spanIDs := make([]uniqueid.Id, 6)
	for i := range spanIDs {
		spanIDs[i] = id()
	}
	tr := vtrace.TraceRecord{
		Id: trid,
		Spans: []vtrace.SpanRecord{
			{
				Id:     spanIDs[0],
				Parent: trid,
				Name:   "",
				Start:  trstart,
			},
			{
				Id:     spanIDs[1],
				Parent: spanIDs[0],
				Name:   "Child1",
				Start:  trstart.Add(time.Second),
				End:    trstart.Add(10 * time.Second),
			},
			{
				Id:     spanIDs[2],
				Parent: spanIDs[0],
				Name:   "Child2",
				Start:  trstart.Add(20 * time.Second),
				End:    trstart.Add(30 * time.Second),
			},
			{
				Id:              spanIDs[3],
				Parent:          spanIDs[0],
				Name:            "Child2WithMetadata",
				Start:           trstart.Add(20 * time.Second),
				End:             trstart.Add(30 * time.Second),
				RequestMetadata: []byte{0x01, 0x02},
			},
			{
				Id:              spanIDs[4],
				Parent:          spanIDs[0],
				Name:            "Child2WithLongMetadata",
				Start:           trstart.Add(20 * time.Second),
				End:             trstart.Add(30 * time.Second),
				RequestMetadata: bytes.Repeat([]byte{0x01, 0x02}, 20),
				Annotations: []vtrace.Annotation{
					{
						Message: "First Annotation",
						When:    trstart.Add(4 * time.Second),
					},
				},
			},
			{
				Id:     spanIDs[5],
				Parent: spanIDs[1],
				Name:   "GrandChild1",
				Start:  trstart.Add(3 * time.Second),
				End:    trstart.Add(8 * time.Second),
				Annotations: []vtrace.Annotation{
					{
						Message: "First Annotation",
						When:    trstart.Add(4 * time.Second),
					},
					{
						Message: "Second Annotation",
						When:    trstart.Add(6 * time.Second),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	vtrace.FormatTrace(&buf, &tr, time.UTC)
	want := `Trace - 0x00000000000000000000000000000001 (2014-11-06 13:01:22.400000 UTC, ??)
    Span - Child1 [id: 00000003 parent 00000002] (1s, 10s: 9s)
        Span - GrandChild1 [id: 00000007 parent 00000003] (3s, 8s: 5s)
            @4s First Annotation
            @6s Second Annotation
    Span - Child2 [id: 00000004 parent 00000002] (20s, 30s: 10s)
    Span - Child2WithMetadata [id: 00000005 parent 00000002] (20s, 30s: 10s)
        [0x0102]
    Span - Child2WithLongMetadata [id: 00000006 parent 00000002] (20s, 30s: 10s)
        @4s First Annotation
        [0x01020102010201020102]
`
	if got := buf.String(); got != want {
		t.Errorf("Incorrect output, want\n%sgot\n%s", want, got)
	}
}

func TestFormatWithMissingSpans(t *testing.T) {
	trid := id()
	trstart := time.Date(2014, 11, 6, 13, 1, 22, 400000000, time.UTC)
	spanIDs := make([]uniqueid.Id, 6)
	for i := range spanIDs {
		spanIDs[i] = id()
	}
	tr := vtrace.TraceRecord{
		Id: trid,
		Spans: []vtrace.SpanRecord{
			{
				Id:     spanIDs[0],
				Parent: trid,
				Name:   "",
				Start:  trstart,
			},
			{
				Id:     spanIDs[1],
				Parent: spanIDs[0],
				Name:   "Child1",
				Start:  trstart.Add(time.Second),
				End:    trstart.Add(10 * time.Second),
			},
			{
				Id:     spanIDs[3],
				Parent: spanIDs[2],
				Name:   "Decendant2",
				Start:  trstart.Add(15 * time.Second),
				End:    trstart.Add(24 * time.Second),
			},
			{
				Id:     spanIDs[4],
				Parent: spanIDs[2],
				Name:   "Decendant1",
				Start:  trstart.Add(12 * time.Second),
				End:    trstart.Add(18 * time.Second),
			},
			{
				Id:     spanIDs[5],
				Parent: spanIDs[1],
				Name:   "GrandChild1",
				Start:  trstart.Add(3 * time.Second),
				End:    trstart.Add(8 * time.Second),
				Annotations: []vtrace.Annotation{
					{
						Message: "Second Annotation",
						When:    trstart.Add(6 * time.Second),
					},
					{
						Message: "First Annotation",
						When:    trstart.Add(4 * time.Second),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	vtrace.FormatTrace(&buf, &tr, time.UTC)
	want := `Trace - 0x00000000000000000000000000000008 (2014-11-06 13:01:22.400000 UTC, ??)
    Span - Child1 [id: 0000000a parent 00000009] (1s, 10s: 9s)
        Span - GrandChild1 [id: 0000000e parent 0000000a] (3s, 8s: 5s)
            @4s First Annotation
            @6s Second Annotation
    Span - Missing Data [id: 00000000 parent 00000000] (??, ??: ??)
        Span - Decendant1 [id: 0000000d parent 0000000b] (12s, 18s: 6s)
        Span - Decendant2 [id: 0000000c parent 0000000b] (15s, 24s: 9s)
`

	if got := buf.String(); got != want {
		t.Errorf("Incorrect output, want\n%sgot\n%s", want, got)
	}
}
