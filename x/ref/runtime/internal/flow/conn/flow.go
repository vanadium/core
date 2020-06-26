// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"io"
	"net"
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
	writeCh                           chan struct{}
	remote                            naming.Endpoint
	channelTimeout                    time.Duration
	sideChannel                       bool

	// NOTE: The remaining variables are actually protected by conn.mu.

	ctx    *context.T
	cancel context.CancelFunc

	// opened indicates whether the flow has already been opened.  If false
	// we need to send an open flow on the next write.  For accepted flows
	// this will always be true.
	opened bool
	// writing is true if we're in the middle of a write to this flow.
	writing bool
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

	closed bool

	writerList
}

// Ensure that *flw implements flow.Flow.
var _ flow.Flow = &flw{}

func (c *Conn) newFlowLocked(
	ctx *context.T,
	id uint64,
	localBlessings, remoteBlessings security.Blessings,
	localDischarges, remoteDischarges map[string]security.Discharge,
	remote naming.Endpoint,
	dialed, preopen bool,
	channelTimeout time.Duration,
	sideChannel bool) *flw {
	f := &flw{
		id:               id,
		conn:             c,
		q:                newReadQ(c, id),
		localBlessings:   localBlessings,
		localDischarges:  localDischarges,
		remoteBlessings:  remoteBlessings,
		remoteDischarges: remoteDischarges,
		opened:           preopen,
		borrowing:        dialed,
		// It's important that this channel has a non-zero buffer.  Sometimes this
		// flow will be notifying itself, so if there's no buffer a deadlock will
		// occur.
		writeCh:        make(chan struct{}, 1),
		remote:         remote,
		channelTimeout: channelTimeout,
		sideChannel:    sideChannel,
	}
	f.next, f.prev = f, f
	f.ctx, f.cancel = context.WithCancel(ctx)
	if !f.opened {
		c.unopenedFlows.Add(1)
	}
	c.flows[id] = f
	c.healthCheckNewFlowLocked(ctx, channelTimeout)
	return f
}

// Implement the writer interface.
func (f *flw) notify()       { f.writeCh <- struct{}{} }
func (f *flw) priority() int { return flowPriority }

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
	f.markUsed()
	if n, err = f.q.read(f.currentContext(), p); err != nil {
		f.close(f.currentContext(), false, err)
	}
	return
}

// ReadMsg is like read, but it reads bytes in chunks.  Depending on the
// implementation the batch boundaries might or might not be significant.
// Read and ReadMsg should not be called concurrently with themselves
// or each other.
func (f *flw) ReadMsg() (buf []byte, err error) {
	f.markUsed()
	// TODO(mattr): Currently we only ever release counters when some flow
	// reads.  We may need to do it more or less often.  Currently
	// we'll send counters whenever a new flow is opened.
	if buf, err = f.q.get(f.currentContext()); err != nil {
		f.close(f.currentContext(), false, err)
	}
	return
}

// Implement io.Writer.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) Write(p []byte) (n int, err error) {
	return f.WriteMsg(p)
}

// tokensLocked returns the number of tokens this flow can send right now.
// It is bounded by the channel mtu, the released counters, and possibly
// the number of shared counters for the conn if we are sending on a just
// dialed flow.
func (f *flw) tokensLocked() (int, func(int)) {
	max := f.conn.mtu
	// When	our flow is proxied (i.e. encapsulated), the proxy has added overhead
	// when forwarding the message. This means we must reduce our mtu to ensure
	// that dialer framing reaches the acceptor without being truncated by the
	// proxy.
	if f.conn.IsEncapsulated() {
		max -= proxyOverhead
	}
	if f.borrowing {
		if f.conn.lshared < max {
			max = f.conn.lshared
		}
		return int(max), func(used int) {
			f.conn.lshared -= uint64(used)
			f.borrowed += uint64(used)
			if f.ctx.V(2) {
				f.ctx.Infof("deducting %d borrowed tokens on flow %d(%p), total: %d left: %d", used, f.id, f, f.borrowed, f.conn.lshared)
			}
		}
	}
	if f.released < max {
		max = f.released
	}
	return int(max), func(used int) {
		f.released -= uint64(used)
		if f.ctx.V(2) {
			f.ctx.Infof("flow %d(%p) deducting %d tokens, %d left", f.id, f, used, f.released)
		}
	}
}

// releaseLocked releases some counters from a remote reader to the local
// writer.  This allows the writer to then write more data to the wire.
func (f *flw) releaseLocked(tokens uint64) {
	debug := f.ctx.V(2)
	f.borrowing = false
	if f.borrowed > 0 {
		n := tokens
		if f.borrowed < tokens {
			n = f.borrowed
		}
		if debug {
			f.ctx.Infof("Returning %d/%d tokens borrowed by %d(%p) shared: %d", n, tokens, f.id, f, f.conn.lshared)
		}
		tokens -= n
		f.borrowed -= n
		f.conn.lshared += n
	}
	f.released += tokens
	if debug {
		f.ctx.Infof("Tokens release to %d(%p): %d => %d", f.id, f, tokens, f.released)
	}
	if f.writing {
		if debug {
			f.ctx.Infof("Activating writing flow %d(%p) now that we have tokens.", f.id, f)
		}
		f.conn.activateWriterLocked(f)
		f.conn.notifyNextWriterLocked(nil)
	}
}

func (f *flw) useCurrentContext(ctx *context.T) {
	f.conn.mu.Lock()
	defer f.conn.mu.Unlock()
	f.ctx = ctx
}

func (f *flw) currentContext() *context.T {
	f.conn.mu.Lock()
	defer f.conn.mu.Unlock()
	return f.ctx
}

func (f *flw) currentContextLocked() *context.T {
	return f.ctx
}

func (f *flw) writeMsg(alsoClose bool, parts ...[]byte) (sent int, err error) { //nolint:gocyclo
	ctx := f.currentContext()
	select {
	// Catch cancellations early.  If we caught a cancel when waiting
	// our turn below its possible that we were notified simultaneously.
	// Then the notify channel will be full and we would deadlock
	// notifying ourselves.
	case <-ctx.Done():
		f.close(ctx, false, ctx.Err())
		return 0, io.EOF
	default:
	}

	bkey, dkey, err := f.conn.blessingsFlow.send(ctx, f.localBlessings, f.localDischarges, nil)
	if err != nil {
		return 0, err
	}

	debug := f.ctx.V(2)
	if debug {
		ctx.Infof("starting write on flow %d(%p)", f.id, f)
	}

	totalSize := 0
	for _, p := range parts {
		totalSize += len(p)
	}
	size, sent, tosend := 0, 0, make([][]byte, len(parts))
	f.conn.mu.Lock()
	f.markUsedLocked()
	f.writing = true
	f.conn.activateWriterLocked(f)
	for err == nil && len(parts) > 0 {
		f.conn.notifyNextWriterLocked(f)

		// Wait for our turn.
		f.conn.mu.Unlock()
		select {
		case <-ctx.Done():
			err = io.EOF
		case <-f.writeCh:
		}

		// It's our turn, we lock to learn the current state of our buffer tokens.
		f.conn.mu.Lock()
		if err != nil {
			break
		}
		opened := f.opened
		tokens, deduct := f.tokensLocked()
		if opened && (tokens == 0 || ((f.noEncrypt || f.noFragment) && (tokens < totalSize))) {
			// Oops, we really don't have data to send, probably because we've exhausted
			// the remote buffer.  deactivate ourselves but keep trying.
			// Note that if f.noEncrypt is set we're actually acting as a conn
			// for higher level flows.  In this case we don't want to fragment the writes
			// of the higher level flows, we want to transmit their messages whole.
			// Similarly if noFragment is set we prefer to wait and not to send
			// a partial write based on the available tokens. This is only required
			// by proxies which need to pass on messages without refragmenting.
			if debug {
				f.currentContextLocked().Infof("Deactivating write on flow %d(%p) due to lack of tokens", f.id, f)
			}
			f.conn.deactivateWriterLocked(f)
			continue
		}
		parts, tosend, size = popFront(parts, tosend[:0], tokens)
		deduct(size)
		f.conn.mu.Unlock()

		// Actually write to the wire.  This is also where encryption
		// happens, so this part can be slow.
		d := &message.Data{ID: f.id, Payload: tosend}
		if alsoClose && len(parts) == 0 {
			d.Flags |= message.CloseFlag
		}
		if f.noEncrypt {
			d.Flags |= message.DisableEncryptionFlag
		}
		if opened {
			err = f.conn.mp.writeMsg(ctx, d)
		} else {
			err = f.conn.mp.writeMsg(ctx, &message.OpenFlow{
				ID:              f.id,
				InitialCounters: DefaultBytesBufferedPerFlow,
				BlessingsKey:    bkey,
				DischargeKey:    dkey,
				Flags:           d.Flags,
				Payload:         d.Payload,
			})
		}
		sent += size

		// The top of the loop expects to be locked, so lock here and update
		// opened.  Note that since we've definitely sent a message now opened is surely
		// true.
		f.conn.mu.Lock()
		// We need to ensure that we only call Done() exactly once, so we need to
		// recheck f.opened, to ensure that f.close didn't decrement the wait group
		// while we were not holding the lock.
		if !f.opened {
			f.conn.unopenedFlows.Done()
		}
		f.opened = true
	}
	f.writing = false
	if debug {
		f.currentContextLocked().Infof("finishing write on %d(%p): %v", f.id, f, err)
	}
	f.conn.deactivateWriterLocked(f)
	f.conn.notifyNextWriterLocked(f)
	f.conn.mu.Unlock()

	if alsoClose || err != nil {
		f.close(ctx, false, err)
	}
	return sent, err
}

// WriteMsg is like Write, but allows writing more than one buffer at a time.
// The data in each buffer is written sequentially onto the flow.  Returns the
// number of bytes written.  WriteMsg must return a non-nil error if it writes
// less than the total number of bytes from all buffers.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsg(parts ...[]byte) (int, error) {
	return f.writeMsg(false, parts...)
}

// WriteMsgAndClose performs WriteMsg and then closes the flow.
// Write, WriteMsg, and WriteMsgAndClose should not be called concurrently
// with themselves or each other.
func (f *flw) WriteMsgAndClose(parts ...[]byte) (int, error) {
	return f.writeMsg(true, parts...)
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
	defer f.conn.mu.Unlock()
	f.conn.mu.Lock()

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
	f.conn.mu.Lock()
	cancel := f.cancel
	closed := f.closed
	f.closed = true
	f.conn.mu.Unlock()
	if !closed {
		f.q.close(ctx)
		if f.ctx.V(2) {
			ctx.Infof("closing %d(%p): %v", f.id, f, err)
		}
		cancel()
		// After cancel has been called no new writes will begin for this
		// flow.  There may be a write in progress, but it must finish
		// before another writer gets to use the channel.  Therefore we
		// can simply use sendMessageLocked to send the close flow
		// message.
		f.conn.mu.Lock()
		connClosing := f.conn.status == Closing
		var serr error
		if !f.opened {
			// Closing a flow that was never opened.
			f.conn.unopenedFlows.Done()
			// We mark the flow as opened to prevent multiple calls to
			// f.conn.unopenedFlows.Done().
			f.opened = true
		} else if !closedRemotely && !connClosing {
			// Note: If the conn is closing there is no point in trying to
			// send the flow close message as it will fail.  This is racy
			// with the connection closing, but there are no ill-effects
			// other than spamming the logs a little so it's OK.
			serr = f.conn.sendMessageLocked(ctx, false, expressPriority, &message.Data{
				ID:    f.id,
				Flags: message.CloseFlag,
			})
		}
		if closedRemotely {
			// When the other side closes a flow, it implicitly releases all the
			// counters used by that flow.  That means we should release the shared
			// counter to be used on other new flows.
			f.conn.lshared += f.borrowed
			f.borrowed = 0
		} else if f.borrowed > 0 && f.conn.status < Closing {
			f.conn.outstandingBorrowed[f.id] = f.borrowed
		}
		delete(f.conn.flows, f.id)
		f.conn.mu.Unlock()
		if serr != nil {
			ctx.VI(2).Infof("Could not send close flow message: %v", err)
		}
	}
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

func (f *flw) markUsedLocked() {
	if !f.sideChannel && f.id >= reservedFlows {
		f.conn.markUsedLocked()
	}
}

// popFront removes the first num bytes from in and appends them to out
// returning in, out, and the actual number of bytes appended.
func popFront(in, out [][]byte, num int) ([][]byte, [][]byte, int) {
	i, sofar := 0, 0
	for i < len(in) && sofar < num {
		i, sofar = i+1, sofar+len(in[i])
	}
	out = append(out, in[:i]...)
	if excess := sofar - num; excess > 0 {
		i, sofar = i-1, num
		keep := len(out[i]) - excess
		in[i], out[i] = in[i][keep:], out[i][:keep]
	}
	return in[i:], out, sofar
}
