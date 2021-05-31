// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vtrace defines a system for collecting debugging
// information about operations that span a distributed system.  We
// call the debugging information attached to one operation a Trace.
// A Trace may span many processes on many machines.
//
// Traces are composed of a hierarchy of Spans.  A span is a named
// timespan, that is, it has a name, a start time, and an end time.
// For example, imagine we are making a new blog post.  We may have to
// first authentiate with an auth server, then write the new post to a
// database, and finally notify subscribers of the new content.  The
// trace might look like this:
//
//    Trace:
//    <---------------- Make a new blog post ----------->
//    |                  |                   |
//    <- Authenticate -> |                   |
//                       |                   |
//                       <-- Write to DB --> |
//                                           <- Notify ->
//    0s                      1.5s                      3s
//
// Here we have a single trace with four Spans.  Note that some Spans
// are children of other Spans.  Vtrace works by attaching data to a
// Context, and this hierarchical structure falls directly out of our
// building off of the tree of Contexts.  When you derive a new
// context using WithNewSpan(), you create a Span thats a child of the
// currently active span in the context.  Note that spans that share a
// parent may overlap in time.
//
// In this case the tree would have been created with code like this:
//
//    function MakeBlogPost(ctx *context.T) {
//        authCtx, _ := vtrace.WithNewSpan(ctx, "Authenticate")
//        Authenticate(authCtx)
//        writeCtx, _ := vtrace.WithNewSpan(ctx, "Write To DB")
//        Write(writeCtx)
//        notifyCtx, _ := vtrace.WithNewSpan(ctx, "Notify")
//        Notify(notifyCtx)
//    }
//
// Just as we have Spans to represent time spans we have Annotations
// to attach debugging information that is relevant to the current
// moment. You can add an annotation to the current span by calling
// the Span's Annotate method:
//
//    span := vtrace.GetSpan(ctx)
//    span.Annotate("Just got an error")
//
// When you make an annotation we record the annotation and the time
// when it was attached.
//
// Traces can be composed of large numbers of spans containing data
// collected from large numbers of different processes.  Always
// collecting this information would have a negative impact on
// performance.  By default we don't collect any data.  If a
// particular operation is of special importance you can force it to
// be collected by calling ForceCollect.  You can also use the
// --v23.vtrace.collect-regexp flag to set a regular expression which
// will force us to record any matching trace.
//
// If your trace has collected information you can retrieve the data
// collected so far with the Store's TraceRecord and TraceRecords methods.
//
// By default contexts obtained from v23.Init or in rpc server implementations
// already have an initialized Trace.  The functions in this package allow you
// to add data to existing traces or start new ones.
package vtrace

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
)

// Spans represent a named time period.  You can create new spans
// to represent new parts of your computation.
// Spans are safe to use from multiple goroutines simultaneously.
type Span interface {
	// Name returns the name of the span.
	Name() string

	// ID returns the uniqueid.ID of the span.
	ID() uniqueid.Id

	// Parent returns the uniqueid.ID of this spans parent span.
	Parent() uniqueid.Id

	// Annotate adds a string annotation to the trace.  Where Spans
	// represent time periods Annotations represent data thats relevant
	// at a specific moment.
	Annotate(s string)

	// Annotatef adds an annotation to the trace.  Where Spans represent
	// time periods Annotations represent data thats relevant at a
	// specific moment.
	// format and a are interpreted as with fmt.Printf.
	Annotatef(format string, a ...interface{})

	// AnnotateMetadata can be used to associate key/value with the span. If
	// indexed is false, the underlying implementation may create a searchable
	// index using this metadata. However, indexing itself is an implementation
	// specific feature and need be provided by every implementation.
	// NOTE that metadata is not communicated back to the requestor and is
	// intended for purely per-server/node use.
	AnnotateMetadata(key string, value interface{}, indexed bool) error

	// SetRequestMetadata appends metadata to be associated with this span and
	// hence encoded in any RPC requests made by this span. The interpretation
	// of the metdata is governed by the value of TraceFlags.
	SetRequestMetadata(metadata []byte)

	// Finish ends the span, marking the end time.  The span should
	// not be used after Finish is called.
	Finish(error)

	// Trace returns the id of the trace this Span is a member of.
	Trace() uniqueid.Id

	// Store returns the store that this Span is stored in.
	Store() Store

	Request(ctx *context.T) Request
}

// Store selectively collects information about traces in the system.
type Store interface {
	// TraceRecords returns TraceRecords for all traces saved in the store.
	TraceRecords() []TraceRecord

	// TraceRecord returns a TraceRecord for a given ID.  Returns
	// nil if the given id is not present.
	TraceRecord(traceid uniqueid.Id) *TraceRecord

	// ForceCollect forces the store to collect all information about a given trace and to capture
	// the log messages at the given log level.
	ForceCollect(traceid uniqueid.Id, level int)

	// Merge merges a vtrace.Response into the current store.
	Merge(response Response)

	// Return the log level in effect for this trace.
	LogLevel(traceid uniqueid.Id) int

	Flags(traceid uniqueid.Id) TraceFlags

	Start(traceid uniqueid.Id, span SpanRecord)

	Finish(traceid uniqueid.Id, span SpanRecord, timestamp time.Time)

	// Annotate records the requested annotation.
	Annotate(traceid uniqueid.Id, span SpanRecord, annotation Annotation)

	// AnnotateMetadata records the requested key value data with optional
	// indexing.
	AnnotateMetadata(traceid uniqueid.Id, span SpanRecord, key string, value interface{}, indexed bool) error
}

type Manager interface {
	// WithNewTrace creates a new vtrace context that is not the child of any
	// other span.  This is useful when starting operations that are
	// disconnected from the activity ctx is performing.  For example
	// this might be used to start background tasks.
	WithNewTrace(ctx *context.T, name string) (*context.T, Span)

	// WithContinuedTrace creates a span that represents a continuation of
	// a trace from a remote server.  name is the name of the new span and
	// req contains the parameters needed to connect this span with it's
	// trace.
	WithContinuedTrace(ctx *context.T, name string, req Request) (*context.T, Span)

	// WithNewSpan derives a context with a new Span that can be used to
	// trace and annotate operations across process boundaries.
	WithNewSpan(ctx *context.T, name string) (*context.T, Span)

	// Generate a Request from the current context.
	GetRequest(ctx *context.T) Request

	// Generate a Response from the current context.
	GetResponse(ctx *context.T) Response
}

// managerKey is used to store a Manger in the context.
type managerKey struct{}
type storeKey struct{}
type spanKey struct{}

// WithManager returns a new context with a Vtrace manager attached.
func WithManager(ctx *context.T, manager Manager) *context.T {
	return context.WithValue(ctx, managerKey{}, manager)
}

func manager(ctx *context.T) Manager {
	manager, _ := ctx.Value(managerKey{}).(Manager)
	if manager == nil {
		// TODO(mattr): I would log an error, but vlog is not legal to use
		// from this package.
		manager = emptyManager{}
	}
	return manager
}

// WithNewTrace creates a new vtrace context that is not the child of any
// other span.  This is useful when starting operations that are
// disconnected from the activity ctx is performing.  For example
// this might be used to start background tasks.
func WithNewTrace(ctx *context.T, name string) (*context.T, Span) {
	return manager(ctx).WithNewTrace(ctx, name)
}

// WithContinuedTrace creates a span that represents a continuation of
// a trace from a remote server.  name is the name of the new span and
// req contains the parameters needed to connect this span with it's
// trace.
func WithContinuedTrace(ctx *context.T, name string, req Request) (*context.T, Span) {
	return manager(ctx).WithContinuedTrace(ctx, name, req)
}

// WithNewSpan derives a context with a new Span that can be used to
// trace and annotate operations across process boundaries.
func WithNewSpan(ctx *context.T, name string) (*context.T, Span) {
	return manager(ctx).WithNewSpan(ctx, name)
}

func WithSpan(ctx *context.T, span Span) *context.T {
	return context.WithValue(ctx, spanKey{}, span)
}

func WithStore(ctx *context.T, store Store) *context.T {
	return context.WithValue(ctx, storeKey{}, store)
}

// Span finds the currently active span.
func GetSpan(ctx *context.T) Span {
	span, _ := ctx.Value(spanKey{}).(Span)
	if span == nil {
		return &emptySpan{}
	}
	return span
}

// VtraceStore returns the current Store.
func GetStore(ctx *context.T) Store {
	store, _ := ctx.Value(storeKey{}).(Store)
	if store == nil {
		return &emptyStore{}
	}
	return store
}

// ForceCollect forces the store to collect all information about the
// current trace.
func ForceCollect(ctx *context.T, level int) {
	store, _ := ctx.Value(storeKey{}).(Store)
	span, _ := ctx.Value(spanKey{}).(Span)
	store.ForceCollect(span.Trace(), level)
}

// Generate a Request from the current context.
func GetRequest(ctx *context.T) Request {
	return manager(ctx).GetRequest(ctx)
}

// Generate a Response from the current context.
func GetResponse(ctx *context.T) Response {
	return manager(ctx).GetResponse(ctx)
}

type emptyManager struct{}

func (emptyManager) WithNewTrace(ctx *context.T, name string) (*context.T, Span) {
	return ctx, emptySpan{}
}
func (emptyManager) WithContinuedTrace(ctx *context.T, name string, req Request) (*context.T, Span) {
	return ctx, emptySpan{}
}
func (emptyManager) WithNewSpan(ctx *context.T, name string) (*context.T, Span) {
	return ctx, emptySpan{}
}

func (emptyManager) GetRequest(ctx *context.T) (r Request)   { return }
func (emptyManager) GetResponse(ctx *context.T) (r Response) { return }

type emptySpan struct{}

func (emptySpan) Name() string                              { return "" }
func (emptySpan) ID() (id uniqueid.Id)                      { return }
func (emptySpan) Parent() (id uniqueid.Id)                  { return }
func (emptySpan) Annotate(s string)                         {}
func (emptySpan) Annotatef(format string, a ...interface{}) {}
func (emptySpan) AnnotateMetadata(key string, value interface{}, indexed bool) error {
	return nil
}
func (emptySpan) SetRequestMetadata([]byte)        {}
func (emptySpan) Finish(error)                     {}
func (emptySpan) Trace() (id uniqueid.Id)          { return }
func (emptySpan) Store() Store                     { return nil }
func (emptySpan) Request(*context.T) (req Request) { return }

type emptyStore struct{}

func (emptyStore) TraceRecords() []TraceRecord                  { return nil }
func (emptyStore) TraceRecord(traceid uniqueid.Id) *TraceRecord { return nil }
func (emptyStore) ForceCollect(traceid uniqueid.Id, level int)  {}
func (emptyStore) Start(traceid uniqueid.Id, span SpanRecord)   {}

func (emptyStore) Finish(traceid uniqueid.Id, span SpanRecord, timestamp time.Time) {}

func (emptyStore) Annotate(traceid uniqueid.Id, span SpanRecord, annotation Annotation) {}
func (emptyStore) AnnotateMetadata(traceid uniqueid.Id, span SpanRecord, key string, value interface{}, indexed bool) error {
	return nil
}
func (emptyStore) Merge(response Response)              {}
func (emptyStore) LogLevel(traceid uniqueid.Id) int     { return 0 }
func (emptyStore) Flags(traceid uniqueid.Id) TraceFlags { return 0 }
