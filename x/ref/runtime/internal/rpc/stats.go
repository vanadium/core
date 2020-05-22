// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"

	"v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/counter"
	"v.io/x/ref/lib/stats/histogram"

	"v.io/v23/naming"
)

type outstandingCall struct {
	remote naming.Endpoint
	method string
	when   time.Time
}

type outstandingCalls []*outstandingCall

func (oc outstandingCalls) Less(i, j int) bool {
	return oc[i].when.Before(oc[j].when)
}
func (oc outstandingCalls) Swap(i, j int) {
	oc[i], oc[j] = oc[j], oc[i]
}
func (oc outstandingCalls) Len() int {
	return len(oc)
}

type outstandingStats struct {
	prefix      string
	mu          sync.Mutex
	outstanding map[*outstandingCall]bool
}

func newOutstandingStats(prefix string) *outstandingStats {
	o := &outstandingStats{
		prefix:      prefix,
		outstanding: make(map[*outstandingCall]bool),
	}
	stats.NewStringFunc(prefix, o.String)
	return o
}

func (o *outstandingStats) String() string {
	defer o.mu.Unlock()
	o.mu.Lock()
	if len(o.outstanding) == 0 {
		return "No outstanding calls."
	}
	calls := make(outstandingCalls, 0, len(o.outstanding))
	for o := range o.outstanding {
		calls = append(calls, o)
	}
	sort.Sort(calls)
	now := time.Now()
	buf := &bytes.Buffer{}
	for _, o := range calls {
		fmt.Fprintf(buf, "%s age:%v from:%v\n", o.method, now.Sub(o.when), o.remote)
	}
	return buf.String()
}

func (o *outstandingStats) close() {
	stats.Delete(o.prefix) //nolint:errcheck
}

func (o *outstandingStats) start(method string, remote naming.Endpoint) func() {
	o.mu.Lock()
	nw := &outstandingCall{
		method: method,
		remote: remote,
		when:   time.Now(),
	}
	o.outstanding[nw] = true
	o.mu.Unlock()

	return func() {
		o.mu.Lock()
		delete(o.outstanding, nw)
		o.mu.Unlock()
	}
}

type rpcStats struct {
	mu                  sync.RWMutex
	prefix              string
	methods             map[string]*perMethodStats
	blessingsCacheStats *blessingsCacheStats
}

func newRPCStats(prefix string) *rpcStats {
	return &rpcStats{
		prefix:              prefix,
		methods:             make(map[string]*perMethodStats),
		blessingsCacheStats: newBlessingsCacheStats(prefix),
	}
}

type perMethodStats struct {
	latency *histogram.Histogram
}

func (s *rpcStats) stop() {
	stats.Delete(s.prefix) //nolint:errcheck
}

func (s *rpcStats) record(method string, latency time.Duration) {
	// Try first with a read lock. This will succeed in the most common
	// case. If it fails, try again with a write lock and create the stats
	// objects if they are still not there.
	s.mu.RLock()
	m, ok := s.methods[method]
	s.mu.RUnlock()
	if !ok {
		m = s.newPerMethodStats(method)
	}
	m.latency.Add(int64(latency / time.Millisecond)) //nolint:errcheck
}

//nolint:deadcode,unused
func (s *rpcStats) recordBlessingCache(hit bool) {
	s.blessingsCacheStats.incr(hit)
}

// newPerMethodStats creates a new perMethodStats object if one doesn't exist
// already. It returns the newly created object, or the already existing one.
func (s *rpcStats) newPerMethodStats(method string) *perMethodStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.methods[method]
	if !ok {
		name := naming.Join(s.prefix, "methods", method, "latency-ms")
		s.methods[method] = &perMethodStats{
			latency: stats.NewHistogram(name, histogram.Options{
				NumBuckets:         25,
				GrowthFactor:       1,
				SmallestBucketSize: 1,
				MinValue:           0,
			}),
		}
		m = s.methods[method]
	}
	return m
}

// blessingsCacheStats keeps blessing cache hits and total calls received to determine
// how often the blessingCache is being used.
type blessingsCacheStats struct {
	callsReceived, cacheHits *counter.Counter
}

func newBlessingsCacheStats(prefix string) *blessingsCacheStats {
	cachePrefix := naming.Join(prefix, "security", "blessings", "cache")
	return &blessingsCacheStats{
		callsReceived: stats.NewCounter(naming.Join(cachePrefix, "attempts")),
		cacheHits:     stats.NewCounter(naming.Join(cachePrefix, "hits")),
	}
}

// Incr increments the cache attempt counter and the cache hit counter if hit is true.
//nolint:deadcode,unused
func (s *blessingsCacheStats) incr(hit bool) {
	s.callsReceived.Incr(1)
	if hit {
		s.cacheHits.Incr(1)
	}
}
