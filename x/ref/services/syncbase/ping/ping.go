// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ping provides a mechanism for pinging a set of servers in parallel to
// identify responsive ones.
package ping

import (
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/options"
)

// PingResult is the result of a single ping.
type PingResult struct {
	Name string          // name that was pinged
	Err  error           // either a connection error or a timeout/canceled error
	Conn flow.PinnedConn // the PinnedConn to the remote end.
}

// PingInParallel attempts to make a connection in parallel to all specified
// names, waits for some time, then returns a map of name to PingResult.
// - 'timeout' is the maximum duration that PingInParallel will wait before
//   cancelling any outstanding calls and returning.
// - 'channelTimeout' is the channelTimeout that will be used for healthchecks
//   on any created connections.
// Notes:
// - PingInParallel may report erroneous ping latencies if the system clock
//   changes during the call. Clients must be resilient to such errors.
func PingInParallel(ctx *context.T, names []string, timeout, channelTimeout time.Duration) (map[string]PingResult, error) {
	results := map[string]PingResult{}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resultChan := make(chan PingResult)
	// Spawn goroutines.
	for _, name := range names {
		// Note, timeoutCtx is thread-safe and is not mutated by the spawned goroutines.
		go ping(timeoutCtx, name, channelTimeout, resultChan)
	}
	for range names {
		r := <-resultChan
		results[r.Name] = r
	}
	return results, nil
}

////////////////////////////////////////
// Internal helpers

// ping pings a single name and sends the PingResult to resultChan.
func ping(ctx *context.T, name string, channelTimeout time.Duration, resultChan chan<- PingResult) {
	c, err := v23.GetClient(ctx).PinConnection(ctx, name, options.ChannelTimeout(channelTimeout))
	resultChan <- PingResult{Name: name, Err: err, Conn: c}
}
