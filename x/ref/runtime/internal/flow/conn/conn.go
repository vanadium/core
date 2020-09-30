// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/verror"
	slib "v.io/x/ref/lib/security"
)

const (
	invalidFlowID = iota
	blessingsFlowID
	reservedFlows = 10
)

const (
	expressPriority = iota
	flowPriority
	tearDownPriority

	// Must be last.
	numPriorities
)

// minChannelTimeout keeps track of minimum values that we allow for channel
// timeout on a per-protocol basis.  This is to prevent people setting some
// overall limit that doesn't make sense for very slow protocols.
// TODO(mattr): We should consider allowing users to set this per-protocol, or
// perhaps having the protocol implementation expose it via some kind of
// ChannelOpts interface.
var minChannelTimeout = map[string]time.Duration{
	"bt": 10 * time.Second,
}

const (
	defaultMtu                  = 1 << 16
	defaultChannelTimeout       = 30 * time.Minute
	DefaultBytesBufferedPerFlow = 1 << 20
	proxyOverhead               = 32
)

// A FlowHandler processes accepted flows.
type FlowHandler interface {
	// HandleFlow processes an accepted flow.
	HandleFlow(flow.Flow) error
}

type Status int

// Note that this is a progression of states that can only
// go in one direction.  We use inequality operators to see
// how far along in the progression we are, so the order of
// these is important.
const (
	Active Status = iota
	EnteringLameDuck
	LameDuckAcknowledged
	Closing
	Closed
)

type healthCheckState struct {
	requestSent     time.Time
	requestTimer    *time.Timer
	requestDeadline time.Time
	lastRTT         time.Duration

	closeTimer    *time.Timer
	closeDeadline time.Time
}

// A Conn acts as a multiplexing encrypted channel that can host Flows.
type Conn struct {
	// All the variables here are set before the constructor returns
	// and never changed after that.
	mp            *messagePipe
	version       version.RPCVersion
	local, remote naming.Endpoint
	remoteAddr    net.Addr
	closed        chan struct{}
	lameDucked    chan struct{}
	blessingsFlow *blessingsFlow
	loopWG        sync.WaitGroup
	unopenedFlows sync.WaitGroup
	cancel        context.CancelFunc
	handler       FlowHandler
	mtu           uint64

	mu sync.Mutex // All the variables below here are protected by mu.

	localBlessings, remoteBlessings   security.Blessings
	localDischarges, remoteDischarges map[string]security.Discharge
	localValid                        <-chan struct{}
	remoteValid                       chan struct{}
	rPublicKey                        security.PublicKey
	status                            Status
	remoteLameDuck                    bool
	nextFid                           uint64
	lastUsedTime                      time.Time
	flows                             map[uint64]*flw
	hcstate                           *healthCheckState
	acceptChannelTimeout              time.Duration

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

	// activeWriters keeps track of all the flows that are currently
	// trying to write, indexed by priority.  activeWriters[0] is a list
	// (note that writers form a linked list for this purpose)
	// of all the highest priority flows.  activeWriters[len-1] is a list
	// of all the lowest priority writing flows.
	activeWriters []writer
	writing       writer
}

// Ensure that *Conn implements flow.ManagedConn.
var _ flow.ManagedConn = &Conn{}

// NewDialed dials a new Conn on the given conn. In the case when it is not
// dialing a proxy, it can return an error indicating that the context was canceled
// (verror.ErrCanceled) along with a handshake completes within the
// specified handshakeTimeout duration. Or put another way, NewDialed will
// always waut for at most handshakeTimeout duration to complete the handshake
// even if the context is canceled. The behaviour is different for a proxy
// connection, in which case a cancelation is immediate and no attempt is made
// to establish the connection.
func NewDialed(
	ctx *context.T,
	conn flow.MsgReadWriteCloser,
	local, remote naming.Endpoint,
	versions version.RPCVersionRange,
	auth flow.PeerAuthorizer,
	proxy bool,
	handshakeTimeout time.Duration,
	channelTimeout time.Duration,
	handler FlowHandler) (c *Conn, names []string, rejected []security.RejectedBlessing, err error) {
	var remoteAddr net.Addr
	if flowConn, ok := conn.(flow.Conn); ok {
		remoteAddr = flowConn.RemoteAddr()
	}

	dctx := ctx
	ctx, cancel := context.WithRootCancel(ctx)
	if channelTimeout == 0 {
		channelTimeout = defaultChannelTimeout
	}
	if min := minChannelTimeout[local.Protocol]; channelTimeout < min {
		channelTimeout = min
	}

	// If the conn is being built on an encapsulated flow, we must update the
	// cancellation of the flow, to ensure that the conn doesn't get killed
	// when the context passed in is cancelled.
	if f, ok := conn.(*flw); ok {
		ctx = f.SetDeadlineContext(ctx, time.Time{})
	}
	c = &Conn{
		mp:                   newMessagePipe(conn),
		handler:              handler,
		local:                local,
		remote:               remote,
		remoteAddr:           remoteAddr,
		closed:               make(chan struct{}),
		lameDucked:           make(chan struct{}),
		nextFid:              reservedFlows,
		flows:                map[uint64]*flw{},
		lastUsedTime:         time.Now(),
		toRelease:            map[uint64]uint64{},
		borrowing:            map[uint64]bool{},
		cancel:               cancel,
		outstandingBorrowed:  make(map[uint64]uint64),
		activeWriters:        make([]writer, numPriorities),
		acceptChannelTimeout: channelTimeout,
	}
	done := make(chan struct{})
	var rtt time.Duration
	c.loopWG.Add(1)
	go func() {
		defer c.loopWG.Done()
		defer close(done)
		// We only send our real blessings if we are a server in addition to being a client.
		// Otherwise, we only send our public key through a nameless blessings object.
		// TODO(suharshs): Should we reveal server blessings if we are connecting to proxy here.
		if handler != nil {
			c.localBlessings, c.localValid = v23.GetPrincipal(ctx).BlessingStore().Default()
		} else {
			c.localBlessings, _ = security.NamelessBlessing(v23.GetPrincipal(ctx).PublicKey())
		}
		names, rejected, rtt, err = c.dialHandshake(ctx, versions, auth)
	}()
	var canceled bool
	timer := time.NewTimer(handshakeTimeout)
	var ferr error
	select {
	case <-done:
		ferr = err
	case <-timer.C:
		ferr = verror.ErrTimeout.Errorf(ctx, "timeout")
	case <-dctx.Done():
		if proxy {
			ferr = verror.ErrCanceled.Errorf(ctx, "canceled")
		} else {
			// The context has been canceled, but let's give this connection
			// an opportunity to run to completion just in case this connection
			// is racing to become established as per the race documented in:
			// https://github.com/vanadium/core/issues/40.
			select {
			case <-done:
				canceled = true
				// There is always the possibility that the handshake fails
				// (eg. the client doesn't trust the server), so make sure to
				// record any such error.
				ferr = err
				// Handshake done.
			case <-timer.C:
				// Report the timeout not the cancelation, hence
				// leave canceled as false.
				ferr = verror.ErrTimeout.Errorf(ctx, "timeout")
			}
		}
	}

	timer.Stop()
	if ferr != nil {
		c.Close(ctx, ferr)
		return nil, nil, nil, ferr
	}
	c.initializeHealthChecks(ctx, rtt)
	// We send discharges asynchronously to prevent making a second RPC while
	// trying to build up the connection for another. If the two RPCs happen to
	// go to the same server a deadlock will result.
	// This commonly happens when we make a Resolve call.  During the Resolve we
	// will try to fetch discharges to send to the mounttable, leading to a
	// Resolve of the discharge server name.  The two resolve calls may be to
	// the same mounttable.
	if handler != nil {
		c.loopWG.Add(1)
		go c.blessingsLoop(ctx, time.Now(), nil)
	}
	c.loopWG.Add(1)
	go c.readLoop(ctx)

	c.mu.Lock()
	c.lastUsedTime = time.Now()
	c.mu.Unlock()
	if canceled {
		ferr = verror.ErrCanceled.Errorf(ctx, "canceled")
	}
	return c, names, rejected, ferr
}

// NewAccepted accepts a new Conn on the given conn.
//
// NOTE: that the FlowHandler must be called asynchronously since it may
//       block until this function returns.
func NewAccepted(
	ctx *context.T,
	lAuthorizedPeers []security.BlessingPattern,
	conn flow.MsgReadWriteCloser,
	local naming.Endpoint,
	versions version.RPCVersionRange,
	handshakeTimeout time.Duration,
	channelTimeout time.Duration,
	handler FlowHandler) (*Conn, error) {
	var remoteAddr net.Addr
	if flowConn, ok := conn.(flow.Conn); ok {
		remoteAddr = flowConn.RemoteAddr()
	}
	ctx, cancel := context.WithCancel(ctx)
	if channelTimeout == 0 {
		channelTimeout = defaultChannelTimeout
	}
	if min := minChannelTimeout[local.Protocol]; channelTimeout < min {
		channelTimeout = min
	}
	c := &Conn{
		mp:                   newMessagePipe(conn),
		handler:              handler,
		local:                local,
		remoteAddr:           remoteAddr,
		closed:               make(chan struct{}),
		lameDucked:           make(chan struct{}),
		nextFid:              reservedFlows + 1,
		flows:                map[uint64]*flw{},
		lastUsedTime:         time.Now(),
		toRelease:            map[uint64]uint64{},
		borrowing:            map[uint64]bool{},
		cancel:               cancel,
		outstandingBorrowed:  make(map[uint64]uint64),
		activeWriters:        make([]writer, numPriorities),
		acceptChannelTimeout: channelTimeout,
	}
	done := make(chan struct{}, 1)
	var rtt time.Duration
	var err error
	var refreshTime time.Time
	c.loopWG.Add(1)
	go func() {
		defer c.loopWG.Done()
		defer close(done)
		principal := v23.GetPrincipal(ctx)
		c.localBlessings, c.localValid = principal.BlessingStore().Default()
		if c.localBlessings.IsZero() {
			c.localBlessings, _ = security.NamelessBlessing(principal.PublicKey())
		}
		c.localDischarges, refreshTime = slib.PrepareDischarges(
			ctx, c.localBlessings, nil, "", nil)
		rtt, err = c.acceptHandshake(ctx, versions, lAuthorizedPeers)
	}()
	timer := time.NewTimer(handshakeTimeout)
	var ferr error
	select {
	case <-done:
		ferr = err
	case <-timer.C:
		ferr = verror.ErrTimeout.Errorf(ctx, "timeout")
	case <-ctx.Done():
		ferr = verror.ErrCanceled.Errorf(ctx, "canceled")
	}
	timer.Stop()
	if ferr != nil {
		// Call internalClose with closedWhileAccepting set to true
		// to avoid waiting on the go routine above to complete.
		// This avoids blocking on the loopWG waitgroup which is
		// pointless since we've decided to not wait on it!
		c.internalClose(ctx, false, true, ferr)
		<-c.closed
		return nil, ferr
	}
	c.initializeHealthChecks(ctx, rtt)
	c.loopWG.Add(2)
	go c.blessingsLoop(ctx, refreshTime, lAuthorizedPeers)
	go c.readLoop(ctx)
	c.mu.Lock()
	c.lastUsedTime = time.Now()
	c.mu.Unlock()
	return c, nil
}

func (c *Conn) blessingsLoop(
	ctx *context.T,
	refreshTime time.Time,
	authorizedPeers []security.BlessingPattern) {
	defer c.loopWG.Done()
	for {
		if refreshTime.IsZero() {
			select {
			case <-c.localValid:
			case <-ctx.Done():
				return
			}
		} else {
			timer := time.NewTimer(time.Until(refreshTime))
			select {
			case <-timer.C:
			case <-c.localValid:
			case <-ctx.Done():
				timer.Stop()
				return
			}
			timer.Stop()
		}
		var dis map[string]security.Discharge
		blessings, valid := v23.GetPrincipal(ctx).BlessingStore().Default()
		dis, refreshTime = slib.PrepareDischarges(ctx, blessings, nil, "", nil)
		bkey, dkey, err := c.blessingsFlow.send(ctx, blessings, dis, authorizedPeers)
		if err != nil {
			c.internalClose(ctx, false, false, err)
			return
		}
		c.mu.Lock()
		c.localBlessings = blessings
		c.localDischarges = dis
		c.localValid = valid
		err = c.sendMessageLocked(ctx, true, expressPriority, &message.Auth{
			BlessingsKey: bkey,
			DischargeKey: dkey,
		})
		c.mu.Unlock()
		if err != nil {
			c.internalClose(ctx, false, false, err)
			return
		}
	}
}

// MTU Returns the maximum transimission unit for the connection in bytes.
func (c *Conn) MTU() uint64 {
	return c.mtu
}

// RTT returns the round trip time of a message to the remote end.
// Note the initial estimate of the RTT from the accepted side of a connection
// my be long because we don't fully factor out certificate verification time.
// The RTT will be updated with the receipt of every healthCheckResponse, so
// this overestimate doesn't remain for long when the channel timeout is low.
func (c *Conn) RTT() time.Duration {
	defer c.mu.Unlock()
	c.mu.Lock()
	rtt := c.hcstate.lastRTT
	if !c.hcstate.requestSent.IsZero() {
		if waitRTT := time.Since(c.hcstate.requestSent); waitRTT > rtt {
			rtt = waitRTT
		}
	}
	return rtt
}

func (c *Conn) newHealthChecksLocked(ctx *context.T, firstRTT time.Duration) *healthCheckState {
	now := time.Now()
	h := &healthCheckState{
		requestDeadline: now.Add(c.acceptChannelTimeout / 2),

		closeTimer: time.AfterFunc(c.acceptChannelTimeout, func() {
			c.internalClose(ctx, false, false, ErrChannelTimeout.Errorf(ctx, "the channel has become unresponsive"))
		}),
		closeDeadline: now.Add(c.acceptChannelTimeout),
		lastRTT:       firstRTT,
	}
	requestTimer := time.AfterFunc(c.acceptChannelTimeout/2, func() {
		c.mu.Lock()
		//nolint:errcheck
		c.sendMessageLocked(ctx, true, expressPriority, &message.HealthCheckRequest{})
		h.requestSent = time.Now()
		c.mu.Unlock()
	})
	h.requestTimer = requestTimer
	return h
}

func (c *Conn) initializeHealthChecks(ctx *context.T, firstRTT time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hcstate != nil {
		return
	}
	c.hcstate = c.newHealthChecksLocked(ctx, firstRTT)
}

func (c *Conn) handleHealthCheckResponse(ctx *context.T) {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.status < Closing {
		timeout := c.acceptChannelTimeout
		for _, f := range c.flows {
			if f.channelTimeout > 0 && f.channelTimeout < timeout {
				timeout = f.channelTimeout
			}
		}
		if min := minChannelTimeout[c.local.Protocol]; timeout < min {
			timeout = min
		}
		c.hcstate.closeTimer.Reset(timeout)
		c.hcstate.closeDeadline = time.Now().Add(timeout)
		c.hcstate.requestTimer.Reset(timeout / 2)
		c.hcstate.requestDeadline = time.Now().Add(timeout / 2)
		c.hcstate.lastRTT = time.Since(c.hcstate.requestSent)
		c.hcstate.requestSent = time.Time{}
	}
}

func (c *Conn) healthCheckNewFlowLocked(ctx *context.T, timeout time.Duration) {
	if timeout != 0 {
		if c.hcstate == nil {
			// There's a scheduling race between initializing healthchecks
			// and accepting a connection since each is handled on a different
			// goroutine. Hence there may be an attempt to update the health
			// checks before they have been initialized. The simplest fix is
			// to just initialize them here.
			c.hcstate = c.newHealthChecksLocked(ctx, timeout)
			return
		}
		if min := minChannelTimeout[c.local.Protocol]; timeout < min {
			timeout = min
		}
		now := time.Now()
		if rd := now.Add(timeout / 2); rd.Before(c.hcstate.requestDeadline) {
			c.hcstate.requestDeadline = rd
			c.hcstate.requestTimer.Reset(timeout / 2)
		}
		if cd := now.Add(timeout); cd.Before(c.hcstate.closeDeadline) {
			c.hcstate.closeDeadline = cd
			c.hcstate.closeTimer.Reset(timeout)
		}
	}
}

func (c *Conn) healthCheckCloseDeadline() time.Time {
	defer c.mu.Unlock()
	c.mu.Lock()
	return c.hcstate.closeDeadline
}

// EnterLameDuck enters lame duck mode, the returned channel will be closed when
// the remote end has ack'd or the Conn is closed.
func (c *Conn) EnterLameDuck(ctx *context.T) chan struct{} {
	var err error
	c.mu.Lock()
	if c.status < EnteringLameDuck {
		c.status = EnteringLameDuck
		err = c.sendMessageLocked(ctx, false, expressPriority, &message.EnterLameDuck{})
	}
	c.mu.Unlock()
	if err != nil {
		c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
	}
	return c.lameDucked
}

// Dial dials a new flow on the Conn.
func (c *Conn) Dial(ctx *context.T, blessings security.Blessings, discharges map[string]security.Discharge,
	remote naming.Endpoint, channelTimeout time.Duration, sideChannel bool) (flow.Flow, error) {
	if c.remote.RoutingID == naming.NullRoutingID {
		return nil, ErrDialingNonServer.Errorf(ctx, "attempting to dial on a connection with no remote server: %v", c.remote.String())
	}
	if blessings.IsZero() {
		// its safe to ignore this error since c.lBlessings must be valid, so the
		// encoding of the publicKey can never error out.
		blessings, _ = security.NamelessBlessing(v23.GetPrincipal(ctx).PublicKey())
	}
	defer c.mu.Unlock()
	c.mu.Lock()

	// It may happen that in the case of bidirectional RPC the dialer of the connection
	// has sent blessings,  but not yet discharges.  In this case we will wait for them
	// to send the discharges before allowing a bidirectional flow dial.
	if valid := c.remoteValid; valid != nil && len(c.remoteDischarges) == 0 && len(c.remoteBlessings.ThirdPartyCaveats()) > 0 {
		c.mu.Unlock()
		<-valid
		c.mu.Lock()
	}

	if c.remoteLameDuck || c.status >= Closing {
		return nil, ErrConnectionClosed.Errorf(ctx, "connection closed")
	}
	id := c.nextFid
	c.nextFid += 2
	remote = c.remote
	remote = remote.WithBlessingNames(c.remote.BlessingNames())
	flw := c.newFlowLocked(
		ctx,
		id,
		blessings,
		c.remoteBlessings,
		discharges,
		c.remoteDischarges,
		remote,
		true,
		false,
		channelTimeout,
		sideChannel)
	return flw, nil
}

// LocalEndpoint returns the local vanadium Endpoint
func (c *Conn) LocalEndpoint() naming.Endpoint { return c.local }

// RemoteEndpoint returns the remote vanadium Endpoint
func (c *Conn) RemoteEndpoint() naming.Endpoint {
	return c.remote
}

// LocalBlessings returns the local blessings.
func (c *Conn) LocalBlessings() security.Blessings {
	c.mu.Lock()
	localBlessings := c.localBlessings
	c.mu.Unlock()
	return localBlessings
}

// RemoteBlessings returns the remote blessings.
func (c *Conn) RemoteBlessings() security.Blessings {
	c.mu.Lock()
	remoteBlessings := c.remoteBlessings
	c.mu.Unlock()
	return remoteBlessings
}

// LocalDischarges fetches the most recently sent discharges for the local
// ends blessings.
func (c *Conn) LocalDischarges() map[string]security.Discharge {
	c.mu.Lock()
	localDischarges := c.localDischarges
	c.mu.Unlock()
	return localDischarges
}

// RemoteDischarges fetches the most recently received discharges for the remote
// ends blessings.
func (c *Conn) RemoteDischarges() map[string]security.Discharge {
	c.mu.Lock()
	// It may happen that in the case of bidirectional RPC the dialer of the connection
	// has sent blessings,  but not yet discharges.  In this case we will wait for them
	// to send the discharges instead of returning the initial nil discharges.
	if valid := c.remoteValid; valid != nil && len(c.remoteDischarges) == 0 && len(c.remoteBlessings.ThirdPartyCaveats()) > 0 {
		c.mu.Unlock()
		<-valid
		c.mu.Lock()
	}
	remoteDischarges := c.remoteDischarges
	c.mu.Unlock()
	return remoteDischarges
}

// CommonVersion returns the RPCVersion negotiated between the local and remote endpoints.
func (c *Conn) CommonVersion() version.RPCVersion { return c.version }

// LastUsed returns the time at which the Conn had bytes read or written on it.
func (c *Conn) LastUsed() time.Time {
	defer c.mu.Unlock()
	c.mu.Lock()
	return c.lastUsedTime
}

// RemoteLameDuck returns true if the other end of the connection has announced that
// it is in lame duck mode indicating that new flows should not be dialed on this
// conn.
func (c *Conn) RemoteLameDuck() bool {
	defer c.mu.Unlock()
	c.mu.Lock()
	return c.remoteLameDuck
}

// Closed returns a channel that will be closed after the Conn is shutdown.
// After this channel is closed it is guaranteed that all Dial calls will fail
// with an error and no more flows will be sent to the FlowHandler.
func (c *Conn) Closed() <-chan struct{} { return c.closed }

func (c *Conn) Status() Status {
	c.mu.Lock()
	status := c.status
	c.mu.Unlock()
	return status
}

// Close shuts down a conn.
func (c *Conn) Close(ctx *context.T, err error) {
	c.internalClose(ctx, false, false, err)
	<-c.closed
}

// CloseIfIdle closes the connection if the conn has been idle for idleExpiry,
// returning true if it closed it.
func (c *Conn) CloseIfIdle(ctx *context.T, idleExpiry time.Duration) bool {
	defer c.mu.Unlock()
	c.mu.Lock()
	if c.isIdleLocked(ctx, idleExpiry) {
		c.internalCloseLocked(ctx, false, false, ErrIdleConnKilled.Errorf(ctx, "connection killed because idle expiry was reached"))
		return true
	}
	return false
}

func (c *Conn) IsIdle(ctx *context.T, idleExpiry time.Duration) bool {
	defer c.mu.Unlock()
	c.mu.Lock()
	return c.isIdleLocked(ctx, idleExpiry)
}

// isIdleLocked returns true if the connection has been idle for idleExpiry.
func (c *Conn) isIdleLocked(ctx *context.T, idleExpiry time.Duration) bool {
	if c.hasActiveFlowsLocked() {
		return false
	}
	return c.lastUsedTime.Add(idleExpiry).Before(time.Now())
}

func (c *Conn) HasActiveFlows() bool {
	defer c.mu.Unlock()
	c.mu.Lock()
	return c.hasActiveFlowsLocked()
}

func (c *Conn) hasActiveFlowsLocked() bool {
	for _, f := range c.flows {
		if !f.sideChannel {
			return true
		}
	}
	return false
}

func (c *Conn) internalClose(ctx *context.T, closedRemotely, closedWhileAccepting bool, err error) {
	c.mu.Lock()
	c.internalCloseLocked(ctx, closedRemotely, closedWhileAccepting, err)
	c.mu.Unlock()
}

func (c *Conn) internalCloseLocked(ctx *context.T, closedRemotely, closedWhileAccepting bool, err error) {
	debug := ctx.VI(2)
	debug.Infof("Closing connection: %v", err)

	if c.status >= Closing {
		// This conn is already being torn down.
		return
	}
	if c.status < LameDuckAcknowledged {
		close(c.lameDucked)
	}
	c.status = Closing
	if c.remoteValid != nil {
		close(c.remoteValid)
		c.remoteValid = nil
	}

	flows := make([]*flw, 0, len(c.flows))
	for _, f := range c.flows {
		flows = append(flows, f)
	}

	go func(c *Conn) {
		if c.hcstate != nil {
			c.hcstate.requestTimer.Stop()
			c.hcstate.closeTimer.Stop()
		}
		if !closedRemotely {
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			c.mu.Lock()
			cerr := c.sendMessageLocked(ctx, false, tearDownPriority, &message.TearDown{
				Message: msg,
			})
			c.mu.Unlock()
			if cerr != nil {
				ctx.VI(2).Infof("Error sending tearDown on connection to %s: %v", c.remote, cerr)
			}
		}
		if err == nil {
			err = ErrConnectionClosed.Errorf(ctx, "connection closed")
		}
		for _, f := range flows {
			f.close(ctx, false, err)
		}
		if c.blessingsFlow != nil {
			c.blessingsFlow.close(ctx, err)
		}
		if cerr := c.mp.rw.Close(); cerr != nil {
			debug.Infof("Error closing underlying connection for %s: %v", c.remote, cerr)
		}
		if c.cancel != nil {
			c.cancel()
		}
		if !closedWhileAccepting {
			// given that the accept handshake timed out or was
			// cancelled it doesn't make sense to wait for it here.
			c.loopWG.Wait()
		}
		c.mu.Lock()
		c.status = Closed
		close(c.closed)
		c.mu.Unlock()
	}(c)
}

func (c *Conn) release(ctx *context.T, fid, count uint64) {
	var toRelease map[uint64]uint64
	var release bool
	c.mu.Lock()
	c.toRelease[fid] += count
	if c.borrowing[fid] {
		c.toRelease[invalidFlowID] += count
		release = c.toRelease[invalidFlowID] > DefaultBytesBufferedPerFlow/2
	} else {
		release = c.toRelease[fid] > DefaultBytesBufferedPerFlow/2
	}
	if release {
		toRelease = c.toRelease
		c.toRelease = make(map[uint64]uint64, len(c.toRelease))
		c.borrowing = make(map[uint64]bool, len(c.borrowing))
	}
	var err error
	if toRelease != nil {
		delete(toRelease, invalidFlowID)
		err = c.sendMessageLocked(ctx, false, expressPriority, &message.Release{
			Counters: toRelease,
		})
	}
	c.mu.Unlock()
	if err != nil {
		c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
	}
}

func (c *Conn) releaseOutstandingBorrowedLocked(fid, val uint64) {
	borrowed := c.outstandingBorrowed[fid]
	released := val
	if borrowed == 0 {
		return
	} else if borrowed < released {
		released = borrowed
	}
	c.lshared += released
	if released == borrowed {
		delete(c.outstandingBorrowed, fid)
	} else {
		c.outstandingBorrowed[fid] = borrowed - released
	}
}

func (c *Conn) handleMessage(ctx *context.T, m message.Message) error { //nolint:gocyclo
	switch msg := m.(type) {
	case *message.TearDown:
		var err error
		if msg.Message != "" {
			err = ErrRemoteError.Errorf(ctx, "remote end received err: %v", msg.Message)
		}
		c.internalClose(ctx, true, false, err)
		return nil

	case *message.EnterLameDuck:
		c.mu.Lock()
		c.remoteLameDuck = true
		c.mu.Unlock()
		go func() {
			// We only want to send the lame duck acknowledgment after all outstanding
			// OpenFlows are sent.
			c.unopenedFlows.Wait()
			c.mu.Lock()
			err := c.sendMessageLocked(ctx, true, expressPriority, &message.AckLameDuck{})
			c.mu.Unlock()
			if err != nil {
				c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
			}
		}()

	case *message.AckLameDuck:
		c.mu.Lock()
		if c.status < LameDuckAcknowledged {
			c.status = LameDuckAcknowledged
			close(c.lameDucked)
		}
		c.mu.Unlock()

	case *message.HealthCheckRequest:
		c.mu.Lock()
		//nolint:errcheck
		c.sendMessageLocked(ctx, true, expressPriority, &message.HealthCheckResponse{})
		c.mu.Unlock()

	case *message.HealthCheckResponse:
		c.handleHealthCheckResponse(ctx)

	case *message.OpenFlow:
		remoteBlessings, remoteDischarges, err := c.blessingsFlow.getRemote(
			ctx, msg.BlessingsKey, msg.DischargeKey)
		if err != nil {
			return err
		}
		c.mu.Lock()
		if c.nextFid%2 == msg.ID%2 {
			c.mu.Unlock()
			return ErrInvalidPeerFlow.Errorf(ctx, "peer has chosen flow id from local domain")
		}
		if c.handler == nil {
			c.mu.Unlock()
			return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", msg)
		} else if c.status == Closing {
			c.mu.Unlock()
			return nil // Conn is already being closed.
		}
		sideChannel := msg.Flags&message.SideChannelFlag != 0
		f := c.newFlowLocked(
			ctx,
			msg.ID,
			c.localBlessings,
			remoteBlessings,
			c.localDischarges,
			remoteDischarges,
			c.remote,
			false,
			true,
			c.acceptChannelTimeout,
			sideChannel)
		f.releaseLocked(msg.InitialCounters)
		c.toRelease[msg.ID] = DefaultBytesBufferedPerFlow
		c.borrowing[msg.ID] = true
		c.mu.Unlock()

		c.handler.HandleFlow(f) //nolint:errcheck

		if err := f.q.put(ctx, msg.Payload); err != nil {
			return err
		}
		if msg.Flags&message.CloseFlag != 0 {
			f.close(ctx, true, nil)
		}

	case *message.Release:
		c.mu.Lock()
		for fid, val := range msg.Counters {
			if f := c.flows[fid]; f != nil {
				f.releaseLocked(val)
			} else {
				c.releaseOutstandingBorrowedLocked(fid, val)
			}
		}
		c.mu.Unlock()

	case *message.Data:
		c.mu.Lock()
		if c.status == Closing {
			c.mu.Unlock()
			return nil // Conn is already being shut down.
		}
		f := c.flows[msg.ID]
		if f == nil {
			// If the flow is closing then we assume the remote side releases
			// all borrowed counters for that flow.
			c.releaseOutstandingBorrowedLocked(msg.ID, math.MaxUint64)
			c.mu.Unlock()
			return nil
		}
		c.mu.Unlock()
		if err := f.q.put(ctx, msg.Payload); err != nil {
			return err
		}
		if msg.Flags&message.CloseFlag != 0 {
			f.close(ctx, true, nil)
		}

	case *message.Auth:
		blessings, discharges, err := c.blessingsFlow.getRemote(
			ctx, msg.BlessingsKey, msg.DischargeKey)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.remoteBlessings = blessings
		c.remoteDischarges = discharges
		if c.remoteValid != nil {
			close(c.remoteValid)
			c.remoteValid = make(chan struct{})
		}
		c.mu.Unlock()
	default:
		return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", m)
	}
	return nil
}

func (c *Conn) readLoop(ctx *context.T) {
	defer c.loopWG.Done()
	var err error
	for {
		msg, rerr := c.mp.readMsg(ctx)
		if rerr != nil {
			err = ErrRecv.Errorf(ctx, "error reading from: %v: %v", c.remote.String(), rerr)
			break
		}
		if err = c.handleMessage(ctx, msg); err != nil {
			break
		}
	}
	c.internalClose(ctx, false, false, err)
}

func (c *Conn) markUsed() {
	c.mu.Lock()
	c.markUsedLocked()
	c.mu.Unlock()
}

func (c *Conn) markUsedLocked() {
	c.lastUsedTime = time.Now()
}

func (c *Conn) IsEncapsulated() bool {
	_, ok := c.mp.rw.(*flw)
	return ok
}

type writer interface {
	notify()
	priority() int
	neighbors() (prev, next writer)
	setNeighbors(prev, next writer)
}

// activateWriterLocked adds a given writer to the list of active writers.
// The writer will be given a turn when the channel becomes available.
// You should try to only have writers with actual work to do in the
// list of activeWriters because we will switch to that thread to allow it
// to do work, and it will be wasteful if it turns out there is no work to do.
// After calling this you should typically call notifyNextWriterLocked.
func (c *Conn) activateWriterLocked(w writer) {
	priority := w.priority()
	_, wn := w.neighbors()
	head := c.activeWriters[priority]
	if head == w || wn != w {
		// We're already active.
		return
	}
	if head == nil { // We're the head of the list.
		c.activeWriters[priority] = w
	} else { // Insert us before head, which is the end of the list.
		hp, _ := head.neighbors()
		w.setNeighbors(hp, head)
		hp.setNeighbors(nil, w)
		head.setNeighbors(w, nil)
	}
}

// deactivateWriterLocked removes a writer from the active writer list.  After
// this function is called it is certain that the writer will not be given any
// new turns.  If the writer is already in the middle of a turn, that turn is
// not terminated, workers must end their turn explicitly by calling
// notifyNextWriterLocked.
func (c *Conn) deactivateWriterLocked(w writer) {
	priority := w.priority()
	p, n := w.neighbors()
	if head := c.activeWriters[priority]; head == w {
		if w == n { // We're the only one in the list.
			c.activeWriters[priority] = nil
		} else {
			c.activeWriters[priority] = n
		}
	}
	n.setNeighbors(p, nil)
	p.setNeighbors(nil, n)
	w.setNeighbors(w, w)
}

// notifyNextWriterLocked notifies the highest priority activeWriter to take
// a turn writing.  If w is the active writer give up w's claim and choose
// the next writer.  If there is already an active writer != w, this function does
// nothing.
func (c *Conn) notifyNextWriterLocked(w writer) {
	if c.writing == w {
		c.writing = nil
	}
	if c.writing == nil {
		for p, head := range c.activeWriters {
			if head != nil {
				_, c.activeWriters[p] = head.neighbors()
				c.writing = head
				head.notify()
				return
			}
		}
	}
}

type writerList struct {
	// next and prev are protected by c.mu
	next, prev writer
}

func (s *writerList) neighbors() (prev, next writer) { return s.prev, s.next }
func (s *writerList) setNeighbors(prev, next writer) {
	if prev != nil {
		s.prev = prev
	}
	if next != nil {
		s.next = next
	}
}

// singleMessageWriter is used to send a single message with a given priority.
type singleMessageWriter struct {
	writeCh chan struct{}
	p       int
	writerList
}

func (s *singleMessageWriter) notify()       { close(s.writeCh) }
func (s *singleMessageWriter) priority() int { return s.p }

// sendMessageLocked sends a single message on the conn with the given priority.
// if cancelWithContext is true, then this write attempt will fail when the context
// is canceled.  Otherwise context cancellation will have no effect and this call
// will block until the message is sent.
// NOTE: The mutex is not held for the entirety of this call,
// therefore this call will interrupt your critical section. This
// should be called only at the end of a mutex protected region.
func (c *Conn) sendMessageLocked(
	ctx *context.T,
	cancelWithContext bool,
	priority int,
	m message.Message) (err error) {
	s := &singleMessageWriter{writeCh: make(chan struct{}), p: priority}
	s.next, s.prev = s, s
	c.activateWriterLocked(s)
	c.notifyNextWriterLocked(s)
	c.mu.Unlock()
	// wait for my turn.
	if cancelWithContext {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-s.writeCh:
		}
	} else {
		<-s.writeCh
	}
	// send the actual message.
	if err == nil {
		err = c.mp.writeMsg(ctx, m)
	}
	c.mu.Lock()
	c.deactivateWriterLocked(s)
	c.notifyNextWriterLocked(s)
	return err
}

func (c *Conn) DebugString() string {
	defer c.mu.Unlock()
	c.mu.Lock()
	return fmt.Sprintf(`
Remote:
  Endpoint   %v
  Blessings: %v (claimed)
  PublicKey: %v
Local:
  Endpoint:  %v
  Blessings: %v
Version:     %v
MTU:         %d
LastUsed:    %v
#Flows:      %d
`,
		c.remote,
		c.remoteBlessings,
		c.rPublicKey,
		c.local,
		c.localBlessings,
		c.version,
		c.mtu,
		c.lastUsedTime,
		len(c.flows))
}
