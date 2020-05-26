// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vine

import (
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/rpc"
	"v.io/v23/security"
	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/timekeeper"
)

type store struct {
	mu sync.Mutex

	adinfoMap   map[string]map[discovery.AdId]*idiscovery.AdInfo // GUARDED_BY(mu)
	expirations map[discovery.AdId]time.Time                     // GUARDED_BY(mu)

	updated   *sync.Cond
	updateSeq int // GUARDED_BY(mu)

	clock timekeeper.TimeKeeper

	stop func()
}

func (s *store) Add(_ *context.T, _ rpc.ServerCall, adinfo idiscovery.AdInfo, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Added adinfo to empty key ("") as well to simplify wildcard scan.
	for _, key := range []string{adinfo.Ad.InterfaceName, ""} {
		adinfos := s.adinfoMap[key]
		if adinfos == nil {
			adinfos = make(map[discovery.AdId]*idiscovery.AdInfo)
			s.adinfoMap[key] = adinfos
		}
		adinfos[adinfo.Ad.Id] = &adinfo
	}
	s.expirations[adinfo.Ad.Id] = s.clock.Now().Add(ttl)

	s.updateSeq++
	s.updated.Broadcast()
	return nil
}

func (s *store) Delete(_ *context.T, _ rpc.ServerCall, id discovery.AdId) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleteLocked(id)
	return nil
}

func (s *store) lookup(interfaceName string) map[discovery.AdId]idiscovery.AdInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	adinfos := make(map[discovery.AdId]idiscovery.AdInfo, len(s.adinfoMap[interfaceName]))
	for id, adinfo := range s.adinfoMap[interfaceName] {
		adinfos[id] = *adinfo
	}
	return adinfos
}

func (s *store) deleteLocked(id discovery.AdId) {
	for key, adinfos := range s.adinfoMap {
		delete(adinfos, id)
		if len(adinfos) == 0 {
			delete(s.adinfoMap, key)
		}
	}
	delete(s.expirations, id)

	s.updateSeq++
	s.updated.Broadcast()
}

func (s *store) listenToUpdates(ctx *context.T) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)

		var updateSeqSeen int
		for {
			s.mu.Lock()
			for updateSeqSeen == s.updateSeq {
				s.updated.Wait()
			}
			updateSeqSeen = s.updateSeq
			s.mu.Unlock()

			select {
			case ch <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func (s *store) flushExpired(ctx *context.T) {
	updated := s.listenToUpdates(ctx)

	for {
		s.mu.Lock()
		now := s.clock.Now()

		var nextExpiry time.Time
		for id, expiry := range s.expirations {
			if expiry.After(now) {
				if nextExpiry.IsZero() || expiry.Before(nextExpiry) {
					nextExpiry = expiry
				}
				continue
			}

			s.deleteLocked(id)
		}
		s.mu.Unlock()

		var timer <-chan time.Time
		if !nextExpiry.IsZero() {
			timer = s.clock.After(nextExpiry.Sub(now))
		}
		select {
		case <-timer:
		case <-updated:
		case <-ctx.Done():
			return
		}
	}
}

func newStore(ctx *context.T, name string, clock timekeeper.TimeKeeper) (*store, error) {
	s := &store{
		adinfoMap:   make(map[string]map[discovery.AdId]*idiscovery.AdInfo),
		expirations: make(map[discovery.AdId]time.Time),
		clock:       clock,
	}
	s.updated = sync.NewCond(&s.mu)

	ctx, cancel := context.WithCancel(ctx)
	// TODO(suharshs): Make the authorizer configurable?
	_, server, err := v23.WithNewServer(ctx, name, StoreServer(s), security.AllowEveryone())
	if err != nil {
		cancel()
		return nil, err
	}
	go s.flushExpired(ctx)

	s.stop = func() {
		cancel()
		<-server.Closed()
	}
	return s, nil
}
