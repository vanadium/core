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
func NewStore(opts flags.VtraceFlags) (vtrace.Store, error) {
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
	if ts := s.traces[id]; ts != nil {
		return ts
	}
	ts := newTraceStore(id, level)
	s.traces[id] = ts
	ts.moveAfter(s.head)
	// Trim elements beyond our size limit.
	for len(s.traces) > s.opts.CacheSize {
		el := s.head.prev
		el.removeFromList()
		delete(s.traces, el.id)
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

func (s *Store) LogLevel(id uniqueid.Id) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.traces[id]
	if ts == nil {
		return 0
	}
	return ts.level
}

func (s *Store) spanRegexpRecordingLocked(traceid uniqueid.Id, msg string) *traceStore {
	if ts := s.traces[traceid]; ts != nil {
		return ts
	}
	if s.collectRegexp != nil && s.collectRegexp.MatchString(msg) {
		// If the supplied msg matches collectRegexp, then force collect its trace.
		return s.forceCollectLocked(traceid, s.defaultLevel)
	}
	return nil
}

func (s *Store) rootRecordingLocked(traceid, parentid uniqueid.Id, name string) *traceStore {
	if ts := s.traces[traceid]; ts != nil {
		return ts
	}
	sr := s.opts.SampleRate
	if traceid == parentid && sr > 0.0 && (sr >= 1.0 || rand.Float64() < sr) {
		// If this is a root span, we may automatically sample it for collection.
		return s.forceCollectLocked(traceid, s.defaultLevel)
	}
	return s.spanRegexpRecordingLocked(traceid, name)
}

func (s *Store) Start(traceid uniqueid.Id, span vtrace.SpanRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := s.rootRecordingLocked(traceid, span.Parent, span.Name)
	if ts == nil {
		return
	}
	ts.newOrUse(span)
	ts.moveAfter(s.head)
}

func (s *Store) Finish(traceid uniqueid.Id, span vtrace.SpanRecord, timestamp time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.traces[traceid]; ts != nil {
		ts.finish(span, timestamp)
		ts.moveAfter(s.head)
	}
}

func (s *Store) Annotate(traceid uniqueid.Id, span vtrace.SpanRecord, annotation vtrace.Annotation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ts := s.spanRegexpRecordingLocked(traceid, annotation.Message); ts != nil {
		ts.annotate(span, annotation)
		ts.moveAfter(s.head)
	}
}

// method returns the collection method for the given trace.
func (s *Store) Flags(id uniqueid.Id) vtrace.TraceFlags {
	s.mu.Lock()
	defer s.mu.Unlock()
	mask := vtrace.Empty
	if ts := s.traces[id]; ts != nil {
		mask |= vtrace.CollectInMemory
	}
	if s.opts.EnableAWSXRay {
		mask |= vtrace.AWSXRay
	}
	return mask
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
	if ts := s.traces[id]; ts != nil {
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

func (ts *traceStore) newOrUse(s vtrace.SpanRecord) *vtrace.SpanRecord {
	if sr, ok := ts.spans[s.Id]; ok {
		return sr
	}
	cpy := &vtrace.SpanRecord{
		Id:     s.Id,
		Parent: s.Parent,
		Name:   s.Name,
		Start:  s.Start,
	}
	ts.spans[s.Id] = cpy
	return cpy
}

func (ts *traceStore) finish(s vtrace.SpanRecord, timestamp time.Time) {
	sr := ts.newOrUse(s)
	sr.End = timestamp
}

func (ts *traceStore) annotate(s vtrace.SpanRecord, annotation vtrace.Annotation) {
	sr := ts.newOrUse(s)
	sr.Annotations = append(sr.Annotations, annotation)
}

func (ts *traceStore) merge(spans []vtrace.SpanRecord) {
	// TODO(mattr): We need to carefully merge here to correct for
	// clock skew and ordering.  We should estimate the clock skew
	// by assuming that children of parent need to start after parent
	// and end before now.
	for _, span := range spans {
		if ts.spans[span.Id] == nil {
			sr := &vtrace.SpanRecord{}
			copySpanRecord(sr, &span)
			ts.spans[span.Id] = sr
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

func copySpanRecord(out, in *vtrace.SpanRecord) {
	*out = *in
	out.Metadata = make([]byte, len(in.Metadata))
	copy(out.Metadata, in.Metadata)
	out.Annotations = make([]vtrace.Annotation, len(in.Annotations))
	copy(out.Annotations, in.Annotations)
}

func (ts *traceStore) traceRecord(out *vtrace.TraceRecord) {
	spans := make([]vtrace.SpanRecord, len(ts.spans))
	i := 0
	for _, span := range ts.spans {
		copySpanRecord(&spans[i], span)
		i++
	}
	out.Id = ts.id
	out.Spans = spans
}
