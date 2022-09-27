// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"

	"v.io/v23/context"
)

// flowControlConnStats represents the flow control counters for all flows
// supported by the Conn hosting it. The MTU and lshared are only known
// after the initial connection 'setup' handshake is complete and must
// be specified via the 'configure' method. In addition to these shared
// counters, each flow maintains an instance of flowControlFlowStats which
// contains the per-flow counters and supports the token calculations
// required for that flow. Thus, the flow control state consists of
// a single instance of flowControlConnStats, shared by all flows hosted
// on that connection, and for each flow, an instance of flowControlFlowStats.
// The locking strategy is simply to use the mutex in flowControlConnStats
// to guard access to it and to all of the flowControlFlowStats instances
// in each flow.
type flowControlConnStats struct {
	mu sync.Mutex

	mtu uint64

	// TODO(mattr): Integrate these maps back into the flows themselves as
	// has been done with the sending counts.
	// toRelease is a map from flowID to a number of tokens which are pending
	// to be released.  We only send release messages when some flow has
	// used up at least half it's buffer, and then we send the counters for
	// every flow.  This reduces the number of release messages that are sent.
	toRelease map[uint64]uint64

	// borrowing is a map from flowID to a boolean indicating whether the remote
	// dialer of the flow is using shared counters for his sends because we've not
	// yet sent a release for this flow.
	borrowing map[uint64]bool

	// In our protocol new flows are opened by the dialer by immediately
	// starting to write data for that flow (in an OpenFlow message).
	// Since the other side doesn't yet know of the existence of this new
	// flow, it couldn't have allocated us any counters via a Release message.
	// In order to deal with this the conn maintains a pool of shared tokens
	// which are used by dialers of new flows.
	// lshared is the number of shared tokens available for new flows dialed
	// locally.
	lshared uint64

	// outstandingBorrowed is a map from flowID to a number of borrowed tokens.
	// This map is populated when a flow closes locally before it receives a remote close
	// or a release message.  In this case we need to remember that we have already
	// used these counters and return them to the shared pool when we get
	// a close or release.
	outstandingBorrowed map[uint64]uint64
}

// flowControlFlowStats represents per-flow flow control counters. Access to it
// must be guarded by the mutex in flowControlConnStats.
type flowControlFlowStats struct {
	*flowControlConnStats

	id uint64

	// released counts tokens already released by the remote end, that is, the number
	// of tokens we are allowed to send.
	released uint64
	// borrowed indicates the number of tokens we have borrowed from the shared pool for
	// sending on newly dialed flows.
	borrowed uint64
	// borrowing indicates whether this flow is using borrowed counters for a newly
	// dialed flow.  This will be set to false after we first receive a
	// release from the remote end.  This is always false for accepted flows.
	borrowing bool
}

func (fs *flowControlConnStats) init() {
	fs.toRelease = map[uint64]uint64{}
	fs.borrowing = map[uint64]bool{}
	fs.lshared = 0
	fs.outstandingBorrowed = make(map[uint64]uint64)
}

// configure must be called after the connection setup handshake is complete
// and the mtu and shared tokens are known.
func (fs *flowControlConnStats) configure(mtu, shared uint64) {
	fs.mtu, fs.lshared = mtu, shared
}

func (fs *flowControlConnStats) lock() {
	fs.mu.Lock()
}

func (fs *flowControlConnStats) unlock() {
	fs.mu.Unlock()
}

// newCounters creates a new entry for the specific flow id.
func (fs *flowControlConnStats) newCounters(fid uint64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.toRelease[fid] = DefaultBytesBufferedPerFlow
	fs.borrowing[fid] = true
}

// incrementToRelease increments the 'toRelease' count for the specified flow id.
func (fs *flowControlConnStats) incrementToRelease(fid, count uint64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.toRelease[fid] += count
}

// createReleaseMessageContents creates the data to be sent in a release
// message to this connection's peer.
func (fs *flowControlConnStats) createReleaseMessageContents(fid, count uint64) map[uint64]uint64 {
	var release bool
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.borrowing[fid] {
		fs.toRelease[invalidFlowID] += count
		release = fs.toRelease[invalidFlowID] > DefaultBytesBufferedPerFlow/2
	} else {
		release = fs.toRelease[fid] > DefaultBytesBufferedPerFlow/2
	}
	if !release {
		return nil
	}
	toRelease := fs.toRelease
	fs.toRelease = make(map[uint64]uint64, len(fs.toRelease))
	fs.borrowing = make(map[uint64]bool, len(fs.borrowing))
	return toRelease
}

// releaseOutstandingBorrowed is called for a flow that is no longer in
// use locally (eg. closed) but which is included in a release message received
// from the peer. This is required to ensure that borrowed tokens are returned
// to the shared pool.
func (fs *flowControlConnStats) releaseOutstandingBorrowed(fid, val uint64) {
	fs.lock()
	defer fs.unlock()
	borrowed := fs.outstandingBorrowed[fid]
	released := val
	if borrowed == 0 {
		return
	} else if borrowed < released {
		released = borrowed
	}
	fs.lshared += released
	if released == borrowed {
		delete(fs.outstandingBorrowed, fid)
	} else {
		fs.outstandingBorrowed[fid] = borrowed - released
	}
}

func (fs *flowControlConnStats) clearCountersLocked(fid uint64) {
	if !fs.borrowing[fid] {
		delete(fs.toRelease, fid)
		delete(fs.borrowing, fid)
	}
	// Need to keep borrowed counters around so that they can be sent
	// to the dialer to allow for the shared counter to be incremented
	// for all the past flows that borrowed counters (ie. pretty much
	// any/all short lived connections). A much better approach would be
	// to use a 'special' flow ID (e.g use the invalidFlowID) to use
	// for referring to all borrowed tokens for closed flows.
}

// tokens returns the number of tokens this flow can send right now.
// It is bounded by the channel mtu, the released counters, and possibly
// the number of shared counters for the conn if we are sending on a just
// dialed flow.
func (fs *flowControlFlowStats) tokens(ctx *context.T, encapsulated bool) (int, func(int)) {
	fs.lock()
	defer fs.unlock()
	max := fs.mtu
	// When	our flow is proxied (i.e. encapsulated), the proxy has added overhead
	// when forwarding the message. This means we must reduce our mtu to ensure
	// that dialer framing reaches the acceptor without being truncated by the
	// proxy.
	if encapsulated {
		max -= proxyOverhead
	}
	if fs.borrowing {
		if fs.lshared < max {
			max = fs.lshared
		}
		return int(max), func(used int) {
			fs.lshared -= uint64(used)
			fs.borrowed += uint64(used)
			if ctx.V(2) {
				ctx.Infof("deducting %d borrowed tokens on flow %d, total: %d left: %d", used, fs.id, fs.borrowed, fs.lshared)
			}
		}
	}
	if fs.released < max {
		max = fs.released
	}
	return int(max), func(used int) {
		fs.released -= uint64(used)
		if ctx.V(2) {
			ctx.Infof("flow %d deducting %d tokens, %d left", fs.id, used, fs.released)
		}
	}
}

func (fs *flowControlFlowStats) handleFlowClose(closedRemotely, notConnClosing bool) {
	fs.lock()
	defer fs.unlock()
	if closedRemotely {
		// When the other side closes a flow, it implicitly releases all the
		// counters used by that flow.  That means we should release the shared
		// counter to be used on other new flows.
		fs.lshared += fs.borrowed
		fs.borrowed = 0
	} else if fs.borrowed > 0 && notConnClosing {
		fs.outstandingBorrowed[fs.id] = fs.borrowed
	}
	fs.clearCountersLocked(fs.id)
}
