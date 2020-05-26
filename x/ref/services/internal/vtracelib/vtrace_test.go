// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtracelib_test

import (
	"io"
	"testing"

	v23 "v.io/v23"
	s_vtrace "v.io/v23/services/vtrace"
	"v.io/v23/vtrace"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/internal/vtracelib"
	"v.io/x/ref/test"
)

func TestVtraceServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	_, server, err := v23.WithNewServer(ctx, "", vtracelib.NewVtraceService(), nil)
	if err != nil {
		t.Fatalf("Could not create server: %s", err)
	}
	endpoints := server.Status().Endpoints

	sctx, span := vtrace.WithNewSpan(ctx, "The Span")
	vtrace.ForceCollect(sctx, 0)
	span.Finish()
	id := span.Trace()

	client := s_vtrace.StoreClient(endpoints[0].Name())

	sctx, _ = vtrace.WithNewTrace(sctx)
	trace, err := client.Trace(sctx, id)
	if err != nil {
		t.Fatalf("Unexpected error getting trace: %s", err)
	}
	if len(trace.Spans) != 1 {
		t.Errorf("Returned trace should have 1 span, found %#v", trace)
	}
	if trace.Spans[0].Name != "The Span" {
		t.Errorf("Returned span has wrong name: %#v", trace)
	}

	sctx, _ = vtrace.WithNewTrace(sctx)
	call, err := client.AllTraces(sctx)
	if err != nil {
		t.Fatalf("Unexpected error getting traces: %s", err)
	}
	ntraces := 0
	stream := call.RecvStream()
	var tr *vtrace.TraceRecord
	for stream.Advance() {
		trace := stream.Value()
		if trace.Id == id {
			tr = &trace
		}
		ntraces++
	}
	if err = stream.Err(); err != nil && err != io.EOF {
		t.Fatalf("Unexpected error reading trace stream: %s", err)
	}
	if ntraces != 1 {
		t.Fatalf("Expected 1 trace, got %#v", ntraces)
	}
	if tr == nil {
		t.Fatalf("Desired trace %x not found.", id)
	}
	if len(tr.Spans) != 1 {
		t.Errorf("Returned trace should have 1 span, found %#v", tr)
	}
	if tr.Spans[0].Name != "The Span" {
		t.Fatalf("Returned span has wrong name: %#v", tr)
	}
}
