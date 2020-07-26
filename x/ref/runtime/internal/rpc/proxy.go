package rpc

import (
	"math/rand"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
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
		s.ctx.Infof("Using first proxy %v/%v", idle[0], nidle)
		return idle[:1]
	case rpc.UseRandomProxy:
		chosen := pm.rand.Intn(nidle)
		s.ctx.Infof("Using random proxy %v/%v", chosen, nidle)
		return []naming.Endpoint{idle[chosen]}
	}
	if pm.limit != 0 && pm.limit < nidle {
		// randomize the selection of the subset of proxies.
		selected := make([]naming.Endpoint, pm.limit)
		i := 0
		for idx := range pm.selectRandomSubsetLocked(pm.limit, nidle) {
			selected[i] = idle[idx]
			i++
		}
		s.ctx.Infof("Using %v of %v proxies: %v", pm.limit, nidle, selected)
		return selected
	}
	s.ctx.Infof("Using all proxies %v", nidle)
	return idle
}

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
		select {
		case <-ctx.Done():
			return
		default:
		}
		time.Sleep(delay - reconnectDelay)
		ch, err := s.flowMgr.ProxyListen(ctx, name, ep)
		if err != nil {
			ctx.Errorf("ProxyListen(%q) failed: %v. Reconnecting...", ep, err)
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
	for {
		pm.updateAvailableProxies(ctx, s)
		select {
		case <-ctx.Done():
			return
		default:
		}
		idle := pm.idle(s)
		if len(idle) == 0 {
			time.Sleep(proxyResolveUpdatePeriod)
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
	}
}
