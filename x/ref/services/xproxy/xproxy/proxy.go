// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xproxy contains the implementation of the proxy service.
//
// Design document at https://docs.google.com/document/d/1ONrnxGhOrA8pd0pK0aN5Ued2Q1Eju4zn7vlLt9nzIRA/edit?usp=sharing
package xproxy

import (
	"io"
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/x/ref/lib/publisher"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/counter"
)

const (
	reconnectDelay   = 50 * time.Millisecond
	maxBackoff       = time.Minute
	bidiProtocol     = "bidi"
	relistenInterval = time.Second
)

type perDirection struct {
	msgs  *counter.Counter
	bytes *counter.Counter
}

type perServerStats struct {
	requests *counter.Counter
	to, from perDirection
}

func newPerServerStats(ep string) *perServerStats {
	return &perServerStats{
		requests: stats.NewCounter(naming.Join(ep, "requests")),
		to: perDirection{
			msgs:  stats.NewCounter(naming.Join(ep, "to", "msgs")),
			bytes: stats.NewCounter(naming.Join(ep, "to", "bytes")),
		},
		from: perDirection{
			msgs:  stats.NewCounter(naming.Join(ep, "from", "msgs")),
			bytes: stats.NewCounter(naming.Join(ep, "from", "bytes")),
		},
	}
}

// Proxy represents an instance of a proxy service.
type Proxy struct {
	m      flow.Manager
	pub    *publisher.T
	closed chan struct{}
	auth   security.Authorizer
	wg     sync.WaitGroup

	mu                 sync.Mutex
	listeningEndpoints map[string]naming.Endpoint   // keyed by endpoint string
	proxyEndpoints     map[string][]naming.Endpoint // keyed by proxy address
	proxiedStats       map[string]*perServerStats   // stats for each server that is listening through us.
	proxiedProxies     []flow.Flow                  // flows of proxies that are listening through us.
	proxiedServers     []flow.Flow                  // flows of servers that are listening through us.
	closing            bool
}

// New returns a new instance of Proxy.
func New(ctx *context.T, name string, auth security.Authorizer) (*Proxy, error) {
	mgr, err := v23.NewFlowManager(ctx, 0)
	if err != nil {
		return nil, err
	}
	p := &Proxy{
		m:                  mgr,
		auth:               auth,
		proxiedStats:       make(map[string]*perServerStats),
		proxyEndpoints:     make(map[string][]naming.Endpoint),
		listeningEndpoints: make(map[string]naming.Endpoint),
		pub:                publisher.New(ctx, v23.GetNamespace(ctx), time.Minute),
		closed:             make(chan struct{}),
	}
	if p.auth == nil {
		p.auth = security.DefaultAuthorizer()
	}
	if len(name) > 0 {
		p.pub.AddName(name, false, true)
	}
	lspec := v23.GetListenSpec(ctx)
	if len(lspec.Proxy) > 0 {
		p.wg.Add(1)
		go p.connectToProxy(ctx, lspec.Proxy)
	}
	for _, addr := range lspec.Addrs {
		ch, err := p.m.Listen(ctx, addr.Protocol, addr.Address)
		if err != nil {
			ctx.Errorf("proxy failed to listen on %v, %v", addr.Protocol, addr.Address)
		}
		p.wg.Add(1)
		go p.relisten(ctx, addr.Protocol, addr.Address, ch, err)
	}
	mgrStat := p.m.Status()
	leps, changed := mgrStat.Endpoints, mgrStat.Dirty
	p.updateListeningEndpoints(ctx, leps)
	p.wg.Add(2)
	go p.updateEndpointsLoop(ctx, changed)
	go p.listenLoop(ctx)
	go func() {
		<-ctx.Done()
		p.mu.Lock()
		p.closing = true
		p.mu.Unlock()
		<-p.pub.Closed()
		p.wg.Wait()
		<-p.m.Closed()
		close(p.closed)
	}()
	return p, nil
}

// Closed returns a channel that will be closed when the proxy is shutdown.
func (p *Proxy) Closed() <-chan struct{} {
	return p.closed
}

func (p *Proxy) updateEndpointsLoop(ctx *context.T, changed <-chan struct{}) {
	defer p.wg.Done()
	for changed != nil {
		<-changed
		mgrStat := p.m.Status()
		changed = mgrStat.Dirty
		p.updateListeningEndpoints(ctx, mgrStat.Endpoints)
	}
}

func (p *Proxy) updateListeningEndpoints(ctx *context.T, leps []naming.Endpoint) {
	p.mu.Lock()
	endpoints := make(map[string]naming.Endpoint)
	for _, ep := range leps {
		endpoints[ep.String()] = ep
	}
	rmEps := setDiff(p.listeningEndpoints, endpoints)
	addEps := setDiff(endpoints, p.listeningEndpoints)
	for k := range rmEps {
		delete(p.listeningEndpoints, k)
	}
	for k, ep := range addEps {
		p.listeningEndpoints[k] = ep
	}

	p.sendUpdatesLocked(ctx)
	p.mu.Unlock()

	for k, ep := range rmEps {
		if ep.Addr().Network() != bidiProtocol {
			p.pub.RemoveServer(k)
		}
	}
	for k, ep := range addEps {
		if ep.Addr().Network() != bidiProtocol {
			p.pub.AddServer(k)
		}
	}
}

func (p *Proxy) sendUpdatesLocked(ctx *context.T) {
	// Send updates to the proxies and servers that are listening through us.
	// TODO(suharshs): Should we send these in parallel?
	i := 0
	for _, f := range p.proxiedProxies {
		if !isClosed(f) {
			if err := p.replyToProxyLocked(ctx, f); err != nil {
				ctx.Error(err)
				continue
			}
			p.proxiedProxies[i] = f
			i++
		}
	}
	p.proxiedProxies = p.proxiedProxies[:i]
	i = 0
	for _, f := range p.proxiedServers {
		if !isClosed(f) {
			if err := p.replyToServerLocked(ctx, f); err != nil {
				ctx.Error(err)
				continue
			}
			p.proxiedServers[i] = f
			i++
		}
	}
	p.proxiedServers = p.proxiedServers[:i]
}

func isClosed(f flow.Flow) bool {
	select {
	case <-f.Closed():
		return true
	default:
	}
	return false
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

// ListeningEndpoints returns the endpoints the proxy is listening on.
func (p *Proxy) ListeningEndpoints() []naming.Endpoint {
	// TODO(suharshs): Return other struct information here as well.
	return p.m.Status().Endpoints
}

/*
// MultipleProxyEndpoints returns the endpoints that the proxy is forwarding.
func (p *Proxy) MultipleProxyEndpoints() []naming.Endpoint {
	var eps []naming.Endpoint
	p.mu.Lock()
	for _, v := range p.proxyEndpoints {
		eps = append(eps, v...)
	}
	p.mu.Unlock()
	return eps
}
*/

func (p *Proxy) handleConnection(ctx *context.T, f flow.Flow) {
	defer p.wg.Done()
	msg, err := readMessage(ctx, f)
	if err != nil {
		ctx.Errorf("reading message failed: %v", err)
		return
	}

	switch m := msg.(type) {
	case *message.Setup:
		err = p.startRouting(ctx, f, m)
		if err == nil {
			ctx.VI(1).Infof("Routing client flow from %v", f.RemoteEndpoint())
		} else {
			ctx.Errorf("failed to handle incoming client flow from %v: %v", f.RemoteEndpoint(), err)
		}
	case *message.MultiProxyRequest:
		p.mu.Lock()
		err = p.replyToProxyLocked(ctx, f)
		if err == nil {
			p.proxiedProxies = append(p.proxiedProxies, f)
			ctx.Infof("Proxying proxy at %v", f.RemoteEndpoint())
		} else {
			ctx.Errorf("failed to multi-proxy proxy at %v: %v", f.RemoteEndpoint(), err)
		}
		p.mu.Unlock()
	case *message.ProxyServerRequest:
		p.mu.Lock()
		stats := p.statsForLocked(f.RemoteEndpoint())
		stats.requests.Incr(1)
		err = p.replyToServerLocked(ctx, f)
		if err == nil {
			p.proxiedServers = append(p.proxiedServers, f)
			ctx.Infof("Proxying server at %v", f.RemoteEndpoint())
		} else {
			ctx.Errorf("failed to proxy server at %v: %v", f.RemoteEndpoint(), err)
		}
		p.mu.Unlock()
	default:
		ctx.VI(1).Infof("Ignoring unexpected proxy message: %#v", msg)
	}
}

func (p *Proxy) listenLoop(ctx *context.T) {
	defer p.wg.Done()
	for {
		f, err := p.m.Accept(ctx)
		if err != nil {
			ctx.Infof("p.m.Accept failed: %v", err)
			break
		}
		p.mu.Lock()
		if p.closing {
			p.mu.Unlock()
			break
		}
		p.wg.Add(1)
		p.mu.Unlock()
		go p.handleConnection(ctx, f)
	}
}

func (p *Proxy) statsForLocked(ep naming.Endpoint) *perServerStats {
	rid := ep.RoutingID.String()
	stats := p.proxiedStats[rid]
	if stats == nil {
		stats = newPerServerStats(rid)
		p.proxiedStats[rid] = stats
	}
	return stats
}

func (p *Proxy) startRouting(ctx *context.T, f flow.Flow, m *message.Setup) error {
	fout, err := p.dialNextHop(ctx, f, m)
	if err != nil {
		f.Close()
		return err
	}
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return ErrProxyAlreadyClosed.Errorf(ctx, "proxy has already been closed")
	}
	// Configure stats.
	stats := p.statsForLocked(m.PeerRemoteEndpoint)
	f.DisableFragmentation()
	fout.DisableFragmentation()
	p.wg.Add(2)
	p.mu.Unlock()
	go p.forwardLoop(ctx, f, fout, &stats.to)
	go p.forwardLoop(ctx, fout, f, &stats.from)
	return nil
}

func (p *Proxy) forwardLoop(ctx *context.T, fin, fout flow.Flow, ps *perDirection) {
	defer p.wg.Done()
	n, err := framedCopy(fin, fout)
	if err != nil && err != io.EOF {
		ctx.Errorf("Error forwarding: %v", err)
	}
	ps.msgs.Incr(1)
	ps.bytes.Incr(n)
	fin.Close()
	fout.Close()
}

func framedCopy(fin, fout flow.Flow) (int64, error) {
	total := 0
	for {
		msg, err := fin.ReadMsg()
		total += len(msg)
		if err != nil {
			if err == io.EOF {
				_, err = fout.WriteMsg(msg)
			}
			return int64(total), err
		}
		if _, err = fout.WriteMsg(msg); err != nil {
			return int64(total), err
		}
	}
}

func (p *Proxy) dialNextHop(ctx *context.T, f flow.Flow, m *message.Setup) (flow.Flow, error) {
	var (
		rid naming.RoutingID
		err error
	)
	ep := m.PeerRemoteEndpoint.WithBlessingNames(nil)
	ep.Protocol = bidiProtocol
	if routes := ep.Routes(); len(routes) > 0 {
		if err := rid.FromString(routes[0]); err != nil {
			return nil, err
		}
		// Make an endpoint with the correct routingID to dial out. All other fields
		// do not matter.
		// TODO(suharshs): Make sure that the routingID from the route belongs to a
		// connection that is stored in the manager's cache. (i.e. a Server has connected
		// with the routingID before)
		ep.RoutingID = rid
		// Remove the read route from the setup message endpoint.
		m.PeerRemoteEndpoint = m.PeerRemoteEndpoint.WithRoutes(routes[1:])
	}
	fout, err := p.m.Dial(ctx, ep, proxyAuthorizer{}, 0)
	if err != nil {
		return nil, err
	}
	if err := p.authorizeFlow(ctx, fout); err != nil {
		return nil, err
	}
	// Write the setup message back onto the flow for the next hop to read.
	return fout, writeMessage(ctx, m, fout)
}

func (p *Proxy) replyToServerLocked(ctx *context.T, f flow.Flow) error {
	if err := p.authorizeFlow(ctx, f); err != nil {
		if f.Conn().CommonVersion() >= version.RPCVersion13 {
			//nolint:errcheck
			writeMessage(ctx, &message.ProxyErrorResponse{Error: err.Error()}, f)
		}
		return err
	}
	rid := f.RemoteEndpoint().RoutingID
	eps, err := p.returnEndpointsLocked(ctx, rid, "")
	if err != nil {
		if f.Conn().CommonVersion() >= version.RPCVersion13 {
			//nolint:errcheck
			writeMessage(ctx, &message.ProxyErrorResponse{Error: err.Error()}, f)
		}
		return err
	}
	return writeMessage(ctx, &message.ProxyResponse{Endpoints: eps}, f)
}

func (p *Proxy) authorizeFlow(ctx *context.T, f flow.Flow) error {
	call := security.NewCall(&security.CallParams{
		LocalPrincipal:   v23.GetPrincipal(ctx),
		LocalBlessings:   f.LocalBlessings(),
		RemoteBlessings:  f.RemoteBlessings(),
		LocalEndpoint:    f.LocalEndpoint(),
		RemoteEndpoint:   f.RemoteEndpoint(),
		RemoteDischarges: f.RemoteDischarges(),
	})
	return p.auth.Authorize(ctx, call)
}

func (p *Proxy) replyToProxyLocked(ctx *context.T, f flow.Flow) error {
	// Add the routing id of the incoming proxy to the routes. The routing id of the
	// returned endpoint doesn't matter because it will eventually be replaced
	// by a server's rid by some later proxy.
	// TODO(suharshs): Use a local route instead of this global routingID.
	rid := f.RemoteEndpoint().RoutingID
	eps, err := p.returnEndpointsLocked(ctx, naming.NullRoutingID, rid.String())
	if err != nil {
		if f.Conn().CommonVersion() >= version.RPCVersion13 {
			//nolint:errcheck
			writeMessage(ctx, &message.ProxyErrorResponse{Error: err.Error()}, f)
		}
		return err
	}
	return writeMessage(ctx, &message.ProxyResponse{Endpoints: eps}, f)
}

func (p *Proxy) returnEndpointsLocked(ctx *context.T, rid naming.RoutingID, route string) ([]naming.Endpoint, error) {
	eps := p.m.Status().Endpoints
	for _, peps := range p.proxyEndpoints {
		eps = append(eps, peps...)
	}
	if len(eps) == 0 {
		return nil, ErrNotListening.Errorf(ctx, "proxy has already been closed")
	}
	for idx, ep := range eps {
		if rid != naming.NullRoutingID {
			ep.RoutingID = rid
		}
		if len(route) > 0 {
			var cp []string
			cp = append(cp, ep.Routes()...)
			cp = append(cp, route)
			ep = ep.WithRoutes(cp)
		}
		eps[idx] = ep
	}
	return eps, nil
}

// relisten continuously tries to listen on the protocol, address.
// If err != nil, relisten will attempt to listen on the protocol, address immediately, since
// the previous attempt failed.
// Otherwise, ch will be non-nil, and relisten will attempt to relisten once ch is closed.
func (p *Proxy) relisten(ctx *context.T, protocol, address string, ch <-chan struct{}, err error) {
	defer p.wg.Done()
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
		if ch, err = p.m.Listen(ctx, protocol, address); err != nil {
			ctx.Errorf("Listen(%q, %q, ...) failed: %v", protocol, address, err)
		}
	}
}

func (p *Proxy) connectToProxy(ctx *context.T, name string) {
	defer p.wg.Done()
	for delay := reconnectDelay; ; delay = nextDelay(delay) {
		time.Sleep(delay - reconnectDelay)
		select {
		case <-ctx.Done():
			return
		default:
		}
		eps, err := resolveToEndpoint(ctx, name)
		if err != nil {
			ctx.Error(err)
			continue
		}
		if err = p.tryProxyEndpoints(ctx, name, eps); err != nil {
			ctx.Error(err)
		} else {
			delay = reconnectDelay / 2
		}
	}
}

func (p *Proxy) tryProxyEndpoints(ctx *context.T, name string, eps []naming.Endpoint) error {
	var lastErr error
	for _, ep := range eps {
		if lastErr = p.proxyListen(ctx, name, ep); lastErr == nil {
			break
		}
	}
	return lastErr
}

func (p *Proxy) proxyListen(ctx *context.T, name string, ep naming.Endpoint) error {
	defer p.updateProxyEndpoints(ctx, name, nil)
	f, err := p.m.Dial(ctx, ep, proxyAuthorizer{}, 0)
	if err != nil {
		return err
	}
	// Send a byte telling the acceptor that we are a proxy.
	if err := writeMessage(ctx, &message.MultiProxyRequest{}, f); err != nil {
		return err
	}
	for {
		// we keep reading updates until we encounter an error, usually because the
		// flow has been closed.
		eps, err := readProxyResponse(ctx, f)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		p.updateProxyEndpoints(ctx, name, eps)
	}
}

func nextDelay(delay time.Duration) time.Duration {
	delay *= 2
	if delay > maxBackoff {
		delay = maxBackoff
	}
	return delay
}

func (p *Proxy) updateProxyEndpoints(ctx *context.T, address string, eps []naming.Endpoint) {
	p.mu.Lock()
	if len(eps) > 0 {
		p.proxyEndpoints[address] = eps
	} else {
		delete(p.proxyEndpoints, address)
	}
	p.sendUpdatesLocked(ctx)
	p.mu.Unlock()
}

func resolveToEndpoint(ctx *context.T, name string) ([]naming.Endpoint, error) {
	ns := v23.GetNamespace(ctx)
	ns.FlushCacheEntry(ctx, name)
	resolved, err := ns.Resolve(ctx, name)
	if err != nil {
		return nil, err
	}
	var eps []naming.Endpoint
	for _, n := range resolved.Names() {
		address, suffix := naming.SplitAddressName(n)
		if len(suffix) > 0 {
			continue
		}
		if ep, err := naming.ParseEndpoint(address); err == nil {
			eps = append(eps, ep)
			continue
		}
	}
	if len(eps) > 0 {
		return eps, nil
	}
	return nil, ErrorfFailedToResolveToEndpoint(ctx, "failed to resolve '%v' to endpoint", name)
}
