// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"math/rand"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
)

const (
	proxyReconnectDelay = 50 * time.Millisecond
	proxyReResolveDelay = time.Second
)

// serverProxyAPI represents the operations the proxy manager expects of
// the rpc server.
type serverProxyAPI interface {
	resolveToEndpoint(ctx *context.T, address string) ([]naming.Endpoint, error)
	proxyListen(ctx *context.T, name string, ep naming.Endpoint) (<-chan struct{}, error)
}

// proxymanager is used to managed connections to one or more proxies.
type proxyManager struct {
	sync.Mutex
	reconnectDelay time.Duration
	resolveDelay   time.Duration
	server         serverProxyAPI
	policy         rpc.ProxyPolicy
	rand           *rand.Rand //
	limit          int        // max number of proxies to use.
	proxyName      string
	proxies        []naming.Endpoint
	active         map[string]bool
}

func newProxyManager(s serverProxyAPI, proxyName string, policy rpc.ProxyPolicy, limit int) *proxyManager {
	pm := &proxyManager{
		reconnectDelay: proxyReconnectDelay,
		resolveDelay:   proxyReResolveDelay,
		server:         s,
		policy:         policy,
		limit:          limit,
		proxyName:      proxyName,
		active:         map[string]bool{},
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	switch policy {
	case rpc.UseFirstProxy, rpc.UseRandomProxy:
		pm.limit = 1
	}
	return pm
}

func (pm *proxyManager) selectRandomSubsetLocked(needed, available int) map[int]bool {
	selected := map[int]bool{}
	for {
		candidate := pm.rand.Intn(available)
		if selected[candidate] {
			continue
		}
		selected[candidate] = true
		if len(selected) == needed {
			return selected
		}
	}
}

func (pm *proxyManager) isAvailable(proxy naming.Endpoint) bool {
	addr := proxy.String()
	pm.Lock()
	defer pm.Unlock()
	for _, ep := range pm.proxies {
		if ep.String() == addr {
			return true
		}
	}
	return false
}

// idle returns the set of proxies that are availabled but not currently active.
// The returned number of proxies is capped at pm.limit if is not zero.
func (pm *proxyManager) idle(ctx *context.T) []naming.Endpoint {
	idle := []naming.Endpoint{}
	pm.Lock()
	defer pm.Unlock()
	for _, v := range pm.proxies {
		if !pm.active[v.String()] {
			idle = append(idle, v)
		}
	}
	nidle := len(idle)
	if nidle == 0 {
		return idle
	}
	switch pm.policy {
	case rpc.UseFirstProxy:
		ctx.Infof("idle proxies: first proxy %v/%v", idle[:1], nidle)
		return idle[:1]
	case rpc.UseRandomProxy:
		chosen := pm.rand.Intn(nidle)
		ctx.Infof("idle proxies: random proxy %v/%v", chosen, nidle)
		return []naming.Endpoint{idle[chosen]}
	}
	needed := pm.limit - len(pm.active)
	if pm.limit == 0 || needed > nidle {
		ctx.Infof("idle proxies: all idle proxies %v", nidle)
		return idle
	}
	// randomize the selection of the subset of proxies.
	selected := make([]naming.Endpoint, needed)
	i := 0
	for idx := range pm.selectRandomSubsetLocked(needed, nidle) {
		selected[i] = idle[idx]
		i++
	}
	ctx.Infof("idle proxies: %v of %v idle proxies: %v", needed, nidle, selected)
	return selected
}

// shouldGrow returns true if the proxyManager should attempt to connect
// to more proxies as they become available.
func (pm *proxyManager) shouldGrow() bool {
	pm.Lock()
	defer pm.Unlock()
	switch pm.policy {
	case rpc.UseFirstProxy, rpc.UseRandomProxy:
		return len(pm.active) == 0
	}
	return (pm.limit == 0) || (pm.limit > len(pm.active))
}

// canGrow returns true if there are inactive available proxies.
func (pm *proxyManager) canGrow() bool {
	pm.Lock()
	defer pm.Unlock()
	switch pm.policy {
	case rpc.UseFirstProxy, rpc.UseRandomProxy:
		return len(pm.active) == 0 && len(pm.proxies) > 0
	}
	return len(pm.proxies) > len(pm.active)
}

// updateAvailableProxies reresolves the proxy with the mounttable to determine
// the cureent set of available proxies.
func (pm *proxyManager) updateAvailableProxies(ctx *context.T) {
	eps, err := pm.server.resolveToEndpoint(ctx, pm.proxyName)
	if err != nil {
		ctx.VI(2).Infof("resolveToEndpoint(%q) failed: %v", pm.proxyName, err)
		return
	}
	updated := make([]naming.Endpoint, len(eps))
	copy(updated, eps)
	pm.Lock()
	defer pm.Unlock()
	pm.proxies = updated
}

func (pm *proxyManager) markActive(ep naming.Endpoint) {
	pm.Lock()
	defer pm.Unlock()
	pm.active[ep.String()] = true
}

func (pm *proxyManager) markInActive(ep naming.Endpoint) {
	pm.Lock()
	defer pm.Unlock()
	delete(pm.active, ep.String())
}

func (pm *proxyManager) connectToSingleProxy(ctx *context.T, name string, ep naming.Endpoint) {
	pm.markActive(ep)
	defer pm.markInActive(ep)
	for delay := pm.reconnectDelay; ; delay = nextDelay(delay) {
		if !pm.isAvailable(ep) {
			ctx.Infof("connectToSingleProxy(%q): proxy is no longer available\n", ep)
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		time.Sleep(delay - pm.reconnectDelay)
		ch, err := pm.server.proxyListen(ctx, name, ep)
		if err != nil {
			ctx.Errorf("connectToSingleProxy(%q) failed: %v. Reconnecting..", ep, err)
			// Update the set of available proxies on encountering an error and
			// possible exit at the top the loop.
			pm.updateAvailableProxies(ctx)
		} else {
			select {
			case <-ctx.Done():
				return
			case <-ch:
			}
			delay = pm.reconnectDelay / 2
		}
	}
}

func (pm *proxyManager) tryConnections(ctx *context.T, notifyCh chan struct{}) bool {
	idle := pm.idle(ctx)
	if len(idle) == 0 {
		pm.updateAvailableProxies(ctx)
		return false
	}
	for _, ep := range idle {
		go func(ep naming.Endpoint) {
			pm.connectToSingleProxy(ctx, pm.proxyName, ep)
			notifyCh <- struct{}{}
		}(ep)
	}
	return true
}

func drainNotifyChan(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func (pm *proxyManager) manageProxyConnections(ctx *context.T) {
	notifyCh := make(chan struct{}, 10)
	pm.updateAvailableProxies(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if !pm.tryConnections(ctx, notifyCh) {
			// Nothing to do, sleep before retrying.
			time.Sleep(pm.resolveDelay)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-notifyCh:
			// Wait for a change and then drain the channel of any pending
			// notifications before re-evaluating the set of available proxies.
			drainNotifyChan(notifyCh)
		}
		// Wait for a change in the set of available proxies.
		if pm.shouldGrow() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(pm.resolveDelay):
				}
				pm.updateAvailableProxies(ctx)
				if pm.canGrow() {
					break
				}
			}
		}
	}
}
