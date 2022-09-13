// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"math"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow/message"
)

func (c *Conn) handleAnyMessage(ctx *context.T, m message.Message) error {
	switch msg := m.(type) {
	case *message.Data:
		return c.handleData(ctx, msg)

	case *message.OpenFlow:
		return c.handleOpenFlow(ctx, msg)

	case *message.Release:
		return c.handleRelease(ctx, msg)

	case *message.Auth:
		return c.handleAuth(ctx, msg)

	case *message.HealthCheckRequest:
		return c.handleHealthCheckRequest(ctx)

	case *message.HealthCheckResponse:
		return c.handleHealthCheckResponse(ctx)

	case *message.TearDown:
		return c.handleTearDown(ctx, msg)

	case *message.EnterLameDuck:
		return c.handleEnterLameDuck(ctx, msg)

	case *message.AckLameDuck:
		return c.handleAckLameDuck(ctx, msg)

	default:
		return ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", m)
	}
}

func (c *Conn) handleData(ctx *context.T, msg *message.Data) error {
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
	return nil
}

func (c *Conn) handleOpenFlow(ctx *context.T, msg *message.OpenFlow) error {
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
	c.newFlowCountersLocked(msg.ID)
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

func (c *Conn) handleTearDown(ctx *context.T, msg *message.TearDown) error {
	var err error
	if msg.Message != "" {
		err = ErrRemoteError.Errorf(ctx, "remote end received err: %v", msg.Message)
	}
	c.internalClose(ctx, true, false, err)
	return nil
}

func (c *Conn) handleEnterLameDuck(ctx *context.T, msg *message.EnterLameDuck) error {
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
	return nil
}

func (c *Conn) handleAckLameDuck(ctx *context.T, msg *message.AckLameDuck) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status < LameDuckAcknowledged {
		c.status = LameDuckAcknowledged
		close(c.lameDucked)
	}
	return nil
}

func (c *Conn) handleHealthCheckResponse(ctx *context.T) error {
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
	return nil
}

func (c *Conn) handleHealthCheckRequest(ctx *context.T) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sendMessageLocked(ctx, true, expressPriority, &message.HealthCheckResponse{})
	return nil
}

func (c *Conn) handleRelease(ctx *context.T, msg *message.Release) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for fid, val := range msg.Counters {
		if f := c.flows[fid]; f != nil {
			f.releaseLocked(val)
		} else {
			c.releaseOutstandingBorrowedLocked(fid, val)
		}
	}
	return nil
}

func (c *Conn) handleAuth(ctx *context.T, msg *message.Auth) error {
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
