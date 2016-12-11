// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cache

import (
	"fmt"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/agent"
	"v.io/x/ref/services/agent/internal/lru"
)

const pkgPath = "v.io/x/ref/services/agent/internal/cache"

var (
	errNotImplemented = verror.Register(pkgPath+".errNotImplemented", verror.NoRetry, "{1:}{2:} Not implemented{:_}")
)

const (
	maxNegativeCacheEntries = 100
)

// cachedRoots is a security.BlessingRoots implementation that
// wraps over another implementation and adds caching.
type cachedRoots struct {
	mu    *sync.RWMutex
	impl  security.BlessingRoots
	cache map[string][]security.BlessingPattern // GUARDED_BY(mu)

	// TODO(ataly): Get rid of the following fields once all agents have been
	// updated to support the 'Dump' method.
	dumpExists bool       // GUARDED_BY(my)
	negative   *lru.Cache // key + blessing -> error
}

func newCachedRoots(impl security.BlessingRoots, mu *sync.RWMutex) (*cachedRoots, error) {
	roots := &cachedRoots{mu: mu, impl: impl}
	roots.flush()
	if err := roots.fetchAndCacheRoots(); err != nil {
		return nil, err
	}
	return roots, nil
}

func (r *cachedRoots) Add(root []byte, pattern security.BlessingPattern) error {
	cachekey := string(root)

	defer r.mu.Unlock()
	r.mu.Lock()

	if err := r.impl.Add(root, pattern); err != nil {
		return err
	}

	if r.cache != nil {
		r.cache[cachekey] = append(r.cache[cachekey], pattern)
	}
	return nil
}

func (r *cachedRoots) Recognized(root []byte, blessing string) (result error) {
	cachekey := string(root)

	var cacheExists bool
	var err error
	r.mu.RLock()
	if r.cache != nil {
		err = r.recognizeFromCache(cachekey, root, blessing)
		cacheExists = true
	}
	r.mu.RUnlock()

	if !cacheExists {
		r.mu.Lock()
		if err := r.fetchAndCacheRoots(); err != nil {
			r.mu.Unlock()
			return err
		}
		err = r.recognizeFromCache(cachekey, root, blessing)
		r.mu.Unlock()
	}

	// TODO(ataly): Get rid of the following block once all agents have been updated
	// to support the 'Dump' method.
	r.mu.RLock()
	if !r.dumpExists && err != nil {
		negKey := cachekey + blessing
		negErr, ok := r.negative.Get(negKey)
		if !ok {
			r.mu.RUnlock()
			return r.recognizeFromImpl(root, blessing)
		}
		r.negative.Put(negKey, err)
		err = negErr.(error)
	}
	r.mu.RUnlock()

	return err
}

func (r *cachedRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	var (
		cacheExists bool
		dump        map[security.BlessingPattern][]security.PublicKey
	)

	r.mu.RLock()
	if r.cache != nil {
		cacheExists = true
		dump = r.dumpFromCache()
	}
	r.mu.RUnlock()

	if !cacheExists {
		r.mu.Lock()
		if err := r.fetchAndCacheRoots(); err != nil {
			logger.Global().Errorf("failed to cache roots: %v", err)
			r.mu.Unlock()
			return nil
		}
		dump = r.dumpFromCache()
		r.mu.Unlock()
	}
	return dump
}

func (r *cachedRoots) DebugString() string {
	return r.impl.DebugString()
}

// Must be called while holding mu.
func (r *cachedRoots) fetchAndCacheRoots() error {
	dump := r.impl.Dump()
	r.cache = make(map[string][]security.BlessingPattern)
	if dump == nil {
		r.dumpExists = false
		return nil
	}

	for p, pubkeys := range dump {
		for _, pubkey := range pubkeys {
			keybytes, err := pubkey.MarshalBinary()
			if err != nil {
				return err
			}
			cachekey := string(keybytes)
			r.cache[cachekey] = append(r.cache[cachekey], p)
		}
	}
	r.dumpExists = true
	return nil
}

// Must be called while holding mu.
func (r *cachedRoots) flush() {
	r.cache = nil
	r.negative = lru.New(maxNegativeCacheEntries)
}

// Must be called while holding mu.
func (r *cachedRoots) dumpFromCache() map[security.BlessingPattern][]security.PublicKey {
	if !r.dumpExists {
		return nil
	}
	dump := make(map[security.BlessingPattern][]security.PublicKey)
	for keyStr, patterns := range r.cache {
		pubkey, err := security.UnmarshalPublicKey([]byte(keyStr))
		if err != nil {
			logger.Global().Errorf("security.UnmarshalPublicKey(%v) returned error: %v", []byte(keyStr), err)
			return nil
		}
		for _, p := range patterns {
			dump[p] = append(dump[p], pubkey)
		}
	}
	return dump
}

// Must be called while holding mu.
func (r *cachedRoots) recognizeFromCache(cachekey string, root []byte, blessing string) error {
	for _, p := range r.cache[cachekey] {
		if p.MatchedBy(blessing) {
			return nil
		}
	}
	// Silly to do this unmarshaling work on an error. Change the error string?
	object, err := security.UnmarshalPublicKey(root)
	if err != nil {
		return err
	}
	return security.NewErrUnrecognizedRoot(nil, object.String(), nil)
}

// TODO(ataly): Get rid of this method once all agents have been updated
// to support the 'Dump' method.
func (r *cachedRoots) recognizeFromImpl(root []byte, blessing string) error {
	cachekey := string(root)
	err := r.impl.Recognized(root, blessing)

	r.mu.Lock()
	if err == nil {
		r.cache[cachekey] = append(r.cache[cachekey], security.BlessingPattern(blessing))
	} else {
		negKey := cachekey + blessing
		r.negative.Put(negKey, err)
	}
	r.mu.Unlock()
	return err
}

// cachedStore is a security.BlessingStore implementation that
// wraps over another implementation and adds caching.
type cachedStore struct {
	mu     *sync.RWMutex
	pubkey security.PublicKey
	def    security.Blessings
	defCh  chan struct{}
	peers  map[security.BlessingPattern]security.Blessings
	impl   security.BlessingStore
}

func (s *cachedStore) Default() (security.Blessings, <-chan struct{}) {
	s.mu.RLock()
	if s.defCh == nil {
		s.mu.RUnlock()
		return s.fetchAndCacheDefault()
	}
	b := s.def
	c := s.defCh
	s.mu.RUnlock()
	return b, c
}

func (s *cachedStore) SetDefault(blessings security.Blessings) error {
	defer s.mu.Unlock()
	s.mu.Lock()
	err := s.impl.SetDefault(blessings)
	if s.defCh != nil {
		close(s.defCh)
		s.defCh = nil
	}
	return err
}

func (s *cachedStore) fetchAndCacheDefault() (security.Blessings, <-chan struct{}) {
	defCh := make(chan struct{})
	s.mu.Lock()
	result, _ := s.impl.Default()
	s.def = result
	s.defCh = defCh
	s.mu.Unlock()
	return s.def, s.defCh
}

func (s *cachedStore) ForPeer(peerBlessings ...string) security.Blessings {
	var ret security.Blessings
	for pat, b := range s.PeerBlessings() {
		if pat.MatchedBy(peerBlessings...) {
			if union, err := security.UnionOfBlessings(ret, b); err == nil {
				ret = union
			} else {
				logger.Global().Errorf("UnionOfBlessings(%v, %v) failed: %v, dropping the latter from BlessingStore.ForPeers(%v)", ret, b, err, peerBlessings)
			}
		}
	}
	return ret
}

func (s *cachedStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	s.mu.RLock()
	ret := s.peers
	s.mu.RUnlock()
	if ret != nil {
		return ret
	}
	return s.fetchAndCacheBlessings()
}

func (s *cachedStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	defer s.mu.Unlock()
	s.mu.Lock()
	oldBlessings, err := s.impl.Set(blessings, forPeers)
	if err == nil && s.peers != nil {
		s.peers[forPeers] = blessings
	}
	return oldBlessings, err
}

func (s *cachedStore) fetchAndCacheBlessings() map[security.BlessingPattern]security.Blessings {
	ret := s.impl.PeerBlessings()
	s.mu.Lock()
	s.peers = ret
	s.mu.Unlock()
	return ret
}

func (s *cachedStore) PublicKey() security.PublicKey {
	return s.pubkey
}

func (s *cachedStore) DebugString() string {
	return s.impl.DebugString()
}

func (s *cachedStore) String() string {
	return fmt.Sprintf("cached[%s]", s.impl)
}

func (s *cachedStore) CacheDischarge(d security.Discharge, c security.Caveat, i security.DischargeImpetus) {
	s.mu.Lock()
	s.impl.CacheDischarge(d, c, i)
	s.mu.Unlock()
}

func (s *cachedStore) ClearDischarges(discharges ...security.Discharge) {
	s.mu.Lock()
	s.impl.ClearDischarges(discharges...)
	s.mu.Unlock()
}

func (s *cachedStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	defer s.mu.Unlock()
	s.mu.Lock()
	return s.impl.Discharge(caveat, impetus)
}

// Must be called while holding mu.
func (s *cachedStore) flush() {
	if s.defCh != nil {
		close(s.defCh)
		s.defCh = nil
	}
	s.peers = nil
}

// cachedPrincipal is a security.Principal implementation that
// wraps over another implementation and adds caching.
type cachedPrincipal struct {
	cache security.Principal
	/* impl */ agent.Principal
}

func (p *cachedPrincipal) BlessingStore() security.BlessingStore {
	return p.cache.BlessingStore()
}

func (p *cachedPrincipal) Roots() security.BlessingRoots {
	return p.cache.Roots()
}

func (p *cachedPrincipal) Close() error {
	return p.Principal.Close()
}

type dummySigner struct {
	pubkey security.PublicKey
}

func (s dummySigner) Sign(purpose, message []byte) (security.Signature, error) {
	var sig security.Signature
	return sig, verror.New(errNotImplemented, nil)
}

func (s dummySigner) PublicKey() security.PublicKey {
	return s.pubkey
}

func NewCachedPrincipalX(impl agent.Principal) (p agent.Principal, flush func(), err error) {
	var mu sync.RWMutex
	cachedRoots, err := newCachedRoots(impl.Roots(), &mu)
	if err != nil {
		return
	}
	cachedStore := &cachedStore{
		mu:     &mu,
		pubkey: impl.PublicKey(),
		impl:   impl.BlessingStore(),
	}
	flush = func() {
		defer mu.Unlock()
		mu.Lock()
		cachedRoots.flush()
		cachedStore.flush()
	}
	sp, err := security.CreatePrincipal(dummySigner{impl.PublicKey()}, cachedStore, cachedRoots)
	if err != nil {
		return
	}

	p = &cachedPrincipal{sp, impl}
	return
}
