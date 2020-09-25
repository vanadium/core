// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/nsync"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/flow/conn"
)

const maxErrorAge = time.Minute * 5

// ConnCache is a cache from (protocol, address) and (routingID) to a set of Conns.
// Multiple goroutines may invoke methods on the ConnCache simultaneously.
type ConnCache struct {
	mu   nsync.Mu
	cond nsync.CV

	conns    map[CachedConn]*connEntry
	cache    map[interface{}][]*connEntry
	errors   map[interface{}]dialError
	reserved map[interface{}]*Reservation

	idleExpiry time.Duration
}

type connEntry struct {
	conn  CachedConn
	rid   naming.RoutingID
	proxy bool
	// cancel is a context.CancelFunc that corresponds to the context
	// used to dial the connection.  Since connections live longer than the
	// RPC calls which precipiated their being dialed, we have to use
	// context.WithRootCancel to make a context to dial.  This means we need
	// to cancel that context at some point when the connection is no longer
	// needed.  In our case that's when we eject the context from the cache.
	cancel context.CancelFunc
	keys   []interface{}
}

type dialError struct {
	err  error
	when time.Time
}

// Reservation represents the right to dial a connection.  We only
// hand out one reservation for a given connection at a time.
type Reservation struct {
	cache        *ConnCache
	ctx          *context.T
	cancel       context.CancelFunc
	keys         []interface{}
	remote       naming.Endpoint
	waitForProxy bool
}

// Context returns the context that should be used to dial the new connection.
func (r *Reservation) Context() *context.T {
	return r.ctx
}

// ProxyConn returns a connection to a relevant proxy if it exists.  Otherwise
// it returns nil and the reservation holder should dial the proxy if necessary.
func (r *Reservation) ProxyConn() CachedConn {
	if !r.waitForProxy {
		return nil
	}
	keys := []interface{}{key(r.remote.Protocol, r.remote.Address)}
	// We ignore the error here.  The worst thing that can happen is we try
	// to dial the proxy again.
	conn, _, _, _ := r.cache.internalFind(r.ctx, r.remote, keys, nil, true)
	return conn
}

// Unreserve removes this reservation, and broadcasts waiting threads to
// continue with their halted Find call.
func (r *Reservation) Unreserve(conn, proxyConn CachedConn, err error) error {
	c := r.cache

	defer c.mu.Unlock()
	c.mu.Lock()

	if c.conns == nil {
		r.cancel()
		return errCacheClosed.Errorf(r.ctx, "cache is closed")
	}

	for _, k := range r.keys {
		delete(c.reserved, k)
	}

	if proxyConn != nil {
		if c.insertConnLocked(r.remote, proxyConn, true, true, r.cancel) {
			r.cancel = nil
		}
	}

	if conn != nil {
		c.insertConnLocked(r.remote, conn, proxyConn != nil, proxyConn == nil, r.cancel)
		r.cancel = nil
	} else if err != nil {
		e := dialError{
			err:  err,
			when: time.Now(),
		}
		c.errors[key(r.remote.Protocol, r.remote.Address)] = e
		c.errors[r.remote.RoutingID] = e
	}

	if r.cancel != nil {
		r.cancel()
	}

	c.cond.Broadcast()
	return nil
}

// CachedConn is the interface implemented by *conn.Conn that is used by ConnCache.
// We make the ConnCache API take this interface to make testing easier.
type CachedConn interface {
	Status() conn.Status
	IsEncapsulated() bool
	IsIdle(*context.T, time.Duration) bool
	EnterLameDuck(*context.T) chan struct{}
	RemoteLameDuck() bool
	CloseIfIdle(*context.T, time.Duration) bool
	Close(*context.T, error)
	RemoteEndpoint() naming.Endpoint
	LocalEndpoint() naming.Endpoint
	RemoteBlessings() security.Blessings
	RemoteDischarges() map[string]security.Discharge
	RTT() time.Duration
	LastUsed() time.Time
	DebugString() string
}

// NewConnCache creates a ConnCache with an idleExpiry for connections.
// If idleExpiry is zero, connections will never expire.
func NewConnCache(idleExpiry time.Duration) *ConnCache {
	return &ConnCache{
		conns:      make(map[CachedConn]*connEntry),
		cache:      make(map[interface{}][]*connEntry),
		errors:     make(map[interface{}]dialError),
		reserved:   make(map[interface{}]*Reservation),
		idleExpiry: idleExpiry,
	}
}

// Insert adds conn to the cache, keyed by both (protocol, address) and (routingID).
// An error will be returned iff the cache has been closed.
func (c *ConnCache) Insert(conn CachedConn, proxy bool) error {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.conns == nil {
		return errCacheClosed.Errorf(nil, "cache is closed")
	}
	c.insertConnLocked(conn.RemoteEndpoint(), conn, proxy, true, nil)
	return nil
}

// InsertWithRoutingID adds conn to the cache keyed only by conn's RoutingID.
func (c *ConnCache) InsertWithRoutingID(conn CachedConn, proxy bool) error {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.conns == nil {
		return errCacheClosed.Errorf(nil, "cache is closed")
	}
	c.insertConnLocked(conn.RemoteEndpoint(), conn, proxy, false, nil)
	return nil
}

// Find returns a Conn based on the input remoteEndpoint.
// nil is returned if there is no such Conn.
//
// Find calls will block if the desired connections are currently being dialed.
// Find will return immediately if the given context is canceled.
func (c *ConnCache) Find(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer,
) (conn CachedConn, names []string, rejected []security.RejectedBlessing, err error) {
	var keys []interface{}
	if keys, conn, names, rejected, _ = c.internalFindCached(ctx, remote, auth); conn != nil {
		return conn, names, rejected, nil
	}
	// Finally try waiting for any outstanding dials to complete.
	nc, n, r, err := c.internalFind(ctx, remote, keys, auth, true)
	return nc, n, r, err
}

// FindCached returns a Conn only if it's already in the cache.
func (c *ConnCache) FindCached(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer) (conn CachedConn, names []string, rejected []security.RejectedBlessing, err error) {
	_, conn, names, rejected, err = c.internalFindCached(ctx, remote, auth)
	return
}

// FindAllCached returns all Conns for the specified endpoint that are
// already in the cache.
func (c *ConnCache) FindAllCached(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer) ([]CachedConn, error) {
	return c.internalFindAllCached(ctx, remote, auth)
}

func (c *ConnCache) internalFind(
	ctx *context.T,
	remote naming.Endpoint,
	keys []interface{},
	auth flow.PeerAuthorizer,
	wait bool,
) (CachedConn, []string, []security.RejectedBlessing, error) {
	c.mu.Lock()
	var err error
	var entries rttEntries
	for {
		if c.conns == nil {
			c.mu.Unlock()
			return nil, nil, nil, errCacheClosed.Errorf(ctx, "cache is closed")
		}
		entries, err = c.rttEntriesLocked(ctx, keys)
		if len(entries) > 0 || !wait || !c.hasReservationsLocked(keys) {
			break
		}
		if c.cond.WaitWithDeadline(&c.mu, nsync.NoDeadline, ctx.Done()) != nsync.OK {
			c.mu.Unlock()
			switch ctx.Err() {
			case context.Canceled:
				return nil, nil, nil, verror.ErrCanceled.Errorf(ctx, "canceled")
			default:
				return nil, nil, nil, verror.ErrTimeout.Errorf(ctx, "timeout")
			}
		}
	}
	c.mu.Unlock()

	if len(entries) == 0 {
		if err == nil {
			err = errConnNotInCache.Errorf(ctx, "connection to %s not in cache", remote.String())
		}
		return nil, nil, nil, err
	}
	return c.pickFirstAuthorizedConn(ctx, remote, entries, auth)
}

func (c *ConnCache) internalFindCached(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer) (keys []interface{}, conn CachedConn, names []string, rejected []security.RejectedBlessing, err error) {
	// If we have an RID, there's no point in looking under anything else.
	if rid := remote.RoutingID; rid != naming.NullRoutingID {
		keys = []interface{}{rid, pathkey(remote.Protocol, remote.Address, rid)}
		conn, names, rejected, err = c.internalFind(ctx, remote, keys, auth, false)
		return keys, conn, names, rejected, err
	}

	// Try looking under the address if there wasn't a routing ID.
	addrKey := key(remote.Protocol, remote.Address)
	keys = []interface{}{addrKey}
	if conn, names, rejected, err = c.internalFind(ctx, remote, keys, auth, false); conn != nil {
		return keys, conn, names, rejected, nil
	}

	// If that didn't work, try resolving the address and looking again.
	p, _ := flow.RegisteredProtocol(remote.Protocol)
	network, addresses, rerr := resolve(ctx, p, remote.Protocol, remote.Address)
	if rerr != nil {
		// TODO(suharshs): Add a unittest for failed resolution.
		ctx.Errorf("Failed to resolve (%v, %v): %v", remote.Protocol, remote.Address, rerr)
	}

	for _, a := range addresses {
		if k := key(network, a); k != addrKey {
			keys = append(keys, k)
		}
	}

	if len(keys) > 1 {
		conn, names, rejected, err = c.internalFind(ctx, remote, keys, auth, false)
	}
	return keys, conn, names, rejected, err
}

func (c *ConnCache) internalFindAllCached(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer) ([]CachedConn, error) {

	// Collect all of the possible keys that could be used for this endpoint.
	keys := make([]interface{}, 0, 4)
	if rid := remote.RoutingID; rid != naming.NullRoutingID {
		keys = append(keys, []interface{}{rid, pathkey(remote.Protocol, remote.Address, rid)}...)
	}

	// protocol, address key.
	addrKey := key(remote.Protocol, remote.Address)
	keys = append(keys, addrKey)

	// key for all resolved addresses.
	p, _ := flow.RegisteredProtocol(remote.Protocol)
	network, addresses, rerr := resolve(ctx, p, remote.Protocol, remote.Address)
	if rerr != nil {
		// TODO(suharshs): Add a unittest for failed resolution.
		ctx.Errorf("Failed to resolve (%v, %v): %v", remote.Protocol, remote.Address, rerr)
	}
	for _, a := range addresses {
		if k := key(network, a); k != addrKey {
			keys = append(keys, k)
		}
	}

	c.mu.Lock()
	entries, err := c.rttEntriesLocked(ctx, keys)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	conns := make([]CachedConn, 0, len(entries))
	for _, e := range entries {
		if e.proxy || auth == nil {
			conns = append(conns, e.conn)
		}
		_, _, rerr := auth.AuthorizePeer(ctx,
			e.conn.LocalEndpoint(),
			remote,
			e.conn.RemoteBlessings(),
			e.conn.RemoteDischarges())
		if rerr == nil {
			conns = append(conns, e.conn)
		}
	}
	return conns, nil
}

// Reserve reserves the right to dial a remote endpoint.
func (c *ConnCache) Reserve(ctx *context.T, remote naming.Endpoint) *Reservation {
	if remote.Protocol == "bidi" {
		return nil
	}
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.conns == nil {
		// Cache is closed.
		return nil
	}

	k := key(remote.Protocol, remote.Address)

	if remote.RoutingID == naming.NullRoutingID {
		if len(c.cache[k]) > 0 || c.reserved[k] != nil {
			// There are either connections or a reservation for this
			// address, and the routing id is not given, so no reservations
			// are needed.
			return nil
		}
		res := &Reservation{
			cache:  c,
			remote: remote,
			keys:   []interface{}{k},
		}
		res.ctx, res.cancel = context.WithCancel(ctx)
		c.reserved[k] = res
		return res
	}

	// OK, now we're in the more complicated case when there is an address and
	// routing ID, so a proxy might be involved.
	// TODO(mattr): We should include the routes in the case of multi-proxying.
	pk := pathkey(remote.Protocol, remote.Address, remote.RoutingID)

	if len(c.cache[k]) == 0 && c.reserved[k] == nil {
		// Nobody is dialing the address.  We'll reserve both the address and the path.
		res := &Reservation{
			cache:  c,
			remote: remote,
			keys:   []interface{}{k, pk},
		}
		res.ctx, res.cancel = context.WithCancel(ctx)
		c.reserved[k] = res
		c.reserved[pk] = res
		return res
	}

	if len(c.cache[pk]) == 0 && c.reserved[pk] == nil {
		// The address connection exists (or is being dialed), but the path doesn't.
		// We'll only reserve the path.
		res := &Reservation{
			cache:        c,
			remote:       remote,
			keys:         []interface{}{pk},
			waitForProxy: true,
		}
		res.ctx, res.cancel = context.WithCancel(ctx)
		c.reserved[pk] = res
		return res
	}

	// No need to reserve anything.
	return nil
}

// KillConnections will closes at least num Conns in the cache.
// This is useful when the manager is approaching system FD limits.
//
// The policy is as follows:
// (1) Remove undialable (closing/closed) conns from the cache, there is no point
//     in closing undialable connections to address a FD limit.
// (2) Close and remove lameducked, expired connections from the cache,
//     counting non-proxied connections towards the removed FD count (num).
// (3) LameDuck idle expired connections, killing them if num is still greater
//     than 0.
// (4) Finally if 'num' hasn't been reached, remove the LRU remaining conns
//     until num is reached.
//
// If num is greater than the number of connections in the cache, all cached
// connections will be closed and removed.
// KillConnections returns an error iff the cache is closed.
func (c *ConnCache) KillConnections(ctx *context.T, num int) error { //nolint:gocyclo
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.conns == nil {
		return errCacheClosed.Errorf(ctx, "cache is closed")
	}

	// kill old error records.  We keep them for a while to allow new finds
	// to return errors for recent dial attempts, but we need to eliminate
	// them eventually.
	now := time.Now()
	for k, v := range c.errors {
		if v.when.Add(maxErrorAge).Before(now) {
			delete(c.errors, k)
		}
	}

	entries := make(lruEntries, 0, len(c.conns))
	for _, e := range c.conns {
		entries = append(entries, e)
	}
	k := 0
	for _, e := range entries {
		status := e.conn.Status()
		switch {
		case status >= conn.Closing:
			// Remove undialable conns.
			c.removeEntryLocked(e)
		case status == conn.LameDuckAcknowledged && e.conn.CloseIfIdle(ctx, c.idleExpiry):
			// Close and remove lameducked or idle connections.
			c.removeEntryLocked(e)
			num--
		default:
			entries[k] = e
			k++
		}
	}
	entries = entries[:k]
	// Lameduck or kill idle connections.
	// If num > 0, up to num idle connections will be killed instead of lameducked
	// to free FD resources.
	// Otherwise, the the lameducked connections will be closed when all active
	// in subsequent calls of KillConnections, once they become idle.
	// TODO(suharshs): This policy is not ideal as we should try to close everything
	// we can close without potentially losing RPCs first. The ideal policy would
	// close idle client only connections before closing server connections.
	k = 0
	for _, e := range entries {
		// Kill idle connections.
		if num > 0 && !e.conn.IsEncapsulated() && e.conn.CloseIfIdle(ctx, c.idleExpiry) {
			num--
			c.removeEntryLocked(e)
			continue
		}
		// Lameduck idle connections.
		if e.conn.IsIdle(ctx, c.idleExpiry) {
			e.conn.EnterLameDuck(ctx)
		}
		// No point in closing encapsulated connections when we reach an FD limit.
		if !e.conn.IsEncapsulated() {
			entries[k] = e
			k++
		}
	}
	entries = entries[:k]

	// If we have killed enough idle connections we can exit early.
	if num <= 0 {
		return nil
	}
	// Otherwise we need to kill the LRU conns.
	sort.Sort(entries)
	err := errConnKilledToFreeResources.Errorf(ctx, "connection killed to free resources")
	for i := 0; i < num && i < len(entries); i++ {
		e := entries[i]
		e.conn.Close(ctx, err)
		c.removeEntryLocked(e)
	}
	return nil
}

// EnterLameDuckMode lame ducks all connections and waits for the the remote
// end to acknowledge the lameduck.
func (c *ConnCache) EnterLameDuckMode(ctx *context.T) {
	c.mu.Lock()
	waitfor := make([]chan struct{}, 0, len(c.conns))
	for _, e := range c.conns {
		waitfor = append(waitfor, e.conn.EnterLameDuck(ctx))
	}
	c.mu.Unlock()
	for _, w := range waitfor {
		<-w
	}
}

// Close closes all connections in the cache.
func (c *ConnCache) Close(ctx *context.T) {
	defer c.mu.Unlock()
	c.mu.Lock()
	err := errCacheClosed.Errorf(ctx, "cache is closed")
	for _, e := range c.conns {
		e.conn.Close(ctx, err)
		if e.cancel != nil {
			e.cancel()
		}
	}
	for _, r := range c.reserved {
		if r.cancel != nil {
			r.cancel()
		}
	}
	c.conns = nil
	c.cache = nil
	c.reserved = nil
	c.errors = nil
}

// String returns a user friendly representation of the connections in the cache.
func (c *ConnCache) String() string {
	defer c.mu.Unlock()
	c.mu.Lock()
	buf := &bytes.Buffer{}
	if c.conns == nil {
		return "conncache closed"
	}
	fmt.Fprintln(buf, "Cached:")
	for _, e := range c.conns {
		fmt.Fprintf(buf, "%v: %v\n", e.keys, e.conn)
	}
	fmt.Fprintln(buf, "Reserved:")
	for _, r := range c.reserved {
		fmt.Fprintf(buf, "%v: %p\n", r.keys, r)
	}
	return buf.String()
}

// ExportStats exports cache information to the global stats.
func (c *ConnCache) ExportStats(prefix string) {
	stats.NewStringFunc(naming.Join(prefix, "cache"), c.debugStringForCache)
	stats.NewStringFunc(naming.Join(prefix, "reserved"), c.debugStringForDialing)
}

func (c *ConnCache) insertConnLocked(remote naming.Endpoint, conn CachedConn, proxy bool, keyByAddr bool, cancel context.CancelFunc) bool {
	if _, ok := c.conns[conn]; ok {
		// If the conn is already in the cache, don't re-add it.
		return false
	}
	ep := conn.RemoteEndpoint()
	entry := &connEntry{
		conn:   conn,
		rid:    ep.RoutingID,
		proxy:  proxy,
		cancel: cancel,
		keys:   make([]interface{}, 0, 3),
	}
	if entry.rid != naming.NullRoutingID {
		entry.keys = append(entry.keys, entry.rid)
	}
	kdialed := key(remote.Protocol, remote.Address)
	if keyByAddr {
		entry.keys = append(entry.keys, kdialed)
	}
	entry.keys = append(entry.keys, pathkey(remote.Protocol, remote.Address, ep.RoutingID))
	if kresolved := key(ep.Protocol, ep.Address); kresolved != kdialed {
		if keyByAddr {
			entry.keys = append(entry.keys, kresolved)
		}
		entry.keys = append(entry.keys, pathkey(ep.Protocol, ep.Address, ep.RoutingID))
	}

	for _, k := range entry.keys {
		c.cache[k] = append(c.cache[k], entry)
	}
	c.conns[entry.conn] = entry
	return true
}

func (c *ConnCache) rttEntriesLocked(ctx *context.T, keys []interface{}) (rttEntries, error) {
	var entries rttEntries
	var firstError error
	for _, k := range keys {
		if found := c.cache[k]; len(found) > 0 {
			for _, e := range found {
				if status := e.conn.Status(); status >= conn.Closing {
					c.removeEntryLocked(e)
				} else if !e.conn.RemoteLameDuck() {
					entries = append(entries, e)
				}
			}
		} else if err := c.errors[k].err; firstError == nil && err != nil {
			firstError = err
		}
	}
	sort.Sort(entries)
	return entries, firstError
}

func (c *ConnCache) hasReservationsLocked(keys []interface{}) bool {
	for _, k := range keys {
		if c.reserved[k] != nil {
			return true
		}
	}
	return false
}

func (c *ConnCache) pickFirstAuthorizedConn(
	ctx *context.T,
	remote naming.Endpoint,
	entries rttEntries,
	auth flow.PeerAuthorizer) (conn CachedConn, names []string, rejected []security.RejectedBlessing, err error) {
	for _, e := range entries {
		if e.proxy || auth == nil {
			return e.conn, nil, nil, nil
		}
		names, rejected, err = auth.AuthorizePeer(ctx,
			e.conn.LocalEndpoint(),
			remote,
			e.conn.RemoteBlessings(),
			e.conn.RemoteDischarges())
		if err == nil {
			return e.conn, names, rejected, nil
		}
	}
	return nil, nil, nil, err
}

func (c *ConnCache) removeEntryLocked(entry *connEntry) {
	for _, k := range entry.keys {
		entries, ok := c.cache[k]
		if ok {
			entries = removeEntryFromSlice(entries, entry)
			if len(entries) == 0 {
				delete(c.cache, k)
			} else {
				c.cache[k] = entries
			}
		}
	}
	delete(c.conns, entry.conn)
	if entry.cancel != nil {
		entry.cancel()
	}
}

func removeEntryFromSlice(entries []*connEntry, entry *connEntry) []*connEntry {
	for i, e := range entries {
		if e == entry {
			n := len(entries)
			entries[i], entries = entries[n-1], entries[:n-1]
			break
		}
	}
	return entries
}

func (c *ConnCache) debugStringForCache() string {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.cache == nil {
		return "<closed>"
	}
	// map iteration is unstable, so sort the keys first
	keys := make(sortedKeys, 0, len(c.cache))
	for k := range c.cache {
		keys = append(keys, k)
	}
	sort.Sort(keys)
	buf := &bytes.Buffer{}
	for _, k := range keys {
		fmt.Fprintf(buf, "KEY: %v\n", k)
		for _, e := range c.cache[k] {
			fmt.Fprintf(buf, "%v\n", e.conn.DebugString())
		}
		fmt.Fprintf(buf, "\n")
	}
	return buf.String()
}

func (c *ConnCache) debugStringForDialing() string {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.reserved == nil {
		return "<closed>"
	}
	keys := make(sortedKeys, 0, len(c.cache))
	for k := range c.cache {
		keys = append(keys, k)
	}
	sort.Sort(keys)
	buf := &bytes.Buffer{}
	for _, k := range keys {
		fmt.Fprintf(buf, "KEY: %v\n", k)
		fmt.Fprintf(buf, "%#v\n", c.reserved[k])
		fmt.Fprintf(buf, "\n")
	}
	return buf.String()
}

type sortedKeys []interface{}

func (e sortedKeys) Len() int {
	return len(e)
}

func (e sortedKeys) Less(i, j int) bool {
	return fmt.Sprint(e[i]) < fmt.Sprint(e[j])
}

func (e sortedKeys) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func key(protocol, address string) string {
	// TODO(mattr): Unalias the default protocol?
	return protocol + "," + address
}

func pathkey(protocol, address string, rid naming.RoutingID) string {
	// TODO(mattr): Unalias the default protocol?
	return protocol + "," + address + "," + rid.String()
}

type rttEntries []*connEntry

func (e rttEntries) Len() int {
	return len(e)
}

func (e rttEntries) Less(i, j int) bool {
	return e[i].conn.RTT() < e[j].conn.RTT()
}

func (e rttEntries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

type lruEntries []*connEntry

func (e lruEntries) Len() int {
	return len(e)
}

func (e lruEntries) Less(i, j int) bool {
	return e[i].conn.LastUsed().Before(e[j].conn.LastUsed())
}

func (e lruEntries) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func resolve(ctx *context.T, p flow.Protocol, protocol, address string) (string, []string, error) {
	if p != nil {
		net, addrs, err := p.Resolve(ctx, protocol, address)
		if err != nil {
			return "", nil, err
		}
		if len(addrs) > 0 {
			return net, addrs, nil
		}
	}
	return "", nil, errUnknownProtocol.Errorf(ctx, "unknown protocol: %s", protocol)
}
