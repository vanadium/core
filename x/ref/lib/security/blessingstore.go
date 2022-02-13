// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

// TODO(ashankar,ataly): This file is a bit of a mess!! Define a serialization
// format for the blessing store and rewrite this file before release!

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/security/serialization"
)

const cacheKeyFormat = uint32(1)

// blessingStore implements security.BlessingStore.
type blessingStore struct {
	ctx       context.Context
	publicKey security.PublicKey
	mu        sync.RWMutex
	state     blessingStoreState // GUARDED_BY(mu)
	defCh     chan struct{}      // GUARDED_BY(mu) - Notifications for changes in the default blessings
}

func (bs *blessingStore) signalChange() {
	ch := bs.defCh
	defer close(ch)
	bs.defCh = make(chan struct{})
}

func (bs *blessingStore) setLocked(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, func(), error) {
	if !forPeers.IsValid() {
		return security.Blessings{}, nil, fmt.Errorf("%v is an invalid BlessingPattern", forPeers)
	}
	if !blessings.IsZero() && !reflect.DeepEqual(blessings.PublicKey(), bs.publicKey) {
		return security.Blessings{}, nil, fmt.Errorf("blessing's public key does not match store's public key")
	}
	old, hadold := bs.state.PeerBlessings[forPeers]
	if !blessings.IsZero() {
		bs.state.PeerBlessings[forPeers] = blessings
	} else {
		delete(bs.state.PeerBlessings, forPeers)
	}
	var undo func()
	if hadold {
		undo = func() { bs.state.PeerBlessings[forPeers] = old }
	} else {
		undo = func() { delete(bs.state.PeerBlessings, forPeers) }
	}
	return old, undo, nil
}

func (bs *blessingStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	old, _, err := bs.setLocked(blessings, forPeers)
	if err != nil {
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

func (bs *blessingStore) setDefaultLocked(blessings security.Blessings) (func(), error) {
	if !blessings.IsZero() && !reflect.DeepEqual(blessings.PublicKey(), bs.publicKey) {
		return nil, fmt.Errorf("blessing's public key does not match store's public key")
	}
	oldDefault := bs.state.DefaultBlessings
	undo := func() { bs.state.DefaultBlessings = oldDefault }
	bs.state.DefaultBlessings = blessings
	return undo, nil
}

func (bs *blessingStore) SetDefault(blessings security.Blessings) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if _, err := bs.setDefaultLocked(blessings); err != nil {
		return err
	}
	bs.signalChange()
	return nil
}

func (bs *blessingStore) PublicKey() security.PublicKey {
	return bs.publicKey
}

func (bs *blessingStore) String() string {
	return fmt.Sprintf("{state: %v, publicKey: %v}", bs.state, bs.publicKey)
}

func (bs *blessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	m := make(map[security.BlessingPattern]security.Blessings)
	for pattern, b := range bs.state.PeerBlessings {
		m[pattern] = b
	}
	return m
}

func (bs *blessingStore) cacheDischargeLocked(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) func() {
	id := discharge.ID()
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if id == "" || !cacheable {
		return func() {}
	}
	old, hadold := bs.state.Discharges[key]
	bs.state.Discharges[key] = CachedDischarge{
		Discharge: discharge,
		CacheTime: time.Now(),
	}
	var undo func()
	if hadold {
		undo = func() { bs.state.Discharges[key] = old }
	} else {
		undo = func() { delete(bs.state.Discharges, key) }
	}
	return undo
}

func (bs *blessingStore) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.cacheDischargeLocked(discharge, caveat, impetus)
	return nil
}

func (bs *blessingStore) clearDischargesLocked(discharges []security.Discharge) func() {
	deleted := map[dischargeCacheKey]CachedDischarge{}
	for _, d := range discharges {
		for k, cached := range bs.state.Discharges {
			if cached.Discharge.Equivalent(d) {
				deleted[k] = cached
				delete(bs.state.Discharges, k)
			}
		}
	}
	return func() {
		for k, v := range deleted {
			bs.state.Discharges[k] = v
		}
	}
}

func (bs *blessingStore) ClearDischarges(discharges ...security.Discharge) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.clearDischargesLocked(discharges)
}

func (bs *blessingStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	defer bs.mu.Unlock()
	bs.mu.Lock()
	discharge, when, _ := bs.dischargeFromCacheLocked(caveat, impetus)
	return discharge, when
}

func (bs *blessingStore) dischargeFromCacheLocked(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time, func()) {
	undo := func() {}
	key, cacheable := dcacheKey(caveat.ThirdPartyDetails(), impetus)
	if !cacheable {
		return security.Discharge{}, time.Time{}, undo
	}
	cached, exists := bs.state.Discharges[key]
	if !exists {
		return security.Discharge{}, time.Time{}, undo
	}
	if expiry := cached.Discharge.Expiry(); expiry.IsZero() || expiry.After(time.Now()) {
		return cached.Discharge, cached.CacheTime, undo
	}
	old := bs.state.Discharges[key]
	undo = func() {
		bs.state.Discharges[key] = old
	}
	delete(bs.state.Discharges, key)
	return security.Discharge{}, time.Time{}, undo
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
	h.Write(hashString(id))      //nolint:errcheck
	h.Write(hashString(method))  //nolint:errcheck
	h.Write(hashString(servers)) //nolint:errcheck
	copy(key[:], h.Sum(nil))
	return key, true
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
	bs.mu.Lock()
	defer bs.mu.Unlock()
	const format = "%-30s   %s\n"
	buff := bytes.NewBufferString(fmt.Sprintf(format, "Default Blessings", bs.state.DefaultBlessings))

	buff.WriteString(fmt.Sprintf(format, "Peer pattern", "Blessings"))
	writePattern := func(pattern security.BlessingPattern) {
		buff.WriteString(fmt.Sprintf(format, pattern, bs.state.PeerBlessings[pattern]))
	}
	sorted := make([]string, 0, len(bs.state.PeerBlessings))
	for k := range bs.state.PeerBlessings {
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

func newBlessingStoreState() blessingStoreState {
	return blessingStoreState{
		PeerBlessings: make(map[security.BlessingPattern]security.Blessings),
		Discharges:    make(map[dischargeCacheKey]CachedDischarge),
	}
}

// NewBlessingStore returns an in-memory security.BlessingStore for a
// principal with the provided PublicKey.
//
// The returned BlessingStore is initialized with an empty set of blessings.
func NewBlessingStore(publicKey security.PublicKey) security.BlessingStore {
	return newBlessingStore(context.TODO(), publicKey)
}

func newBlessingStore(ctx context.Context, publicKey security.PublicKey) security.BlessingStore {
	return &blessingStore{
		ctx:       ctx,
		publicKey: publicKey,
		state:     newBlessingStoreState(),
		defCh:     make(chan struct{}),
	}
}

// NewBlessingStore returns an implementation of security.BlessingStore
// according to the supplied options.
// If no options are supplied all state is kept in memory.
func NewBlessingStoreOpts(ctx context.Context, publicKey security.PublicKey, opts ...CredentialsStoreOption) (security.BlessingStore, error) {
	var o credentialsStoreOption
	for _, fn := range opts {
		fn(&o)
	}
	if o.reader == nil && o.writer == nil {
		return newBlessingStore(ctx, publicKey), nil
	}
	if o.writer != nil {
		return newWritableBlessingStore(ctx, o)
	}
	return newReadonlyBlessingStore(ctx, o)
}

func verifyState(publicKey security.PublicKey, state blessingStoreState) error {
	for _, b := range state.PeerBlessings {
		if !reflect.DeepEqual(b.PublicKey(), publicKey) {
			return fmt.Errorf("read Blessings: %v that are not for provided PublicKey: %v", b, publicKey)
		}
	}
	if !state.DefaultBlessings.IsZero() && !reflect.DeepEqual(state.DefaultBlessings.PublicKey(), publicKey) {
		return fmt.Errorf("read Blessings: %v that are not for provided PublicKey: %v", state.DefaultBlessings, publicKey)
	}
	return nil
}

type blessingStoreReader struct {
	blessingStore
	interval time.Duration
}

func newBlessingStoreReader(ctx context.Context, interval time.Duration, key security.PublicKey) blessingStoreReader {
	return blessingStoreReader{
		blessingStore: blessingStore{
			ctx:       ctx,
			publicKey: key,
			state:     newBlessingStoreState(),
			defCh:     make(chan struct{}),
		},
		interval: interval,
	}
}

func (bs *blessingStoreReader) loadLocked(ctx context.Context, reader CredentialsStoreReader) error {
	rd, err := reader.BlessingsReader(ctx)
	if err != nil {
		return err
	}
	data, signature, err := rd.Readers()
	if err != nil {
		return err
	}
	if data == nil && signature == nil {
		return nil
	}
	state := newBlessingStoreState()
	if err := decodeFromStorage(&state, data, signature, bs.publicKey); err != nil {
		return err
	}
	if state.CacheKeyFormat != cacheKeyFormat {
		state.CacheKeyFormat = cacheKeyFormat
		state.Discharges = make(map[dischargeCacheKey]CachedDischarge)
	} else if len(state.DischargeCache) > 0 {
		// If the old DischargeCache field is present, upgrade to the new field.
		for k, v := range state.DischargeCache {
			state.Discharges[k] = CachedDischarge{
				Discharge: v,
			}
		}
		state.DischargeCache = nil
	}
	if state.PeerBlessings == nil {
		state.PeerBlessings = make(map[security.BlessingPattern]security.Blessings)
	}
	if state.Discharges == nil {
		state.Discharges = make(map[dischargeCacheKey]CachedDischarge)
	}
	if err := verifyState(bs.publicKey, state); err != nil {
		return err
	}
	if !state.DefaultBlessings.Equivalent(bs.state.DefaultBlessings) {
		bs.signalChange()
	}
	bs.state = state
	return nil
}

func (bs *blessingStoreReader) load(ctx context.Context, reader CredentialsStoreReader) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	unlock, err := reader.RLock(ctx, LockBlessingStore)
	if err != nil {
		return err
	}
	defer unlock()
	return bs.loadLocked(ctx, reader)
}

func (bs *blessingStoreReader) refresh(ctx context.Context, store CredentialsStoreReader) error {
	if err := bs.load(ctx, store); err != nil {
		return err
	}
	if bs.interval == 0 {
		return nil
	}
	handleRefresh(ctx, bs.interval, func() error {
		return bs.load(ctx, store)
	})
	return nil
}

type blessingStoreWriteable struct {
	blessingStoreReader
	store  CredentialsStoreReadWriter
	signer serialization.Signer
}

func (bs *blessingStoreWriteable) reload() (func(), error) {
	bs.mu.Lock()
	unlock, err := bs.store.Lock(bs.ctx, LockBlessingStore)
	if err != nil {
		return func() { bs.mu.Unlock() }, err
	}

	unlockfn := func() {
		unlock()
		bs.mu.Unlock()
	}

	return unlockfn, bs.loadLocked(bs.ctx, bs.store)
}

func (bs *blessingStoreWriteable) saveLocked(ctx context.Context) error {
	wr, err := bs.store.BlessingsWriter(ctx)
	if err != nil {
		return err
	}
	data, signature, err := wr.Writers()
	if err != nil {
		return err
	}
	return encodeAndStore(bs.state, data, signature, bs.signer)
}

func (bs *blessingStoreWriteable) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	unlock, err := bs.reload()
	if err != nil {
		return security.Blessings{}, err
	}
	defer unlock()

	old, undo, err := bs.setLocked(blessings, forPeers)
	if err != nil {
		return security.Blessings{}, err
	}
	if err := bs.saveLocked(bs.ctx); err != nil {
		undo()
		return security.Blessings{}, err
	}
	return old, nil
}

func (bs *blessingStoreWriteable) SetDefault(blessings security.Blessings) error {
	unlock, err := bs.reload()
	if err != nil {
		return err
	}
	defer unlock()

	undo, err := bs.setDefaultLocked(blessings)
	if err != nil {
		return err
	}
	if err := bs.saveLocked(bs.ctx); err != nil {
		undo()
		return err
	}
	bs.signalChange()
	return nil
}

func (bs *blessingStoreWriteable) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	unlock, err := bs.reload()
	if err != nil {
		return security.Discharge{}, time.Time{}
	}
	defer unlock()

	discharge, when, undo := bs.dischargeFromCacheLocked(caveat, impetus)
	if err := bs.saveLocked(bs.ctx); err != nil {
		undo()
		return discharge, when
	}
	return discharge, when
}

func (bs *blessingStoreWriteable) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) error {
	unlock, err := bs.reload()
	if err != nil {
		return err
	}
	defer unlock()

	undo := bs.cacheDischargeLocked(discharge, caveat, impetus)
	if err := bs.saveLocked(bs.ctx); err != nil {
		undo()
		return err
	}
	return nil
}

func (bs *blessingStoreWriteable) ClearDischarges(discharges ...security.Discharge) {
	unlock, err := bs.reload()
	if err != nil {
		return
	}
	defer unlock()
	undo := bs.clearDischargesLocked(discharges)
	if err := bs.saveLocked(bs.ctx); err != nil {
		undo()
		return
	}
}

func newWritableBlessingStore(ctx context.Context, opts credentialsStoreOption) (security.BlessingStore, error) {
	bs := &blessingStoreWriteable{
		blessingStoreReader: newBlessingStoreReader(ctx, opts.updateInterval, opts.publicKey),
		store:               opts.writer,
		signer:              opts.signer,
	}
	if err := bs.refresh(ctx, opts.writer); err != nil {
		return nil, err
	}
	return bs, nil
}

type blessingsStoreReadonly struct {
	blessingStoreReader
	store CredentialsStoreReader
}

func (bs *blessingsStoreReadonly) Add(root []byte, pattern security.BlessingPattern) error {
	return fmt.Errorf("Add is not implemented for readonly blessings roots")
}

func newReadonlyBlessingStore(ctx context.Context, opts credentialsStoreOption) (security.BlessingStore, error) {
	bs := &blessingsStoreReadonly{
		blessingStoreReader: newBlessingStoreReader(ctx, opts.updateInterval, opts.publicKey),
		store:               opts.reader,
	}
	if err := bs.refresh(ctx, opts.reader); err != nil {
		return nil, err
	}
	return bs, nil
}

func (bs *blessingsStoreReadonly) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	return security.Blessings{}, fmt.Errorf("Set is not implemented for readonly blessings store")
}

func (bs *blessingsStoreReadonly) SetDefault(blessings security.Blessings) error {
	return fmt.Errorf("SetDefault is not implemented for readonly blessings store")
}

func (bs *blessingsStoreReadonly) ClearDischarges(discharges ...security.Discharge) {
	vlog.Errorf("ClearDischarges is not implemented for readonly blessings store")
}

func (bs *blessingsStoreReadonly) CacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) error {
	return fmt.Errorf("CacheDischarge is not implemented for readonly blessings store")
}

func (bs *blessingsStoreReadonly) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time) {
	return security.Discharge{}, time.Time{}
}
