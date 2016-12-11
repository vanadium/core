// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/vom"
)

type typeCacheEntry struct {
	enc    *vom.TypeEncoder
	dec    *vom.TypeDecoder
	cancel context.CancelFunc
	writer bool
	ready  chan struct{}
}

const typeCacheCollectInterval = 1000

type typeCache struct {
	mu     sync.Mutex
	flows  map[flow.ManagedConn]*typeCacheEntry
	writes int
}

func newTypeCache() *typeCache {
	return &typeCache{flows: make(map[flow.ManagedConn]*typeCacheEntry)}
}

func (tc *typeCache) writer(c flow.ManagedConn) (write func(flow.Flow, context.CancelFunc)) {
	tc.mu.Lock()
	tce := tc.flows[c]
	if tce == nil {
		tce = &typeCacheEntry{ready: make(chan struct{})}
		tc.flows[c] = tce
	}
	if !tce.writer {
		// TODO(mattr): This is a very course garbage collection policy.
		// Every 1000 connections we clean out expired entries.
		if tc.writes++; tc.writes%typeCacheCollectInterval == 0 {
			go tc.collect()
		}
		tce.writer = true
		write = func(f flow.Flow, c context.CancelFunc) {
			if f != nil {
				tce.cancel = c
				tce.enc = vom.NewTypeEncoder(f)
				tce.dec = vom.NewTypeDecoder(f)
				tce.dec.Start() // Stopped in collect()
			} else {
				c()
			}
			close(tce.ready)
		}
	}
	tc.mu.Unlock()
	return
}

func (tc *typeCache) get(ctx *context.T, c flow.ManagedConn) (*vom.TypeEncoder, *vom.TypeDecoder, error) {
	tc.mu.Lock()
	tce := tc.flows[c]
	if tce == nil {
		tce = &typeCacheEntry{ready: make(chan struct{})}
		tc.flows[c] = tce
	}
	tc.mu.Unlock()
	select {
	case <-c.Closed():
		return nil, nil, newErrTypeFlowFailure(ctx, nil)
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-tce.ready:
		if tce.enc == nil || tce.dec == nil {
			return nil, nil, newErrTypeFlowFailure(ctx, nil)
		}
	}
	return tce.enc, tce.dec, nil
}

func (tc *typeCache) collect() {
	tc.mu.Lock()
	conns := make([]flow.ManagedConn, len(tc.flows))
	i := 0
	for c, _ := range tc.flows {
		conns[i] = c
		i++
	}
	tc.mu.Unlock()

	last := 0
	for _, c := range conns {
		select {
		case <-c.Closed():
			conns[last] = c
			last++
		default:
		}
	}

	if last > 0 {
		tc.mu.Lock()
		for idx := 0; idx < last; idx++ {
			conn := conns[idx]
			tce := tc.flows[conn]
			if tce != nil {
				if tce.cancel != nil {
					tce.cancel()
				}
				tce.dec.Stop()
				delete(tc.flows, conn)
			}
		}
		tc.mu.Unlock()
	}
}

func (tc *typeCache) close() {
	tc.mu.Lock()
	for _, tce := range tc.flows {
		if tce != nil {
			if tce.cancel != nil {
				tce.cancel()
			}
			if tce.dec != nil {
				tce.dec.Stop()
			}
		}
	}
	tc.mu.Unlock()
}
