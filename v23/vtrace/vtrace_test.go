// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
)

func spanNoPanic(span Span) {
	span.Name()
	span.ID()
	span.Parent()
	span.Annotate("")
	span.Annotatef("")
	span.Finish()
	span.Trace()
}

func storeNoPanic(store Store) {
	store.TraceRecords()
	store.TraceRecord(uniqueid.Id{})
	store.ForceCollect(uniqueid.Id{}, 0)
	store.Merge(Response{})
}

func TestNoPanic(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	initialctx := ctx

	ctx, span := WithNewTrace(ctx)
	spanNoPanic(span)

	ctx, span = WithContinuedTrace(ctx, "", Request{})
	spanNoPanic(span)
	spanNoPanic(GetSpan(ctx))
	GetRequest(ctx)
	GetResponse(ctx)

	storeNoPanic(GetStore(ctx))

	if ctx != initialctx {
		t.Errorf("context was unexpectedly changed.")
	}
}
