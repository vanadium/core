// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"sync"

	"v.io/v23/naming"
	libstats "v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/counter"
)

// stats contains various exported stats we maintain for the device manager.
type stats struct {
	sync.Mutex
	// Prefix for each of the stat names.
	prefix string
	// How many times apps were run via the Run rpc.
	runs *counter.Counter
	// How many times apps were auto-restarted by the reaper.
	restarts *counter.Counter
	// Same as above, but broken down by instance.
	// This is not of type lib/stats::Map since we want the detailed rate
	// and delta stats per instance.
	// TODO(caprita): Garbage-collect old instances?
	runsPerInstance     map[string]*counter.Counter
	restartsPerInstance map[string]*counter.Counter
}

func newCounter(names ...string) *counter.Counter {
	return libstats.NewCounter(naming.Join(names...))
}

func newStats(prefix string) *stats {
	return &stats{
		runs:                newCounter(prefix, "runs"),
		runsPerInstance:     make(map[string]*counter.Counter),
		restarts:            newCounter(prefix, "restarts"),
		restartsPerInstance: make(map[string]*counter.Counter),
		prefix:              prefix,
	}
}

func (s *stats) incrRestarts(instance string) {
	s.Lock()
	defer s.Unlock()
	s.restarts.Incr(1)
	perInstanceCtr, ok := s.restartsPerInstance[instance]
	if !ok {
		perInstanceCtr = newCounter(s.prefix, "restarts", instance)
		s.restartsPerInstance[instance] = perInstanceCtr
	}
	perInstanceCtr.Incr(1)
}

func (s *stats) incrRuns(instance string) {
	s.Lock()
	defer s.Unlock()
	s.runs.Incr(1)
	perInstanceCtr, ok := s.runsPerInstance[instance]
	if !ok {
		perInstanceCtr = newCounter(s.prefix, "runs", instance)
		s.runsPerInstance[instance] = perInstanceCtr
	}
	perInstanceCtr.Incr(1)
}
