// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/i18n"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	vtime "v.io/v23/vdlroot/time"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	slib "v.io/x/ref/lib/security"
	"v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/flow/manager"

	"golang.org/x/net/trace"
)

const (
	reconnectDelay = 50 * time.Millisecond
)

var (
	errClientCloseAlreadyCalled  = verror.NewID("errCloseAlreadyCalled")
	errClientFinishAlreadyCalled = verror.NewID("errFinishAlreadyCalled")
	errRequestEncoding           = verror.NewID("errRequestEncoding")
	errArgEncoding               = verror.NewID("errArgEncoding")
	errResponseDecoding          = verror.NewID("errResponseDecoding")
	errResultDecoding            = verror.NewID("errResultDecoding")
	errRemainingStreamResults    = verror.NewID("errRemaingStreamResults")
	errPeerAuthorizeFailed       = verror.NewID("errPeerAuthorizedFailed")
	errTypeFlowFailure           = verror.NewID("errTypeFlowFailure")

	// These were originally defined in vdl and hence the same IDs that
	// vdl would generate must be used here to ensure backwards compatibility.
	errBadNumInputArgs = verror.NewID("badNumInputArgs")
	errBadInputArg     = verror.NewID("badInputArg")
)

func preferNonTimeout(curr, prev error) error {
	if prev != nil && errors.Is(curr, verror.ErrTimeout) {
		return prev
	}
	return curr
}

const (
	dataFlow = 'd'
	typeFlow = 't'
)

type clientFlowManagerOpt struct {
	mgr flow.Manager
}

func (clientFlowManagerOpt) RPCClientOpt() {}

type client struct {
	flowMgr            flow.Manager
	preferredProtocols []string
	ctx                *context.T
	outstanding        *outstandingStats
	// stop is kept for backward compatibility to implement Close().
	// TODO(mattr): deprecate Close.
	stop func()

	// typeCache maintains a cache of type encoders and decoders.
	typeCache *typeCache

	wg      sync.WaitGroup
	mu      sync.Mutex
	closing bool
	closed  chan struct{}
}

var _ rpc.Client = (*client)(nil)

func NewClient(ctx *context.T, opts ...rpc.ClientOpt) rpc.Client {
	ctx, cancel := context.WithCancel(ctx)
	statsPrefix := fmt.Sprintf("rpc/client/outstanding/%p", ctx)
	c := &client{
		ctx:         ctx,
		typeCache:   newTypeCache(),
		stop:        cancel,
		closed:      make(chan struct{}),
		outstanding: newOutstandingStats(statsPrefix),
	}

	connIdleExpiry := time.Duration(0)
	for _, opt := range opts {
		switch v := opt.(type) {
		case PreferredProtocols:
			c.preferredProtocols = v
		case clientFlowManagerOpt:
			c.flowMgr = v.mgr
		case IdleConnectionExpiry:
			connIdleExpiry = time.Duration(v)
		}
	}

	if c.flowMgr == nil {
		c.flowMgr = manager.New(ctx, naming.NullRoutingID, nil, 0, connIdleExpiry, nil)
	}

	go func() {
		<-ctx.Done()
		c.mu.Lock()
		c.closing = true
		c.mu.Unlock()

		<-c.flowMgr.Closed()
		c.wg.Wait()
		c.outstanding.close()
		close(c.closed)
		c.typeCache.close()
	}()

	return c
}

func (c *client) StartCall(ctx *context.T, name, method string, args []interface{}, opts ...rpc.CallOpt) (rpc.ClientCall, error) {
	if !ctx.Initialized() {
		return nil, verror.ExplicitNew(verror.ErrBadArg, i18n.LangID("en-us"), "<rpc.Client>", "StartCall", "context not initialized")
	}
	connOpts := getConnectionOptions(ctx, opts)
	return c.startCall(ctx, name, method, args, connOpts, opts)
}

func (c *client) Call(ctx *context.T, name, method string, inArgs, outArgs []interface{}, opts ...rpc.CallOpt) error {

	tr := trace.New("Sent."+name, method)
	defer tr.Finish()

	connOpts := getConnectionOptions(ctx, opts)
	var prevErr error
	for retries := uint(0); ; retries++ {
		call, err := c.startCall(ctx, name, method, inArgs, connOpts, opts)
		if err != nil {
			// See explanation in connectToName.
			tr.LazyPrintf("%s\n", err)
			tr.SetError()
			return preferNonTimeout(err, prevErr)
		}
		switch err := call.Finish(outArgs...); {
		case err == nil:
			return nil
		case !shouldRetryBackoff(verror.Action(err), connOpts):
			ctx.VI(4).Infof("Cannot retry after error: %s", err)
			// See explanation in connectToName.
			tr.LazyPrintf("%s\n", err)
			tr.SetError()
			return preferNonTimeout(err, prevErr)
		case !backoff(retries, connOpts.connDeadline):
			tr.LazyPrintf("%s\n", err)
			tr.SetError()
			return err
		default:
			ctx.VI(4).Infof("Retrying due to error: %s", err)
			tr.LazyPrintf("%s\n", err)
			tr.SetError()
		}
		prevErr = err
	}
}

func (c *client) startCall(ctx *context.T, name, method string, args []interface{}, connOpts *connectionOpts, opts []rpc.CallOpt) (rpc.ClientCall, error) {
	ctx, _ = vtrace.WithNewSpan(ctx, fmt.Sprintf("<rpc.Client>%q.%s", name, method))
	r, err := c.connectToName(ctx, name, method, args, connOpts, opts)
	if err != nil {
		return nil, err
	}

	removeStat := c.outstanding.start(method, r.flow.RemoteEndpoint())
	fc, err := newFlowClient(ctx, removeStat, r.flow, r.typeEnc, r.typeDec)
	if err != nil {
		return nil, err
	}

	if verr := fc.start(r.suffix, method, args, opts); verr != nil {
		return nil, verr
	}
	return fc, nil
}

func (c *client) PinConnection(ctx *context.T, name string, opts ...rpc.CallOpt) (flow.PinnedConn, error) {
	ctx, cancel := context.WithCancel(ctx)
	connOpts := getConnectionOptions(ctx, opts)
	r, err := c.connectToName(ctx, name, "", nil, connOpts, opts)
	if err != nil {
		cancel()
		return nil, err
	}
	pinned := &pinnedConn{
		cancel: cancel,
		done:   ctx.Done(),
		conn:   r.flow.Conn(),
	}
	c.wg.Add(1)
	go c.reconnectPinnedConn(ctx, pinned, name, connOpts, opts...)
	return pinned, nil
}

type pinnedConn struct {
	cancel context.CancelFunc
	done   <-chan struct{}

	mu   sync.Mutex
	conn flow.ManagedConn
}

func (p *pinnedConn) Conn() flow.ManagedConn {
	defer p.mu.Unlock()
	p.mu.Lock()
	return p.conn
}

func (p *pinnedConn) Unpin() {
	p.cancel()
}

func (c *client) reconnectPinnedConn(ctx *context.T, p *pinnedConn, name string, connOpts *connectionOpts, opts ...rpc.CallOpt) {
	defer c.wg.Done()
	delay := reconnectDelay
	for {
		p.mu.Lock()
		closed := p.conn.Closed()
		p.mu.Unlock()
		select {
		case <-closed:
			r, err := c.connectToName(ctx, name, "", nil, connOpts, opts)
			if err != nil {
				time.Sleep(delay)
				delay = nextDelay(delay)
			} else {
				delay = reconnectDelay
				p.mu.Lock()
				p.conn = r.flow.Conn()
				p.mu.Unlock()
			}
		case <-p.done:
			// Reaching here means that the ctx passed to PinConnection is cancelled,
			// so the flow on the conn created in PinConnection is closed here.
			return
		}
	}
}

type serverStatus struct {
	index          int
	server, suffix string
	flow           flow.Flow
	serverErr      *verror.SubErr
	typeEnc        *vom.TypeEncoder
	typeDec        *vom.TypeDecoder
}

// connectToName attempts to connect to the provided name. It may retry connecting
// to the servers the name resolves to based on the type of error encountered.
// Once connOpts.connDeadline is reached, it will stop retrying.
func (c *client) connectToName(ctx *context.T, name, method string, args []interface{}, connOpts *connectionOpts, opts []rpc.CallOpt) (*serverStatus, error) {
	span := vtrace.GetSpan(ctx)
	var prevErr error
	for retries := uint(0); ; retries++ {
		r, action, requireResolve, err := c.tryConnectToName(ctx, name, method, args, connOpts, opts)
		switch {
		case err == nil:
			return r, nil
		case !shouldRetry(action, requireResolve, connOpts, opts):
			span.Annotatef("Cannot retry after error: %s", err)
			span.Finish()
			// If the latest error is a timeout, prefer the
			// retryable error from the previous iteration, both to
			// be consistent with what we'd return if backoff were
			// to return false, and to give a more helpful error to
			// the client (since the current timeout error is likely
			// just a result of the context timing out).
			return nil, preferNonTimeout(err, prevErr)
		case !backoff(retries, connOpts.connDeadline):
			span.Annotatef("Retries exhausted")
			span.Finish()
			return nil, err
		default:
			span.Annotatef("Retrying due to error: %s", err)
			ctx.VI(2).Infof("Retrying due to error: %s", err)
		}
		prevErr = err
	}
}

// tryConnectToName makes a single attempt in connecting to a name. It may
// connect to multiple servers (all that serve "name"), but will return a
// serverStatus for at most one of them (the server running on the most
// preferred protocol and network amongst all the servers that were successfully
// connected to and authorized).
// If requireResolve is true on return, then we shouldn't bother retrying unless
// you can re-resolve.
//
// TODO(toddw): Remove action from out-args, the error should tell us the action.
func (c *client) tryConnectToName(ctx *context.T, name, method string, args []interface{}, connOpts *connectionOpts, opts []rpc.CallOpt) (*serverStatus, verror.ActionCode, bool, error) { //nolint:gocyclo
	blessingPattern, name := security.SplitPatternName(name)
	resolved, err := v23.GetNamespace(ctx).Resolve(ctx, name, getNamespaceOpts(opts)...)
	switch {
	case errors.Is(err, naming.ErrNoSuchName):
		return nil, verror.RetryRefetch, false, verror.ErrNoServers.Errorf(ctx, "no usable servers found for: %v: %v", name, err)
	case errors.Is(err, verror.ErrNoServers):
		return nil, verror.NoRetry, false, err // avoid unnecessary wrapping
	case errors.Is(err, verror.ErrTimeout):
		return nil, verror.NoRetry, false, err // return timeout without wrapping
	case err != nil:
		return nil, verror.NoRetry, false, verror.ErrNoServers.Errorf(ctx, "no usable servers found for: %v: %v", name, err)
	case len(resolved.Servers) == 0:
		// This should never happen.
		return nil, verror.NoRetry, true, verror.ErrInternal.Errorf(ctx, "internal error: %v", name)
	}
	if resolved.Servers, err = filterAndOrderServers(resolved.Servers, c.preferredProtocols); err != nil {
		return nil, verror.RetryRefetch, true, verror.ErrNoServers.Errorf(ctx, "no usable servers found for: %v: %v", name, err)
	}

	// servers is now ordered by the priority heurestic implemented in
	// filterAndOrderServers.
	//
	// Try to connect to all servers in parallel.  Provide sufficient
	// buffering for all of the connections to finish instantaneously. This
	// is important because we want to process the responses in priority
	// order; that order is indicated by the order of entries in servers.
	// So, if two responses come in at the same 'instant', we prefer the
	// first in the resolved.Servers)
	//
	// TODO(toddw): Refactor the parallel dials so that the policy can be changed,
	// and so that the goroutines for each Call are tracked separately.
	responses := make([]*serverStatus, len(resolved.Servers))
	ch := make(chan *serverStatus, len(resolved.Servers))
	authorizer := newServerAuthorizer(blessingPattern, opts...)
	peerAuth := peerAuthorizer{authorizer, method, args}

	for i, server := range resolved.Names() {
		c.mu.Lock()
		if c.closing {
			c.mu.Unlock()
			return nil, verror.NoRetry, false, errClientCloseAlreadyCalled.Errorf(ctx, "rpc.Client.Close has already been called")
		}
		c.wg.Add(1)
		c.mu.Unlock()

		go c.tryConnectToServer(ctx, i, name, server, method, args, peerAuth, connOpts, ch)
	}

	for {
		// Block for at least one new response from the server, or the timeout.
		select {
		case r := <-ch:
			responses[r.index] = r
			// Read as many more responses as we can without blocking.
		LoopNonBlocking:
			for {
				select {
				default:
					break LoopNonBlocking
				case r := <-ch:
					responses[r.index] = r
				}
			}
		case <-ctx.Done():
			return c.failedTryConnectToName(ctx, name, method, responses, ch)
		}

		// Process new responses, in priority order.
		numResponses := 0
		for _, r := range responses {
			if r != nil {
				numResponses++
			}
			if r == nil || r.flow == nil {
				continue
			}
			// We must ensure that all flows other than r.flow are closed.
			go cleanupTryConnectToName(r, responses, ch)
			return r, verror.NoRetry, false, nil
		}
		if numResponses == len(responses) {
			return c.failedTryConnectToName(ctx, name, method, responses, ch)
		}
	}
}

// tryConnectToServer attempts to establish a Flow to a single "server"
// (which must be a rooted name), over which a method invocation request
// could be sent.
//
// The server at the remote end of the flow is authorized using the provided
// authorizer, both during creation of the VC underlying the flow and the
// flow itself.
// TODO(cnicolaou): implement real, configurable load balancing.
func (c *client) tryConnectToServer(
	ctx *context.T,
	index int,
	name, server, method string,
	args []interface{},
	auth flow.PeerAuthorizer,
	connOpts *connectionOpts,
	ch chan<- *serverStatus) {
	defer c.wg.Done()
	status := &serverStatus{index: index, server: server}
	var span vtrace.Span
	ctx, span = vtrace.WithNewSpan(ctx, "<client>tryConnectToServer "+server)
	defer func() {
		ch <- status
		span.Finish()
	}()
	suberr := func(err error) *verror.SubErr {
		return &verror.SubErr{
			Name:    suberrName(server, name, method),
			Err:     err,
			Options: verror.Print,
		}
	}

	address, suffix := naming.SplitAddressName(server)
	if len(address) == 0 {
		status.serverErr = suberr(fmt.Errorf("%v does not appear to contain an address", server))
		return
	}
	status.suffix = suffix

	ep, err := naming.ParseEndpoint(address)
	if err != nil {
		status.serverErr = suberr(fmt.Errorf("failed to parse endpoint"))
		return
	}
	var flw flow.Flow
	if connOpts.useOnlyCached {
		flw, err = c.flowMgr.DialCached(ctx, ep, auth, connOpts.channelTimeout)
		if err != nil {
			ctx.VI(2).Infof("rpc: failed to find cached Conn to %v: %v", server, err)
			status.serverErr = suberr(err)
			return
		}
	} else {
		flw, err = c.flowMgr.Dial(ctx, ep, auth, connOpts.channelTimeout)
		if err != nil {
			ctx.VI(2).Infof("rpc: failed to create Flow with %v: %v", server, err)
			status.serverErr = suberr(err)
			return
		}
	}
	if write := c.typeCache.writer(flw.Conn()); write != nil {
		// Create the type flow with a root-cancellable context.
		// This flow must outlive the flow we're currently creating.
		// It lives as long as the connection to which it is bound.
		tctx, tcancel := context.WithRootCancel(ctx)
		tflow, err := c.flowMgr.DialSideChannelCached(tctx, flw.RemoteEndpoint(), typeFlowAuthorizer{}, flw.Conn(), 0)
		if err != nil {
			write(nil, tcancel)
		} else if tflow.Conn() != flw.Conn() {
			ctx.Infof("Existing: %p, new side channel: %p: %v", flw.Conn(), tflow.Conn(), flw.Conn().RemoteEndpoint())
			tflow.Close()
			write(nil, tcancel)
		} else if _, err = tflow.Write([]byte{typeFlow}); err != nil {
			tflow.Close()
			write(nil, tcancel)
		} else {
			write(tflow, tcancel)
		}
	}

	status.typeEnc, status.typeDec, err = c.typeCache.get(ctx, flw.Conn())
	if err != nil {
		status.serverErr = suberr(errTypeFlowFailure.Errorf(ctx, "type flow could not be constructed: %v", err))
		flw.Close()
		return
	}
	status.flow = flw
}

// cleanupTryConnectToName ensures we've waited for every response from the tryConnectToServer
// goroutines, and have closed the flow from each one except skip.  This is a
// blocking function; it should be called in its own goroutine.
func cleanupTryConnectToName(skip *serverStatus, responses []*serverStatus, ch chan *serverStatus) {
	numPending := 0
	for _, r := range responses {
		switch {
		case r == nil:
			// The response hasn't arrived yet.
			numPending++
		case r == skip || r.flow == nil:
			// Either we should skip this flow, or we've closed the flow for this
			// response already; nothing more to do.
		default:
			// We received the response, but haven't closed the flow yet.
			//
			// TODO(toddw): Currently we only notice cancellation when we read or
			// write the flow.  Decide how to handle this.
			r.flow.Close()
		}
	}
	// Now we just need to wait for the pending responses and close their flows.
	for i := 0; i < numPending; i++ {
		if r := <-ch; r.flow != nil {
			r.flow.Close()
		}
	}
}

func errorForAction(ctx *context.T, id verror.IDAction, name string) error {
	switch id {
	case verror.ErrBadProtocol:
		return id.Errorf(ctx, "bad protocol or type")
	case verror.ErrCanceled:
		return id.Errorf(ctx, "canceled")
	case verror.ErrTimeout:
		return id.Errorf(ctx, "timeout")
	case verror.ErrNotTrusted:
		return id.Errorf(ctx, "client does not trust server: %s", name)
	case verror.ErrNoServers:
		return id.Errorf(ctx, "no usable servers found for %s", name)
	}
	return verror.Errorf(ctx, "unclassified error: %v", id)
}

// failedTryConnectToName performs asynchronous cleanup for connectToName, and returns an
// appropriate error from the responses we've already received.  All parallel
// calls in tryConnectToName failed or we timed out if we get here.
func (c *client) failedTryConnectToName(ctx *context.T, name, method string, responses []*serverStatus, ch chan *serverStatus) (*serverStatus, verror.ActionCode, bool, error) {
	go cleanupTryConnectToName(nil, responses, ch)
	v23.GetNamespace(ctx).FlushCacheEntry(ctx, name)
	suberrs := []error{}
	topLevelIDAction := verror.ErrNoServers
	topLevelAction := verror.RetryRefetch
	onlyErrNetwork := true
	for _, r := range responses {
		if r != nil && r.serverErr != nil && r.serverErr.Err != nil {
			switch verror.ErrorID(r.serverErr.Err) {
			case verror.ErrNotTrusted.ID, errPeerAuthorizeFailed.ID:
				topLevelIDAction = verror.ErrNotTrusted
				topLevelAction = verror.NoRetry
				onlyErrNetwork = false
			case verror.ErrTimeout.ID:
				topLevelIDAction = verror.ErrTimeout
				onlyErrNetwork = false
			default:
				onlyErrNetwork = false
			}
			suberrs = append(suberrs, *r.serverErr)
		}
	}

	if onlyErrNetwork {
		// If we only encountered network errors, then report ErrBadProtocol.
		topLevelIDAction = verror.ErrBadProtocol
	}

	switch ctx.Err() {
	case context.Canceled:
		topLevelIDAction = verror.ErrCanceled
		topLevelAction = verror.NoRetry
	case context.DeadlineExceeded:
		topLevelIDAction = verror.ErrTimeout
		topLevelAction = verror.NoRetry
	default:
	}
	topLevelError := errorForAction(ctx, topLevelIDAction, name)

	// TODO(cnicolaou): we get system errors for things like dialing using
	// the 'ws' protocol which can never succeed even if we retry the connection,
	// hence we return RetryRefetch below except for the case where the servers
	// are not trusted, in case there's no point in retrying at all.
	// TODO(cnicolaou): implementing at-most-once rpc semantics in the future
	// will require thinking through all of the cases where the RPC can
	// be retried by the client whilst it's actually being executed on the
	// server.
	return nil, topLevelAction, false, verror.WithSubErrors(topLevelError, suberrs...)
}

func (c *client) Close() {
	c.stop()
	<-c.Closed()
}

func (c *client) Closed() <-chan struct{} {
	return c.closed
}

// flowClient implements the RPC client-side protocol for a single RPC, over a
// flow that's already connected to the server.
type flowClient struct {
	ctx          *context.T          // context to annotate with call details
	flow         *conn.BufferingFlow // the underlying flow
	dec          *vom.Decoder        // to decode responses and results from the server
	enc          *vom.Encoder        // to encode requests and args to the server
	response     rpc.Response        // each decoded response message is kept here
	remoteBNames []string
	secCall      security.Call

	sendClosedMu sync.Mutex
	sendClosed   bool // is the send side already closed? GUARDED_BY(sendClosedMu)
	finished     bool // has Finish() already been called?
	removeStat   func()
}

var _ rpc.ClientCall = (*flowClient)(nil)
var _ rpc.Stream = (*flowClient)(nil)

func newFlowClient(ctx *context.T, removeStat func(), flow flow.Flow, typeEnc *vom.TypeEncoder, typeDec *vom.TypeDecoder) (*flowClient, error) {
	bf := conn.NewBufferingFlow(ctx, flow)
	if _, err := bf.Write([]byte{dataFlow}); err != nil {
		flow.Close()
		removeStat()
		return nil, err
	}
	fc := &flowClient{
		ctx:        ctx,
		flow:       bf,
		dec:        vom.NewDecoderWithTypeDecoder(bf, typeDec),
		enc:        vom.NewEncoderWithTypeEncoder(bf, typeEnc),
		removeStat: removeStat,
	}
	return fc, nil
}

func (fc *flowClient) Conn() flow.ManagedConn {
	return fc.flow.Conn()
}

// close determines the appropriate error to return, in particular,
// if a timeout or cancelation has occurred then any error
// is turned into a timeout or cancelation as appropriate.
// Cancelation takes precedence over timeout. This is needed because
// a timeout can lead to any other number of errors due to the underlying
// network connection being shutdown abruptly.
func (fc *flowClient) close(err error) error {
	fc.removeStat()
	if err == nil {
		return nil
	}
	subErr := verror.SubErr{Err: err, Options: verror.Print}
	subErr.Name = "remote=" + fc.flow.RemoteEndpoint().String()
	//nolint:staticcheck //lint:ignore SA9003
	if cerr := fc.flow.Close(); cerr != nil && err == nil {
		// TODO(mattr): The context is often already canceled here, in
		// which case we'll get an error.  Not clear what to do.
		// return verror.ErrInternal.Errorf(fc.ctx, "Iiternal error: %v", subErr)
	}
	switch verror.ErrorID(err) {
	case verror.ErrCanceled.ID:
		return err
	case verror.ErrTimeout.ID:
		// Canceled trumps timeout.
		if fc.ctx.Err() == context.Canceled {
			canceled := verror.ErrCanceled.Errorf(fc.ctx, "canceled")
			return verror.WithSubErrors(canceled, subErr)
		}
		return err
	default:
		switch fc.ctx.Err() {
		case context.DeadlineExceeded:
			timeout := verror.ErrTimeout.Errorf(fc.ctx, "timeout")
			err := verror.WithSubErrors(timeout, subErr)
			return err
		case context.Canceled:
			canceled := verror.ErrCanceled.Errorf(fc.ctx, "canceled")
			err := verror.WithSubErrors(canceled, subErr)
			return err
		}
	}
	switch verror.ErrorID(err) {
	case errRequestEncoding.ID, errArgEncoding.ID, errResponseDecoding.ID, errResultDecoding.ID, errBadNumInputArgs.ID, errBadInputArg.ID:
		return verror.ErrBadProtocol.Errorf(fc.ctx, "bad protocol or type: %v", err)
	}
	return err
}

func (fc *flowClient) start(suffix, method string, args []interface{}, opts []rpc.CallOpt) error {
	grantedB, err := fc.initSecurity(fc.ctx, method, suffix, opts)
	if err != nil {
		berr := verror.ErrNotTrusted.Errorf(fc.ctx, "client does not trust server: %v", err)
		return fc.close(berr)
	}
	deadline, _ := fc.ctx.Deadline()
	req := rpc.Request{
		Suffix:           suffix,
		Method:           method,
		NumPosArgs:       uint64(len(args)),
		Deadline:         vtime.Deadline{Time: deadline},
		GrantedBlessings: grantedB,
		TraceRequest:     vtrace.GetRequest(fc.ctx),
		Language:         string(i18n.GetLangID(fc.ctx)),
	}
	if err := fc.enc.Encode(req); err != nil {
		berr := errRequestEncoding.Errorf(fc.ctx, "failed to encode request %#v: %v", req, err)
		return fc.close(berr)
	}
	for ix, arg := range args {
		if err := fc.enc.Encode(arg); err != nil {
			berr := errArgEncoding.Errorf(fc.ctx, "failed to encode arg #%v: %v", ix, err)
			return fc.close(berr)
		}
	}
	return fc.flow.Flush()
}

func (fc *flowClient) initSecurity(ctx *context.T, method, suffix string, opts []rpc.CallOpt) (security.Blessings, error) {
	// The "Method" and "Suffix" fields of the call are not populated
	// as they are considered irrelevant for authorizing server blessings.
	// (This makes the call used here consistent with
	// peerAuthorizer.AuthorizePeer that is used during Conn creation)
	callparams := &security.CallParams{
		LocalPrincipal:   v23.GetPrincipal(ctx),
		LocalBlessings:   fc.flow.LocalBlessings(),
		RemoteBlessings:  fc.flow.RemoteBlessings(),
		LocalEndpoint:    fc.flow.LocalEndpoint(),
		RemoteEndpoint:   fc.flow.RemoteEndpoint(),
		LocalDischarges:  fc.flow.LocalDischarges(),
		RemoteDischarges: fc.flow.RemoteDischarges(),
	}
	call := security.NewCall(callparams)
	var grantedB security.Blessings
	for _, o := range opts {
		if v, ok := o.(rpc.Granter); ok {
			if b, err := v.Grant(ctx, call); err != nil {
				return grantedB, fmt.Errorf("failed to grant blessing to server with blessings: %v", err)
			} else if grantedB, err = security.UnionOfBlessings(grantedB, b); err != nil {
				return grantedB, fmt.Errorf("failed to add blessing granted to server: %v", err)
			}
		}
	}
	// TODO(suharshs): Its unfortunate that we compute these here and also in the
	// peerAuthorizer struct. Find a way to only do this once.
	fc.remoteBNames, _ = security.RemoteBlessingNames(ctx, call)
	// Going forward though, we can provide the security.Call with Method and Suffix
	callparams.Method = method
	callparams.Suffix = suffix
	fc.secCall = security.NewCall(callparams)
	return grantedB, nil
}

func (fc *flowClient) Send(item interface{}) error {
	if fc.sendClosed {
		return verror.ErrAborted.Errorf(fc.ctx, "aborted")
	}
	// The empty request header indicates what follows is a streaming arg.
	if err := fc.enc.Encode(rpc.Request{}); err != nil {
		berr := errRequestEncoding.Errorf(fc.ctx, "failed to encode stream request: %v", err)
		return fc.close(berr)
	}
	if err := fc.enc.Encode(item); err != nil {
		berr := errArgEncoding.Errorf(fc.ctx, "failed to encode stream item: %v", err)
		return fc.close(berr)
	}
	return fc.flow.Flush()
}

func (fc *flowClient) Recv(itemptr interface{}) error {
	switch {
	case fc.response.Error != nil:
		return verror.ErrBadProtocol.Errorf(fc.ctx, "bad protocol or type: %v", fc.response.Error)
	case fc.response.EndStreamResults:
		return io.EOF
	}

	// Decode the response header and handle errors and EOF.
	if err := fc.dec.Decode(&fc.response); err != nil {
		/*id, verr := decodeNetError(fc.ctx, err)
		suberr := errResponseDecoding.Errorf(fc.ctx, "failed to decode response: %v", verr)
		berr := id.Errorf(fc.ctx, "decode error: %v", suberr)*/
		berr := decodingResponseError(fc.ctx, err, "streaming header")
		return fc.close(berr)
	}
	if fc.response.Error != nil {
		return fc.response.Error
	}
	if fc.response.EndStreamResults {
		// Return EOF to indicate to the caller that there are no more stream
		// results.  Any error sent by the server is kept in fc.response.Error, and
		// returned to the user in Finish.
		return io.EOF
	}
	// Decode the streaming result.
	if err := fc.dec.Decode(itemptr); err != nil {
		/*id, verr := decodeNetError(fc.ctx, err)
		suberr := errResponseDecoding.Errorf(fc.ctx, "failed to decode response: %v", verr)
		berr := id.Errorf(fc.ctx, "streaming decode error: %v", suberr)*/
		berr := decodingResponseError(fc.ctx, err, "streaming item")
		// TODO(cnicolaou): should we be caching this?
		fc.response.Error = berr
		return fc.close(berr)
	}
	return nil
}

func (fc *flowClient) CloseSend() error {
	return fc.closeSend()
}

func (fc *flowClient) closeSend() error {
	fc.sendClosedMu.Lock()
	defer fc.sendClosedMu.Unlock()
	if fc.sendClosed {
		return nil
	}
	//nolint:staticcheck //lint:ignore SA9003
	if err := fc.enc.Encode(rpc.Request{EndStreamArgs: true}); err != nil {
		// TODO(caprita): Indiscriminately closing the flow below causes
		// a race as described in:
		// https://docs.google.com/a/google.com/document/d/1C0kxfYhuOcStdV7tnLZELZpUhfQCZj47B0JrzbE29h8/edit
		//
		// There should be a finer grained way to fix this (for example,
		// encoding errors should probably still result in closing the
		// flow); on the flip side, there may exist other instances
		// where we are closing the flow but should not.
		//
		// For now, commenting out the line below removes the flakiness
		// from our existing unit tests, but this needs to be revisited
		// and fixed correctly.
		//
		//   return fc.close(verror.ErrBadProtocolf("rpc: end stream args encoding failed: %v", err))
	}
	// We ignore the error on this flush for the same reason we ignore the error above.
	fc.flow.Flush()
	fc.sendClosed = true
	return nil
}

// TODO(toddw): Should we require Finish to be called, even if send or recv
// return an error?
func (fc *flowClient) Finish(resultptrs ...interface{}) error {
	defer vtrace.GetSpan(fc.ctx).Finish()
	if fc.finished {
		err := errClientFinishAlreadyCalled.Errorf(fc.ctx, "rpc.ClientCall.Finish has already been called")
		return fc.close(verror.ErrBadState.Errorf(fc.ctx, "%v", err))
	}
	fc.finished = true

	// Call closeSend implicitly, if the user hasn't already called it.  There are
	// three cases:
	// 1) Server is blocked on Recv waiting for the final request message.
	// 2) Server has already finished processing, the final response message and
	//    out args are queued up on the client, and the flow is closed.
	// 3) Between 1 and 2: the server isn't blocked on Recv, but the final
	//    response and args aren't queued up yet, and the flow isn't closed.
	//
	// We must call closeSend to handle case (1) and unblock the server; otherwise
	// we'll deadlock with both client and server waiting for each other.  We must
	// ignore the error (if any) to handle case (2).  In that case the flow is
	// closed, meaning writes will fail and reads will succeed, and closeSend will
	// always return an error.  But this isn't a "real" error; the client should
	// read the rest of the results and succeed.
	_ = fc.closeSend()
	// Decode the response header, if it hasn't already been decoded by Recv.
	if fc.response.Error == nil && !fc.response.EndStreamResults {
		if err := fc.dec.Decode(&fc.response); err != nil {
			berr := decodingResponseError(fc.ctx, err, "finish")
			return fc.close(berr)
		}

		// The response header must indicate the streaming results have ended.
		if fc.response.Error == nil && !fc.response.EndStreamResults {
			berr := errRemainingStreamResults.Errorf(fc.ctx, "stream closed with remaining stream results")
			return fc.close(berr)
		}
	}
	// Incorporate any VTrace info that was returned.
	vtrace.GetStore(fc.ctx).Merge(fc.response.TraceResponse)
	if fc.response.Error != nil {
		if errors.Is(fc.response.Error, verror.ErrNoAccess) {
			// In case the error was caused by a bad discharge, we do not want to get stuck
			// with retrying again and again with this discharge. As there is no direct way
			// to detect it, we conservatively flush all discharges we used from the cache.
			// TODO(ataly,andreser): add verror.BadDischarge and handle it explicitly?
			l := len(fc.flow.LocalDischarges())
			dis := make([]security.Discharge, 0, l)
			for _, d := range fc.flow.LocalDischarges() {
				dis = append(dis, d)
			}
			fc.ctx.VI(3).Infof("Discarding %d discharges as RPC failed with %v", l, fc.response.Error)
			v23.GetPrincipal(fc.ctx).BlessingStore().ClearDischarges(dis...)
		}
		if verror.IsAny(fc.response.Error) {
			return fc.close(fc.response.Error)
		}
		return fc.close(verror.ErrInternal.Errorf(fc.ctx, "Iiternal error: %v", fc.response.Error))
	}
	if got, want := fc.response.NumPosResults, uint64(len(resultptrs)); got != want {
		suberr := fmt.Errorf("got %v results, but want %v", got, want)
		berr := verror.ErrBadProtocol.Errorf(fc.ctx, "failed to decode number of results: %v", suberr)
		return fc.close(berr)
	}
	for ix, r := range resultptrs {
		if err := fc.dec.Decode(r); err != nil {
			id, verr := decodeNetError(fc.ctx, err)
			berr := id.Errorf(fc.ctx, "error decoding results: %v", errResultDecoding.Errorf(fc.ctx, "failed to decode result #%v:%v", ix, verr))
			return fc.close(berr)
		}
	}
	fc.close(nil)
	return nil
}

func (fc *flowClient) RemoteBlessings() ([]string, security.Blessings) {
	return fc.remoteBNames, fc.flow.RemoteBlessings()
}

func (fc *flowClient) Security() security.Call {
	return fc.secCall
}

type typeFlowAuthorizer struct{}

func (a typeFlowAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEP, remoteEP naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges map[string]security.Discharge) ([]string, []security.RejectedBlessing, error) {
	return nil, nil, nil
}

func (a typeFlowAuthorizer) BlessingsForPeer(ctx *context.T, peerNames []string) (
	security.Blessings, map[string]security.Discharge, error) {
	return security.Blessings{}, nil, nil
}

type peerAuthorizer struct {
	auth   security.Authorizer
	method string
	args   []interface{}
}

func (x peerAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEP, remoteEP naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges map[string]security.Discharge) ([]string, []security.RejectedBlessing, error) {
	localPrincipal := v23.GetPrincipal(ctx)
	// The "Method" and "Suffix" fields of the call are not populated
	// as they are considered irrelevant for authorizing server blessings.
	call := security.NewCall(&security.CallParams{
		Timestamp:        time.Now(),
		LocalPrincipal:   localPrincipal,
		LocalEndpoint:    localEP,
		RemoteBlessings:  remoteBlessings,
		RemoteDischarges: remoteDischarges,
		RemoteEndpoint:   remoteEP,
	})
	if err := x.auth.Authorize(ctx, call); err != nil {
		return nil, nil, errPeerAuthorizeFailed.Errorf(ctx, "failed to authorize flow with remote blessings: %v: %v", call.RemoteBlessings(), err)
	}
	peerNames, rejectedPeerNames := security.RemoteBlessingNames(ctx, call)
	return peerNames, rejectedPeerNames, nil
}

func (x peerAuthorizer) BlessingsForPeer(ctx *context.T, peerNames []string) (
	security.Blessings, map[string]security.Discharge, error) {
	localPrincipal := v23.GetPrincipal(ctx)
	clientB := localPrincipal.BlessingStore().ForPeer(peerNames...)
	dis, _ := slib.PrepareDischarges(ctx, clientB, peerNames, x.method, x.args)
	return clientB, dis, nil
}

func shouldRetryBackoff(action verror.ActionCode, connOpts *connectionOpts) bool {
	switch {
	case connOpts.noRetry:
		return false
	case action != verror.RetryBackoff:
		return false
	case time.Now().After(connOpts.connDeadline):
		return false
	}
	return true
}

func shouldRetry(action verror.ActionCode, requireResolve bool, connOpts *connectionOpts, opts []rpc.CallOpt) bool {
	switch {
	case connOpts.noRetry:
		return false
	case connOpts.useOnlyCached:
		// If we should only used cached connections, it doesn't make sense to retry
		// looking in the cache.
		return false
	case action != verror.RetryConnection && action != verror.RetryRefetch:
		return false
	case time.Now().After(connOpts.connDeadline):
		return false
	case requireResolve && getNoNamespaceOpt(opts):
		// If we're skipping resolution and there are no servers for
		// this call retrying is not going to help, we can't come up
		// with new servers if there is no resolution.
		return false
	}
	return true
}

// A randomized exponential backoff. The randomness deters error convoys
// from forming.  The first time you retry n should be 0, then 1 etc.
func backoff(n uint, deadline time.Time) bool {
	// This is ((100 to 200) * 2^n) ms.
	b := time.Duration((100+rand.Intn(100))<<n) * time.Millisecond
	if b > maxBackoff {
		b = maxBackoff
	}
	r := time.Until(deadline)
	// We need to budget some time for the call to have a chance to complete
	// lest we'll timeout before we actually do anything.  If we just don't
	// have enough time left, give up.
	//
	// The value should cover a sensible call duration (which includes name
	// resolution and the actual server RPC) on most supported platforms;
	// use https://vanadium.github.io/performance.html for inspiration.
	const reserveTime = 100 * time.Millisecond
	if r <= reserveTime {
		return false
	}
	r -= reserveTime
	if b > r {
		b = r
	}
	time.Sleep(b)
	return true
}

func suberrName(server, name, method string) string {
	// In the case the client directly dialed an endpoint we want to avoid printing
	// the endpoint twice.
	if server == name {
		return fmt.Sprintf("%s.%s", server, method)
	}
	return fmt.Sprintf("%s:%s.%s", server, name, method)
}

func decodingResponseError(ctx *context.T, err error, detail string) error {
	id, _ := decodeNetError(ctx, err)
	if id.ID != verror.ErrBadProtocol.ID {
		return errResponseDecoding.Errorf(ctx, "failed to decode results: %v: %v", detail, id.Errorf(ctx, "network error: %v", err))
	}
	return errResponseDecoding.Errorf(ctx, "failed to decode results: %v: %v", detail, err)
}

// decodeNetError tests for a net.Error from the lower stream code and
// translates it into an appropriate error to be returned by the higher level
// RPC api calls. It also tests for the net.Error being a stream.NetError
// and if so, uses the error it stores rather than the stream.NetError itself
// as its retrun value. This allows for the stack trace of the original
// error to be chained to that of any verror created with it as a first parameter.
func decodeNetError(ctx *context.T, err error) (verror.IDAction, error) {
	if neterr, ok := err.(net.Error); ok {
		if neterr.Timeout() || neterr.Temporary() {
			// If a read is canceled in the lower levels we see
			// a timeout error - see readLocked in vc/reader.go
			if ctx.Err() == context.Canceled {
				return verror.ErrCanceled, err
			}
			return verror.ErrTimeout, err
		}
	}
	if id := verror.ErrorID(err); id != verror.ErrUnknown.ID {
		return verror.IDAction{
			ID:     id,
			Action: verror.Action(err),
		}, err
	}
	return verror.ErrBadProtocol, err
}
