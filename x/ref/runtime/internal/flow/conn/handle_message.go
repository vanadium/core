// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"os"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow/message"
	"v.io/x/lib/vlog"
)

func (c *Conn) handleAnyMessage(ctx *context.T, m message.Message, nBuf *netBuf) error {
	var err error
	fmt.Printf("handle %T\n", m)
	switch msg := m.(type) {
	case message.Data:
		return c.handleData(ctx, msg, nBuf)
	case message.OpenFlow:
		return c.handleOpenFlow(ctx, msg, nBuf)
	case message.Release:
		err = c.handleRelease(ctx, msg)
	case message.Auth:
		err = c.handleAuth(ctx, msg)
	case message.HealthCheckRequest:
		err = c.handleHealthCheckRequest(ctx)
	case message.HealthCheckResponse:
		err = c.handleHealthCheckResponse(ctx)
	case message.TearDown:
		err = c.handleTearDown(ctx, msg)
	case message.EnterLameDuck:
		err = c.handleEnterLameDuck(ctx, msg)
	case message.AckLameDuck:
		err = c.handleAckLameDuck(ctx, msg)
	default:
		putNetBuf(nBuf)
		return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", m)
	}
	putNetBuf(nBuf)
	return err
}

func (c *Conn) handleData(ctx *context.T, msg message.Data, nBuf *netBuf) error {
	c.mu.Lock()
	if c.status == Closing {
		c.mu.Unlock()
		putNetBuf(nBuf)
		return nil // Conn is already being shut down.
	}
	if msg.ID == blessingsFlowID {
		c.mu.Unlock()
		err := c.blessingsFlow.writeMsg(msg.Payload)
		putNetBuf(nBuf)
		return err
	}
	f := c.flows[msg.ID]
	if f == nil {
		// The data message likely has the CloseFlag set but we treat
		// all messages received for a locally close flow the same way.
		c.flowControl.releaseOutstandingBorrowedClosed(msg.ID)
		c.mu.Unlock()
		putNetBuf(nBuf)
		return nil
	}
	c.mu.Unlock()
	if err := f.q.put(ctx, msg.Payload, nBuf); err != nil {
		putNetBuf(nBuf)
		return err
	}
	if msg.Flags&message.CloseFlag != 0 {
		f.close(ctx, true, nil)
	}
	return nil
}

func (c *Conn) handleOpenFlow(ctx *context.T, msg message.OpenFlow, nBuf *netBuf) error {
	remoteBlessings, remoteDischarges, err := c.blessingsFlow.getRemote(
		ctx, msg.BlessingsKey, msg.DischargeKey)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.nextFid%2 == msg.ID%2 {
		c.mu.Unlock()
		putNetBuf(nBuf)
		return ErrInvalidPeerFlow.Errorf(ctx, "peer has chosen flow id from local domain")
	}
	if c.handler == nil {
		c.mu.Unlock()
		putNetBuf(nBuf)
		return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", msg)
	} else if c.status == Closing {
		c.mu.Unlock()
		putNetBuf(nBuf)
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
		sideChannel,
		msg.InitialCounters)
	c.flowControl.newCounters(&f.flowControl)
	c.mu.Unlock()

	c.handler.HandleFlow(f) //nolint:errcheck

	if err := f.q.put(ctx, msg.Payload, nBuf); err != nil {
		putNetBuf(nBuf)
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
		err := c.sendLameDuckMessage(ctx, true, false)
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
	c.sendHealthCheckMessage(ctx, false)
	return nil
}

func (c *Conn) handleRelease(ctx *context.T, msg message.Release) error {
	c.flowControl.handleRelease(ctx, c, msg.Counters)
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
	fmt.Fprintf(os.Stderr, "%p: handleAuth: %v\n", c, len(discharges))

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
		msg, nBuf, err := c.mp.readAnyMsg(ctx)
		if err != nil {
			return message.Auth{}, ErrRecv.Errorf(ctx, "conn.readRemoteAuth: error reading from %v: %v", c.remoteEndpointForError(), err)
		}
		if rauth, ok := msg.(message.Auth); ok {
			fmt.Fprintf(os.Stderr, "%p: readRemoteAuth: %#v\n", c, rauth)

			defer putNetBuf(nBuf)
			// Take care to return a copy of the message in order to allow
			// the netBuf to be freed.
			return rauth.CopyDirect(), nil
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
			putNetBuf(nBuf)
			return message.Auth{}, ErrConnectionClosed.Errorf(ctx, "conn.readRemoteAuth: connection closed")
		case message.OpenFlow:
			// If we get an OpenFlow message here it needs to be handled
			// asynchronously since it will call the flow handler
			// which will block until NewAccepted (which calls
			// this method) returns. OpenFlow is generally expected
			// to be handled by readLoop.

			// Take a copy of the message since and free the netBuf here.
			m = m.CopyDirect()
			putNetBuf(nBuf)
			go func(m message.OpenFlow) {
				if err := c.handleOpenFlow(ctx, m, nil); err != nil {
					vlog.Infof("conn.readRemoteAuth: handleMessage for openFlow for flow %v: failed: %v", m.ID, err)
				}
			}(m)
			continue
		}
		if err = c.handleAnyMessage(ctx, msg, nBuf); err != nil {
			return message.Auth{}, err
		}
	}
}
