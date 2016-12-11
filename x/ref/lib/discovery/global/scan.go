// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"fmt"
	"reflect"
	"sort"

	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/naming"

	idiscovery "v.io/x/ref/lib/discovery"
)

func (d *gdiscovery) Scan(ctx *context.T, query string) (<-chan discovery.Update, error) {
	matcher, err := idiscovery.NewMatcher(ctx, query)
	if err != nil {
		return nil, err
	}

	updateCh := make(chan discovery.Update, 10)
	go func() {
		defer close(updateCh)

		var prevFound map[discovery.AdId]*idiscovery.AdInfo
		for {
			found, err := d.doScan(ctx, matcher)
			if found == nil {
				if err != nil {
					ctx.Error(err)
				}
			} else {
				sendUpdates(ctx, prevFound, found, updateCh)
				prevFound = found
			}

			select {
			case <-d.clock.After(d.scanInterval):
			case <-ctx.Done():
				return
			}
		}
	}()
	return updateCh, nil
}

func (d *gdiscovery) doScan(ctx *context.T, matcher idiscovery.Matcher) (map[discovery.AdId]*idiscovery.AdInfo, error) {
	scanCh, err := d.ns.Glob(ctx, generateGlobQuery(matcher))
	if err != nil {
		return nil, err
	}
	defer func() {
		for range scanCh {
		}
	}()

	found := make(map[discovery.AdId]*idiscovery.AdInfo)
	for {
		select {
		case glob, ok := <-scanCh:
			if !ok {
				return found, nil
			}
			adinfo, err := convToAdInfo(glob)
			if err != nil {
				ctx.Error(err)
				continue
			}
			// Since mount operations are not atomic, we may not have addresses yet.
			// Ignore it. It will be re-scanned in the next cycle.
			if len(adinfo.Ad.Addresses) == 0 {
				continue
			}
			// Filter out advertisements from the same discovery instance.
			if d.hasAd(&adinfo.Ad) {
				continue
			}
			matched, err := matcher.Match(&adinfo.Ad)
			if err != nil {
				ctx.Error(err)
				continue
			}
			if !matched {
				continue
			}
			found[adinfo.Ad.Id] = adinfo
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func generateGlobQuery(matcher idiscovery.Matcher) string {
	// The suffixes are of the form "id/interfaceName/timestamp/attrs" so we need
	// to replace wildcards in our query with values based on the query matcher.
	id, interfaceName, timestamp, attrs := "*", "*", "*", "*"
	// Currently we support query by id or interfaceName.
	if targetKey := matcher.TargetKey(); targetKey != "" {
		id = targetKey
	}
	if targetInterface := matcher.TargetInterfaceName(); targetInterface != "" {
		interfaceName = naming.EncodeAsNameElement(targetInterface)
	}
	return naming.Join(id, interfaceName, timestamp, attrs)
}

func (d *gdiscovery) hasAd(ad *discovery.Advertisement) bool {
	d.mu.Lock()
	_, ok := d.ads[ad.Id]
	d.mu.Unlock()
	return ok
}

func convToAdInfo(glob naming.GlobReply) (*idiscovery.AdInfo, error) {
	switch g := glob.(type) {
	case *naming.GlobReplyEntry:
		ad, timestampNs, err := decodeAdFromSuffix(g.Value.Name)
		if err != nil {
			return nil, err
		}
		addrs := make([]string, 0, len(g.Value.Servers))
		for _, server := range g.Value.Servers {
			addrs = append(addrs, server.Server)
		}
		// We sort the addresses to avoid false updates.
		sort.Strings(addrs)
		ad.Addresses = addrs
		return &idiscovery.AdInfo{Ad: *ad, TimestampNs: timestampNs}, nil
	case *naming.GlobReplyError:
		return nil, fmt.Errorf("glob error on %s: %v", g.Value.Name, g.Value.Error)
	default:
		return nil, fmt.Errorf("unexpected glob reply %v", g)
	}
}

func sendUpdates(ctx *context.T, prevFound, found map[discovery.AdId]*idiscovery.AdInfo, updateCh chan<- discovery.Update) {
	for id, adinfo := range found {
		var updates []discovery.Update
		if prev := prevFound[id]; prev == nil {
			updates = []discovery.Update{idiscovery.NewUpdate(adinfo)}
		} else {
			if !reflect.DeepEqual(prev.Ad, adinfo.Ad) {
				prev.Lost = true
				updates = []discovery.Update{
					idiscovery.NewUpdate(prev),
					idiscovery.NewUpdate(adinfo),
				}
			}
			delete(prevFound, id)
		}
		for _, update := range updates {
			select {
			case updateCh <- update:
			case <-ctx.Done():
				return
			}
		}
	}

	for _, prev := range prevFound {
		prev.Lost = true
		update := idiscovery.NewUpdate(prev)
		select {
		case updateCh <- update:
		case <-ctx.Done():
			return
		}
	}
}
