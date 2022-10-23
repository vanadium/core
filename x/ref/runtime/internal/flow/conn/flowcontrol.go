// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow/message"
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
	// bytesBufferedPerFlow is the max number of bytes that can be sent
	// before a flow control release message is required.
	bytesBufferedPerFlow uint64

	// releaseMessageLimit is the max number of release counters that can be
	// sent in a single message taking into account the current
	// bytesBufferedPerFlow value.
	releaseMessageLimit int

	mu sync.Mutex

	mtu uint64

	// toReleaseClosed contains the counters associated with flows
	// that have been closed so that those counters can be returned to
	// the remote end of the connection. This is required so that
	// counters that were borrowed on the remote end can be returned
	// to the shared pool.
	toReleaseClosed []message.Counter

	// toRelease is the number of tokens released by all flows that are
	// borrowing tokens at their remote ends. It is used to trigger
	// sending a release message to remote end to transition those
	// flows from borrowing to using released tokens.
	toReleaseBorrowed uint64

	// In our protocol new flows are opened by the dialer by immediately
	// starting to write data for that flow (in an OpenFlow message).
	// Since the other side doesn't yet know of the existence of this new
	// flow, it couldn't have allocated us any counters via a Release message.
	// In order to deal with this the conn maintains a pool of shared tokens
	// which are used by dialers of new flows.
	// lshared is the number of shared tokens available for new flows dialed
	// locally.
	lshared uint64

	// lsharedCh is used to notify any flows that are blocked waiting for
	// tokens that there may be some available to be borrowed, ie. it is
	// closed whenever the size of the shared pool changes. The channel
	// is closed (and a new created to replace it) to broadcast to all
	// waiting flows that they should wake and check to see if tokens
	// are available for them.
	lsharedCh chan struct{}

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
	shared *flowControlConnStats

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
	// remoteBorrowing indicates whether the remote end of this flow is using
	// borrowed tokens.
	remoteBorrowing bool
	// toRelease is the number of tokens that are pending to be released for this
	// flow. Release messages are sent when some flow has  used up at least half
	// it's buffer, at which point the token counts are sent for every flow.
	// This reduces the number of release messages that are sent.
	toRelease uint64
}

func binaryEncodeUintSize(v uint64) int {
	switch {
	case v <= 0x7f:
		return 1
	case v <= 0xff:
		return 2
	case v <= 0xffff:
		return 3
	case v <= 0xffffff:
		return 4
	case v <= 0xffffffff:
		return 5
	case v <= 0xffffffffff:
		return 6
	case v <= 0xffffffffffff:
		return 7
	case v <= 0xffffffffffffff:
		return 8
	default:
		return 9
	}
}

func (fs *flowControlConnStats) init(bytesBufferedPerFlow uint64) {
	fs.bytesBufferedPerFlow = bytesBufferedPerFlow
	fs.lshared = 0
	fs.lsharedCh = make(chan struct{})
	fs.outstandingBorrowed = make(map[uint64]uint64)
}

// configure must be called after the connection setup handshake is complete
// and the mtu and shared tokens are known.
func (fs *flowControlConnStats) configure(mtu, shared uint64) {
	fs.mtu, fs.lshared = mtu, shared
	// Assume at most 2^32 flows per connection.
	bytesPerFlowID := binaryEncodeUintSize(1 << 32)
	bytesPerCounter := binaryEncodeUintSize(fs.bytesBufferedPerFlow)
	fs.releaseMessageLimit = int(mtu) / (bytesPerFlowID + bytesPerCounter)
}

// newCounters creates a new entry for the specific flow id.
func (fs *flowControlConnStats) newCounters(ffs *flowControlFlowStats) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	ffs.toRelease = fs.bytesBufferedPerFlow
}

// determineReleaseMessage updates the flow control counters
// for the specified flow and then determines if a release message
// should be sent for all flows.
func (fs *flowControlConnStats) determineReleaseMessage(ffs *flowControlFlowStats, count uint64, open bool) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	// Only bump the toRelease value for that flow if it is still active.
	if open {
		ffs.toRelease += count
	}
	if ffs.remoteBorrowing {
		fs.toReleaseBorrowed += count
		return fs.toReleaseBorrowed > fs.bytesBufferedPerFlow/2
	}
	return ffs.toRelease > fs.bytesBufferedPerFlow/2
}

func (fs *flowControlConnStats) clearToReleaseLocked() {
	fs.toReleaseBorrowed = 0
	fs.toReleaseClosed = nil
}

// releaseOutstandingBorrowed is called for a flow that is no longer in
// use locally (eg. closed) but which is included in a release message received
// from the peer. This is required to ensure that borrowed tokens are returned
// to the shared pool.
func (fs *flowControlConnStats) releaseOutstandingBorrowedLocked(fid, val uint64) {
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

func (fs *flowControlConnStats) releaseOutstandingBorrowed(fid, val uint64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.releaseOutstandingBorrowedLocked(fid, val)
}

func (fs *flowControlConnStats) handleRelease(ctx *context.T, c *Conn, counters []message.Counter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fs.mu.Lock()
	defer fs.mu.Unlock()
	prevShared := fs.lshared
	for _, counter := range counters {
		if f := c.flows[counter.FlowID]; f != nil {
			f.flowControl.releaseCountersLocked(counter.Tokens)
			select {
			case f.tokenWait <- struct{}{}:
			default:
			}
		} else {
			fs.releaseOutstandingBorrowedLocked(counter.FlowID, counter.Tokens)
		}
	}
	if fs.lshared != prevShared {
		close(fs.lsharedCh)
		fs.lsharedCh = make(chan struct{})
	}
}

func (fs *flowControlFlowStats) init(shared *flowControlConnStats, id uint64, borrowing, remoteBorrowing bool, initialTokens uint64) {
	fs.shared = shared
	fs.id = id
	fs.borrowing = borrowing
	fs.remoteBorrowing = remoteBorrowing
	if initialTokens != 0 {
		fs.releaseCountersLocked(initialTokens)
	}
}

func (fs *flowControlFlowStats) releaseCountersLocked(tokens uint64) {
	fs.borrowing = false
	if fs.borrowed > 0 {
		n := tokens
		if fs.borrowed < tokens {
			n = fs.borrowed
		}
		tokens -= n
		fs.shared.lshared += n
		fs.borrowed -= n
	}
	fs.released += tokens
}

// lockForTokens must be called before calling getTokensLocked below.
func (fs *flowControlFlowStats) lockForTokens() {
	fs.shared.mu.Lock()
}

// unlockForTokens must be called after calling returnTokensLocked below.
func (fs *flowControlFlowStats) unlockForTokens() {
	fs.shared.mu.Unlock()
}

// getTokensLocked returns the number of tokens this flow can send right now.
// It is bounded by the channel mtu, the released counters, and possibly
// the number of shared counters for the conn if we are sending on a just
// dialed flow. It will never return more than mtu bytes as being available.
// It will immediately deduct the tokens returned and the caller must
// return any unused tokens via the returnTokensLocked method.
// IMPORTANT: the calls to getTokensLocked and returnTokensLocked constitute
// a critical region and hence must be guarded by calls to lockForTokens
// and unlockForTokens.
func (fs *flowControlFlowStats) getTokensLocked(ctx *context.T, encapsulated bool) (bool, <-chan struct{}, int) {
	max := fs.shared.mtu
	// When	our flow is proxied (i.e. encapsulated), the proxy has added overhead
	// when forwarding the message. This means we must reduce our mtu to ensure
	// that dialer framing reaches the acceptor without being truncated by the
	// proxy.
	if encapsulated {
		max -= proxyOverhead
	}
	if fs.borrowing {
		if fs.shared.lshared < max {
			max = fs.shared.lshared
		}
		fs.shared.lshared -= max
		fs.borrowed += max
		return true, fs.shared.lsharedCh, int(max)
	}
	if fs.released < max {
		max = fs.released
	}
	fs.released -= max
	return false, fs.shared.lsharedCh, int(max)
}

func (fs *flowControlFlowStats) returnTokensLocked(ctx *context.T, borrowed bool, unused int) {
	if unused == 0 {
		return
	}
	if borrowed {
		fs.shared.lshared += uint64(unused)
		fs.borrowed -= uint64(unused)
		return
	}
	fs.released += uint64(unused)
}

func (fs *flowControlFlowStats) clearLocked() {
	fs.toRelease = 0
	fs.remoteBorrowing = false
}

func (fs *flowControlFlowStats) handleFlowClose(closedRemotely, notConnClosing bool) {
	fs.shared.mu.Lock()
	defer fs.shared.mu.Unlock()
	fid := fs.id
	if closedRemotely {
		// When the other side closes a flow, it implicitly releases all the
		// counters used by that flow.  That means we should release the shared
		// counter to be used on other new flows.
		fs.shared.lshared += fs.borrowed
		fs.borrowed = 0
	} else if fs.borrowed > 0 && notConnClosing {
		fs.shared.outstandingBorrowed[fid] = fs.borrowed
	}
	if !fs.remoteBorrowing {
		fs.clearLocked()
		return
	}
	fs.shared.toReleaseClosed = append(fs.shared.toReleaseClosed, message.Counter{FlowID: fs.id, Tokens: fs.toRelease})
	// Need to keep borrowed counters around so that they can be sent
	// to the dialer to allow for the shared counter to be incremented
	// for all the past flows that borrowed counters (ie. pretty much
	// any/all short lived connections). A much better approach would be
	// to use a 'special' flow ID (e.g use the invalidFlowID) to use
	// for referring to all borrowed tokens for closed flows.
}
