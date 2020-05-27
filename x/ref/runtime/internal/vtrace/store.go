// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace

import (
	"math/rand"
	"regexp"
	"sync"
	"time"

	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"

	"v.io/x/ref/lib/flags"
)

// Store implements a store for traces.  The idea is to keep all the
// information we have about some subset of traces that pass through
// the server.  For now we just implement an LRU cache, so the least
// recently started/finished/annotated traces expire after some
// maximum trace count is reached.
// TODO(mattr): LRU is the wrong policy in the long term, we should
// try to keep some diverse set of traces and allow users to
// specifically tell us to capture a specific trace.  LRU will work OK
// for many testing scenarios and low volume applications.
type Store struct {
	opts          flags.VtraceFlags
	collectRegexp *regexp.Regexp
	defaultLevel  int

	// traces and head together implement a linked-hash-map.
	// head points to the head and tail of the doubly-linked-list
	// of recently used items (the tail is the LRU traceStore).
	// TODO(mattr): Use rwmutex.
	mu     sync.Mutex
	traces map[uniqueid.Id]*traceStore // GUARDED_BY(mu)
	head   *traceStore                 // GUARDED_BY(mu)
}

// NewStore creates a new store according to the passed in opts.
func NewStore(opts flags.VtraceFlags) (*Store, error) {
	head := &traceStore{}
	head.next, head.prev = head, head

	var collectRegexp *regexp.Regexp
	if opts.CollectRegexp != "" {
		var err error
		if collectRegexp, err = regexp.Compile(opts.CollectRegexp); err != nil {
			return nil, err
		}
	}

	return &Store{
		opts:          opts,
		defaultLevel:  opts.LogLevel,
		collectRegexp: collectRegexp,
		traces:        make(map[uniqueid.Id]*traceStore),
		head:          head,
	}, nil
}

func (s *Store) ForceCollect(id uniqueid.Id, level int) {
	s.mu.Lock()
	s.forceCollectLocked(id, level)
	s.mu.Unlock()
}

func (s *Store) forceCollectLocked(id uniqueid.Id, level int) *traceStore {
	ts := s.traces[id]
	if ts == nil {
		ts = newTraceStore(id, level)
		s.traces[id] = ts
		ts.moveAfter(s.head)
		// Trim elements beyond our size limit.
		for len(s.traces) > s.opts.CacheSize {
			el := s.head.prev
			el.removeFromList()
			delete(s.traces, el.id)
		}
	}
	return ts
}

// Merge merges a vtrace.Response into the current store.
func (s *Store) Merge(t vtrace.Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ts *traceStore
	if t.Flags&vtrace.CollectInMemory != 0 {
		ts = s.forceCollectLocked(t.Trace.Id, s.defaultLevel)
	} else {
		ts = s.traces[t.Trace.Id]
	}
	if ts != nil {
		ts.merge(t.Trace.Spans)
	}
}

// annotate stores an annotation for the trace if it is being collected.
func (s *Store) annotate(span *span, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.traces[span.trace]
	if ts == nil {
		if s.collectRegexp != nil && s.collectRegexp.MatchString(msg) {
			ts = s.forceCollectLocked(span.trace, s.defaultLevel)
		}
	}

	if ts != nil {
		ts.annotate(span, msg)
		ts.moveAfter(s.head)
	}
}

func (s *Store) logLevel(id uniqueid.Id) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.traces[id]
	if ts == nil {
		return 0
	}
	return ts.level
}

// start stores data about a starting span if the trace is being collected.
func (s *Store) start(span *span) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := s.traces[span.trace]
	if ts == nil {
		sr := s.opts.SampleRate
		if span.trace == span.parent && sr > 0.0 && (sr >= 1.0 || rand.Float64() < sr) {
			// If this is a root span, we may automatically sample it for collection.
			ts = s.forceCollectLocked(span.trace, s.defaultLevel)
		} else if s.collectRegexp != nil && s.collectRegexp.MatchString(span.name) {
			// If this span matches collectRegexp, then force collect its trace.
			ts = s.forceCollectLocked(span.trace, s.defaultLevel)
		}
	}
	if ts != nil {
		ts.start(span)
		ts.moveAfter(s.head)
	}
}

// finish stores data about a finished span if the trace is being collected.
func (s *Store) finish(span *span) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[span.trace]; ts != nil {
		ts.finish(span)
		ts.moveAfter(s.head)
	}
}

// method returns the collection method for the given trace.
func (s *Store) flags(id uniqueid.Id) vtrace.TraceFlags {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[id]; ts != nil {
		return vtrace.CollectInMemory
	}
	return vtrace.Empty
}

// TraceRecords returns TraceRecords for all traces saved in the store.
func (s *Store) TraceRecords() []vtrace.TraceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]vtrace.TraceRecord, len(s.traces))
	i := 0
	for _, ts := range s.traces {
		ts.traceRecord(&out[i])
		i++
	}
	return out
}

// TraceRecord returns a TraceRecord for a given Id.  Returns
// nil if the given id is not present.
func (s *Store) TraceRecord(id uniqueid.Id) *vtrace.TraceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := &vtrace.TraceRecord{}
	ts := s.traces[id]
	if ts != nil {
		ts.traceRecord(out)
	}
	return out
}

type traceStore struct {
	id         uniqueid.Id
	level      int
	spans      map[uniqueid.Id]*vtrace.SpanRecord
	prev, next *traceStore
}

func newTraceStore(id uniqueid.Id, level int) *traceStore {
	return &traceStore{
		id:    id,
		level: level,
		spans: make(map[uniqueid.Id]*vtrace.SpanRecord),
	}
}

func (ts *traceStore) record(s *span) *vtrace.SpanRecord {
	record, ok := ts.spans[s.id]
	if !ok {
		record = &vtrace.SpanRecord{
			Id:     s.id,
			Parent: s.parent,
			Name:   s.name,
			Start:  s.start,
		}
		ts.spans[s.id] = record
	}
	return record
}

func (ts *traceStore) annotate(s *span, msg string) {
	record := ts.record(s)
	record.Annotations = append(record.Annotations, vtrace.Annotation{
		When:    time.Now(),
		Message: msg,
	})
}

func (ts *traceStore) start(s *span) {
	ts.record(s)
}

func (ts *traceStore) finish(s *span) {
	ts.record(s).End = time.Now()
}

func (ts *traceStore) merge(spans []vtrace.SpanRecord) {
	// TODO(mattr): We need to carefully merge here to correct for
	// clock skew and ordering.  We should estimate the clock skew
	// by assuming that children of parent need to start after parent
	// and end before now.
	for _, span := range spans {
		if ts.spans[span.Id] == nil {
			ts.spans[span.Id] = copySpanRecord(span)
		}
	}
}

func (ts *traceStore) removeFromList() {
	if ts.prev != nil {
		ts.prev.next = ts.next
	}
	if ts.next != nil {
		ts.next.prev = ts.prev
	}
	ts.next = nil
	ts.prev = nil
}

func (ts *traceStore) moveAfter(prev *traceStore) {
	ts.removeFromList()
	ts.prev = prev
	ts.next = prev.next
	prev.next.prev = ts
	prev.next = ts
}

func copySpanRecord(in vtrace.SpanRecord) *vtrace.SpanRecord {
	return &vtrace.SpanRecord{
		Id:          in.Id,
		Parent:      in.Parent,
		Name:        in.Name,
		Start:       in.Start,
		End:         in.End,
		Annotations: append([]vtrace.Annotation{}, in.Annotations...),
	}
}

func (ts *traceStore) traceRecord(out *vtrace.TraceRecord) {
	spans := make([]vtrace.SpanRecord, 0, len(ts.spans))
	for _, span := range ts.spans {
		spans = append(spans, *copySpanRecord(*span))
	}
	out.Id = ts.id
	out.Spans = spans
}
