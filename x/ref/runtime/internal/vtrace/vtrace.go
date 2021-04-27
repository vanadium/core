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
	trace, id, parent uniqueid.Id
	metadata          []byte
	name              string
	start             time.Time
	store             vtrace.Store
}

func newSpan(parent uniqueid.Id, name string, trace uniqueid.Id, store vtrace.Store) (*span, error) {
	id, err := uniqueid.Random()
	if err != nil {
		return nil, fmt.Errorf("vtrace: couldn't generate Span ID, debug data may be lost: %v", err)
	}
	s := &span{
		id:     id,
		parent: parent,
		name:   name,
		start:  time.Now(),
		trace:  trace,
		store:  store,
	}
	store.Start(trace, vtrace.SpanRecord{
		Id:     s.id,
		Parent: s.parent,
		Name:   s.name,
		Start:  s.start,
	})
	return s, nil
}

func (s *span) ID() uniqueid.Id {
	return s.id
}

func (s *span) Parent() uniqueid.Id {
	return s.parent
}

func (s *span) Name() string {
	return s.name
}

func (s *span) Trace() uniqueid.Id {
	return s.trace
}

func (s *span) annotate(msg string) {
	s.store.Annotate(s.trace,
		vtrace.SpanRecord{
			Id:     s.id,
			Parent: s.parent,
			Name:   s.name,
			Start:  s.start,
		},
		vtrace.Annotation{
			When:    time.Now(),
			Message: msg,
		})
}

func (s *span) Annotate(msg string) {
	s.annotate(msg)
}

func (s *span) Annotatef(format string, a ...interface{}) {
	s.annotate(fmt.Sprintf(format, a...))
}

func (s *span) SetMetadata(metadata []byte) {
	s.metadata = make([]byte, len(metadata))
	copy(s.metadata, metadata)
}

func (s *span) Finish() {
	s.store.Finish(s.trace,
		vtrace.SpanRecord{
			Id:     s.id,
			Parent: s.parent,
			Name:   s.name,
			Start:  s.start,
		}, time.Now())
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
	if span := getSpan(ctx); span != nil {
		return span
	}
	return nil
}

// Request generates a vtrace.Request from the active Span.
func (m manager) GetRequest(ctx *context.T) vtrace.Request {
	if span := getSpan(ctx); span != nil {
		return vtrace.Request{
			SpanId:   span.id,
			TraceId:  span.trace,
			Metadata: span.metadata,
			Flags:    span.store.Flags(span.trace),
			LogLevel: int32(GetVTraceLevel(ctx)),
		}
	}
	return vtrace.Request{}
}

// Response captures the vtrace.Response for the active Span.
func (m manager) GetResponse(ctx *context.T) vtrace.Response {
	if span := getSpan(ctx); span != nil {
		return vtrace.Response{
			Flags: span.store.Flags(span.trace),
			Trace: *span.store.TraceRecord(span.trace),
		}
	}
	return vtrace.Response{}
}

// Store returns the current vtrace.Store.
func (m manager) GetStore(ctx *context.T) vtrace.Store {
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
func getStore(ctx *context.T) vtrace.Store {
	store, _ := ctx.Value(storeKey).(vtrace.Store)
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
	return store.LogLevel(span.trace)
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
