// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

// TODO(ashankar,ataly): This file is a bit of a mess!! Define a serialization
// format for the blessing store and rewrite this file before release!

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/security/serialization"
)

var (
	errStoreAddMismatch        = verror.Register(pkgPath+".errStoreAddMismatch", verror.NoRetry, "{1:}{2:} blessing's public key does not match store's public key{:_}")
	errBadBlessingPattern      = verror.Register(pkgPath+".errBadBlessingPattern", verror.NoRetry, "{1:}{2:} {3} is an invalid BlessingPattern{:_}")
	errBlessingsNotForKey      = verror.Register(pkgPath+".errBlessingsNotForKey", verror.NoRetry, "{1:}{2:} read Blessings: {3} that are not for provided PublicKey{:_}")
	errDataOrSignerUnspecified = verror.Register(pkgPath+".errDataOrSignerUnspecified", verror.NoRetry, "{1:}{2:} persisted data or signer is not specified{:_}")
)

const cacheKeyFormat = uint32(1)

// blessingStore implements security.BlessingStore.
type blessingStore struct {
	publicKey  security.PublicKey
	serializer SerializerReaderWriter
	signer     serialization.Signer
	mu         sync.RWMutex
	state      blessingStoreState // GUARDED_BY(mu)
	defCh      chan struct{}      // GUARDED_BY(mu) - Notifications for changes in the default blessings
}

func (bs *blessingStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	if !forPeers.IsValid() {
		return security.Blessings{}, verror.New(errBadBlessingPattern, nil, forPeers)
	}
	if !blessings.IsZero() && !reflect.DeepEqual(blessings.PublicKey(), bs.publicKey) {
		return security.Blessings{}, verror.New(errStoreAddMismatch, nil)
	}
	bs.mu.Lock()
	defer bs.mu.Unlock()
	old, hadold := bs.state.PeerBlessings[forPeers]
	if !blessings.IsZero() {
		bs.state.PeerBlessings[forPeers] = blessings
	} else {
		delete(bs.state.PeerBlessings, forPeers)
	}
	if err := bs.save(); err != nil {
		if hadold {
			bs.state.PeerBlessings[forPeers] = old
		} else {
			delete(bs.state.PeerBlessings, forPeers)
		}
		return security.Blessings{}, err
	}
	return old, nil
}

func (bs *blessingStore) ForPeer(peerBlessings ...string) security.Blessings {
	bs.mu.RLock()
	defer bs.mu.RUnlock()

	var ret security.Blessings
	for pattern, b := range bs.state.PeerBlessings {
		if pattern.MatchedBy(peerBlessings...) {
			if union, err := security.UnionOfBlessings(ret, b); err != nil {
				vlog.Errorf("UnionOfBlessings(%v, %v) failed: %v, dropping the latter from BlessingStore.ForPeers(%v)", ret, b, err, peerBlessings)
			} else {
				ret = union
			}
		}
	}
	return ret
}

func (bs *blessingStore) Default() (security.Blessings, <-chan struct{}) {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.state.DefaultBlessings, bs.defCh
}

func (bs *blessingStore) SetDefault(blessings security.Blessings) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if !blessings.IsZero() && !reflect.DeepEqual(blessings.PublicKey(), bs.publicKey) {
		return verror.New(errStoreAddMismatch, nil)
	}
	oldDefault := bs.state.DefaultBlessings
	bs.state.DefaultBlessings = blessings
	if err := bs.save(); err != nil {
		bs.state.DefaultBlessings = oldDefault
		return err
	}
	close(bs.defCh)
	bs.defCh = make(chan struct{})
	return nil
}

func (bs *blessingStore) PublicKey() security.PublicKey {
	return bs.publicKey
}

func (bs *blessingStore) String() string {
	return fmt.Sprintf("{state: %v, publicKey: %v}", bs.state, bs.publicKey)
}

func (bs *blessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	m := make(map[security.BlessingPattern]security.Blessings)
	for pattern, b := range bs.state.PeerBlessings {
		m[pattern] = b
	}
	return m
}

func (bs *blessingStore) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) {
	id := discharge.ID()
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if id == "" || !cacheable {
		return
	}
	bs.mu.Lock()
	defer bs.mu.Unlock()
	old, hadold := bs.state.Discharges[key]
	bs.state.Discharges[key] = CachedDischarge{
		Discharge: discharge,
		CacheTime: time.Now(),
	}
	if err := bs.save(); err != nil {
		if hadold {
			bs.state.Discharges[key] = old
		} else {
			delete(bs.state.Discharges, key)
		}
	}
	return
}

func (bs *blessingStore) ClearDischarges(discharges ...security.Discharge) {
	bs.mu.Lock()
	clearDischargesFromCache(bs.state.Discharges, discharges...)
	bs.mu.Unlock()
	return
}

func (bs *blessingStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if !cacheable {
		return security.Discharge{}, time.Time{}
	}
	defer bs.mu.Unlock()
	bs.mu.Lock()
	return dischargeFromCache(bs.state.Discharges, key)
}

func dischargeFromCache(dcache map[dischargeCacheKey]CachedDischarge, key dischargeCacheKey) (security.Discharge, time.Time) {
	cached, exists := dcache[key]
	if !exists {
		return security.Discharge{}, time.Time{}
	}
	if expiry := cached.Discharge.Expiry(); expiry.IsZero() || expiry.After(time.Now()) {
		return cached.Discharge, cached.CacheTime
	}
	delete(dcache, key)
	return security.Discharge{}, time.Time{}
}

func dcacheKey(tp security.ThirdPartyCaveat, impetus security.DischargeImpetus) (key dischargeCacheKey, cacheable bool) {
	// The cache key is based on the method and servers for impetus, it ignores arguments.
	// So fail if the caveat requires arguments.
	if tp == nil || tp.Requirements().ReportArguments {
		return key, false
	}
	// If the algorithm for computing dcacheKey changes, cacheKeyFormat must be changed as well.
	id := tp.ID()
	r := tp.Requirements()
	var method, servers string
	// We currently do not cache on impetus.Arguments because there it seems there is no
	// general way to generate a key from them.
	if r.ReportMethod {
		method = impetus.Method
	}
	if r.ReportServer && len(impetus.Server) > 0 {
		// Sort the server blessing patterns to increase cache usage.
		var bps []string
		for _, bp := range impetus.Server {
			bps = append(bps, string(bp))
		}
		sort.Strings(bps)
		servers = strings.Join(bps, ",")
	}
	h := sha256.New()
	h.Write(hashString(id))
	h.Write(hashString(method))
	h.Write(hashString(servers))
	copy(key[:], h.Sum(nil))
	return key, true
}

func clearDischargesFromCache(dcache map[dischargeCacheKey]CachedDischarge, discharges ...security.Discharge) {
	for _, d := range discharges {
		for k, cached := range dcache {
			if cached.Discharge.Equivalent(d) {
				delete(dcache, k)
			}
		}
	}
}

func hashString(d string) []byte {
	h := sha256.Sum256([]byte(d))
	return h[:]
}

// DebugString return a human-readable string encoding of the store
// in the following format
// Default Blessings <blessings>
// Peer pattern   Blessings
// <pattern>      <blessings>
// ...
// <pattern>      <blessings>
func (bs *blessingStore) DebugString() string {
	const format = "%-30s   %s\n"
	buff := bytes.NewBufferString(fmt.Sprintf(format, "Default Blessings", bs.state.DefaultBlessings))

	buff.WriteString(fmt.Sprintf(format, "Peer pattern", "Blessings"))
	writePattern := func(pattern security.BlessingPattern) {
		buff.WriteString(fmt.Sprintf(format, pattern, bs.state.PeerBlessings[security.BlessingPattern(pattern)]))
	}
	sorted := make([]string, 0, len(bs.state.PeerBlessings))
	for k, _ := range bs.state.PeerBlessings {
		if k == security.AllPrincipals {
			writePattern(k)
		} else {
			sorted = append(sorted, string(k))
		}
	}
	sort.Strings(sorted)
	for _, pattern := range sorted {
		writePattern(security.BlessingPattern(pattern))
	}
	return buff.String()
}

func (bs *blessingStore) save() error {
	if (bs.signer == nil) && (bs.serializer == nil) {
		return nil
	}
	data, signature, err := bs.serializer.Writers()
	if err != nil {
		return err
	}
	return encodeAndStore(bs.state, data, signature, bs.signer)
}

// newInMemoryBlessingStore returns an in-memory security.BlessingStore for a
// principal with the provided PublicKey.
//
// The returned BlessingStore is initialized with an empty set of blessings.
func newInMemoryBlessingStore(publicKey security.PublicKey) security.BlessingStore {
	return &blessingStore{
		publicKey: publicKey,
		state: blessingStoreState{
			PeerBlessings: make(map[security.BlessingPattern]security.Blessings),
			Discharges:    make(map[dischargeCacheKey]CachedDischarge),
		},
		defCh: make(chan struct{}),
	}
}

func (bs *blessingStore) verifyState() error {
	for _, b := range bs.state.PeerBlessings {
		if !reflect.DeepEqual(b.PublicKey(), bs.publicKey) {
			return verror.New(errBlessingsNotForKey, nil, b, bs.publicKey)
		}
	}
	if !bs.state.DefaultBlessings.IsZero() && !reflect.DeepEqual(bs.state.DefaultBlessings.PublicKey(), bs.publicKey) {
		return verror.New(errBlessingsNotForKey, nil, bs.state.DefaultBlessings, bs.publicKey)
	}
	return nil
}

func (bs *blessingStore) deserialize() error {
	data, signature, err := bs.serializer.Readers()
	if err != nil {
		return err
	}
	if data == nil && signature == nil {
		return nil
	}
	if err := decodeFromStorage(&bs.state, data, signature, bs.signer.PublicKey()); err != nil {
		return err
	}
	if bs.state.CacheKeyFormat != cacheKeyFormat {
		bs.state.CacheKeyFormat = cacheKeyFormat
		bs.state.Discharges = make(map[dischargeCacheKey]CachedDischarge)
	} else if len(bs.state.DischargeCache) > 0 {
		// If the old DischargeCache field is present, upgrade to the new field.
		for k, v := range bs.state.DischargeCache {
			bs.state.Discharges[k] = CachedDischarge{
				Discharge: v,
			}
		}
		bs.state.DischargeCache = nil
	}
	return bs.verifyState()
}

// newPersistingBlessingStore returns a security.BlessingStore for a principal
// that is initialized with the persisted data. The returned security.BlessingStore
// also persists any updates to its state.
func newPersistingBlessingStore(serializer SerializerReaderWriter, signer serialization.Signer) (security.BlessingStore, error) {
	if serializer == nil || signer == nil {
		return nil, verror.New(errDataOrSignerUnspecified, nil)
	}
	bs := &blessingStore{
		publicKey:  signer.PublicKey(),
		serializer: serializer,
		signer:     signer,
		defCh:      make(chan struct{}),
	}
	if err := bs.deserialize(); err != nil {
		return nil, err
	}
	if bs.state.PeerBlessings == nil {
		bs.state.PeerBlessings = make(map[security.BlessingPattern]security.Blessings)
	}
	if bs.state.Discharges == nil {
		bs.state.Discharges = make(map[dischargeCacheKey]CachedDischarge)
	}
	return bs, nil
}
