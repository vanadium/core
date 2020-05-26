// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mock

import (
	"reflect"
	"sync"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
)

type plugin struct {
	mu sync.Mutex

	updated   *sync.Cond
	updateSeq int // GUARDED_BY(mu)

	adStatus  idiscovery.AdStatus
	adinfoMap map[string]map[discovery.AdId]*idiscovery.AdInfo // GUARDED_BY(mu)
}

func (p *plugin) Advertise(ctx *context.T, adinfo *idiscovery.AdInfo, done func()) error {
	p.RegisterAd(adinfo)
	go func() {
		<-ctx.Done()
		p.UnregisterAd(adinfo)
		done()
	}()
	return nil
}

func (p *plugin) Scan(ctx *context.T, interfaceName string, callback func(*idiscovery.AdInfo), done func()) error { //nolint:gocyclo
	rescan := make(chan struct{})
	go func() {
		defer close(rescan)

		var updateSeqSeen int
		for {
			p.mu.Lock()
			for updateSeqSeen == p.updateSeq {
				p.updated.Wait()
			}
			updateSeqSeen = p.updateSeq
			p.mu.Unlock()
			select {
			case rescan <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer done()

		seen := make(map[discovery.AdId]idiscovery.AdInfo)
		for {
			current := make(map[discovery.AdId]idiscovery.AdInfo)
			p.mu.Lock()
			for key, adinfos := range p.adinfoMap {
				if len(interfaceName) > 0 && key != interfaceName {
					continue
				}
				for id, adinfo := range adinfos {
					current[id] = copyAd(adinfo, p.adStatus)
				}
			}
			p.mu.Unlock()

			changed := make([]idiscovery.AdInfo, 0, len(current))
			for id, adinfo := range current {
				old, ok := seen[id]
				if !ok || !reflect.DeepEqual(old, adinfo) {
					changed = append(changed, adinfo)
				}
			}
			for id, adinfo := range seen {
				if _, ok := current[id]; !ok {
					adinfo.Status = idiscovery.AdReady
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
			case <-rescan:
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (p *plugin) Close() {}

func copyAd(adinfo *idiscovery.AdInfo, status idiscovery.AdStatus) idiscovery.AdInfo {
	copied := *adinfo
	copied.Status = status
	switch copied.Status {
	case idiscovery.AdReady:
		copied.DirAddrs = nil
	case idiscovery.AdNotReady:
		copied.Ad = discovery.Advertisement{Id: adinfo.Ad.Id}
	case idiscovery.AdPartiallyReady:
		copied.Ad.Attachments = nil
	}
	return copied
}

// RegisterAd registers an advertisement to the plugin. If there is already an
// advertisement with the same id, it will be updated with the given advertisement.
func (p *plugin) RegisterAd(adinfo *idiscovery.AdInfo) {
	p.mu.Lock()
	adinfos := p.adinfoMap[adinfo.Ad.InterfaceName]
	if adinfos == nil {
		adinfos = make(map[discovery.AdId]*idiscovery.AdInfo)
		p.adinfoMap[adinfo.Ad.InterfaceName] = adinfos
	}
	adinfos[adinfo.Ad.Id] = adinfo
	p.updateSeq++
	p.mu.Unlock()
	p.updated.Broadcast()
}

// UnregisterAd unregisters a registered advertisement from the plugin.
func (p *plugin) UnregisterAd(adinfo *idiscovery.AdInfo) {
	p.mu.Lock()
	adinfos := p.adinfoMap[adinfo.Ad.InterfaceName]
	delete(adinfos, adinfo.Ad.Id)
	if len(adinfos) == 0 {
		p.adinfoMap[adinfo.Ad.InterfaceName] = nil
	}
	p.updateSeq++
	p.mu.Unlock()
	p.updated.Broadcast()
}

func New() *plugin { //nolint:golint // exported func New returns unexported type
	// which can be annoying to use.
	return NewWithAdStatus(idiscovery.AdReady)
}

func NewWithAdStatus(status idiscovery.AdStatus) *plugin { //nolint:golint
	p := &plugin{adStatus: status, adinfoMap: make(map[string]map[discovery.AdId]*idiscovery.AdInfo)}
	p.updated = sync.NewCond(&p.mu)
	return p
}
