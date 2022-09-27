// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
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
	rpcversion "v.io/x/ref/runtime/internal/rpc/version"
)

const (
	invalidFlowID = iota
	blessingsFlowID
	reservedFlows = 10
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
	defaultMtu            = 1 << 16
	defaultChannelTimeout = 30 * time.Minute
	// DefaultBytesBufferedPerFlow defines the default number
	// of bytes that can be buffered by a single flow.
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

	// The following fields for managing flow control and the writeq
	// are locked indepdently.
	flowControl flowControlConnStats

	writeq      writeq
	writeqEntry writer

	mux sync.Mutex // All the variables below here are protected by mu.

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
func NewDialed( //nolint:gocyclo
	ctx *context.T,
	conn flow.MsgReadWriteCloser,
	local, remote naming.Endpoint,
	versions version.RPCVersionRange,
	auth flow.PeerAuthorizer,
	proxy bool,
	handshakeTimeout time.Duration,
	channelTimeout time.Duration,
	handler FlowHandler) (c *Conn, names []string, rejected []security.RejectedBlessing, err error) {

	if _, err = version.CommonVersion(ctx, rpcversion.Supported, versions); err != nil {
		return
	}

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
		cancel:               cancel,
		acceptChannelTimeout: channelTimeout,
	}
	// It's important that this channel has a non-zero buffer.  Sometimes this
	// flow will be notifying itself, so if there's no buffer a deadlock will
	// occur.
	initWriter(&c.writeqEntry, 1)
	c.flowControl.init()
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

	c.lock()
	c.lastUsedTime = time.Now()
	c.unlock()
	if canceled {
		ferr = verror.ErrCanceled.Errorf(ctx, "canceled")
	}
	return c, names, rejected, ferr
}

// NewAccepted accepts a new Conn on the given conn.
//
// NOTE: that the FlowHandler must be called asynchronously since it may
//
//	block until this function returns.
func NewAccepted(
	ctx *context.T,
	lAuthorizedPeers []security.BlessingPattern,
	conn flow.MsgReadWriteCloser,
	local naming.Endpoint,
	versions version.RPCVersionRange,
	handshakeTimeout time.Duration,
	channelTimeout time.Duration,
	handler FlowHandler) (*Conn, error) {

	if _, err := version.CommonVersion(ctx, rpcversion.Supported, versions); err != nil {
		return nil, err
	}

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
		cancel:               cancel,
		acceptChannelTimeout: channelTimeout,
	}
	initWriter(&c.writeqEntry, 1)
	c.flowControl.init()
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
	// NOTE: there is a race for refreshTime since it gets set above
	// in a goroutine but read here without any synchronization.
	go c.blessingsLoop(ctx, refreshTime, lAuthorizedPeers)
	go c.readLoop(ctx)
	c.lock()
	c.lastUsedTime = time.Now()
	c.unlock()
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
		// Need to access the underlying message pipe with the connections
		// lock held.
		bkey, dkey, err := c.blessingsFlow.send(ctx, blessings, dis, authorizedPeers)
		if err != nil {
			c.internalClose(ctx, false, false, err)
			return
		}
		c.lock()
		c.localBlessings = blessings
		c.localDischarges = dis
		c.localValid = valid
		c.unlock()
		err = c.sendMessage(ctx, true, expressPriority, &message.Auth{
			BlessingsKey: bkey,
			DischargeKey: dkey,
		})
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
	defer c.unlock()
	c.lock()
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
		c.sendMessage(ctx, true, expressPriority, &message.HealthCheckRequest{})
		c.lock()
		h.requestSent = time.Now()
		c.unlock()
	})
	h.requestTimer = requestTimer
	return h
}

func (c *Conn) initializeHealthChecks(ctx *context.T, firstRTT time.Duration) {
	c.lock()
	defer c.unlock()
	if c.hcstate != nil {
		return
	}
	c.hcstate = c.newHealthChecksLocked(ctx, firstRTT)
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
	defer c.unlock()
	c.lock()
	return c.hcstate.closeDeadline
}

// EnterLameDuck enters lame duck mode, the returned channel will be closed when
// the remote end has ack'd or the Conn is closed.
func (c *Conn) EnterLameDuck(ctx *context.T) chan struct{} {
	var enterLameDuck bool
	c.lock()
	if c.status < EnteringLameDuck {
		c.status = EnteringLameDuck
		enterLameDuck = true
	}
	c.unlock()
	if enterLameDuck {
		err := c.sendMessage(ctx, false, expressPriority, &message.EnterLameDuck{})
		if err != nil {
			c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
		}
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
	defer c.unlock()
	c.lock()

	// It may happen that in the case of bidirectional RPC the dialer of the connection
	// has sent blessings,  but not yet discharges.  In this case we will wait for them
	// to send the discharges before allowing a bidirectional flow dial.
	if valid := c.remoteValid; valid != nil && len(c.remoteDischarges) == 0 && len(c.remoteBlessings.ThirdPartyCaveats()) > 0 {
		c.unlock()
		<-valid
		c.lock()
	}

	if c.remoteLameDuck || c.status >= Closing {
		return nil, ErrConnectionClosed.Errorf(ctx, "connection closed")
	}
	id := c.nextFid
	c.nextFid += 2
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
	c.lock()
	localBlessings := c.localBlessings
	c.unlock()
	return localBlessings
}

// RemoteBlessings returns the remote blessings.
func (c *Conn) RemoteBlessings() security.Blessings {
	c.lock()
	defer c.unlock()
	return c.remoteBlessings
}

// LocalDischarges fetches the most recently sent discharges for the local
// ends blessings.
func (c *Conn) LocalDischarges() map[string]security.Discharge {
	c.lock()
	defer c.unlock()
	return c.localDischarges
}

// RemoteDischarges fetches the most recently received discharges for the remote
// ends blessings.
func (c *Conn) RemoteDischarges() map[string]security.Discharge {
	c.lock()
	defer c.unlock()
	// It may happen that in the case of bidirectional RPC the dialer of the connection
	// has sent blessings,  but not yet discharges.  In this case we will wait for them
	// to send the discharges instead of returning the initial nil discharges.
	if valid := c.remoteValid; valid != nil && len(c.remoteDischarges) == 0 && len(c.remoteBlessings.ThirdPartyCaveats()) > 0 {
		c.unlock()
		<-valid
		c.lock()
	}
	return c.remoteDischarges
}

// CommonVersion returns the RPCVersion negotiated between the local and remote endpoints.
func (c *Conn) CommonVersion() version.RPCVersion { return c.version }

// LastUsed returns the time at which the Conn had bytes read or written on it.
func (c *Conn) LastUsed() time.Time {
	defer c.unlock()
	c.lock()
	return c.lastUsedTime
}

// RemoteLameDuck returns true if the other end of the connection has announced that
// it is in lame duck mode indicating that new flows should not be dialed on this
// conn.
func (c *Conn) RemoteLameDuck() bool {
	defer c.unlock()
	c.lock()
	return c.remoteLameDuck
}

// Closed returns a channel that will be closed after the Conn is shutdown.
// After this channel is closed it is guaranteed that all Dial calls will fail
// with an error and no more flows will be sent to the FlowHandler.
func (c *Conn) Closed() <-chan struct{} { return c.closed }

func (c *Conn) Status() Status {
	c.lock()
	defer c.unlock()
	return c.status
}

// Close shuts down a conn.
func (c *Conn) Close(ctx *context.T, err error) {
	c.internalClose(ctx, false, false, err)
	<-c.closed
}

// CloseIfIdle closes the connection if the conn has been idle for idleExpiry,
// returning true if it closed it.
func (c *Conn) CloseIfIdle(ctx *context.T, idleExpiry time.Duration) bool {
	defer c.unlock()
	c.lock()
	if c.isIdleLocked(ctx, idleExpiry) {
		c.internalCloseLocked(ctx, false, false, ErrIdleConnKilled.Errorf(ctx, "connection killed because idle expiry was reached"))
		return true
	}
	return false
}

func (c *Conn) IsIdle(ctx *context.T, idleExpiry time.Duration) bool {
	defer c.unlock()
	c.lock()
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
	defer c.unlock()
	c.lock()
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
	c.lock()
	c.internalCloseLocked(ctx, closedRemotely, closedWhileAccepting, err)
	c.unlock()
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
			cerr := c.sendMessage(ctx, false, tearDownPriority, &message.TearDown{
				Message: msg,
			})
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
		if cerr := c.mp.Close(); cerr != nil {
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
		c.lock()
		c.status = Closed
		close(c.closed)
		c.unlock()
	}(c)
}

func (c *Conn) state() Status {
	c.lock()
	defer c.unlock()
	return c.status
}

func (c *Conn) deleteFlow(fid uint64) {
	c.lock()
	defer c.unlock()
	delete(c.flows, fid)
}

func (c *Conn) fragmentReleaseMessage(ctx *context.T, toRelease map[uint64]uint64, limit int) error {
	if len(toRelease) < limit {
		return c.sendMessage(ctx, false, expressPriority, &message.Release{
			Counters: toRelease,
		})
	}
	for {
		var send, remaining map[uint64]uint64
		rem := len(toRelease) - limit
		if rem <= 0 {
			send = toRelease
		} else {
			send = make(map[uint64]uint64, limit)
			remaining = make(map[uint64]uint64, rem)
			i := 0
			for k, v := range toRelease {
				if i < limit {
					send[k] = v
				} else {
					remaining[k] = v
				}
				i++
			}
		}
		if err := c.sendMessage(ctx, false, expressPriority, &message.Release{
			Counters: send,
		}); err != nil {
			return err
		}
		if remaining == nil {
			break
		}
		toRelease = remaining
	}
	return nil
}

func (c *Conn) sendRelease(ctx *context.T, fid, count uint64) {
	c.lock()
	_, ok := c.flows[fid]
	c.unlock()
	if ok {
		// Handle the case where the flow is already closed but a message
		// is received for it, hence only bump the toRelease value for
		// that flow if it is still active.
		c.flowControl.incrementToRelease(fid, count)
	}
	toRelease := c.flowControl.createReleaseMessageContents(fid, count)
	var err error
	if toRelease != nil {
		delete(toRelease, invalidFlowID)
		err = c.fragmentReleaseMessage(ctx, toRelease, 8000)
	}
	if err != nil {
		c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
	}
}

func (c *Conn) readLoop(ctx *context.T) {
	defer c.loopWG.Done()
	var err error
	var dataMsg message.Data
	for {
		msg, rerr := c.mp.readDataMsg(ctx, nil, &dataMsg)
		if rerr != nil {
			err = ErrRecv.Errorf(ctx, "error reading from: %v: %v", c.remote.String(), rerr)
			break
		}
		if msg == nil {
			if err = c.handleData(ctx, &dataMsg); err != nil {
				break
			}
			continue
		}
		if err = c.handleAnyMessage(ctx, msg); err != nil {
			break
		}
	}
	c.internalClose(ctx, false, false, err)
}

func (c *Conn) markUsed() {
	c.lock()
	c.markUsedLocked()
	c.unlock()
}

func (c *Conn) markUsedLocked() {
	c.lastUsedTime = time.Now()
}

func (c *Conn) IsEncapsulated() bool {
	return c.mp.isEncapsulated()
}

func (c *Conn) DebugString() string {
	defer c.unlock()
	c.lock()
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

func (c *Conn) writeEncodedBlessings(ctx *context.T, w *writer, data []byte) error {
	w.lock()
	defer w.unlock()
	c.writeq.activateAndNotify(w, flowPriority)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-w.notify:
	}
	err := c.mp.writeMsg(ctx, &message.Data{
		ID:      blessingsFlowID,
		Payload: [][]byte{data}})
	c.writeq.deactivateAndNotify(w, flowPriority)
	return err
}

// sendMessage sends a single message on the conn with the given priority.
// It should never be called with the conn lock held since it may block.
// if cancelWithContext is true, then this write attempt will fail when the context
// is canceled.  Otherwise context cancellation will have no effect and this call
// will block until the message is sent.
func (c *Conn) sendMessage(
	ctx *context.T,
	cancelWithContext bool,
	priority int,
	m message.Message) error {
	c.writeqEntry.lock()
	defer c.writeqEntry.unlock()
	c.writeq.activateAndNotify(&c.writeqEntry, priority)
	if cancelWithContext {
		select {
		case <-ctx.Done():
			c.writeq.deactivateAndNotify(&c.writeqEntry, priority)
			return ctx.Err()
		case <-c.writeqEntry.notify:
			//fmt.Printf("%p: sendMessage cancel notified: %v\n", &c.writeqEntry, &c.writeqEntry.notify)
		}
	} else {
		//		fmt.Printf("waiting our turn: %p .. %v\n%s\n", &c.writeqEntry, c.writeqEntry.notify, &c.writeq)
		<-c.writeqEntry.notify
		//		fmt.Printf("got our turn: %p .. %v\n", &c.writeqEntry, c.writeqEntry.notify)
		//fmt.Printf("%p: sendMessage notified: %v\n", &c.writeqEntry, &c.writeqEntry.notify)
	}
	// send the actual message.
	err := c.mp.writeMsg(ctx, m)
	c.writeq.deactivateAndNotify(&c.writeqEntry, priority)
	return err
}

func (c *Conn) lock() {
	//	_, file, line, _ := runtime.Caller(1)
	//	file = filepath.Base(file)
	//	fmt.Printf("%p: locking @ %v:%v\n", c, file, line)
	c.mux.Lock()
	// fmt.Printf("%p: locked @ %v:%v\n", c, file, line)
}

func (c *Conn) unlock() {
	//	_, file, line, _ := runtime.Caller(1)
	//	file = filepath.Base(file)
	c.mux.Unlock()
	// fmt.Printf("%p: unlocked @ %v:%v\n", c, file, line)
}
