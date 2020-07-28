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
	reconnectDelay           = 50 * time.Millisecond
	proxyResolveUpdatePeriod = time.Second
)

// proxymanager is used to managed connections to one or more proxies.
type proxyManager struct {
	sync.Mutex
	policy    rpc.ProxyPolicy
	rand      *rand.Rand //
	limit     int        // max number of proxies to use.
	proxyName string
	proxies   []naming.Endpoint
	active    map[string]bool
}

func newProxyManager(proxyName string, policy rpc.ProxyPolicy, limit int) *proxyManager {
	pm := &proxyManager{
		policy:    policy,
		limit:     limit,
		proxyName: proxyName,
		active:    map[string]bool{},
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
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
func (pm *proxyManager) idle(s *server) []naming.Endpoint {
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
		s.ctx.Infof("idle proxies: first proxy %v/%v", idle[:1], nidle)
		return idle[:1]
	case rpc.UseRandomProxy:
		chosen := pm.rand.Intn(nidle)
		s.ctx.Infof("idle proxies: random proxy %v/%v", chosen, nidle)
		return []naming.Endpoint{idle[chosen]}
	}
	needed := pm.limit - len(pm.active)
	if pm.limit == 0 || needed > nidle {
		s.ctx.Infof("idle proxies: all idle proxies %v", nidle)
		return idle
	}
	// randomize the selection of the subset of proxies.
	selected := make([]naming.Endpoint, needed)
	i := 0
	for idx := range pm.selectRandomSubsetLocked(needed, nidle) {
		selected[i] = idle[idx]
		i++
	}
	s.ctx.Infof("idle proxies: %v of %v idle proxies: %v", needed, nidle, selected)
	return selected
}

// shouldGrow returns true if the proxyManager should attempt to connect
// to more proxies as they become available.
func (pm *proxyManager) shouldGrow() bool {
	pm.Lock()
	defer pm.Unlock()
	switch pm.policy {
	case rpc.UseFirstProxy, rpc.UseRandomProxy:
		return len(pm.active) < 1
	}
	return (pm.limit == 0) || (pm.limit > len(pm.active))
}

// canGrow returns true if there are inactive available proxies.
func (pm *proxyManager) canGrow() bool {
	pm.Lock()
	defer pm.Unlock()
	return len(pm.proxies) > len(pm.active)
}

/*
func compareEndpointSlices(a, b []naming.Endpoint) bool {
	if len(a) != len(b) {
		return true
	}
	al, bl := make([]string, len(a)), make([]string, len(b))
	for i := 0; i < len(a); i++ {
		al[i] = a[i].String()
		bl[i] = b[i].String()
	}
	sort.Strings(al)
	sort.Strings(bl)
	for i, e := range al {
		if e != bl[i] {
			return true
		}
	}
	return false
}*/

// updateAvailableProxies reresolves the proxy with the mounttable to determine
// the cureent set of available proxies.
func (pm *proxyManager) updateAvailableProxies(ctx *context.T, s *server) {
	eps, err := s.resolveToEndpoint(ctx, pm.proxyName)
	if err != nil {
		s.ctx.VI(2).Infof("resolveToEndpoint(%q) failed: %v", pm.proxyName, err)
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

func (pm *proxyManager) connectToSingleProxy(ctx *context.T, s *server, name string, ep naming.Endpoint) {
	pm.markActive(ep)
	defer pm.markInActive(ep)
	for delay := reconnectDelay; ; delay = nextDelay(delay) {
		if !pm.isAvailable(ep) {
			ctx.Infof("ProxyListen(%q): proxy is no longer available\n", ep)
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		time.Sleep(delay - reconnectDelay)
		ch, err := s.flowMgr.ProxyListen(ctx, name, ep)
		if err != nil {
			ctx.Errorf("ProxyListen(%q) failed: %v. Reconnecting..", ep, err)
			// Update the set of available proxies on encountering an error and
			// popssible exit at the top the loop.
			pm.updateAvailableProxies(ctx, s)
		} else {
			select {
			case <-ctx.Done():
				return
			case <-ch:
			}
			delay = reconnectDelay / 2
		}
	}
}

func (pm *proxyManager) manageProxyConnections(ctx *context.T, s *server) {
	notifyCh := make(chan struct{}, 10)
	pm.updateAvailableProxies(ctx, s)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		idle := pm.idle(s)
		if len(idle) == 0 {
			time.Sleep(proxyResolveUpdatePeriod)
			pm.updateAvailableProxies(ctx, s)
			continue
		}
		for _, ep := range idle {
			go func(ep naming.Endpoint) {
				pm.connectToSingleProxy(ctx, s, pm.proxyName, ep)
				notifyCh <- struct{}{}
			}(ep)
		}
		select {
		case <-ctx.Done():
			return
		case <-notifyCh:
			// Wait for a change and then drain the channel of any pending
			// notifications before re-evaluating the set of available proxies.
			for {
				select {
				case <-notifyCh:
				default:
					goto done
				}
			}
		done:
		}
		// Wait for a change in the set of available proxies.
		if pm.shouldGrow() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(proxyResolveUpdatePeriod):
				}
				pm.updateAvailableProxies(ctx, s)
				if pm.canGrow() {
					ctx.Infof("can grow....")
					break
				}
			}
		}
	}
}
