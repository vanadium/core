// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/verror"
	"v.io/x/lib/set"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/services/syncbase/common"
	"v.io/x/ref/services/syncbase/ping"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

// Policies to pick a peer to sync with.
const (
	// Picks a peer at random from the available set.
	selectRandom = iota

	// TODO(hpucha): implement these policies.
	// Picks a peer with most differing generations.
	selectMostDiff

	// Picks a peer that was synced with the furthest in the past.
	selectOldest
)

// peerManager defines the interface that a peer manager module must provide.
type peerManager interface {
	// managePeers runs the feedback loop to manage and maintain a list of
	// healthy peers available for syncing.
	managePeers(ctx *context.T)

	// pickPeer picks a Syncbase to sync with.
	pickPeer(ctx *context.T) (connInfo, error)

	// updatePeerFromSyncer updates information for a peer that the syncer
	// attempts to connect to.
	updatePeerFromSyncer(ctx *context.T, peer connInfo, attemptTs time.Time, failed bool) error

	// updatePeerFromResponder updates information for a peer that the
	// responder responds to.
	updatePeerFromResponder(ctx *context.T, peer string, connTs time.Time, gvs interfaces.Knowledge) error

	exportStats(prefix string)
}

////////////////////////////////////////
// peerManager implementation.

// Every 'peerManagementInterval', the peerManager thread wakes up, picks up to
// 'pingFanout' peers based upon the configured policy that are not already in
// its peer cache, and pings them to determine if they are reachable. The
// reachable peers are added to the peer cache. When the syncer thread calls the
// peerManager to pick a peer to sync with, the peer is chosen from the peer
// cache. This improves the odds that we are contacting a peer that is
// available.
//
// peerManager selects peers to ping by incorporating the neighborhood
// information. The heuristic bootstraps by picking a set of random peers for
// each iteration and pinging them via their mount tables. This continues until
// all the peers in an iteration are unreachable via the mount tables. Upon such
// connection errors, the heuristic switches to picking a set of random peers
// from the neighborhood while still probing to see if peers are reachable via
// the syncgroup mount tables using an exponential backoff. For example, when
// the pings fail for the first time, the heuristic waits for two rounds before
// contacting peers via mount tables again. If this attempt also fails, it then
// backs off to wait four rounds before trying the mount tables, and so on. Upon
// success, these counters are reset, and the heuristic goes back to randomly
// selecting peers and communicating via the syncgroup mount tables.

// peerSyncInfo is the running statistics collected per peer in both sync
// directions; for a peer which syncs with this node or with which this node
// syncs.
//
// TODO(hpucha): Incorporate ping related statistics here.
type peerSyncInfo struct {
	// Number of continuous failures noticed when attempting to connect with
	// this peer, either via any of its advertised mount tables or via
	// neighborhood. These counters are reset when the connection to the
	// peer succeeds.
	numFailuresMountTable   uint64
	numFailuresNeighborhood uint64

	// The most recent timestamp when a connection to this peer was attempted.
	attemptTs time.Time
	// The most recent timestamp when a connection to this peer succeeded.
	successTs time.Time
	// The most recent timestamp when this peer synced with this node.
	fromTs time.Time
	// Map of database names to their corresponding generation vectors for
	// data and syncgroups.
	gvs map[string]interfaces.Knowledge
}

// connInfo holds the information needed to connect to a peer.
//
// TODO(hpucha): Add hints to decide if both neighborhood and mount table must
// be tried. Currently, if addrs are set, only addrs are tried.
type connInfo struct {
	// Name of the peer relative to the mount table chosen by the syncgroup
	// creator.
	relName string

	// Mount tables via which this peer might be reachable.
	mtTbls []string

	// Network addresses of the peer if available. For example, this can be
	// obtained from neighborhood discovery.
	addrs []string

	// pinned is a flow.PinnedConn to the remote end.
	// pinned.Conn() returns the last successful connection to the remote end.
	// If the channel returned by pinned.Conn.Closed() is closed, the connection can
	// no longer be used.
	// Once pinned.Unpin() is called, the connection will no longer be pinned in
	// rpc cache, and healthCheck will return to the rpc default health check interval.
	pinned flow.PinnedConn

	// addedTime is the time at which the connection was put into the peer cache.
	addedTime time.Time
}

type peerManagerImpl struct {
	sync.RWMutex
	s      *syncService
	policy int // peer selection policy

	// In-memory cache of information relevant to syncing with a peer. This
	// information could potentially be used in peer selection.

	// In-memory state to detect and handle the case when a peer has
	// restricted connectivity.
	numFailuresMountTable uint64 // total number of mount table failures across peers.
	curCount              uint64 // remaining number of sync rounds before any mount table will be retried.

	// In-memory cache of peer specific information.
	peerTbl map[string]*peerSyncInfo

	// In-memory cache of healthy peers.
	healthyPeerCache map[string]*connInfo
}

func newPeerManager(ctx *context.T, s *syncService, peerSelectionPolicy int) peerManager {
	return &peerManagerImpl{
		s:                s,
		policy:           peerSelectionPolicy,
		peerTbl:          make(map[string]*peerSyncInfo),
		healthyPeerCache: make(map[string]*connInfo),
	}
}

func (pm *peerManagerImpl) exportStats(prefix string) {
	stats.NewStringFunc(naming.Join(prefix, "peers"), pm.debugStringForPeers)
}

func (pm *peerManagerImpl) debugStringForPeers() string {
	pm.Lock()
	defer pm.Unlock()
	buf := &bytes.Buffer{}
	for _, c := range pm.healthyPeerCache {
		fmt.Fprintf(buf, "%v\n", c.debugString())
		fmt.Fprintln(buf)
	}
	return buf.String()
}

func (c *connInfo) debugString() string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "RELNAME: %v\n", c.relName)
	fmt.Fprintf(buf, "MTTBLS: %v\n", c.mtTbls)
	fmt.Fprintf(buf, "ADDRS: %v\n", c.addrs)
	fmt.Fprintf(buf, "ADDEDTIME: %v\n", c.addedTime)
	return buf.String()
}

func (pm *peerManagerImpl) managePeers(ctx *context.T) {
	defer pm.s.pending.Done()

	ticker := time.NewTicker(peerManagementInterval)
	defer ticker.Stop()

	for !pm.s.isClosed() {
		select {
		case <-ticker.C:
			if pm.s.isClosed() {
				break
			}
			pm.managePeersInternal(ctx)

		case <-pm.s.closed:
			break
		}
	}
	vlog.VI(1).Info("sync: managePeers: channel closed, stop work and exit")
}

func (pm *peerManagerImpl) managePeersInternal(ctx *context.T) {
	vlog.VI(2).Info("sync: managePeersInternal: begin")
	defer vlog.VI(2).Info("sync: managePeersInternal: end")

	var peers []*connInfo
	var viaMtTbl bool

	switch pm.policy {
	case selectRandom:
		peers, viaMtTbl = pm.pickPeersToPingRandom(ctx)
	default:
		return
	}

	if len(peers) == 0 {
		return
	}

	peers = pm.pingPeers(ctx, peers)

	pm.Lock()
	defer pm.Unlock()

	if len(peers) == 0 {
		if viaMtTbl {
			// Since all pings via mount tables failed, we treat
			// this as if the node lost general connectivity and
			// operate in neighborhood only mode for some time.
			if pm.numFailuresMountTable == 0 {
				// Drop the peer cache when we switch to using
				// neighborhood.
				for _, p := range pm.healthyPeerCache {
					p.pinned.Unpin()
				}
				pm.healthyPeerCache = make(map[string]*connInfo)
			}
			pm.numFailuresMountTable++
			pm.curCount = roundsToBackoff(pm.numFailuresMountTable)
		}
		return
	}

	if viaMtTbl {
		pm.numFailuresMountTable = 0
		// We do not drop the healthyPeerCache here and continue using the
		// neighborhood peers until the cache entries expire.
	}

	now := time.Now()
	for _, p := range peers {
		if old, ok := pm.healthyPeerCache[p.relName]; ok {
			old.pinned.Unpin()
		}
		p.addedTime = now
		pm.healthyPeerCache[p.relName] = p
	}
}

// pickPeersToPingRandom picks up to 'pingFanout' peers that are not already in
// the healthyPeerCache. It returns 'true' to indicate that these peers are picked to
// be reached via the mount tables, 'false' otherwise.
func (pm *peerManagerImpl) pickPeersToPingRandom(ctx *context.T) ([]*connInfo, bool) {
	pm.Lock()
	defer pm.Unlock()

	// Evict any closed connection from the cache.
	pm.evictClosedPeerConnsLocked(ctx)

	var peers []*connInfo

	members := pm.s.getMembers(ctx)

	// Remove myself from the set.
	delete(members, pm.s.name)
	if len(members) == 0 {
		vlog.VI(4).Infof("sync: pickPeersToPingRandom: no sgmembers found")
		return nil, true
	}

	if pm.curCount == 0 {
		vlog.VI(4).Infof("sync: pickPeersToPingRandom: picking from all sgmembers")

		// Compute number of available peers.
		n := 0
		for m := range members {
			if _, ok := pm.healthyPeerCache[m]; !ok {
				n++
			}
		}

		// Pick peers at random up to allowed fanout. Random selection
		// is obtained as follows: As we iterate over the map, the
		// current element is selected with a probability of (num
		// elements remaining to be chosen)/(total elements remaining to
		// be traversed). For example, to choose k elements out of a map
		// of size n, the first element is selected with a probability
		// of k/n. If the first element is included, the second element
		// is selected with a probability of k-1/n-1. If first element
		// was not included, the second element is selected with a
		// probability of k/n-1.
		k := pingFanout
		for m := range members {
			if _, ok := pm.healthyPeerCache[m]; !ok {
				// Decide if this member is to be chosen.
				if pm.s.randIntn(n) < k {
					info := pm.s.copyMemberInfo(ctx, m)
					p := &connInfo{
						relName: m,
						mtTbls:  set.String.ToSlice(info.mtTables),
					}
					peers = append(peers, p)

					k--
					if k == 0 {
						break
					}
				}
				n--
			}
		}
		return peers, true
	}

	pm.curCount--

	// Pick peers from the neighborhood if available.
	neighbors := pm.s.filterDiscoveryPeers(members)
	if len(neighbors) == 0 {
		vlog.VI(4).Infof("sync: pickPeersToPingRandom: no neighbors found")
		return nil, false
	}

	vlog.VI(4).Infof("sync: pickPeersToPingRandom: picking from neighbors")

	// Compute number of available peers.
	n := 0
	for nbr := range neighbors {
		if _, ok := pm.healthyPeerCache[nbr]; !ok {
			n++
		}
	}

	// Pick peers at random up to allowed fanout. Random selection is done
	// as described above.
	k := pingFanout
	for nbr, svc := range neighbors {
		if _, ok := pm.healthyPeerCache[nbr]; !ok {
			if pm.s.randIntn(n) < k {
				p := &connInfo{relName: nbr, addrs: svc.Addresses}
				peers = append(peers, p)

				k--
				if k == 0 {
					break
				}
			}
			n--
		}
	}
	return peers, false
}

func (pm *peerManagerImpl) pickPeer(ctx *context.T) (connInfo, error) {
	switch pm.policy {
	case selectRandom:
		return pm.pickPeerRandom(ctx)
	default:
		return connInfo{}, verror.New(verror.ErrInternal, ctx, "unimplemented peer selection policy")
	}
}

func (pm *peerManagerImpl) pickPeerRandom(ctx *context.T) (connInfo, error) {
	pm.Lock()
	defer pm.Unlock()

	pm.evictClosedPeerConnsLocked(ctx)

	var nullPeer connInfo

	if len(pm.healthyPeerCache) == 0 {
		return nullPeer, verror.New(verror.ErrInternal, ctx, "no usable peer")
	}

	// Pick a peer at random.
	ind := pm.s.randIntn(len(pm.healthyPeerCache))
	for _, info := range pm.healthyPeerCache {
		if ind == 0 {
			return *info, nil
		}
		ind--
	}
	panic("random selection didn't succeed")
}

func (pm *peerManagerImpl) updatePeerFromSyncer(ctx *context.T, peer connInfo, attemptTs time.Time, failed bool) error {
	pm.Lock()
	defer pm.Unlock()

	info, ok := pm.peerTbl[peer.relName]
	if !ok {
		info = &peerSyncInfo{}
		pm.peerTbl[peer.relName] = info
	}

	info.attemptTs = attemptTs
	if failed { // Handle failed sync attempt.
		// Evict the peer from healthyPeerCache.
		delete(pm.healthyPeerCache, peer.relName)
		if peer.pinned != nil {
			peer.pinned.Unpin()
		}

		if peer.addrs != nil {
			info.numFailuresNeighborhood++
		} else {
			info.numFailuresMountTable++
		}
	} else {
		if peer.addrs != nil {
			info.numFailuresNeighborhood = 0
		} else {
			info.numFailuresMountTable = 0
		}
		info.successTs = time.Now()
	}

	return nil
}

// TODO(hpucha): Implement this.
func (pm *peerManagerImpl) updatePeerFromResponder(ctx *context.T, peer string, connTs time.Time, gv interfaces.Knowledge) error {
	return nil
}

////////////////////////////////////////
// Helpers.

// roundsToBackoff computes the exponential backoff with a cap on the backoff.
func roundsToBackoff(failures uint64) uint64 {
	if failures > pingFailuresCap {
		failures = pingFailuresCap
	}

	return 1 << failures
}

// Caller of this function should hold the lock to manipulate the shared
// healthyPeerCache.
func (pm *peerManagerImpl) evictClosedPeerConnsLocked(ctx *context.T) {
	for p, info := range pm.healthyPeerCache {
		// TODO(suharshs): If having many goroutines is not a problem, we should have
		// a goroutine monitoring each conn's status and reconnecting as needed.
		select {
		case <-info.pinned.Conn().Closed():
			delete(pm.healthyPeerCache, p)
			info.pinned.Unpin()
		default:
		}
	}
}

func (pm *peerManagerImpl) pingPeers(ctx *context.T, peers []*connInfo) []*connInfo {
	type nameInfo struct {
		ci    *connInfo
		mtTbl bool
		index int
	}

	nm := make(map[string]*nameInfo)

	names := make([]string, 0, len(peers))
	for _, p := range peers {
		for i, a := range p.addrs {
			n := naming.Join(a, common.SyncbaseSuffix)
			names = append(names, n)
			nm[n] = &nameInfo{
				ci:    p,
				mtTbl: false,
				index: i,
			}
		}
		for i, mt := range p.mtTbls {
			n := naming.Join(mt, p.relName, common.SyncbaseSuffix)
			names = append(names, n)
			nm[n] = &nameInfo{
				ci:    p,
				mtTbl: true,
				index: i,
			}
		}
	}

	vlog.VI(4).Infof("sync: pingPeers: sending names %v", names)

	res, err := ping.PingInParallel(ctx, names, NeighborConnectionTimeout, channelTimeout)
	if err != nil {
		return nil
	}

	vlog.VI(4).Infof("sync: pingPeers: returned result %v", res)

	// Make a list of the successful peers with their mount tables or
	// neighborhood addresses that succeeded.
	speers := make(map[string]*connInfo)
	var speersArr []*connInfo

	for _, r := range res {
		if r.Err != nil {
			continue
		}

		info := nm[r.Name]
		ci, ok := speers[info.ci.relName]
		if !ok {
			ci = &connInfo{relName: info.ci.relName, pinned: r.Conn}
			speers[info.ci.relName] = ci
			speersArr = append(speersArr, ci)
		} else {
			r.Conn.Unpin()
		}

		if info.mtTbl {
			ci.mtTbls = append(ci.mtTbls, info.ci.mtTbls[info.index])
		} else {
			ci.addrs = append(ci.addrs, info.ci.addrs[info.index])
		}
	}

	return speersArr
}
