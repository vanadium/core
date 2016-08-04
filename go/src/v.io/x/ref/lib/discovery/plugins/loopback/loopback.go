// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loopback implements loopback plugin for discovery service.
//
// This plugin simply returns back all the advertisements that are being
// advertised to itself. It would be useful when multiple applications on
// a single runtime advertise their services and want to discover each other.
package loopback

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
)

type plugin struct {
	mu sync.Mutex

	adinfoMap map[string]map[discovery.AdId]*idiscovery.AdInfo // GUARDED_BY(mu)

	updated   *sync.Cond
	updateSeq int // GUARDED_BY(mu)

	adStopper *idiscovery.Trigger
}

func (p *plugin) Advertise(ctx *context.T, adinfo *idiscovery.AdInfo, done func()) error {
	p.addAd(adinfo)
	stop := func() {
		p.removeAd(adinfo)
		done()
	}
	p.adStopper.Add(stop, ctx.Done())
	return nil
}

func (p *plugin) Scan(ctx *context.T, interfaceName string, callback func(*idiscovery.AdInfo), done func()) error {
	updated := p.listenToUpdates(ctx)

	go func() {
		defer done()

		seen := make(map[discovery.AdId]idiscovery.AdInfo)
		for {
			current := p.getAds(interfaceName)

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

func (p *plugin) Close() {}

func (p *plugin) addAd(adinfo *idiscovery.AdInfo) {
	p.mu.Lock()
	// Added adinfo to empty key ("") as well to simplify wildcard scan.
	for _, key := range []string{adinfo.Ad.InterfaceName, ""} {
		adinfos := p.adinfoMap[key]
		if adinfos == nil {
			adinfos = make(map[discovery.AdId]*idiscovery.AdInfo)
			p.adinfoMap[key] = adinfos
		}
		adinfos[adinfo.Ad.Id] = adinfo
	}
	p.updateSeq++
	p.mu.Unlock()
	p.updated.Broadcast()
}

func (p *plugin) removeAd(adinfo *idiscovery.AdInfo) {
	p.mu.Lock()
	for _, key := range []string{adinfo.Ad.InterfaceName, ""} {
		adinfos := p.adinfoMap[key]
		delete(adinfos, adinfo.Ad.Id)
		if len(adinfos) == 0 {
			delete(p.adinfoMap, key)
		}
	}
	p.updateSeq++
	p.mu.Unlock()
	p.updated.Broadcast()
}

func (p *plugin) getAds(interfaceName string) map[discovery.AdId]idiscovery.AdInfo {
	adinfos := make(map[discovery.AdId]idiscovery.AdInfo)
	p.mu.Lock()
	for id, adinfo := range p.adinfoMap[interfaceName] {
		adinfos[id] = *adinfo
	}
	p.mu.Unlock()
	return adinfos
}

func (p *plugin) listenToUpdates(ctx *context.T) <-chan struct{} {
	// TODO(jhahn): Find a more efficient way to notify updates.
	ch := make(chan struct{})
	go func() {
		defer close(ch)

		var updateSeqSeen int
		for {
			p.mu.Lock()
			for updateSeqSeen == p.updateSeq {
				p.updated.Wait()
			}
			updateSeqSeen = p.updateSeq
			p.mu.Unlock()

			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// New returns a new loopback plugin instance.
func New(ctx *context.T, host string) (idiscovery.Plugin, error) {
	p := &plugin{
		adinfoMap: make(map[string]map[discovery.AdId]*idiscovery.AdInfo),
		adStopper: idiscovery.NewTrigger(),
	}
	p.updated = sync.NewCond(&p.mu)
	return p, nil
}
