// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"math"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow/message"
	"v.io/x/lib/vlog"
)

func (c *Conn) handleAnyMessage(ctx *context.T, m message.Message) error {
	switch msg := m.(type) {
	case message.Data:
		return c.handleData(ctx, msg)

	case message.OpenFlow:
		return c.handleOpenFlow(ctx, msg)

	case message.Release:
		return c.handleRelease(ctx, msg)

	case message.Auth:
		return c.handleAuth(ctx, msg)

	case message.HealthCheckRequest:
		return c.handleHealthCheckRequest(ctx)

	case message.HealthCheckResponse:
		return c.handleHealthCheckResponse(ctx)

	case message.TearDown:
		return c.handleTearDown(ctx, msg)

	case message.EnterLameDuck:
		return c.handleEnterLameDuck(ctx, msg)

	case message.AckLameDuck:
		return c.handleAckLameDuck(ctx, msg)

	default:
		return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", m)
	}
}

func (c *Conn) handleData(ctx *context.T, msg message.Data) error {
	c.mu.Lock()
	if c.status == Closing {
		c.mu.Unlock()
		return nil // Conn is already being shut down.
	}
	if msg.ID == blessingsFlowID {
		c.mu.Unlock()
		return c.blessingsFlow.writeMsg(msg.Payload)
	}
	f := c.flows[msg.ID]
	if f == nil {
		// If the flow is closing then we assume the remote side releases
		// all borrowed counters for that flow.
		c.flowControl.releaseOutstandingBorrowed(msg.ID, math.MaxUint64)
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
	return nil
}

func (c *Conn) handleOpenFlow(ctx *context.T, msg message.OpenFlow) error {
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
		c.acceptChannelTimeout,
		sideChannel)
	f.releaseCounters(msg.InitialCounters)
	c.flowControl.newCounters(msg.ID)
	c.mu.Unlock()

	c.handler.HandleFlow(f) //nolint:errcheck

	if err := f.q.put(ctx, msg.Payload); err != nil {
		return err
	}
	if msg.Flags&message.CloseFlag != 0 {
		f.close(ctx, true, nil)
	}
	return nil
}

func (c *Conn) handleTearDown(ctx *context.T, msg message.TearDown) error {
	var err error
	if msg.Message != "" {
		err = ErrRemoteError.Errorf(ctx, "remote end received err: %v", msg.Message)
	}
	c.internalClose(ctx, true, false, err)
	return nil
}

func (c *Conn) handleEnterLameDuck(ctx *context.T, msg message.EnterLameDuck) error {
	c.mu.Lock()
	c.remoteLameDuck = true
	c.mu.Unlock()
	go func() {
		// We only want to send the lame duck acknowledgment after all outstanding
		// OpenFlows are sent.
		c.unopenedFlows.Wait()
		err := c.sendMessage(ctx, true, expressPriority, message.AckLameDuck{})
		if err != nil {
			c.Close(ctx, ErrSend.Errorf(ctx, "failure sending release message to %v: %v", c.remote.String(), err))
		}
	}()
	return nil
}

func (c *Conn) handleAckLameDuck(ctx *context.T, msg message.AckLameDuck) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status < LameDuckAcknowledged {
		c.status = LameDuckAcknowledged
		close(c.lameDucked)
	}
	return nil
}

func (c *Conn) handleHealthCheckResponse(ctx *context.T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	return nil
}

func (c *Conn) handleHealthCheckRequest(ctx *context.T) error {
	c.sendMessage(ctx, true, expressPriority, message.HealthCheckResponse{})
	return nil
}

func (c *Conn) handleRelease(ctx *context.T, msg message.Release) error {
	for fid, val := range msg.Counters {
		c.mu.Lock()
		f := c.flows[fid]
		c.mu.Unlock()
		if f != nil {
			f.releaseCounters(val)
		} else {
			c.flowControl.releaseOutstandingBorrowed(fid, val)
		}
	}
	return nil
}

func (c *Conn) handleAuth(ctx *context.T, msg message.Auth) error {
	// handles a blessings refresh, as sent by blessingsLoop.
	blessings, discharges, err := c.blessingsFlow.getRemote(
		ctx, msg.BlessingsKey, msg.DischargeKey)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.remoteBlessings = blessings
	c.remoteDischarges = discharges
	if c.remoteValid != nil {
		close(c.remoteValid)
		c.remoteValid = make(chan struct{})
	}
	return nil
}

func (c *Conn) remoteEndpointForError() string {
	if c.remote.IsZero() {
		return ""
	}
	return c.remote.String()
}

// handleRemoteAuth reads Data messages (containing blessings) until it sees
// an Auth message that indicates the end of the blessings and hence the
// auth handshake. It may encounter a TearDown message if the remote end
// does trust the new diealer. It is called from accepthHandshake and
// dialHandshake and therefore runs aysnchronously to the other message
// loops and hence must be prepared to handle all message types, although
// in practice this happens extremely very rarely. Note that the Data
// messages will be addressed to the blessings flow, ie. flow ID 1.
func (c *Conn) readRemoteAuthLoop(ctx *context.T) (message.Auth, error) {
	for {
		msg, err := c.mp.readMsg(ctx, nil)
		if err != nil {
			return message.Auth{}, ErrRecv.Errorf(ctx, "conn.readRemoteAuth: error reading from %v: %v", c.remoteEndpointForError(), err)
		}
		if rauth, ok := msg.(message.Auth); ok {
			return rauth, nil
		}
		switch m := msg.(type) {
		case message.TearDown:
			// A teardown message may be sent by the client if it decides
			// that it doesn't trust the server. We handle it here and return
			// a connection closed error rather than waiting for the readMsg
			// above to fail when it tries to read from the closed connection.
			if err := c.handleTearDown(ctx, m); err != nil {
				vlog.Infof("conn.readRemoteAuth: handleMessage teardown: failed: %v", err)
			}
			return message.Auth{}, ErrConnectionClosed.Errorf(ctx, "conn.readRemoteAuth: connection closed")
		case message.OpenFlow:
			// If we get an OpenFlow message here it needs to be handled
			// asynchronously since it will call the flow handler
			// which will block until NewAccepted (which calls
			// this method) returns. OpenFlow is generally expected
			// to be handled by readLoop.
			go func() {
				if err := c.handleOpenFlow(ctx, m); err != nil {
					vlog.Infof("conn.readRemoteAuth: handleMessage for openFlow for flow %v: failed: %v", m.ID, err)
				}
			}()
			continue
		}
		if err = c.handleAnyMessage(ctx, msg); err != nil {
			return message.Auth{}, err
		}
	}
}
