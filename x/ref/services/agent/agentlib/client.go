// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package agentlib provides ways to create Principals that are backed by the
// security agent.  It implements a client for communicating with an agent
// process holding the private key for a Principal.  It also provides a way to
// start an agent for a Principal serialized to disk.
package agentlib

import (
	"fmt"
	"io"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/services/agent"
	"v.io/x/ref/services/agent/internal/cache"
	"v.io/x/ref/services/agent/internal/ipc"
)

const (
	pkgPath               = "v.io/x/ref/services/agent/agentlib"
	defaultConnectTimeout = 10 * time.Second
)

// Errors
var (
	errInvalidProtocol = verror.Register(pkgPath+".errInvalidProtocol",
		verror.NoRetry, "{1:}{2:} invalid agent protocol {3}")
)

type client struct {
	caller caller
	key    security.PublicKey
	// TODO(mattr):  At some point we should remove this backward
	// compatibility mechanism, once all users are updated.
	noCacheTimes bool
}

type caller interface {
	call(name string, results []interface{}, args ...interface{}) error
	io.Closer
}

type ipcCaller struct {
	conn  *ipc.IPCConn
	flush func()
	mu    sync.Mutex
}

func (i *ipcCaller) call(name string, results []interface{}, args ...interface{}) error {
	return i.conn.Call(name, args, results...)
}

func (i *ipcCaller) Close() error {
	i.conn.Close()
	return nil
}

func (i *ipcCaller) FlushAllCaches() error {
	var flush func()
	i.mu.Lock()
	flush = i.flush
	i.mu.Unlock()
	if flush != nil {
		flush()
	}
	return nil
}

func results(inputs ...interface{}) []interface{} {
	return inputs
}

func newUncachedPrincipalX(path string, timeout time.Duration) (*client, error) {
	caller := new(ipcCaller)
	i := ipc.NewIPC()
	i.Serve(caller)
	conn, err := i.Connect(path, timeout)
	if err != nil {
		return nil, err
	}
	caller.conn = conn
	agent := &client{caller: caller}
	if err := agent.fetchPublicKey(); err != nil {
		return nil, err
	}
	var dis security.Discharge
	var cacheTime time.Time
	if err := caller.call("BlessingStoreDischarge2", results(&dis, &cacheTime), security.Caveat{}, security.DischargeImpetus{}); err != nil {
		// If we can't fetch a discharge with two results, then we should fall back
		// to the old one result version.
		agent.noCacheTimes = true
	}
	return agent, nil
}

// NewAgentPrincipal returns a security.Pricipal using the PrivateKey held in a
// remote agent process.
//
// 'path' is the path to the agent socket, typically obtained from
// os.GetEnv(envvar.AgentAddress).
//
// 'timeout' specifies how long to retry connecting to the socket if it's not
// ready.
//
// The caller should call Close on the returned Principal once it's no longer
// used, in order to free up resources.
func NewAgentPrincipal(path string, timeout time.Duration) (agent.Principal, error) {
	p, err := newUncachedPrincipalX(path, timeout)
	if err != nil {
		return nil, err
	}
	cached, flush, err := cache.NewCachedPrincipalX(p)
	if err != nil {
		return nil, err
	}
	caller := p.caller.(*ipcCaller)
	caller.mu.Lock()
	caller.flush = flush
	caller.mu.Unlock()
	return cached, nil
}

// TODO(caprita): Deprecate.

// NewAgentPrincipalX returns a security.Pricipal using the PrivateKey held in a
// remote agent process.
//
// 'path' is the path to the agent socket, typically obtained from
// os.GetEnv(envvar.AgentAddress).  If the socket is not ready,
// NewAgentPrincipalX retries for a minute before giving up.
//
// The caller should call Close on the returned Principal once it's no longer
// used, in order to free up resources.
func NewAgentPrincipalX(path string) (agent.Principal, error) {
	return NewAgentPrincipal(path, defaultConnectTimeout)
}

func (c *client) Close() error {
	return c.caller.Close()
}

func (c *client) fetchPublicKey() (err error) {
	var b []byte
	if err = c.caller.call("PublicKey", results(&b)); err != nil {
		return
	}
	c.key, err = security.UnmarshalPublicKey(b)
	return
}

func (c *client) Bless(key security.PublicKey, with security.Blessings, extension string, caveat security.Caveat, additionalCaveats ...security.Caveat) (security.Blessings, error) {
	var blessings security.Blessings
	marshalledKey, err := key.MarshalBinary()
	if err != nil {
		return security.Blessings{}, err
	}
	err = c.caller.call("Bless", results(&blessings), marshalledKey, with, extension, caveat, additionalCaveats)
	return blessings, err
}

func (c *client) BlessSelf(name string, caveats ...security.Caveat) (security.Blessings, error) {
	var blessings security.Blessings
	err := c.caller.call("BlessSelf", results(&blessings), name, caveats)
	return blessings, err
}

func (c *client) Sign(message []byte) (sig security.Signature, err error) {
	err = c.caller.call("Sign", results(&sig), message)
	return
}

func (c *client) MintDischarge(forCaveat, caveatOnDischarge security.Caveat, additionalCaveatsOnDischarge ...security.Caveat) (security.Discharge, error) {
	var discharge security.Discharge
	if err := c.caller.call("MintDischarge", results(&discharge), forCaveat, caveatOnDischarge, additionalCaveatsOnDischarge); err != nil {
		return security.Discharge{}, err
	}
	return discharge, nil
}

func (c *client) PublicKey() security.PublicKey {
	return c.key
}

func (c *client) BlessingStore() security.BlessingStore {
	closedCh := make(chan struct{})
	close(closedCh)
	return &blessingStore{caller: c.caller, key: c.key, noCacheTimes: c.noCacheTimes, closedCh: closedCh}
}

func (c *client) Roots() security.BlessingRoots {
	return &blessingRoots{c.caller}
}

type blessingStore struct {
	caller       caller
	key          security.PublicKey
	noCacheTimes bool
	closedCh     chan struct{}
}

func (b *blessingStore) Set(blessings security.Blessings, forPeers security.BlessingPattern) (security.Blessings, error) {
	var previous security.Blessings
	err := b.caller.call("BlessingStoreSet", results(&previous), blessings, forPeers)
	return previous, err
}

func (b *blessingStore) ForPeer(peerBlessings ...string) security.Blessings {
	var blessings security.Blessings
	if err := b.caller.call("BlessingStoreForPeer", results(&blessings), peerBlessings); err != nil {
		logger.Global().Infof("error calling BlessingStorePeerBlessings: %v", err)
	}
	return blessings
}

func (b *blessingStore) SetDefault(blessings security.Blessings) error {
	return b.caller.call("BlessingStoreSetDefault", results(), blessings)
}

func (b *blessingStore) Default() (security.Blessings, <-chan struct{}) {
	var blessings security.Blessings
	err := b.caller.call("BlessingStoreDefault", results(&blessings))
	if err != nil {
		logger.Global().Infof("error calling BlessingStoreDefault: %v", err)
		return security.Blessings{}, b.closedCh
	}
	// In practice, this agent based blessing store is always cached and
	// thus this retuned channel isn't used. However, in order to be
	// conservative, return closed channel in the hopes that anyone relying
	// on this being accurate will find this out more easily (for example,
	// constantly refreshing the Default because the returned channel is
	// closed) that if we silently hid the notifications by returning a
	// channel that would never be closed.
	return blessings, b.closedCh
}

func (b *blessingStore) PublicKey() security.PublicKey {
	return b.key
}

func (b *blessingStore) PeerBlessings() map[security.BlessingPattern]security.Blessings {
	var bmap map[security.BlessingPattern]security.Blessings
	err := b.caller.call("BlessingStorePeerBlessings", results(&bmap))
	if err != nil {
		logger.Global().Infof("error calling BlessingStorePeerBlessings: %v", err)
		return nil
	}
	return bmap
}

func (b *blessingStore) DebugString() (s string) {
	err := b.caller.call("BlessingStoreDebugString", results(&s))
	if err != nil {
		s = fmt.Sprintf("error calling BlessingStoreDebugString: %v", err)
		logger.Global().Infof(s)
	}
	return
}

func (b *blessingStore) CacheDischarge(d security.Discharge, c security.Caveat, i security.DischargeImpetus) {
	err := b.caller.call("BlessingStoreCacheDischarge", results(), d, c, i)
	if err != nil {
		logger.Global().Infof("error calling BlessingStoreCacheDischarge: %v", err)
	}
}

func (b *blessingStore) ClearDischarges(discharges ...security.Discharge) {
	err := b.caller.call("BlessingStoreClearDischarges", results(), discharges)
	if err != nil {
		logger.Global().Infof("error calling BlessingStoreClearDischarges: %v", err)
	}
}

func (b *blessingStore) Discharge(caveat security.Caveat, impetus security.DischargeImpetus) (out security.Discharge, cacheTime time.Time) {
	res := []interface{}{&out}
	method := "BlessingStoreDischarge"
	if !b.noCacheTimes {
		res = append(res, &cacheTime)
		method = "BlessingStoreDischarge2"
	}
	err := b.caller.call(method, res, caveat, impetus)
	if err != nil {
		logger.Global().Infof("error calling BlessingStoreDischarge: %v", err)
	}
	return
}

type blessingRoots struct {
	caller caller
}

func (b *blessingRoots) Add(root []byte, pattern security.BlessingPattern) error {
	return b.caller.call("BlessingRootsAdd", results(), root, pattern)
}

func (b *blessingRoots) Recognized(root []byte, blessing string) error {
	return b.caller.call("BlessingRootsRecognized", results(), root, blessing)
}

func (b *blessingRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	var marshaledRoots map[security.BlessingPattern][][]byte
	if err := b.caller.call("BlessingRootsDump", results(&marshaledRoots)); err != nil {
		logger.Global().Infof("error calling BlessingRootsDump: %v", err)
		return nil
	}
	ret := make(map[security.BlessingPattern][]security.PublicKey)
	for p, marshaledKeys := range marshaledRoots {
		for _, marshaledKey := range marshaledKeys {
			key, err := security.UnmarshalPublicKey(marshaledKey)
			if err != nil {
				logger.Global().Infof("security.UnmarshalPublicKey(%v) returned error: %v", marshaledKey, err)
				continue
			}
			ret[p] = append(ret[p], key)
		}
	}
	return ret
}

func (b *blessingRoots) DebugString() (s string) {
	err := b.caller.call("BlessingRootsDebugString", results(&s))
	if err != nil {
		s = fmt.Sprintf("error calling BlessingRootsDebugString: %v", err)
		logger.Global().Infof(s)
	}
	return
}
