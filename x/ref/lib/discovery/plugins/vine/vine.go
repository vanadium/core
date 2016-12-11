// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vine contains a discovery plugin that is using Vine protocol.
//
// This plugin is for testing Vanadium applications under unpredictable and
// unfriendly network conditions.
//
// Usage:
//
//	import (
//		"v.io/x/ref/lib/discovery"
//		"v.io/x/ref/lib/discovery/factory"
//		"v.io/x/ref/lib/discovery/plugins/vine"
//	)
//
//	p, err := vine.New(...)
//	...
//	df, err := discovery.NewFactory(ctx, p)
//	...
//	// The factory should be injected before v23.NewDiscovery() is called.
//	factory.InjectFactory(df)
//
//	d, err := v23.NewDiscovery(ctx)
//	...
package vine

import (
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/timekeeper"
)

const (
	defaultTTL = 90 * time.Second

	minTTLSlack = 2 * time.Second
	maxTTLSlack = 10 * time.Second
)

type plugin struct {
	peers func(*context.T) []string
	store *store

	clock timekeeper.TimeKeeper

	ttl          time.Duration
	ttlWithSlack time.Duration
}

func (p *plugin) Advertise(ctx *context.T, adinfo *idiscovery.AdInfo, done func()) error {
	go func() {
		defer done()

		// We need a context that is detached from the deadlines and cancellation of ctx
		// since we have to unpublish the advertisement from peers after ctx is canceled.
		rctx, cancel := context.WithRootCancel(ctx)
		defer cancel()
		defer p.unpublish(rctx, adinfo.Ad.Id)

		for {
			p.publish(ctx, *adinfo)

			select {
			case <-p.clock.After(p.ttl):
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *plugin) Scan(ctx *context.T, interfaceName string, callback func(*idiscovery.AdInfo), done func()) error {
	updated := p.store.listenToUpdates(ctx)

	go func() {
		defer done()

		seen := make(map[discovery.AdId]idiscovery.AdInfo)
		for {
			current := p.store.lookup(interfaceName)

			changed := make([]idiscovery.AdInfo, 0, len(current))
			for id, adinfo := range current {
				if old, ok := seen[id]; !ok || old.Hash != adinfo.Hash {
					changed = append(changed, adinfo)
				}
			}
			for id, adinfo := range seen {
				if _, ok := current[id]; !ok {
					adinfo.Lost = true
					changed = append(changed, adinfo)
				}
			}
			// Push new changes.
			for i := range changed {
				callback(&changed[i])
			}
			seen = current

			// Wait the next update.
			select {
			case <-updated:
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *plugin) Close() {
	p.store.stop()
}

func (p *plugin) publish(ctx *context.T, adinfo idiscovery.AdInfo) {
	// Clear DirAddrs since we publish a full advertisement information.
	adinfo.DirAddrs = nil

	var wg sync.WaitGroup
	for _, peer := range p.peers(ctx) {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			if err := StoreClient(peer).Add(ctx, adinfo, p.ttlWithSlack); err != nil {
				ctx.Error(err)
			}
		}(peer)
	}
	wg.Wait()
}

func (p *plugin) unpublish(ctx *context.T, id discovery.AdId) {
	var wg sync.WaitGroup
	for _, peer := range p.peers(ctx) {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			if err := StoreClient(peer).Delete(ctx, id); err != nil {
				ctx.Error(err)
			}
		}(peer)
	}
	wg.Wait()
}

// New returns a new vine plugin instance with default ttl (90s).
func New(ctx *context.T, name string, peers func(*context.T) []string) (idiscovery.Plugin, error) {
	return NewWithTTL(ctx, name, peers, defaultTTL)
}

// New returns a new vine plugin instance. If ttl is zero, the default values will be used.
func NewWithTTL(ctx *context.T, name string, peers func(*context.T) []string, ttl time.Duration) (idiscovery.Plugin, error) {
	return newWithClock(ctx, name, peers, ttl, timekeeper.RealTime())
}

func newWithClock(ctx *context.T, name string, peers func(*context.T) []string, ttl time.Duration, clock timekeeper.TimeKeeper) (idiscovery.Plugin, error) {
	store, err := newStore(ctx, name, clock)
	if err != nil {
		return nil, err
	}

	if ttl == 0 {
		ttl = defaultTTL
	}
	ttlSlack := time.Duration(ttl.Nanoseconds() / 10)
	if ttlSlack < minTTLSlack {
		ttlSlack = minTTLSlack
	} else if ttlSlack > maxTTLSlack {
		ttlSlack = maxTTLSlack
	}
	p := &plugin{
		peers:        peers,
		store:        store,
		clock:        clock,
		ttl:          ttl,
		ttlWithSlack: ttl + ttlSlack,
	}
	return p, nil
}
