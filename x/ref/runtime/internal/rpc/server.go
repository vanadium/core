// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/trace"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/i18n"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/ref/lib/publisher"
	"v.io/x/ref/lib/pubsub"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/flow/manager"
)

const (
	bidiProtocol     = "bidi"
	relistenInterval = time.Second
)

type server struct {
	sync.Mutex
	// ctx is used by the server to make internal RPCs, error messages etc.
	ctx               *context.T
	cancel            context.CancelFunc // function to cancel the above context.
	flowMgr           flow.Manager
	settingsPublisher *pubsub.Publisher // pubsub publisher for dhcp
	dirty             chan struct{}
	typeCache         *typeCache
	state             rpc.ServerState // the current state of the server.
	stopListens       context.CancelFunc
	publisher         *publisher.T // publisher to publish mounttable mounts.
	closed            chan struct{}

	endpoints map[string]naming.Endpoint // endpoints that the server is listening on.

	disp               rpc.Dispatcher // dispatcher to serve RPCs
	dispReserved       rpc.Dispatcher // dispatcher for reserved methods
	active             sync.WaitGroup // active goroutines we've spawned.
	preferredProtocols []string       // protocols to use when resolving proxy name to endpoint.
	servesMountTable   bool
	isLeaf             bool
	lameDuckTimeout    time.Duration // the time to wait for inflight operations to finish on shutdown

	stats       *rpcStats // stats for this server.
	outstanding *outstandingStats
}

func WithNewServer(ctx *context.T,
	name string, object interface{}, authorizer security.Authorizer,
	settingsPublisher *pubsub.Publisher,
	opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	if object == nil {
		return ctx, nil, verror.ErrBadArg.Errorf(ctx, "bad argument: nil object")
	}
	invoker, err := objectToInvoker(object)
	if err != nil {
		return ctx, nil, verror.ErrBadArg.Errorf(ctx, "bad argument: bad object: %v", err)
	}
	d := &leafDispatcher{invoker, authorizer}
	opts = append([]rpc.ServerOpt{options.IsLeaf(true)}, opts...)
	return WithNewDispatchingServer(ctx, name, d, settingsPublisher, opts...)
}

//nolint:gocyclo
func WithNewDispatchingServer(ctx *context.T,
	name string, dispatcher rpc.Dispatcher,
	settingsPublisher *pubsub.Publisher,
	opts ...rpc.ServerOpt) (*context.T, rpc.Server, error) {
	if dispatcher == nil {
		return ctx, nil, verror.ErrBadArg.Errorf(ctx, "bad argument: nil dispatcher")
	}

	rid, err := naming.NewRoutingID()
	if err != nil {
		return ctx, nil, err
	}
	origCtx := ctx // the original context may be returned on error paths
	ctx, cancel := context.WithCancel(ctx)
	statsPrefix := naming.Join("rpc", "server", "routing-id", rid.String())
	s := &server{
		ctx:               ctx,
		cancel:            cancel,
		stats:             newRPCStats(statsPrefix),
		settingsPublisher: settingsPublisher,
		dirty:             make(chan struct{}),
		disp:              dispatcher,
		typeCache:         newTypeCache(),
		state:             rpc.ServerActive,
		endpoints:         make(map[string]naming.Endpoint),
		lameDuckTimeout:   5 * time.Second,
		closed:            make(chan struct{}),
		outstanding:       newOutstandingStats(naming.Join("rpc", "server", "outstanding", rid.String())),
	}
	channelTimeout := time.Duration(0)
	connIdleExpiry := time.Duration(0)
	var authorizedPeers []security.BlessingPattern
	for _, opt := range opts {
		switch opt := opt.(type) {
		case options.ServesMountTable:
			s.servesMountTable = bool(opt)
		case options.IsLeaf:
			s.isLeaf = bool(opt)
		case ReservedNameDispatcher:
			s.dispReserved = opt.Dispatcher
		case PreferredServerResolveProtocols:
			s.preferredProtocols = []string(opt)
		case options.ChannelTimeout:
			channelTimeout = time.Duration(opt)
		case options.LameDuckTimeout:
			s.lameDuckTimeout = time.Duration(opt)
		case options.ServerPeers:
			authorizedPeers = []security.BlessingPattern(opt)
			if len(authorizedPeers) == 0 {
				s.cancel()
				return origCtx, nil, verror.ErrBadArg.Errorf(ctx, "bad argument: %v",
					fmt.Errorf("no peers are authorized to communicate with the server"))
			}
			if len(name) != 0 {
				// TODO(ataly, ashankar): Since the server's blessing names are revealed to the
				// mounttable via the server's endpoint, we forbid servers created with the
				// ServerPeers option from publishing themselves. We should relax this restriction
				// and instead check: (1) the mounttable is in the set of peers authorized by the
				// server, and (2) the mounttable reveals the server's endpoint to only the set
				// of authorized peers. (2) can be enforced using Resolve ACLs.
				s.cancel()
				return origCtx, nil, verror.ErrBadArg.Errorf(ctx, "bad argument: %v",
					fmt.Errorf("serverPeers option is not supported for servers that publish their endpoint at a mounttable"))
			}

		case IdleConnectionExpiry:
			connIdleExpiry = time.Duration(opt)
		}
	}

	if s.lameDuckTimeout > 0 {
		// If we're going to lame duck we can't just allow the context we use for all server callbacks
		// to be canceled immediately.  Here we derive a new context that isn't canceled until we cancel it
		// or the runtime shuts down.  The procedure for shutdown is to notice that the context passed in
		// by the user has been canceled, enter lame duck, and only after the lame duck period cancel this
		// context (from which all server call contexts are derived).
		var oldCancel context.CancelFunc = s.cancel
		var newCancel context.CancelFunc
		s.ctx, newCancel = context.WithRootCancel(ctx)
		s.cancel = func() {
			oldCancel()
			newCancel()
		}
	}

	s.flowMgr = manager.New(s.ctx, rid, settingsPublisher, channelTimeout, connIdleExpiry, authorizedPeers)
	s.ctx, _, err = v23.WithNewClient(s.ctx,
		clientFlowManagerOpt{s.flowMgr},
		PreferredProtocols(s.preferredProtocols))
	if err != nil {
		s.cancel()
		return origCtx, nil, err
	}
	pubctx, pubcancel := context.WithCancel(s.ctx)
	s.publisher = publisher.New(pubctx, v23.GetNamespace(s.ctx), publishPeriod)
	s.active.Add(1)
	go s.monitorPubStatus(ctx)

	ls := v23.GetListenSpec(ctx)
	s.listen(s.ctx, ls)
	stats.NewString(naming.Join(statsPrefix, "listenspec")).Set(fmt.Sprintf("%v", ls))
	if len(name) > 0 {
		s.publisher.AddName(name, s.servesMountTable, s.isLeaf)
		vtrace.GetSpan(s.ctx).Annotate("Serving under name: " + name)
	}

	exportStatus(statsPrefix, s)

	go func() {
		blessingsStat := stats.NewString(naming.Join(statsPrefix, "security", "blessings"))
		for {
			blessings, valid := v23.GetPrincipal(ctx).BlessingStore().Default()
			// TODO(caprita): revist printing the blessings with string, and
			// instead expose them as a list.
			blessingsStat.Set(blessings.String())
			done := false
			select {
			case <-ctx.Done():
				done = true
			case <-valid:
			}
			if done {
				break
			}
		}

		s.Lock()
		s.state = rpc.ServerStopping
		s.Unlock()
		serverDebug := fmt.Sprintf("Dispatcher: %T, Status:[%v]", s.disp, s.Status())
		s.ctx.VI(1).Infof("Stop: %s", serverDebug)
		defer s.ctx.VI(1).Infof("Stop done: %s", serverDebug)

		s.stats.stop()
		pubcancel()
		s.stopListens()

		done := make(chan struct{})
		go func() {
			s.flowMgr.StopListening(ctx)
			<-s.publisher.Closed()
			// At this point no new flows should arrive.  Wait for existing calls
			// to complete.
			s.active.Wait()
			close(done)
		}()
		if s.lameDuckTimeout > 0 {
			select {
			case <-done:
			case <-time.After(s.lameDuckTimeout):
				s.ctx.Errorf("%s: Timed out after %v waiting for active requests to complete", serverDebug, s.lameDuckTimeout)
			}
		}
		// Now we cancel the root context which closes all the connections
		// in the flow manager and cancels all the contexts used by
		// ongoing requests.  Hopefully this will bring all outstanding
		// operations to a close.
		s.cancel()
		// Note that since the context has been canceled, <-publisher.Closed() and <-flowMgr.Closed()
		// should return right away.
		<-s.publisher.Closed()
		<-s.flowMgr.Closed()
		s.Lock()
		close(s.dirty)
		s.dirty = nil
		s.Unlock()
		// Now we really will wait forever.  If this doesn't exit, there's something
		// wrong with the users code.
		<-done
		s.Lock()
		s.state = rpc.ServerStopped
		s.Unlock()
		close(s.closed)
		s.typeCache.close()
	}()
	return s.ctx, s, nil
}

// monitorPubStatus guarantees that the ServerStatus.Dirty channel is closed
// when the publisher state becomes dirty. Since we also get the publisher.Status()
// in the Status method, its possible that the Dirty channel in the returned
// ServerStatus will close spuriously by this goroutine.
func (s *server) monitorPubStatus(ctx *context.T) {
	defer s.active.Done()
	var pubDirty <-chan struct{}
	s.Lock()
	_, pubDirty = s.publisher.Status()
	s.Unlock()
	for {
		select {
		case <-pubDirty:
			s.Lock()
			_, pubDirty = s.publisher.Status()
			s.updateDirtyLocked()
			s.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (s *server) Status() rpc.ServerStatus {
	status := rpc.ServerStatus{}
	status.ServesMountTable = s.servesMountTable
	s.Lock()
	// We call s.publisher.Status here instead of using a publisher status cached
	// by s.monitorPubStatus, because we want to guarantee that s.AddName/AddServer
	// calls have the added publisher entries in the returned s.Status() immediately.
	// i.e. s.AddName("foo")
	//      s.Status().PublisherStatus // Should have entry an for "foo".
	status.PublisherStatus, _ = s.publisher.Status()
	status.Dirty = s.dirty
	status.State = s.state
	for _, e := range s.endpoints {
		status.Endpoints = append(status.Endpoints, e)
	}
	// HACK ALERT: Many tests seem to just pick out Endpoints[0] as the
	// address of the server.  Furthermore, many tests run on Kubernetes
	// inside Google Compute Engine (GCE). While GCE doesn't currently
	// support IPv6 (as per
	// https://cloud.google.com/compute/docs/networking), the containers
	// created by kubernetes do show a link-local IPv6 address on the
	// interfaces.
	//
	// Long story short, as a result of this, net.Dial() calls seem to
	// fail.  For now, hack around this by ensuring that
	// status.Endpoints[0] does not correspond to an IPv6 address.
	for i, ep := range status.Endpoints {
		if i > 0 && !mayBeIPv6(ep) {
			status.Endpoints[0], status.Endpoints[i] = status.Endpoints[i], status.Endpoints[0]
			break
		}
	}
	mgrStat := s.flowMgr.Status()
	status.ListenErrors = mgrStat.ListenErrors
	status.ProxyErrors = mgrStat.ProxyErrors
	s.Unlock()
	return status
}

// resolveToEndpoint resolves an object name or address to an endpoint.
func (s *server) resolveToEndpoint(ctx *context.T, address string) ([]naming.Endpoint, error) {
	ns := v23.GetNamespace(ctx)
	ns.FlushCacheEntry(ctx, address)
	resolved, err := ns.Resolve(ctx, address)

	if err != nil {
		return nil, err
	}

	// An empty set of protocols means all protocols...
	if resolved.Servers, err = filterAndOrderServers(resolved.Servers, s.preferredProtocols); err != nil {
		return nil, err
	}

	var eps []naming.Endpoint
	for _, n := range resolved.Names() {
		address, suffix := naming.SplitAddressName(n)
		if suffix != "" {
			continue
		}
		if ep, err := naming.ParseEndpoint(address); err == nil {
			eps = append(eps, ep)
		}
	}
	if len(eps) > 0 {
		return eps, nil
	}
	return nil, errNoCompatibleServers.Errorf(nil, "failed to resolve %v to an endpoint", address)
}

// createEndpoint adds server publishing information to the ep from the manager.
func (s *server) createEndpoint(lep naming.Endpoint) naming.Endpoint {
	lep.ServesMountTable = s.servesMountTable
	return lep
}

func (s *server) proxyListen(ctx *context.T, name string, ep naming.Endpoint) (<-chan struct{}, error) {
	return s.flowMgr.ProxyListen(ctx, name, ep)
}

func (s *server) listen(ctx *context.T, listenSpec rpc.ListenSpec) {
	defer s.Unlock()
	s.Lock()
	var lctx *context.T
	lctx, s.stopListens = context.WithCancel(ctx)
	if len(listenSpec.Proxy) > 0 {
		s.active.Add(1)
		go func() {
			pm := newProxyManager(s,
				listenSpec.Proxy,
				listenSpec.ProxyPolicy,
				listenSpec.ProxyLimit)
			pm.manageProxyConnections(ctx)
			s.active.Done()
		}()
	}
	for _, addr := range listenSpec.Addrs {
		if len(addr.Address) > 0 {
			ch, err := s.flowMgr.Listen(ctx, addr.Protocol, addr.Address)
			if err != nil {
				s.ctx.Infof("Listen(%q, %q, ...) failed: %v", addr.Protocol, addr.Address, err)
			}
			s.active.Add(1)
			go s.relisten(lctx, addr.Protocol, addr.Address, ch, err)
		}
	}

	// We call updateEndpointsLocked in serial once to populate our endpoints for
	// server status with at least one endpoint.
	mgrStat := s.flowMgr.Status()
	s.updateEndpointsLocked(ctx, mgrStat.Endpoints)
	s.active.Add(2)
	go s.updateEndpointsLoop(ctx, mgrStat.Dirty)
	go s.acceptLoop(ctx) //nolint:errcheck
}

// relisten continuously tries to listen on the protocol, address.
// If err != nil, relisten will attempt to listen on the protocol, address immediately, since
// the previous attempt failed.
// Otherwise, ch will be non-nil, and relisten will attempt to relisten once ch is closed.
func (s *server) relisten(ctx *context.T, protocol, address string, ch <-chan struct{}, err error) {
	defer s.active.Done()
	for {
		if err != nil {
			timer := time.NewTimer(relistenInterval)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}
		} else {
			select {
			case <-ch:
			case <-ctx.Done():
				return
			}
		}
		if ch, err = s.flowMgr.Listen(ctx, protocol, address); err != nil {
			s.ctx.Infof("Listen(%q, %q, ...) failed: %v", protocol, address, err)
		}
	}
}

func (s *server) updateDirtyLocked() {
	if s.dirty != nil {
		close(s.dirty)
		s.dirty = make(chan struct{})
	}
}

func (s *server) tryProxyEndpoints(ctx *context.T, name string, eps []naming.Endpoint) (<-chan struct{}, error) {
	var ch <-chan struct{}
	var lastErr error
	for _, ep := range eps {
		if ch, lastErr = s.flowMgr.ProxyListen(ctx, name, ep); lastErr == nil {
			break
		}
	}
	return ch, lastErr
}

func nextDelay(delay time.Duration) time.Duration {
	delay *= 2
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func (s *server) updateEndpointsLoop(ctx *context.T, changed <-chan struct{}) {
	defer s.active.Done()
	for changed != nil {
		<-changed
		mgrStat := s.flowMgr.Status()
		changed = mgrStat.Dirty
		s.Lock()
		s.updateEndpointsLocked(ctx, mgrStat.Endpoints)
		s.Unlock()
	}
}

func (s *server) updateEndpointsLocked(ctx *context.T, leps []naming.Endpoint) {
	endpoints := make(map[string]naming.Endpoint)
	for _, ep := range leps {
		sep := s.createEndpoint(ep)
		endpoints[sep.String()] = sep
	}
	// Endpoints to add and remove.
	rmEps := setDiff(s.endpoints, endpoints)
	addEps := setDiff(endpoints, s.endpoints)
	for k := range rmEps {
		delete(s.endpoints, k)
	}
	for k, ep := range addEps {
		s.endpoints[k] = ep
	}
	s.updateDirtyLocked()

	s.Unlock()
	for k, ep := range rmEps {
		if ep.Addr().Network() != bidiProtocol {
			s.publisher.RemoveServer(k)
		}
	}
	for k, ep := range addEps {
		if ep.Addr().Network() != bidiProtocol {
			s.publisher.AddServer(k)
		}
	}
	s.Lock()
}

// setDiff returns the endpoints in a that are not in b.
func setDiff(a, b map[string]naming.Endpoint) map[string]naming.Endpoint {
	ret := make(map[string]naming.Endpoint)
	for k, ep := range a {
		if _, ok := b[k]; !ok {
			ret[k] = ep
		}
	}
	return ret
}

func (s *server) acceptLoop(ctx *context.T) error {
	var calls sync.WaitGroup
	defer func() {
		calls.Wait()
		s.active.Done()
		s.ctx.VI(1).Infof("rpc: Stopped accepting")
	}()
	for {
		// TODO(mattr): We need to interrupt Accept at some point.
		// Should we interrupt it by canceling the context?
		fl, err := s.flowMgr.Accept(ctx)
		if err != nil {
			s.ctx.VI(10).Infof("rpc: Accept failed: %v", err)
			return err
		}
		calls.Add(1)
		go func(fl flow.Flow) {
			defer calls.Done()
			var ty [1]byte
			if _, err := io.ReadFull(fl, ty[:]); err != nil {
				s.ctx.VI(1).Infof("failed to read flow type: %v", err)
				return
			}
			switch ty[0] {
			case dataFlow:
				fs, err := newXFlowServer(fl, s)
				if err != nil {
					s.ctx.VI(1).Infof("newFlowServer failed %v", err)
					return
				}
				if err := fs.serve(); err != nil {
					// TODO(caprita): Logging errors here is too spammy. For example, "not
					// authorized" errors shouldn't be logged as server errors.
					// TODO(cnicolaou): revisit this when verror2 transition is
					// done.
					if err != io.EOF {
						s.ctx.VI(2).Infof("Flow.serve failed: %v", err)
					}
				}
			case typeFlow:
				if write := s.typeCache.writer(fl.Conn()); write != nil {
					write(fl, nil)
				}
			}
		}(fl)
	}
}

func (s *server) AddName(name string) error {
	if len(name) == 0 {
		return verror.ErrBadArg.Errorf(s.ctx, "bad argument: name is empty")
	}
	s.Lock()
	defer s.Unlock()
	vtrace.GetSpan(s.ctx).Annotate("Serving under name: " + name)
	s.publisher.AddName(name, s.servesMountTable, s.isLeaf)
	return nil
}

func (s *server) RemoveName(name string) {
	s.Lock()
	defer s.Unlock()
	vtrace.GetSpan(s.ctx).Annotate("Removed name: " + name)
	s.publisher.RemoveName(name)
}

func (s *server) Closed() <-chan struct{} {
	return s.closed
}

// flowServer implements the RPC server-side protocol for a single RPC, over a
// flow that's already connected to the client.
type flowServer struct {
	ctx    *context.T
	server *server             // rpc.Server that this flow server belongs to
	disp   rpc.Dispatcher      // rpc.Dispatcher that will serve RPCs on this flow
	flow   *conn.BufferingFlow // underlying flow

	// Fields filled in during the server invocation.
	dec              *vom.Decoder // to decode requests and args from the client
	enc              *vom.Encoder // to encode responses and results to the client
	grantedBlessings security.Blessings
	method, suffix   string
	tags             []*vdl.Value
	discharges       map[string]security.Discharge
	starttime        time.Time
	endStreamArgs    bool // are the stream args at EOF?
	removeStat       func()
}

var (
	_ rpc.StreamServerCall = (*flowServer)(nil)
	_ security.Call        = (*flowServer)(nil)
)

func newXFlowServer(flow flow.Flow, server *server) (*flowServer, error) {
	requestID, err := uuid.NewUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate requestid: %v", err)
	}
	ctx := WithRequestID(server.ctx, requestID)
	ctx = context.WithLoggingPrefix(ctx, requestID)
	fs := &flowServer{
		ctx:        ctx,
		server:     server,
		disp:       server.disp,
		flow:       conn.NewBufferingFlow(ctx, flow),
		discharges: make(map[string]security.Discharge),
	}
	return fs, nil
}

// authorizeVtrace works by simulating a call to __debug/vtrace.Trace.  That
// rpc is essentially equivalent in power to the data we are attempting to
// attach here.
func (fs *flowServer) authorizeVtrace(ctx *context.T) error {
	// Set up a context as though we were calling __debug/vtrace.
	params := &security.CallParams{}
	params.Copy(fs)
	params.Method = "Trace"
	params.MethodTags = []*vdl.Value{vdl.ValueOf(access.Debug)}
	params.Suffix = "__debug/vtrace"

	var auth security.Authorizer
	if fs.server.dispReserved != nil {
		_, auth, _ = fs.server.dispReserved.Lookup(ctx, params.Suffix)
	}
	return authorize(ctx, security.NewCall(params), auth)
}

func authorize(ctx *context.T, call security.Call, auth security.Authorizer) error {
	if call.LocalPrincipal() == nil {
		// LocalPrincipal is nil means that the server wanted to avoid
		// authentication, and thus wanted to skip authorization as well.
		return nil
	}
	if auth == nil {
		auth = security.DefaultAuthorizer()
	}
	if err := auth.Authorize(ctx, call); err != nil {
		nerr := verror.ErrNoAccess.Errorf(ctx, "access denied: %v",
			fmt.Errorf("not authorized to call %v.%v: %v", call.Suffix(), call.Method(), err))
		return nerr
	}
	return nil
}

func (fs *flowServer) serve() error {
	defer fs.flow.Close()
	defer func() {
		if fs.removeStat != nil {
			fs.removeStat()
		}
	}()

	ctx, results, err := fs.processRequest()
	vtrace.GetSpan(ctx).Finish()

	traceResponse := vtrace.GetResponse(ctx)
	// Check if the caller is permitted to view vtrace data.
	if traceResponse.Flags != vtrace.Empty && fs.authorizeVtrace(ctx) != nil {
		traceResponse = vtrace.Response{}
	}

	if err != nil && fs.enc == nil {
		return err
	}

	// Respond to the client with the response header and positional results.
	response := rpc.Response{
		Error:            err,
		EndStreamResults: true,
		NumPosResults:    uint64(len(results)),
		TraceResponse:    traceResponse,
	}
	if err := fs.enc.Encode(response); err != nil {
		if err == io.EOF {
			return err
		}
		return fmt.Errorf("failed to encode RPC response %v <-> %v:%v", fs.LocalEndpoint().String(), fs.RemoteEndpoint().String(), err)
	}
	if response.Error != nil {
		return response.Error
	}
	for ix, res := range results {
		if err := fs.enc.Encode(res); err != nil {
			if err == io.EOF {
				return err
			}
			return fmt.Errorf("failed to encode result #%v [%T=%v]:%v", ix, res, res, err)
		}
	}
	// TODO(ashankar): Should unread data from the flow be drained?
	//
	// Reason to do so:
	// The common stream.Flow implementation (v.io/x/ref/runtime/internal/rpc/stream/vc/reader.go)
	// uses iobuf.Slices backed by an iobuf.Pool. If the stream is not drained, these
	// slices will not be returned to the pool leading to possibly increased memory usage.
	//
	// Reason to not do so:
	// Draining here will conflict with any Reads on the flow in a separate goroutine
	// (for example, see TestStreamReadTerminatedByServer in full_test.go).
	//
	// For now, go with the reason to not do so as having unread data in the stream
	// should be a rare case.
	return nil
}

func (fs *flowServer) readRPCRequest(ctx *context.T) (*rpc.Request, error) {
	// Decode the initial request.
	var req rpc.Request
	if err := fs.dec.Decode(&req); err != nil {
		return nil, verror.ErrBadProtocol.Errorf(ctx, "bad protocol or type: %v",
			fmt.Errorf("failed to decode request: %v", err))
	}
	return &req, nil
}

// note that the error returned from processRequest will be sent to the client.
func (fs *flowServer) processRequest() (*context.T, []interface{}, error) {
	fs.starttime = time.Now()

	// Set an initial deadline on the flow to ensure that we don't wait forever
	// for the initial read.
	ctx := fs.flow.SetDeadlineContext(fs.server.ctx, time.Now().Add(defaultCallTimeout))

	typeEnc, typeDec, err := fs.server.typeCache.get(ctx, fs.flow.Conn())
	if err != nil {
		return ctx, nil, err
	}
	fs.enc = vom.NewEncoderWithTypeEncoder(fs.flow, typeEnc)
	fs.dec = vom.NewDecoderWithTypeDecoder(fs.flow, typeDec)

	req, err := fs.readRPCRequest(ctx)
	if err != nil {
		// We don't know what the rpc call was supposed to be, but we'll create
		// a placeholder span so we can capture annotations.
		// TODO(mattr): I'm not sure this makes sense anymore, but I'll revisit it
		// when I'm doing another round of vtrace next quarter.
		ctx, _ = vtrace.WithNewSpan(ctx, fmt.Sprintf("\"%s\".UNKNOWN", fs.suffix))
		return ctx, nil, err
	}

	tid := req.TraceRequest.TraceId.String()
	sid := req.TraceRequest.SpanId.String()
	title := fmt.Sprintf("(trace_id: %s span_id: %s)", tid, sid)

	tr := trace.New("Recv."+req.Suffix, req.Method+" "+title)

	if deadline, ok := ctx.Deadline(); ok {
		tr.LazyPrintf("RPC has a deadline: %v", deadline)
	}

	defer tr.Finish()

	fs.removeStat = fs.server.outstanding.start(req.Method, fs.flow.RemoteEndpoint())

	// Start building up a new context for the request now that we know
	// the header information.
	ctx = fs.ctx

	// We must call fs.drainDecoderArgs for any error that occurs
	// after this point, and before we actually decode the arguments.
	fs.method = req.Method
	fs.suffix = strings.TrimLeft(req.Suffix, "/")
	if req.Language != "" {
		ctx = i18n.WithLangID(ctx, i18n.LangID(req.Language))
	}

	// TODO(mattr): Currently this allows users to trigger trace collection
	// on the server even if they will not be allowed to collect the
	// results later.  This might be considered a DOS vector.
	spanName := fmt.Sprintf("\"%s\".%s", fs.suffix, fs.method)
	ctx, _ = vtrace.WithContinuedTrace(ctx, spanName, req.TraceRequest)
	ctx = fs.flow.SetDeadlineContext(ctx, req.Deadline.Time)

	if err := fs.readGrantedBlessings(ctx, req); err != nil {
		fs.drainDecoderArgs(int(req.NumPosArgs)) //nolint:errcheck

		tr.LazyPrintf("%s\n", err)
		tr.SetError()
		return ctx, nil, err
	}
	// Lookup the invoker.
	invoker, auth, err := fs.lookup(ctx, fs.suffix, fs.method)
	if err != nil {
		fs.drainDecoderArgs(int(req.NumPosArgs)) //nolint:errcheck
		tr.LazyPrintf("%s\n", err)
		tr.SetError()
		return ctx, nil, err
	}

	// Note that we strip the reserved prefix when calling the invoker so
	// that __Glob will call Glob.  Note that we've already assigned a
	// special invoker so that we never call the wrong method by mistake.
	strippedMethod := naming.StripReserved(fs.method)

	// Prepare invoker and decode args.
	numArgs := int(req.NumPosArgs)
	argptrs, tags, err := invoker.Prepare(ctx, strippedMethod, numArgs)
	fs.tags = tags
	if err != nil {
		fs.drainDecoderArgs(numArgs) //nolint:errcheck
		tr.LazyPrintf("%s\n", err)
		tr.SetError()
		return ctx, nil, err
	}
	if called, want := req.NumPosArgs, uint64(len(argptrs)); called != want {
		fs.drainDecoderArgs(numArgs) //nolint:errcheck
		tr.LazyPrintf("%s\n", err)
		tr.SetError()
		return ctx, nil, errBadNumInputArgs.Errorf(ctx, "wrong number of input arguments for %v.%v (called with {%v} args, want {%v})", fs.suffix, fs.method, called, want)
	}
	for ix, argptr := range argptrs {
		if err := fs.dec.Decode(argptr); err != nil {
			tr.LazyPrintf("%s\n", err)
			tr.SetError()

			return ctx, nil, errBadInputArg.Errorf(ctx, "method %v.%v has bad arg #%v: %v", fs.suffix, fs.method, uint64(ix), err)
		}
	}

	// Check application's authorization policy.
	if err := authorize(ctx, fs, auth); err != nil {
		tr.LazyPrintf("%s\n", err)
		tr.SetError()
		return ctx, nil, err
	}

	defer func() {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			deadline, _ := ctx.Deadline()
			tr.LazyPrintf("RPC exceeded deadline. Should have ended at %v", deadline)
		case context.Canceled:
			tr.LazyPrintf("RPC was cancelled.")
		}
		if ctx.Err() != nil {
			tr.SetError()
		}
	}()

	// Invoke the method.
	results, err := invoker.Invoke(ctx, fs, strippedMethod, argptrs)
	fs.server.stats.record(fs.method, time.Since(fs.starttime))
	return ctx, results, err
}

// drainDecoderArgs drains the next n arguments encoded onto the flows decoder.
// This is needed to ensure that the client is able to encode all of its args
// before the server closes its flow. This guarantees that the client will
// consistently get the server's error response.
// TODO(suharshs): Figure out a better way to solve this race condition without
// unnecessarily reading all arguments.
func (fs *flowServer) drainDecoderArgs(n int) error {
	for i := 0; i < n; i++ {
		if err := fs.dec.Decoder().SkipValue(); err != nil {
			return err
		}
	}
	return nil
}

// lookup returns the invoker and authorizer responsible for serving the given
// name and method.  The suffix is stripped of any leading slashes. If it begins
// with rpc.DebugKeyword, we use the internal debug dispatcher to look up the
// invoker. Otherwise, and we use the server's dispatcher. The suffix and method
// value may be modified to match the actual suffix and method to use.
func (fs *flowServer) lookup(ctx *context.T, suffix string, method string) (rpc.Invoker, security.Authorizer, error) {
	if naming.IsReserved(method) {
		return reservedInvoker(fs.disp, fs.server.dispReserved), security.AllowEveryone(), nil
	}
	disp := fs.disp
	if naming.IsReserved(suffix) {
		disp = fs.server.dispReserved
	} else if fs.server.isLeaf && suffix != "" {
		innerErr := fmt.Errorf("suffix %v was not expected because either server has the option IsLeaf set to true or it served an object and not a dispatcher", suffix)
		return nil, nil, verror.ErrUnknownSuffix.Errorf(ctx, "suffix does not exist: %v: %v", suffix, innerErr)
	}
	if disp != nil {
		obj, auth, err := disp.Lookup(ctx, suffix)
		switch {
		case err != nil:
			return nil, nil, err
		case obj != nil:
			invoker, err := objectToInvoker(obj)
			if err != nil {
				return nil, nil, verror.ErrInternal.Errorf(ctx, "invalid received object: %v", err)
			}
			return invoker, auth, nil
		}
	}
	return nil, nil, verror.ErrUnknownSuffix.Errorf(ctx, "suffix does not exist: %v", suffix)
}

func (fs *flowServer) readGrantedBlessings(ctx *context.T, req *rpc.Request) error {
	if req.GrantedBlessings.IsZero() {
		return nil
	}
	// If additional credentials are provided, make them available in the context
	// Detect unusable blessings now, rather then discovering they are unusable on
	// first use.
	//
	// TODO(ashankar,ataly): Potential confused deputy attack: The client provides
	// the server's identity as the blessing. Figure out what we want to do about
	// this - should servers be able to assume that a blessing is something that
	// does not have the authorizations that the server's own identity has?
	if got, want := req.GrantedBlessings.PublicKey(), fs.LocalPrincipal().PublicKey(); got != nil && !reflect.DeepEqual(got, want) {
		return verror.ErrNoAccess.Errorf(ctx, "access denied: %v", fmt.Errorf("blessing granted not bound to this server: %v vs %v", got, want))
	}
	fs.grantedBlessings = req.GrantedBlessings
	return nil
}

// Send implements the rpc.Stream method.
func (fs *flowServer) Send(item interface{}) error {
	// The empty response header indicates what follows is a streaming result.
	if err := fs.enc.Encode(rpc.Response{}); err != nil {
		return err
	}
	if err := fs.enc.Encode(item); err != nil {
		return err
	}
	return fs.flow.Flush()
}

// Recv implements the rpc.Stream method.
func (fs *flowServer) Recv(itemptr interface{}) error {
	var req rpc.Request
	if err := fs.dec.Decode(&req); err != nil {
		return err
	}
	if req.EndStreamArgs {
		fs.endStreamArgs = true
		return io.EOF
	}
	return fs.dec.Decode(itemptr)
}

// Implementations of rpc.ServerCall and security.Call methods.

func (fs *flowServer) Security() security.Call {
	return fs
}
func (fs *flowServer) LocalDischarges() map[string]security.Discharge {
	return fs.flow.LocalDischarges()
}
func (fs *flowServer) RemoteDischarges() map[string]security.Discharge {
	return fs.flow.RemoteDischarges()
}
func (fs *flowServer) Server() rpc.Server {
	return fs.server
}
func (fs *flowServer) Timestamp() time.Time {
	return fs.starttime
}
func (fs *flowServer) Method() string {
	return fs.method
}
func (fs *flowServer) MethodTags() []*vdl.Value {
	return fs.tags
}
func (fs *flowServer) Suffix() string {
	return fs.suffix
}
func (fs *flowServer) LocalPrincipal() security.Principal {
	return v23.GetPrincipal(fs.server.ctx)
}
func (fs *flowServer) LocalBlessings() security.Blessings {
	return fs.flow.LocalBlessings()
}
func (fs *flowServer) RemoteBlessings() security.Blessings {
	return fs.flow.RemoteBlessings()
}
func (fs *flowServer) GrantedBlessings() security.Blessings {
	return fs.grantedBlessings
}
func (fs *flowServer) LocalEndpoint() naming.Endpoint {
	return fs.flow.LocalEndpoint()
}
func (fs *flowServer) RemoteEndpoint() naming.Endpoint {
	return fs.flow.RemoteEndpoint()
}
func (fs *flowServer) RemoteAddr() net.Addr {
	return fs.flow.RemoteAddr()
}

type leafDispatcher struct {
	invoker rpc.Invoker
	auth    security.Authorizer
}

func (d leafDispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if suffix != "" {
		return nil, nil, verror.ErrUnknownSuffix.Errorf(nil, "Suffix does not exist: %v", suffix)
	}
	return d.invoker, d.auth, nil
}

func objectToInvoker(obj interface{}) (rpc.Invoker, error) {
	if obj == nil {
		return nil, fmt.Errorf("nil object can't be invoked")
	}
	if invoker, ok := obj.(rpc.Invoker); ok {
		return invoker, nil
	}
	return rpc.ReflectInvoker(obj)
}

func exportStatus(prefix string, s *server) {
	prefix = naming.Join(prefix, "status")
	stats.NewStringFunc(naming.Join(prefix, "endpoints"), func() string { return fmt.Sprint(s.Status().Endpoints) })
	stats.NewStringFunc(naming.Join(prefix, "publisher"), func() string {
		var lines []string
		for _, e := range s.Status().PublisherStatus {
			lines = append(lines, fmt.Sprint(e))
		}
		return strings.Join(lines, "\n")
	})
	stats.NewStringFunc(naming.Join(prefix, "proxy-errors"), func() string { return fmt.Sprint(s.Status().ProxyErrors) })
	stats.NewStringFunc(naming.Join(prefix, "listen-errors"), func() string { return fmt.Sprint(s.Status().ListenErrors) })
}

func mayBeIPv6(ep naming.Endpoint) bool {
	// Ignore the protocol because the set of protocols to test for isn't
	// clear (tcp, wsh, vine, udp?) and false positives for this function
	// aren't really troublesome.
	host, _, err := net.SplitHostPort(ep.Addr().String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.To4() == nil
}
