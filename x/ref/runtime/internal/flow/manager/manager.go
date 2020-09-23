// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/verror"

	"v.io/x/lib/netconfig"
	"v.io/x/lib/netstate"
	"v.io/x/ref/lib/pubsub"
	slib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/stats"
	iflow "v.io/x/ref/runtime/internal/flow"
	"v.io/x/ref/runtime/internal/flow/conn"
	"v.io/x/ref/runtime/internal/lib/upcqueue"
	"v.io/x/ref/runtime/internal/rpc/version"
	"v.io/x/ref/runtime/protocols/bidi"
)

const (
	reapCacheInterval = 30 * time.Minute      // this is conservative
	minCacheInterval  = 10 * time.Millisecond // the minimum time we are willing to poll the cache for idle or closed connections.
	handshakeTimeout  = time.Minute
)

type manager struct {
	rid                  naming.RoutingID
	closed               chan struct{}
	cache                *ConnCache
	ls                   *listenState
	ctx                  *context.T
	acceptChannelTimeout time.Duration
	idleExpiry           time.Duration // time after which idle connections will be closed.
	cacheTicker          *time.Ticker
}

type listenState struct {
	q                     *upcqueue.T
	listenLoops           sync.WaitGroup
	dhcpPublisher         *pubsub.Publisher
	serverAuthorizedPeers []security.BlessingPattern // empty list implies all peers are authorized to see the server's blessings.

	mu              sync.Mutex
	serverBlessings security.Blessings
	serverNames     []string
	listeners       map[flow.Listener]*endpointState
	// maintain a list of endpoints per proxy being used.
	proxyEndpoints map[string][]naming.Endpoint
	proxyErrors    map[string]error
	// TODO(suharshs): Look into making the struct{Protocol, Address string} into
	// a named struct. This may involve changing v.io/v23/rpc.ListenAddrs.
	listenErrors map[struct{ Protocol, Address string }]error
	dirty        chan struct{}
	stopRoaming  func()
	proxyFlows   map[string]flow.Flow // keyed by ep.String()
}

type endpointState struct {
	leps         []naming.Endpoint // the list of currently active endpoints.
	tmplEndpoint naming.Endpoint   // endpoint used as a template for creating new endpoints from the network interfaces provided from roaming.
	roaming      bool
}

// New creates a new flow manager.
func New(
	ctx *context.T,
	rid naming.RoutingID,
	dhcpPublisher *pubsub.Publisher,
	channelTimeout time.Duration,
	idleExpiry time.Duration,
	authorizedPeers []security.BlessingPattern) flow.Manager {
	m := &manager{
		rid:                  rid,
		closed:               make(chan struct{}),
		cache:                NewConnCache(idleExpiry),
		ctx:                  ctx,
		acceptChannelTimeout: channelTimeout,
		idleExpiry:           idleExpiry,
	}

	var valid <-chan struct{}
	if rid != naming.NullRoutingID {
		m.ls = &listenState{
			q:                     upcqueue.New(),
			listeners:             make(map[flow.Listener]*endpointState),
			dirty:                 make(chan struct{}),
			dhcpPublisher:         dhcpPublisher,
			proxyFlows:            make(map[string]flow.Flow),
			proxyEndpoints:        make(map[string][]naming.Endpoint),
			proxyErrors:           make(map[string]error),
			listenErrors:          make(map[struct{ Protocol, Address string }]error),
			serverAuthorizedPeers: authorizedPeers,
		}
		p := v23.GetPrincipal(ctx)
		m.ls.serverBlessings, valid = p.BlessingStore().Default()
		m.ls.serverNames = security.BlessingNames(p, m.ls.serverBlessings)
	}
	// Pick a interval that is the minimum of the idleExpiry and the reapCacheInterval.
	cacheInterval := reapCacheInterval
	if idleExpiry > 0 && idleExpiry/2 < cacheInterval {
		cacheInterval = idleExpiry / 2
	}
	if cacheInterval < minCacheInterval {
		cacheInterval = minCacheInterval
	}
	m.cacheTicker = time.NewTicker(cacheInterval)

	statsPrefix := naming.Join("rpc", "flow", rid.String())
	m.cache.ExportStats(naming.Join(statsPrefix, "conn-cache"))
	go func() {
		for {
			select {
			case <-valid:
				m.ls.mu.Lock()
				p := v23.GetPrincipal(ctx)
				m.ls.serverBlessings, valid = p.BlessingStore().Default()
				m.ls.serverNames = security.BlessingNames(p, m.ls.serverBlessings)
				m.updateEndpointBlessingsLocked(m.ls.serverNames)
				m.ls.mu.Unlock()
			case <-ctx.Done():
				m.stopListening()
				m.cache.Close(ctx)
				stats.Delete(statsPrefix) //nolint:errcheck
				close(m.closed)
				return
			case <-m.cacheTicker.C:
				// Periodically kill closed connections and remove expired connections,
				// based on the idleExpiry passed to the NewConnCache constructor.
				m.cache.KillConnections(ctx, 0) //nolint:errcheck
			}
		}
	}()
	return m
}

func (m *manager) stopListening() {
	if m.ls == nil {
		return
	}
	m.cacheTicker.Stop()
	m.ls.mu.Lock()
	listeners := m.ls.listeners
	m.ls.listeners = nil
	if m.ls.dirty != nil {
		close(m.ls.dirty)
		m.ls.dirty = nil
	}
	stopRoaming := m.ls.stopRoaming
	m.ls.stopRoaming = nil
	for _, f := range m.ls.proxyFlows {
		f.Close()
	}
	m.ls.mu.Unlock()
	if stopRoaming != nil {
		stopRoaming()
	}
	for ln := range listeners {
		ln.Close()
	}
	m.ls.listenLoops.Wait()
}

func (m *manager) StopListening(ctx *context.T) {
	if m.ls == nil {
		return
	}
	m.stopListening()
	// Now no more connections can start.  We should lame duck all the conns
	// and wait for all of them to ack.
	m.cache.EnterLameDuckMode(ctx)
	// Now nobody should send any more flows, so close the queue.
	m.ls.q.Close()
}

// Listen causes the Manager to accept flows from the provided protocol and address.
// Listen may be called multiple times.
func (m *manager) Listen(ctx *context.T, protocol, address string) (<-chan struct{}, error) {
	if m.ls == nil {
		return nil, errListeningWithNullRid.Errorf(ctx, "manager cannot listen when created with NullRoutingID")
	}

	ln, lnErr := listen(ctx, protocol, address)
	defer m.ls.mu.Unlock()
	m.ls.mu.Lock()
	if m.ls.listeners == nil {

		if ln != nil {
			ln.Close()
		}
		return nil, flow.ErrBadState.Errorf(ctx, "%v", errManagerClosed.Errorf(ctx, "manager is already closed"))
	}

	errKey := struct{ Protocol, Address string }{Protocol: protocol, Address: address}
	if lnErr != nil {
		m.ls.listenErrors[errKey] = lnErr
		if m.ls.dirty != nil {
			close(m.ls.dirty)
			m.ls.dirty = make(chan struct{})
		}
		return nil, iflow.MaybeWrapError(flow.ErrNetwork, ctx, lnErr)
	}

	local := naming.Endpoint{
		Protocol:  protocol,
		Address:   ln.Addr().String(),
		RoutingID: m.rid,
	}.WithBlessingNames(m.ls.serverNames)
	leps, roam, err := m.createEndpoints(ctx, local)
	if err != nil {
		m.ls.listenErrors[errKey] = err
		if m.ls.dirty != nil {
			close(m.ls.dirty)
			m.ls.dirty = make(chan struct{})
		}
		return nil, iflow.MaybeWrapError(flow.ErrBadArg, ctx, err)
	}
	m.ls.listeners[ln] = &endpointState{
		leps:         leps,
		tmplEndpoint: local,
		roaming:      roam,
	}
	if m.ls.stopRoaming == nil && m.ls.dhcpPublisher != nil && roam {
		ctx2, cancel := context.WithCancel(ctx)
		ch := make(chan struct{})
		m.ls.stopRoaming = func() {
			cancel()
			<-ch
		}
		go m.monitorNetworkChanges(ctx2, ch)
	}

	// The endpoints have changed on this successful listen so notify any watchers.
	m.ls.listenErrors[errKey] = nil
	if m.ls.dirty != nil {
		close(m.ls.dirty)
		m.ls.dirty = make(chan struct{})
	}

	m.ls.listenLoops.Add(1)
	acceptFailed := make(chan struct{})
	go m.lnAcceptLoop(ctx, ln, local, errKey, acceptFailed)
	return acceptFailed, nil
}

func (m *manager) monitorNetworkChanges(ctx *context.T, done chan<- struct{}) {
	defer close(done)
	change, err := netconfig.NotifyChange()
	if err != nil {
		ctx.Errorf("endpoints will not be updated if the network configuration changes, failed to monitor network changes: %v", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-change:
			netstate.InvalidateCache()
			m.updateRoamingEndpoints(ctx)
			if change, err = netconfig.NotifyChange(); err != nil {
				ctx.Errorf("endpoints will not be updated if the network configuration changes, failed to monitor network changes: %v", err)
				return
			}
		}
	}
}

func (m *manager) updateRoamingEndpoints(ctx *context.T) {
	ctx.Infof("Network configuration may have changed, adjusting the addresses to listen on (routing id: %v)", m.rid)
	changed := false
	m.ls.mu.Lock()
	defer m.ls.mu.Unlock()
	for _, epState := range m.ls.listeners {
		if !epState.roaming {
			continue
		}
		newleps, _, err := m.createEndpoints(ctx, epState.tmplEndpoint)
		if err != nil {
			ctx.Infof("Unable to update roaming endpoints for template %v: %v", epState.tmplEndpoint, err)
			continue
		}
		if len(newleps) != len(epState.leps) {
			changed = true
		}
		if !changed {
			newset := make(map[string]bool)
			for _, lep := range newleps {
				newset[lep.String()] = true
			}
			for _, lep := range epState.leps {
				if !newset[lep.String()] {
					changed = true
					break
				}
			}
		}
		if changed {
			ctx.Infof("Changing from %v to %v", epState.leps, newleps)
		}
		epState.leps = newleps
	}
	if changed {
		close(m.ls.dirty)
		m.ls.dirty = make(chan struct{})
	}
}

func (m *manager) createEndpoints(ctx *context.T, lep naming.Endpoint) ([]naming.Endpoint, bool, error) {
	iep := lep
	if !strings.HasPrefix(iep.Protocol, "tcp") &&
		!strings.HasPrefix(iep.Protocol, "ws") {
		// If not tcp, ws, or wsh, just return the endpoint we were given.
		return []naming.Endpoint{iep}, false, nil
	}
	host, port, err := net.SplitHostPort(iep.Address)
	if err != nil {
		return nil, false, err
	}
	chooser := v23.GetListenSpec(ctx).AddressChooser
	addrs, unspecified, err := netstate.PossibleAddresses(iep.Protocol, host, chooser)
	if err != nil {
		return nil, false, err
	}
	ieps := make([]naming.Endpoint, 0, len(addrs))
	for _, addr := range addrs {
		n, err := naming.ParseEndpoint(lep.String())
		if err != nil {
			return nil, false, err
		}
		if _, _, err := net.SplitHostPort(addr.String()); err == nil {
			// Endpoint already has port, do not override it since it
			// was likely selected specifically to be allow for NAT
			// traversal etc.
			n.Address = addr.String()
		} else {
			n.Address = net.JoinHostPort(addr.String(), port)
		}
		ieps = append(ieps, n)
	}
	return ieps, unspecified, nil
}

func (m *manager) updateEndpointBlessingsLocked(names []string) {
	for _, eps := range m.ls.listeners {
		eps.tmplEndpoint = eps.tmplEndpoint.WithBlessingNames(names)
		for i := range eps.leps {
			eps.leps[i] = eps.leps[i].WithBlessingNames(names)
		}
	}
	if m.ls.dirty != nil {
		close(m.ls.dirty)
		m.ls.dirty = make(chan struct{})
	}
}

// ProxyListen causes the Manager to accept flows from the specified endpoint.
// The endpoint must correspond to a vanadium proxy.
// If error != nil, establishing a connection to the Proxy failed.
// Otherwise, if error == nil, the returned chan will block until
// connection to the proxy endpoint fails. The caller may then choose to retry
// the connection.
// name is a identifier of the proxy. It can be used to access errors
// in ListenStatus.ProxyErrors.
func (m *manager) ProxyListen(ctx *context.T, name string, ep naming.Endpoint) (<-chan struct{}, error) {
	if m.ls == nil {
		return nil, errListeningWithNullRid.Errorf(ctx, "manager cannot listen when created with NullRoutingID")
	}
	f, err := m.internalDial(ctx, ep, proxyAuthorizer{}, m.acceptChannelTimeout, true, nil)
	if err != nil {
		return nil, err
	}
	k := ep.String()
	m.ls.mu.Lock()
	m.ls.proxyFlows[k] = f
	serverNames := m.ls.serverNames
	m.ls.mu.Unlock()
	w, err := message.Append(ctx, &message.ProxyServerRequest{}, nil)
	if err != nil {
		m.updateProxyEndpoints(nil, name, err, k)
		return nil, err
	}
	if _, err = f.WriteMsg(w); err != nil {
		m.updateProxyEndpoints(nil, name, err, k)
		return nil, err
	}
	// We connect to the proxy once before we loop.
	if err := m.readAndUpdateProxyEndpoints(ctx, name, f, k, serverNames); err != nil {
		return nil, err
	}
	// We do exponential backoff unless the proxy closes the flow cleanly, in which
	// case we redial immediately.
	done := make(chan struct{})
	go func() {
		for {
			// we keep reading updates until we encounter an error, usually because the
			// flow has been closed.
			if err := m.readAndUpdateProxyEndpoints(ctx, name, f, k, serverNames); err != nil {
				ctx.VI(2).Info(err)
				close(done)
				return
			}
		}
	}()
	return done, nil
}

func (m *manager) readAndUpdateProxyEndpoints(ctx *context.T, name string, f flow.Flow, flowKey string, serverNames []string) error {
	eps, err := m.readProxyResponse(ctx, f)
	if err == nil {
		for i := range eps {
			eps[i] = eps[i].WithBlessingNames(serverNames)
		}
	}
	m.updateProxyEndpoints(eps, name, err, flowKey)
	return err
}

func (m *manager) updateProxyEndpoints(eps []naming.Endpoint, name string, err error, flowKey string) {
	defer m.ls.mu.Unlock()
	m.ls.mu.Lock()
	if err != nil {
		delete(m.ls.proxyFlows, flowKey)
	}
	origErrS, errS := "", ""
	if err != nil {
		errS = err.Error()
	}
	if origErr := m.ls.proxyErrors[name]; origErr != nil {
		errS = origErr.Error()
	}
	if endpointsEqual(m.ls.proxyEndpoints[flowKey], eps) && errS == origErrS {
		return
	}
	m.ls.proxyEndpoints[flowKey] = eps
	m.ls.proxyErrors[name] = err
	// The proxy endpoints have changed so we need to notify any watchers to
	// requery Status.
	if m.ls.dirty != nil {
		close(m.ls.dirty)
		m.ls.dirty = make(chan struct{})
	}
}

func endpointsEqual(a, b []naming.Endpoint) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]struct{})
	for _, ep := range a {
		m[ep.String()] = struct{}{}
	}
	for _, ep := range b {
		key := ep.String()
		if _, ok := m[key]; !ok {
			return false
		}
		delete(m, key)
	}
	return len(m) == 0
}

func (m *manager) readProxyResponse(ctx *context.T, f flow.Flow) ([]naming.Endpoint, error) {
	b, err := f.ReadMsg()
	if err != nil {
		f.Close()
		return nil, err
	}
	msg, err := message.Read(ctx, b)
	if err != nil {
		f.Close()
		return nil, iflow.MaybeWrapError(flow.ErrBadArg, ctx, err)
	}
	switch m := msg.(type) {
	case *message.ProxyResponse:
		return m.Endpoints, nil
	case *message.ProxyErrorResponse:
		f.Close()
		return nil, errProxyResponse.Errorf(ctx, "proxy returned: %v", m.Error)
	default:
		f.Close()
		return nil, flow.ErrBadArg.Errorf(ctx, "%v", errInvalidProxyResponse.Errorf(ctx, "invalid proxy response: %T", m))
	}
}

type proxyAuthorizer struct{}

func (proxyAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEndpoint, remoteEndpoint naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges map[string]security.Discharge,
) ([]string, []security.RejectedBlessing, error) {
	return nil, nil, nil
}

func (a proxyAuthorizer) BlessingsForPeer(ctx *context.T, proxyBlessings []string) (
	security.Blessings, map[string]security.Discharge, error) {
	blessings := v23.GetPrincipal(ctx).BlessingStore().ForPeer(proxyBlessings...)
	discharges, _ := slib.PrepareDischarges(ctx, blessings, proxyBlessings, "", nil)
	return blessings, discharges, nil
}

//nolint:gocyclo
func (m *manager) lnAcceptLoop(ctx *context.T, ln flow.Listener, local naming.Endpoint,
	errKey struct{ Protocol, Address string }, acceptFailed chan struct{}) {
	defer m.ls.listenLoops.Done()
	defer func() {
		close(acceptFailed)
		ln.Close()
		m.ls.mu.Lock()
		delete(m.ls.listeners, ln)
		if m.ls.dirty != nil {
			close(m.ls.dirty)
			m.ls.dirty = make(chan struct{})
		}
		m.ls.mu.Unlock()
	}()
	const killConnectionsRetryDelay = 5 * time.Millisecond
	for {
		flowConn, err := ln.Accept(ctx)
		for tokill := 1; isTemporaryError(err); tokill *= 2 {
			if isTooManyOpenFiles(err) {
				if err := m.cache.KillConnections(ctx, tokill); err != nil {
					ctx.VI(2).Infof("failed to kill connections: %v", err)
					continue
				}
			} else {
				tokill = 1
			}
			time.Sleep(killConnectionsRetryDelay)
			flowConn, err = ln.Accept(ctx)
		}
		if err != nil {
			m.ls.mu.Lock()
			closed := m.ls.listeners == nil
			m.ls.mu.Unlock()
			if !closed {
				ctx.Errorf("ln.Accept on localEP %v failed: %v", local, err)
				return
			}
			m.ls.mu.Lock()
			m.ls.listenErrors[errKey] = err
			if m.ls.dirty != nil {
				close(m.ls.dirty)
				m.ls.dirty = make(chan struct{})
			}
			m.ls.mu.Unlock()
			return
		}

		m.ls.mu.Lock()
		if m.ls.listeners == nil {
			m.ls.mu.Unlock()
			return
		}
		m.ls.listenLoops.Add(1)
		m.ls.mu.Unlock()
		fh := &flowHandler{m, make(chan struct{})}
		go func() {
			defer m.ls.listenLoops.Done()
			c, err := conn.NewAccepted(
				m.ctx,
				m.ls.serverAuthorizedPeers,
				flowConn,
				local,
				version.Supported,
				handshakeTimeout,
				m.acceptChannelTimeout,
				fh)
			if err != nil {
				// We don't want probing from load balancers or Prometheus to cause
				// the error log to be noisy so we skip logging an err in the following
				// cases:
				//
				// (1) 'message of type 127 and size 0 failed decoding at field 0: <nil>'
				//
				// This can be trigger using "echo -e '\xff\xff\xff' | nc ...".
				//
				// (2) 'error reading from unknown: EOF'
				//
				// This can be trigger using nc to connect to a tcp Vanadium endpoint
				// and then typing Ctrl-C after receiving the first message. Another way
				// is using 'nc ... & p=$! && sleep 1 && kill -s SIGINT ${p}'
				//
				// (3) 'read: connection reset by peer'.
				//
				// This can be trigger using 'nmap -sT' to a tcp Vanadium endpoint.
				skip := false
				if errors.Is(err, conn.ErrRecv) {
					wrapped := errors.Unwrap(err)
					if wrapped == io.EOF {
						skip = true
					}
					switch p := wrapped.(type) {
					case *net.OpError:
						sysErr, ok := p.Err.(*os.SyscallError)
						if ok && sysErr.Err == syscall.ECONNRESET {
							skip = true
						}
					case verror.E:
						typ, size, field, ok := message.ParseErrInvalidMessage(err)
						if ok && typ == 127 && size == 0 && field == 0 {
							skip = true
						}
					}
					line := fmt.Sprintf("failed to accept flow.Conn on localEP %v failed: %v", local, err)
					if !skip {
						ctx.Error(line)
					} else {
						ctx.VI(1).Info(line)
					}
				}
				flowConn.Close()
			} else if err = m.cache.InsertWithRoutingID(c, false); err != nil {
				ctx.Errorf("failed to cache conn %v: %v", c, err)
				c.Close(ctx, err)
			}
			// Note: the flow handler created above will block until
			// this channel is closed.
			close(fh.cached)
		}()
	}
}

type hybridHandler struct {
	handler conn.FlowHandler
	ready   chan struct{}
}

func (h *hybridHandler) HandleFlow(f flow.Flow) error {
	<-h.ready
	return h.handler.HandleFlow(f)
}

func (m *manager) handlerReady(fh conn.FlowHandler, proxy bool) {
	if fh != nil {
		if h, ok := fh.(*hybridHandler); ok {
			if proxy {
				h.handler = &proxyFlowHandler{m: m}
			} else {
				h.handler = &flowHandler{m: m}
			}
			close(h.ready)
		}
	}
}

func newHybridHandler(m *manager) *hybridHandler {
	return &hybridHandler{
		ready: make(chan struct{}),
	}
}

type flowHandler struct {
	m      *manager
	cached chan struct{}
}

func (h *flowHandler) HandleFlow(f flow.Flow) error {
	if h.cached != nil {
		<-h.cached
	}
	return h.m.ls.q.Put(f)
}

type proxyFlowHandler struct {
	m *manager
}

func (h *proxyFlowHandler) HandleFlow(f flow.Flow) error {
	go func() {
		fh := &flowHandler{h.m, make(chan struct{})}
		h.m.ls.mu.Lock()
		if h.m.ls.listeners == nil {
			// If we've entered lame duck mode we want to reject new flows
			// from the proxy.  This should come out as a connection failure
			// for the client, which will result in a retry.
			h.m.ls.mu.Unlock()
			f.Close()
			return
		}
		h.m.ls.mu.Unlock()
		c, err := conn.NewAccepted(
			h.m.ctx,
			h.m.ls.serverAuthorizedPeers,
			f,
			f.LocalEndpoint(),
			version.Supported,
			handshakeTimeout,
			h.m.acceptChannelTimeout,
			fh)
		if err != nil {
			h.m.ctx.Errorf("failed to create accepted conn: %v", err)
		} else if err = h.m.cache.InsertWithRoutingID(c, false); err != nil {
			h.m.ctx.Errorf("failed to create accepted conn: %v", err)
		}
		close(fh.cached)
	}()
	return nil
}

// Status returns the current flow.ListenStatus of the manager.
func (m *manager) Status() flow.ListenStatus {
	var status flow.ListenStatus
	if m.ls == nil {
		return status
	}
	m.ls.mu.Lock()
	status.Endpoints = nil
	if len(m.ls.proxyEndpoints) > 0 {
		for _, eps := range m.ls.proxyEndpoints {
			status.Endpoints = append(status.Endpoints, eps...)
		}
	}
	for _, epState := range m.ls.listeners {
		status.Endpoints = append(status.Endpoints, epState.leps...)
	}
	status.ProxyErrors = make(map[string]error, len(m.ls.proxyErrors))
	for k, v := range m.ls.proxyErrors {
		status.ProxyErrors[k] = v
	}
	status.ListenErrors = make(map[struct{ Protocol, Address string }]error, len(m.ls.listenErrors))
	for k, v := range m.ls.listenErrors {
		status.ListenErrors[k] = v
	}
	status.Dirty = m.ls.dirty
	m.ls.mu.Unlock()
	if len(status.Endpoints) == 0 {
		status.Endpoints = append(status.Endpoints, naming.Endpoint{Protocol: bidi.Name, RoutingID: m.rid})
	}
	return status
}

// Accept blocks until a new Flow has been initiated by a remote process.
// Flows are accepted from addresses that the Manager is listening on,
// including outgoing dialed connections.
//
// For example:
//   err := m.Listen(ctx, "tcp", ":0")
//   for {
//     flow, err := m.Accept(ctx)
//     // process flow
//   }
//
// can be used to accept Flows initiated by remote processes.
func (m *manager) Accept(ctx *context.T) (flow.Flow, error) {
	if m.ls == nil {
		return nil, errListeningWithNullRid.Errorf(ctx, "manager cannot listen when created with NullRoutingID")
	}
	item, err := m.ls.q.Get(ctx.Done())
	switch {
	case err == upcqueue.ErrQueueIsClosed:
		return nil, flow.ErrNetwork.Errorf(ctx, "%v", errManagerClosed.Errorf(ctx, "manager is already closed"))
	case err != nil:
		return nil, flow.ErrNetwork.Errorf(ctx, "%v", errAcceptFailed.Errorf(ctx, "accept failed: %v", err))
	default:
		return item.(flow.Flow), nil
	}
}

// Dial creates a Flow to the provided remote endpoint, using 'auth' to
// determine the blessings that will be sent to the remote end.
//
// If the manager has a non-null RoutingID, the Manager will re-use connections
// by Listening on Dialed connections for the lifetime of the Dialed connection.
//
// channelTimeout specifies the duration we are willing to wait before determining
// that connections managed by this Manager are unhealthy and should be
// closed.
func (m *manager) Dial(ctx *context.T, remote naming.Endpoint, auth flow.PeerAuthorizer, channelTimeout time.Duration) (flow.Flow, error) {
	return m.internalDial(ctx, remote, auth, channelTimeout, false, nil)
}

// DialSideChannelCached returns a new flow over an existing cached
// connection that is not factored in when deciding the underlying
// connection's idleness, etc.
func (m *manager) DialSideChannelCached(ctx *context.T, remote naming.Endpoint, auth flow.PeerAuthorizer, underlying flow.ManagedConn, channelTimeout time.Duration) (flow.Flow, error) {
	return m.internalDial(ctx, remote, auth, channelTimeout, false, underlying)
}

// DialCached creates a Flow to the provided remote endpoint using only cached
// connections from previous Listen or Dial calls.
// If no cached connection exists, an error will be returned.
//
// 'auth' is used to determine the blessings that will be sent to the remote end.
//
// channelTimeout specifies the duration we are willing to wait before determining
// that connections managed by this Manager are unhealthy and should be
// closed.
func (m *manager) DialCached(ctx *context.T, remote naming.Endpoint, auth flow.PeerAuthorizer, channelTimeout time.Duration) (flow.Flow, error) {
	var (
		err      error
		cached   CachedConn
		names    []string
		rejected []security.RejectedBlessing
	)
	cached, names, rejected, err = m.cache.FindCached(ctx, remote, auth)
	if err != nil {
		return nil, iflow.MaybeWrapError(flow.ErrBadState, ctx, err)
	}
	if cached == nil {
		return nil, iflow.MaybeWrapError(flow.ErrBadState, ctx, errConnNotInCache.Errorf(ctx, "connection to %v not in cache", remote.String()))
	}
	c := cached.(*conn.Conn)
	return dialFlow(ctx, c, remote, names, rejected, channelTimeout, auth, false)
}

func (m *manager) internalDial(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer,
	channelTimeout time.Duration,
	proxy bool,
	sideChannelChan flow.ManagedConn) (flow.Flow, error) {
	if m.ls != nil && len(m.ls.serverAuthorizedPeers) > 0 {
		auth = &peerAuthorizer{auth, m.ls.serverAuthorizedPeers}
	}

	if res := m.cache.Reserve(ctx, remote); res != nil {
		go m.dialReserved(res, remote, auth, channelTimeout, proxy)
	}

	var cached CachedConn
	var names []string
	var rejected []security.RejectedBlessing
	var err error
	var forSideChannel = false
	if sideChannelChan != nil {
		forSideChannel = true
		all, err := m.cache.FindAllCached(ctx, remote, auth)
		if err == nil {
			for i, c := range all {
				sc := c.(flow.ManagedConn)
				if sideChannelChan == sc {
					cached = c
					if i > 0 {
						ctx.Infof("internalDial: side channel for %v, conn %p, index: %v", remote, c, i)
					}
					break
				}
			}
		}
		if cached == nil {
			return nil, fmt.Errorf("failed to find cached conn %p", sideChannelChan)
		}
	} else {
		cached, names, rejected, err = m.cache.Find(ctx, remote, auth)
		if err != nil {
			return nil, iflow.MaybeWrapError(flow.ErrBadState, ctx, err)
		}
	}

	c, _ := cached.(*conn.Conn)

	// If the connection we found or dialed doesn't have the correct RID, assume it is a Proxy.
	if !c.MatchesRID(remote) {
		if c, names, rejected, err = m.dialProxyConn(ctx, remote, c, auth, channelTimeout); err != nil {
			return nil, err
		}
	}
	return dialFlow(ctx, c, remote, names, rejected, channelTimeout, auth, forSideChannel)
}

func (m *manager) dialReserved(
	res *Reservation,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer,
	channelTimeout time.Duration,
	proxy bool) {
	var fh conn.FlowHandler
	var c, pc *conn.Conn
	var err error
	defer func() {
		// Note we have to do this annoying shuffle because the conncache wants
		// CachedConn and if we just pass c, pc directly then we have trouble because
		// (*conn.Conn)(nil) and (CachedConn)(nil) are not the same.
		var cc, cpc CachedConn
		if c != nil {
			cc = c
		}
		if pc != nil {
			cpc = pc
		}
		res.Unreserve(cc, cpc, err) //nolint:errcheck
		// Note: 'proxy' is true when we are server "listening on" the
		// proxy. 'pc != nil' is true when we are connecting through a
		// proxy as a client. Thus, we only want to enable the
		// proxyFlowHandler when 'proxy' is true, not when 'pc != nil'
		// is true.
		m.handlerReady(fh, proxy)
	}()

	if proxyConn := res.ProxyConn(); proxyConn != nil {
		c = proxyConn.(*conn.Conn)
	}
	if c == nil {
		// We didn't find the connection we want in the cache.  Dial it.
		// TODO(mattr): In this model we're running the auth twice in
		// the case that we use the conn we dialed.  One idea is to have
		// peerAuthorizer remember it's result since we only use it for
		// this one invocation of internalDial.
		c, fh, err = m.dialConn(res.Context(), remote, auth, proxy)
		if err != nil {
			if !errors.Is(err, verror.ErrCanceled) {
				return
			}
			// Allow a canceled connection, whose handshake was completed
			// to be cached.
			if proxy {
				pc, c = c, nil
			}
			err = nil
			return
		}
	}

	// If the connection we found or dialed doesn't have the correct RID, assume it is a Proxy.
	if !c.MatchesRID(remote) {
		pc = c
		if c, _, _, err = m.dialProxyConn(res.Context(), remote, c, auth, channelTimeout); err != nil {
			pc, c = nil, pc
			return
		}
	} else if proxy {
		pc, c = c, nil
	}
}

func (m *manager) dialConn(
	ctx *context.T,
	remote naming.Endpoint,
	auth flow.PeerAuthorizer,
	proxy bool) (*conn.Conn, conn.FlowHandler, error) {
	protocol, _ := flow.RegisteredProtocol(remote.Protocol)
	flowConn, err := dial(ctx, protocol, remote.Protocol, remote.Address)
	if err != nil {
		if err, ok := err.(*net.OpError); ok {
			if err, ok := err.Err.(net.Error); ok && err.Timeout() {
				return nil, nil, iflow.MaybeWrapError(verror.ErrTimeout, ctx, err)
			}
		}
		return nil, nil, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, err)
	}
	var fh conn.FlowHandler
	if m.ls != nil {
		m.ls.mu.Lock()
		if stoppedListening := m.ls.listeners == nil; !stoppedListening {
			fh = newHybridHandler(m)
		}
		m.ls.mu.Unlock()
	}
	c, _, _, err := conn.NewDialed(
		ctx,
		flowConn,
		localEndpoint(flowConn, m.rid),
		remote,
		version.Supported,
		auth,
		false,
		handshakeTimeout,
		0,
		fh,
	)
	if errors.Is(err, verror.ErrCanceled) {
		// If the connection was canceled, it may still be dialed, so
		// allow it to be cached.
		return c, fh, err
	}
	if err != nil {
		flowConn.Close()
		return nil, nil, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, err)
	}
	return c, fh, nil
}

func (m *manager) dialProxyConn(
	ctx *context.T,
	remote naming.Endpoint,
	proxyConn *conn.Conn,
	auth flow.PeerAuthorizer,
	channelTimeout time.Duration) (*conn.Conn, []string, []security.RejectedBlessing, error) {
	f, err := proxyConn.Dial(ctx, security.Blessings{}, nil, remote, channelTimeout, false)
	if err != nil {
		return nil, nil, nil, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, err)
	}
	var fh conn.FlowHandler
	if m.ls != nil {
		m.ls.mu.Lock()
		if stoppedListening := m.ls.listeners == nil; !stoppedListening {
			fh = &flowHandler{m: m}
		}
		m.ls.mu.Unlock()
	}
	c, names, rejected, err := conn.NewDialed(
		ctx,
		f,
		proxyConn.LocalEndpoint(),
		remote,
		version.Supported,
		auth,
		true,
		handshakeTimeout,
		0,
		fh,
	)
	if err != nil {
		return nil, names, rejected, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, err)
	}
	return c, names, rejected, nil
}

func dialFlow(ctx *context.T, c *conn.Conn, remote naming.Endpoint, names []string, rejected []security.RejectedBlessing,
	channelTimeout time.Duration, auth flow.PeerAuthorizer, sideChannel bool) (flow.Flow, error) {
	// Find the proper blessings and dial the final flow.
	blessings, discharges, err := auth.BlessingsForPeer(ctx, names)
	if err != nil {
		return nil, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, errNoBlessingsForPeer.Errorf(ctx, "no blessings tagged for peer %v, rejected %v: %v}", names, rejected, err))
	}
	f, err := c.Dial(ctx, blessings, discharges, remote, channelTimeout, sideChannel)
	if err != nil {
		return nil, iflow.MaybeWrapError(flow.ErrDialFailed, ctx, err)
	}
	return f, nil
}

// RoutingID returns the naming.Routing of the flow.Manager.
func (m *manager) RoutingID() naming.RoutingID {
	return m.rid
}

// Closed returns a channel that remains open for the lifetime of the Manager
// object. Once the channel is closed any operations on the Manager will
// necessarily fail.
func (m *manager) Closed() <-chan struct{} {
	return m.closed
}

func dial(ctx *context.T, p flow.Protocol, protocol, address string) (flow.Conn, error) {
	if p != nil {
		var timeout time.Duration
		if dl, ok := ctx.Deadline(); ok {
			timeout = time.Until(dl)
		}
		type connAndErr struct {
			c flow.Conn
			e error
		}
		ch := make(chan connAndErr, 1)
		go func() {
			conn, err := p.Dial(ctx, protocol, address, timeout)
			ch <- connAndErr{conn, err}
		}()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case cae := <-ch:
			return cae.c, cae.e
		}
	}
	return nil, errUnknownProtocol.Errorf(ctx, "unknown protocol: %s", protocol)
}

func listen(ctx *context.T, protocol, address string) (flow.Listener, error) {
	if p, _ := flow.RegisteredProtocol(protocol); p != nil {
		ln, err := p.Listen(ctx, protocol, address)
		if err != nil {
			return nil, err
		}
		return ln, nil
	}
	return nil, errUnknownProtocol.Errorf(ctx, "unknown protocol: %s", protocol)
}

func localEndpoint(conn flow.Conn, rid naming.RoutingID) naming.Endpoint {
	localAddr := conn.LocalAddr()
	ep := naming.Endpoint{
		Protocol:  localAddr.Network(),
		Address:   localAddr.String(),
		RoutingID: rid,
	}
	return ep
}

func isTemporaryError(err error) bool {
	oErr, ok := err.(*net.OpError)
	return ok && oErr.Temporary()
}

func isTooManyOpenFiles(err error) bool {
	oErr, ok := err.(*net.OpError)
	return ok && strings.Contains(oErr.Err.Error(), syscall.EMFILE.Error())
}

// peerAuthorizer implements flow.PeerAuthorizer. It is meant to be used
// when a server operating in private mode (i.e., with a non-empty set
// of authorized peers) acts as a client. It wraps around the PeerAuthorizer
// specified by the call opts and addiitonally ensures that any peers that
// the client communicates with belong to the set of authorized peers.
type peerAuthorizer struct {
	auth            flow.PeerAuthorizer
	authorizedPeers []security.BlessingPattern
}

func (x *peerAuthorizer) AuthorizePeer(
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

	peerNames, rejectedPeerNames := security.RemoteBlessingNames(ctx, call)
	for _, p := range x.authorizedPeers {
		if p.MatchedBy(peerNames...) {
			return x.auth.AuthorizePeer(ctx, localEP, remoteEP, remoteBlessings, remoteDischarges)
		}
	}
	return nil, nil, fmt.Errorf("peer names: %v (rejected: %v) do not match one of the authorized patterns: %v", peerNames, rejectedPeerNames, x.authorizedPeers)
}

func (x *peerAuthorizer) BlessingsForPeer(ctx *context.T, peerNames []string) (
	security.Blessings, map[string]security.Discharge, error) {
	return x.auth.BlessingsForPeer(ctx, peerNames)
}
