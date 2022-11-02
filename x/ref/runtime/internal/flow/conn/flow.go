// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"io"
	"net"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/security"
)

type flw struct {
	// These variables are all set during flow construction.
	id                                uint64
	conn                              *Conn
	q                                 *readq
	localBlessings, remoteBlessings   security.Blessings
	localDischarges, remoteDischarges map[string]security.Discharge
	noEncrypt                         bool
	noFragment                        bool
	remote                            naming.Endpoint
	channelTimeout                    time.Duration
	sideChannel                       bool
	encapsulated                      bool

	// The following fields for managing flow control and the writeq
	// are locked indepdently.
	flowControl flowControlFlowStats

	writeq      *writeq
	writeqEntry writer

	// mu guards all of the following fields.
	mu     sync.Mutex
	ctx    *context.T
	cancel context.CancelFunc

	// opened indicates whether the flow has already been opened.  If false
	// we need to send an open flow on the next write.  For accepted flows
	// this will always be true.
	opened bool

	// closed is true as soon as f.close has been called.
	closed bool

	// tokenWait is set if a write is blocked waiting for flowcontrol
	// tokens. When tokens become available the channel should be closed.
	tokenWait chan struct{}
}

// Ensure that *flw implements flow.Flow.
var _ flow.Flow = &flw{}

func (c *Conn) newFlowLocked(
	ctx *context.T,
	id uint64,
	localBlessings, remoteBlessings security.Blessings,
	localDischarges, remoteDischarges map[string]security.Discharge,
	remote naming.Endpoint,
	dialed bool,
	channelTimeout time.Duration,
	sideChannel bool,
	initialTokens uint64) *flw {
	f := &flw{
		id:               id,
		conn:             c,
		localBlessings:   localBlessings,
		localDischarges:  localDischarges,
		remoteBlessings:  remoteBlessings,
		remoteDischarges: remoteDischarges,
		opened:           !dialed,
		remote:           remote,
		channelTimeout:   channelTimeout,
		sideChannel:      sideChannel,
		writeq:           &c.writeq,
		encapsulated:     c.IsEncapsulated(),
	}
	// It's important that this channel has a non-zero buffer since flows will
	// be notifying themselve and if there's no buffer a deadlock will occur.
	// The self notification is between the code that handles release
	// messages to notify a flow-controlled writeMgs that it may potentially
	// have tokens to spend on writes.
	f.writeqEntry.notify = make(chan struct{}, 1)

	f.flowControl.init(&c.flowControl, id, dialed, !dialed, initialTokens)

	f.tokenWait = make(chan struct{}, 1)

	f.q = newReadQ(f.flowControl.shared.bytesBufferedPerFlow, f.sendRelease)

	f.ctx, f.cancel = context.WithCancel(ctx)

	if !f.opened {
		c.unopenedFlows.Add(1)
	}

	c.flows[id] = f
	c.healthCheckNewFlowLocked(ctx, channelTimeout)
	return f
}

func (f *flw) sendRelease(ctx *context.T, n int) {
	f.conn.sendRelease(ctx, &f.flowControl, uint64(n))
}

// disableEncrytion should not be called concurrently with Write* methods.
func (f *flw) disableEncryption() {
	f.noEncrypt = true
}

// DisableFragmentation should probably not be called concurrently with
// Write* methods.
func (f *flw) DisableFragmentation() {
	f.noFragment = true
}

// Implement io.Reader.
// Read and ReadMsg should not be called concurrently with themselves
// or each other.
func (f *flw) Read(p []byte) (n int, err error) {
	ctx := f.currentContext()
	f.markUsed()
	if n, err = f.q.read(ctx, p); err != nil {
		f.close(ctx, false, err)
	}
	return
}

// ReadMsg is like read, but it reads bytes in chunks.  Depending on the
// implementation the batch boundaries might or might not be significant.
// Read and ReadMsg should not be called concurrently with themselves
// or each other.
func (f *flw) ReadMsg() (buf []byte, err error) {
	ctx := f.currentContext()
	f.markUsed()
	// TODO(mattr): Currently we only ever release counters when some flow
	// reads.  We may need to do it more or less often.  Currently
	// we'll send counters whenever a new flow is opened.
	if buf, err = f.q.get(ctx); err != nil {
		f.close(ctx, false, err)
	}
	return
}

// ReadMsg2 is like ReadMsg. In this implementation it does not use the
// supplied buffer since doing so would force an extraneous allocation and
// copy.
func (f *flw) ReadMsg2(_ []byte) (buf []byte, err error) {
	return f.ReadMsg()
}

// Implement io.Writer.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) Write(p []byte) (n int, err error) {
	return f.WriteMsg(p)
}

func (f *flw) currentContext() *context.T {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ctx
}

// ensureTokens ensures that there are sufficient flow control tokens available
// to send the current message, and if there are none, it will wait until
// some become available from the peer end of the connection via a release
// message containing tokens for this flow.
func (f *flw) ensureTokens(ctx *context.T, need int, parts [][]byte) ([][]byte, []byte, int, error) {

	for {
		// The critical region needs to capture both reading tokens and
		// setting up to wait for new ones. Similarly, releaseCounters
		// above needs to lock access to updating the available tokens
		// (ie. the call to releaseCounters) as well as signaling the channel
		// used to notify the code below that more tokens may be available.
		f.flowControl.lockForTokens()
		borrowed, borrowedNotification, avail := f.flowControl.getTokensLocked(ctx, f.encapsulated)
		if avail >= need {
			parts, tosend, size := popFront(parts, avail)
			f.flowControl.returnTokensLocked(ctx, borrowed, avail-size)
			f.flowControl.unlockForTokens()
			return parts, tosend, size, nil
		}
		// need to wait for tokens, but first return any tokens that were available.
		f.flowControl.returnTokensLocked(ctx, borrowed, avail)
		ch := f.tokenWait
		f.flowControl.unlockForTokens()
		select {
		case <-ctx.Done():
			return nil, nil, 0, ctx.Err()
		case <-ch:
		case <-borrowedNotification:
		}
	}
}

func (f *flw) writeMsgDone(ctx *context.T, sent int, alsoClose bool, err error) (int, error) {
	if alsoClose || err != nil {
		if err != nil && ctx.V(2) {
			f.ctx.Infof("finishing write on %d(%p): failed: %v", f.id, f, err)
		}
		f.close(ctx, false, err)
		// Translate context cancelation or timeout into io.EOF
		if err == context.Canceled || err == context.DeadlineExceeded {
			return sent, io.EOF
		}
		return sent, err
	}
	return sent, nil
}

func (f *flw) writeMsg(alsoClose bool, parts [][]byte) (sent int, err error) {
	ctx := f.currentContext()

	debug := ctx.V(2)
	if debug {
		ctx.Infof("starting write on flow %d(%p)", f.id, f)
	}

	totalSize := 0
	for _, p := range parts {
		totalSize += len(p)
	}
	var tosend []byte
	size, sent := 0, 0

	f.markUsed()

	f.mu.Lock()
	wasOpened := f.opened
	f.opened = true
	f.mu.Unlock()

	// We're prepared to send at least 1 byte under any circumstance.
	need := 1
	if f.noEncrypt || f.noFragment {
		// Note that if f.noEncrypt is set we're actually acting as a conn
		// for higher level flows. In this case we don't want to fragment the
		// writes of the higher level flows, we want to transmit their messages
		// whole. Similarly if noFragment is set we prefer to wait and not
		// send a partial write based on the available tokens. This is only
		// required by proxies which need to pass on messages without
		// refragmenting.
		need = totalSize
	}

	for len(parts) > 0 {
		if wasOpened {
			// Send a data message, possibly having to wait for flow control
			// tokens before doing so.
			parts, tosend, size, err = f.ensureTokens(ctx, need, parts)
			if err != nil {
				return f.writeMsgDone(ctx, sent, alsoClose, err)
			}
			if err := f.sendDataMessage(ctx, flowPriority, alsoClose, len(parts) == 0, tosend); err != nil {
				return f.writeMsgDone(ctx, sent, alsoClose, err)
			}
			sent += size
			continue
		}
		// Send an open message, use any available flow control tokens, but
		// do not wait for any to be available. This ensures that an openFlow
		// message will always be sent, if it can't include a payload.
		f.flowControl.lockForTokens()
		borrowed, _, avail := f.flowControl.getTokensLocked(ctx, f.encapsulated)
		parts, tosend, size = popFront(parts, avail)
		f.flowControl.returnTokensLocked(ctx, borrowed, avail-size)
		f.flowControl.unlockForTokens()

		if err := f.sendOpenFlow(ctx, alsoClose, len(parts) == 0, tosend); err != nil {
			return f.writeMsgDone(ctx, sent, alsoClose, err)
		}

		sent += size

		wasOpened = true
		f.conn.unopenedFlows.Done()
	}
	if debug {
		f.ctx.Infof("finishing write on %d(%p)", f.id, f)
	}
	return f.writeMsgDone(ctx, sent, alsoClose, err)
}

func (f *flw) messageFlags(alsoClose, finalPart bool) uint64 {
	var flags uint64
	if alsoClose && finalPart {
		flags |= message.CloseFlag
	}
	if f.noEncrypt {
		flags |= message.DisableEncryptionFlag
	}
	return flags
}

func (f *flw) sendOpenFlow(ctx *context.T, alsoClose, finalPart bool, payload []byte) error {
	bkey, dkey, err := f.conn.blessingsFlow.send(ctx, f.localBlessings, f.localDischarges, nil)
	if err != nil {
		return err
	}
	flags := f.messageFlags(alsoClose, finalPart)
	if err := f.writeq.wait(ctx, &f.writeqEntry, flowPriority); err != nil {
		if ctx.V(2) {
			ctx.Infof("writeq.wait: error %v", err)
		}
		return io.EOF
	}
	defer f.writeq.done(&f.writeqEntry)
	// Actually write to the wire.  This is also where encryption happens,
	// so this part can be slow.
	return f.conn.mp.writeOpenFlow(ctx, message.OpenFlow{
		ID:              f.id,
		InitialCounters: f.flowControl.shared.bytesBufferedPerFlow,
		BlessingsKey:    bkey,
		DischargeKey:    dkey,
		Flags:           flags,
		Payload:         payload,
	})
}

func (f *flw) sendDataMessage(ctx *context.T, priority int, alsoClose, finalPart bool, payload []byte) error {
	flags := f.messageFlags(alsoClose, finalPart)
	if err := f.writeq.wait(ctx, &f.writeqEntry, priority); err != nil {
		if ctx.V(2) {
			ctx.Infof("writeq.wait: error %v", err)
		}
		return io.EOF
	}
	defer f.writeq.done(&f.writeqEntry)
	// Actually write to the wire.  This is also where encryption happens,
	// so this part can be slow.
	return f.conn.mp.writeData(ctx, message.Data{ID: f.id, Flags: flags, Payload: payload})
}

// WriteMsg is like Write, but allows writing more than one buffer at a time.
// The data in each buffer is written sequentially onto the flow.  Returns the
// number of bytes written.  WriteMsg must return a non-nil error if it writes
// less than the total number of bytes from all buffers.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsg(parts ...[]byte) (int, error) {
	return f.writeMsg(false, parts)
}

// WriteMsgAndClose performs WriteMsg and then closes the flow.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsgAndClose(parts ...[]byte) (int, error) {
	return f.writeMsg(true, parts)
}

// SetContext sets the context associated with the flow.  Typically this is
// used to set state that is only available after the flow is connected, such
// as a more restricted flow timeout, or the language of the request.
// Calling SetContext may invalidate values previously returned from Closed.
//
// The flow.Manager associated with ctx must be the same flow.Manager that the
// flow was dialed or accepted from, otherwise an error is returned.
// TODO(mattr): enforce this restriction.
//
// TODO(mattr): update v23/flow documentation.
// SetContext may not be called concurrently with other methods.
func (f *flw) SetDeadlineContext(ctx *context.T, deadline time.Time) *context.T {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		// If the flow is already closed, don't allocate a new
		// context, we might end up leaking it.
		return ctx
	}
	if f.cancel != nil {
		f.cancel()
	}
	if !deadline.IsZero() {
		f.ctx, f.cancel = context.WithDeadline(ctx, deadline)
	} else {
		f.ctx, f.cancel = context.WithCancel(ctx)
	}
	return f.ctx
}

// LocalEndpoint returns the local vanadium endpoint.
func (f *flw) LocalEndpoint() naming.Endpoint {
	return f.conn.local
}

// RemoteEndpoint returns the remote vanadium endpoint.
func (f *flw) RemoteEndpoint() naming.Endpoint {
	return f.remote
}

// RemoteAddr returns the remote address of the peer.
func (f *flw) RemoteAddr() net.Addr {
	return f.conn.remoteAddr
}

// LocalBlessings returns the blessings presented by the local end of the flow
// during authentication.
func (f *flw) LocalBlessings() security.Blessings {
	return f.localBlessings
}

// RemoteBlessings returns the blessings presented by the remote end of the
// flow during authentication.
func (f *flw) RemoteBlessings() security.Blessings {
	return f.remoteBlessings
}

// LocalDischarges returns the discharges presented by the local end of the
// flow during authentication.
//
// Discharges are organized in a map keyed by the discharge-identifier.
func (f *flw) LocalDischarges() map[string]security.Discharge {
	return f.localDischarges
}

// RemoteDischarges returns the discharges presented by the remote end of the
// flow during authentication.
//
// Discharges are organized in a map keyed by the discharge-identifier.
func (f *flw) RemoteDischarges() map[string]security.Discharge {
	return f.remoteDischarges
}

// Conn returns the connection the flow is multiplexed on.
func (f *flw) Conn() flow.ManagedConn {
	return f.conn
}

// Closed returns a channel that remains open until the flow has been closed remotely
// or the context attached to the flow has been canceled.
//
// Note that after the returned channel is closed starting new writes will result
// in an error, but reads of previously queued data are still possible.  No
// new data will be queued.
// TODO(mattr): update v23/flow docs.
func (f *flw) Closed() <-chan struct{} {
	return f.currentContext().Done()
}

// ID returns the ID of this flow.
func (f *flw) ID() uint64 {
	return f.id
}

func (f *flw) close(ctx *context.T, closedRemotely bool, err error) {
	f.mu.Lock()
	wasOpened, wasClosed := f.opened, f.closed
	f.opened, f.closed = true, true
	cancel := f.cancel
	f.mu.Unlock()

	if wasClosed {
		return
	}

	// Delete the flow as soon as possible to ensure that flow control
	// releases are handled appropriately for a closed/closing flow.
	// From this point on, all flow control updates are handled as per
	// a closed flow.
	f.conn.deleteFlow(f.id)

	if !wasOpened {
		f.conn.unopenedFlows.Done()
	}

	f.q.close(ctx)

	log := ctx.V(2)
	if log {
		ctx.Infof("closing %d(%p): %v", f.id, f, err)
	}

	cancel()
	// After cancel has been called no new writes will begin for this
	// flow.  There may be a write in progress, but it must finish
	// before another writer gets to use the channel.  Therefore we
	// can simply use sendMessage to send the close flow message.

	if wasOpened && !closedRemotely && (f.conn.state() != Closing) {
		// Note: If the conn is closing there is no point in trying to
		// send the flow close message as it will fail.  This is racy
		// with the connection closing, but there are no ill-effects
		// other than spamming the logs a little so it's OK.
		if serr := f.conn.sendCloseMessage(ctx, f.id); serr != nil && log {
			ctx.Infof("could not send close flow message: %v: close error (if any): %v", serr, err)
		}
	}
	// Update flow control state now that we're guaranteed to no longer
	// send any messages. Messages may still arrive if the flow is closed
	// locally but not remotely, but they will be handled appropriately
	// with regard to flow control.
	f.flowControl.handleFlowClose(closedRemotely, f.conn.state() < Closing)
}

// Close marks the flow as closed. After Close is called, new data cannot be
// written on the flow. Reads of already queued data are still possible.
func (f *flw) Close() error {
	f.close(f.currentContext(), false, nil)
	return nil
}

func (f *flw) markUsed() {
	if !f.sideChannel && f.id >= reservedFlows {
		f.conn.markUsed()
	}
}

// popFront removes at most num bytes from in, returning the remaining
// bytes as a new slice of buffers. The returned number of bytes is always
// the same as the length of the returned buffer.
func popFront(in [][]byte, num int) ([][]byte, []byte, int) {
	nIn := len(in)
	if nIn == 0 {
		return nil, nil, 0
	}
	if l0 := len(in[0]); nIn == 1 {
		// common case, so make it simple and fast.
		if num >= l0 {
			return in[1:], in[0], l0
		}
		return [][]byte{in[0][num:]}, in[0][:num], num
	}
	out := in[0]
	i, total, offset := 0, 0, 0
	for i < nIn {
		li := len(in[i])
		total += li
		if total > num {
			offset = li - (total - num)
			total = num
			if i > 0 {
				out = append(out, in[i][:offset]...)
			}
			break
		}
		if i > 0 {
			out = append(out, in[i]...)
		}
		i++
	}
	if i == nIn {
		return nil, out[:total], total
	}
	rem := make([][]byte, nIn-i)
	copy(rem, in[i:])
	rem[0] = in[i][offset:]
	return rem, out[:total], total
}
