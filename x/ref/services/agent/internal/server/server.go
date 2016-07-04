// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/x/ref/services/agent/internal/ipc"
)

type agentd struct {
	ipc       *ipc.IPC
	principal security.Principal
	mu        sync.RWMutex
}

// ServeAgent registers the agent server with 'ipc'.
// It will respond to requests using 'principal'.
// Must be called before ipc.Listen or ipc.Connect.
func ServeAgent(i *ipc.IPC, principal security.Principal) (err error) {
	server := &agentd{ipc: i, principal: principal}
	return i.Serve(server)
}

func (a *agentd) Bless(key []byte, with security.Blessings, extension string, caveat security.Caveat, additionalCaveats []security.Caveat) (security.Blessings, error) {
	pkey, err := security.UnmarshalPublicKey(key)
	if err != nil {
		return security.Blessings{}, err
	}
	return a.principal.Bless(pkey, with, extension, caveat, additionalCaveats...)
}

func (a *agentd) BlessSelf(name string, caveats []security.Caveat) (security.Blessings, error) {
	return a.principal.BlessSelf(name, caveats...)
}

func (a *agentd) Sign(message []byte) (security.Signature, error) {
	return a.principal.Sign(message)
}

func (a *agentd) MintDischarge(forCaveat, caveatOnDischarge security.Caveat, additionalCaveatsOnDischarge []security.Caveat) (security.Discharge, error) {
	return a.principal.MintDischarge(forCaveat, caveatOnDischarge, additionalCaveatsOnDischarge...)
}

func (a *agentd) unlock() {
	a.mu.Unlock()
	for _, conn := range a.ipc.Connections() {
		go conn.Call("FlushAllCaches", nil)
	}
}

func (a *agentd) PublicKey() ([]byte, error) {
	return a.principal.PublicKey().MarshalBinary()
}

func (a *agentd) BlessingStoreSet(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	defer a.unlock()
	a.mu.Lock()
	return a.principal.BlessingStore().Set(blessings, forPeers)
}

func (a *agentd) BlessingStoreForPeer(peerBlessings []string) (security.Blessings, error) {
	defer a.mu.RUnlock()
	a.mu.RLock()
	return a.principal.BlessingStore().ForPeer(peerBlessings...), nil
}

func (a *agentd) BlessingStoreSetDefault(blessings security.Blessings) error {
	defer a.unlock()
	a.mu.Lock()
	return a.principal.BlessingStore().SetDefault(blessings)
}

func (a *agentd) BlessingStorePeerBlessings() (map[security.BlessingPattern]security.Blessings, error) {
	defer a.mu.RUnlock()
	a.mu.RLock()
	return a.principal.BlessingStore().PeerBlessings(), nil
}

func (a *agentd) BlessingStoreDebugString() (string, error) {
	defer a.mu.RUnlock()
	a.mu.RLock()
	return a.principal.BlessingStore().DebugString(), nil
}

func (a *agentd) BlessingStoreDefault() (security.Blessings, error) {
	defer a.mu.RUnlock()
	a.mu.RLock()
	b, _ := a.principal.BlessingStore().Default()
	return b, nil
}

func (a *agentd) BlessingStoreCacheDischarge(discharge security.Discharge, caveat security.Caveat, impetus security.DischargeImpetus) error {
	defer a.mu.Unlock()
	a.mu.Lock()
	a.principal.BlessingStore().CacheDischarge(discharge, caveat, impetus)
	return nil
}

func (a *agentd) BlessingStoreClearDischarges(discharges []security.Discharge) error {
	defer a.mu.Unlock()
	a.mu.Lock()
	a.principal.BlessingStore().ClearDischarges(discharges...)
	return nil
}

func (a *agentd) BlessingStoreDischarge(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, error) {
	defer a.mu.Unlock()
	a.mu.Lock()
	discharge, _ := a.principal.BlessingStore().Discharge(caveat, impetus)
	return discharge, nil
}

func (a *agentd) BlessingStoreDischarge2(caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, time.Time, error) {
	defer a.mu.Unlock()
	a.mu.Lock()
	discharge, cacheTime := a.principal.BlessingStore().Discharge(caveat, impetus)
	return discharge, cacheTime, nil
}

func (a *agentd) BlessingRootsAdd(root []byte, pattern security.BlessingPattern) error {
	defer a.unlock()
	a.mu.Lock()
	return a.principal.Roots().Add(root, pattern)
}

func (a *agentd) BlessingRootsRecognized(root []byte, blessing string) error {
	defer a.mu.RUnlock()
	a.mu.RLock()
	return a.principal.Roots().Recognized(root, blessing)
}

func (a *agentd) BlessingRootsDump() (map[security.BlessingPattern][][]byte, error) {
	ret := make(map[security.BlessingPattern][][]byte)
	defer a.mu.RUnlock()
	a.mu.RLock()
	for p, keys := range a.principal.Roots().Dump() {
		for _, key := range keys {
			marshaledKey, err := key.MarshalBinary()
			if err != nil {
				return nil, err
			}
			ret[p] = append(ret[p], marshaledKey)
		}
	}
	return ret, nil
}

func (a *agentd) BlessingRootsDebugString() (string, error) {
	defer a.mu.RUnlock()
	a.mu.RLock()
	return a.principal.Roots().DebugString(), nil
}
