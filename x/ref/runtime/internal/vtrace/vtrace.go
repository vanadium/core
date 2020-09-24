// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vtrace implements the Trace and Span interfaces in v.io/v23/vtrace.
// We also provide internal utilities for migrating trace information across
// RPC calls.
package vtrace

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"

	"v.io/x/ref/lib/flags"
)

// A span represents an annotated period of time.
type span struct {
	id     uniqueid.Id
	parent uniqueid.Id
	name   string
	trace  uniqueid.Id
	start  time.Time
	store  *Store
}

func newSpan(parent uniqueid.Id, name string, trace uniqueid.Id, store *Store) (*span, error) {
	id, err := uniqueid.Random()
	if err != nil {
		return nil, fmt.Errorf("vtrace: couldn't generate Span ID, debug data may be lost: %v", err)
	}
	s := &span{
		id:     id,
		parent: parent,
		name:   name,
		trace:  trace,
		start:  time.Now(),
		store:  store,
	}
	store.start(s)
	return s, nil
}

func (s *span) ID() uniqueid.Id {
	// nologcall
	return s.id
}
func (s *span) Parent() uniqueid.Id {
	// nologcall
	return s.parent
}
func (s *span) Name() string {
	// nologcall
	return s.name
}
func (s *span) Trace() uniqueid.Id {
	// nologcall
	return s.trace
}
func (s *span) Annotate(msg string) {
	// nologcall
	s.store.annotate(s, msg)
}
func (s *span) Annotatef(format string, a ...interface{}) {
	// nologcall
	s.store.annotate(s, fmt.Sprintf(format, a...))
}
func (s *span) Finish() {
	// nologcall
	s.store.finish(s)
}
func (s *span) flags() vtrace.TraceFlags {
	return s.store.flags(s.trace)
}

type contextKey int

const (
	storeKey = contextKey(iota)
	spanKey
)

// Manager allows you to create new traces and spans and access the
// vtrace store.
type manager struct{}

// WithNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing.  For example
// this might be used to start background tasks.
func (m manager) WithNewTrace(ctx *context.T) (*context.T, vtrace.Span) {
	// nologcall
	id, err := uniqueid.Random()
	if err != nil {
		ctx.Errorf("vtrace: couldn't generate Trace Id, debug data may be lost: %v", err)
	}
	s, err := newSpan(id, "", id, getStore(ctx))
	if err != nil {
		ctx.Error(err)
	}

	return context.WithValue(ctx, spanKey, s), s
}

// WithContinuedTrace creates a span that represents a continuation of
// a trace from a remote server.  name is the name of the new span and
// req contains the parameters needed to connect this span with it's
// trace.
func (m manager) WithContinuedTrace(ctx *context.T, name string, req vtrace.Request) (*context.T, vtrace.Span) {
	// nologcall
	st := getStore(ctx)
	if req.Flags&vtrace.CollectInMemory != 0 {
		st.ForceCollect(req.TraceId, int(req.LogLevel))
	}
	newSpan, err := newSpan(req.SpanId, name, req.TraceId, st)
	if err != nil {
		ctx.Error(err)
	}
	return context.WithValue(ctx, spanKey, newSpan), newSpan
}

// WithNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func (m manager) WithNewSpan(ctx *context.T, name string) (*context.T, vtrace.Span) {
	// nologcall
	if curSpan := getSpan(ctx); curSpan != nil {
		if curSpan.store == nil {
			panic("nil store")
		}
		s, err := newSpan(curSpan.ID(), name, curSpan.trace, curSpan.store)
		if err != nil {
			ctx.Error(err)
		}
		return context.WithValue(ctx, spanKey, s), s
	}

	ctx.Error("vtrace: creating a new child span from context with no existing span.")
	return m.WithNewTrace(ctx)
}

// Span finds the currently active span.
func (m manager) GetSpan(ctx *context.T) vtrace.Span {
	// nologcall
	if span := getSpan(ctx); span != nil {
		return span
	}
	return nil
}

// Request generates a vtrace.Request from the active Span.
func (m manager) GetRequest(ctx *context.T) vtrace.Request {
	// nologcall
	if span := getSpan(ctx); span != nil {
		return vtrace.Request{
			SpanId:   span.id,
			TraceId:  span.trace,
			Flags:    span.flags(),
			LogLevel: int32(GetVTraceLevel(ctx)),
		}
	}
	return vtrace.Request{}
}

// Response captures the vtrace.Response for the active Span.
func (m manager) GetResponse(ctx *context.T) vtrace.Response {
	// nologcall
	if span := getSpan(ctx); span != nil {
		return vtrace.Response{
			Flags: span.flags(),
			Trace: *span.store.TraceRecord(span.trace),
		}
	}
	return vtrace.Response{}
}

// Store returns the current vtrace.Store.
func (m manager) GetStore(ctx *context.T) vtrace.Store {
	// nologcall
	if store := getStore(ctx); store != nil {
		return store
	}
	return nil
}

// getSpan returns the internal span type.
func getSpan(ctx *context.T) *span {
	span, _ := ctx.Value(spanKey).(*span)
	return span
}

// GetStore returns the *Store attached to the context.
func getStore(ctx *context.T) *Store {
	store, _ := ctx.Value(storeKey).(*Store)
	return store
}

// SetVTraceLevel returns the vtrace level This value is used
// to determine which log messages are sent as part of the vtrace output.
func GetVTraceLevel(ctx *context.T) int {
	store := getStore(ctx)
	if store == nil {
		return 0
	}
	span := getSpan(ctx)
	if span == nil {
		return 0
	}
	return store.logLevel(span.trace)
}

// Init initializes vtrace and attaches some state to the context.
// This should be called by the runtimes initialization function.
func Init(ctx *context.T, opts flags.VtraceFlags) (*context.T, error) {
	nctx := vtrace.WithManager(ctx, manager{})
	store, err := NewStore(opts)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(nctx, storeKey, store), nil
}
