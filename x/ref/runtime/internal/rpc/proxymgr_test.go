// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/x/ref/test"
)

type mockServerAPI struct {
	sync.Mutex
	eps       map[string][]naming.Endpoint
	chs       map[string]chan struct{}
	errors    map[string]error
	listening map[string][]naming.Endpoint
}

func newMockServer() *mockServerAPI {
	return &mockServerAPI{
		eps:       map[string][]naming.Endpoint{},
		chs:       map[string]chan struct{}{},
		errors:    map[string]error{},
		listening: map[string][]naming.Endpoint{},
	}
}

func (ms *mockServerAPI) setEndpoints(name string, eps ...naming.Endpoint) {
	ms.Lock()
	defer ms.Unlock()
	ms.eps[name] = eps
}

func (ms *mockServerAPI) setChan(name string, ch chan struct{}) {
	ms.Lock()
	defer ms.Unlock()
	ms.chs[name] = ch
}

func (ms *mockServerAPI) setError(name string, err error) {
	ms.Lock()
	defer ms.Unlock()
	ms.errors[name] = err
}

func (ms *mockServerAPI) resolveToEndpoint(ctx *context.T, address string) ([]naming.Endpoint, error) {
	ms.Lock()
	defer ms.Unlock()
	return ms.eps[address], nil
}

func (ms *mockServerAPI) clearListen(name string) {
	ms.Lock()
	defer ms.Unlock()
	ms.listening[name] = nil
}

func (ms *mockServerAPI) proxyListen(ctx *context.T, name string, ep naming.Endpoint) (<-chan struct{}, error) {
	ms.Lock()
	defer ms.Unlock()
	if err := ms.errors[name]; err != nil {
		return nil, err
	}
	ms.listening[name] = append(ms.listening[name], ep)
	return ms.chs[name], nil
}

func newEndpoint(port string) naming.Endpoint {
	rid, _ := naming.NewRoutingID()
	return naming.Endpoint{
		Protocol:  "tcp",
		Address:   net.JoinHostPort("127.0.0.1", port),
		RoutingID: rid,
	}
}

func testGrowth(t *testing.T, pm *proxyManager, should, can bool) {
	_, file, line, _ := runtime.Caller(1)
	loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)
	if got, want := pm.shouldGrow(), should; got != want {
		t.Errorf("%v: shouldGrow: got %v, want %v", loc, got, want)
	}
	if got, want := pm.canGrow(), can; got != want {
		t.Errorf("%v: canGrow got %v, want %v", loc, got, want)
	}
}

func TestFirstAndRandomProxyPolicyMethods(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	sm := newMockServer()
	ep1 := newEndpoint("5000")
	ep2 := newEndpoint("5001")

	fpm := newProxyManager(sm, "proxy", rpc.UseFirstProxy, 10)
	rpm := newProxyManager(sm, "proxy", rpc.UseRandomProxy, 10)

	for i, pm := range []*proxyManager{fpm, rpm} {
		sm.setEndpoints("proxy")
		if got, want := pm.limit, 1; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		idle := pm.idle(ctx)
		if got, want := len(idle), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := pm.isAvailable(ep1), false; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		testGrowth(t, pm, true, false)

		sm.setEndpoints("proxy", ep1)
		pm.updateAvailableProxies(ctx)

		if got, want := pm.isAvailable(ep1), true; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		testGrowth(t, pm, true, true)

		pm.markActive(ep1)

		testGrowth(t, pm, false, false)

		pm.markInActive(ep1)

		testGrowth(t, pm, true, true)

		sm.setEndpoints("proxy", ep2, ep1)
		pm.updateAvailableProxies(ctx)

		if got, want := pm.isAvailable(ep2), true; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		idle = pm.idle(ctx)
		if got, want := len(idle), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
			continue
		}
		pm.markActive(idle[0])

		testGrowth(t, pm, false, false)

		idle = pm.idle(ctx)
		if got, want := len(idle), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		pm.markActive(idle[0])
		idle = pm.idle(ctx)
		if got, want := len(idle), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	ep3 := newEndpoint("5002")
	ep4 := newEndpoint("5003")
	sm.setEndpoints("proxy", ep4, ep3, ep2, ep1)

	// Must select the first one.
	fpm.updateAvailableProxies(ctx)
	idle := fpm.idle(ctx)
	if got, want := idle[0].String(), ep4.String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Must at some point select an endpoint other than the first one.
	rpm.updateAvailableProxies(ctx)
	testForRandomSelection(ctx, rpm, ep3, ep2, ep1)
}

func testForRandomSelection(ctx *context.T, pm *proxyManager, eps ...naming.Endpoint) {
	// should eventually chose a proxy other than the first.
	for {
		idle := pm.idle(ctx)
		chosen := idle[0].String()
		// declare victory when any proxy other than the first is returned.
		for _, ep := range eps {
			if ep.String() == chosen {
				return
			}
		}
	}
}

func TestAllProxyPolicyMethods(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	sm := newMockServer()
	ep1 := newEndpoint("5000")
	ep2 := newEndpoint("5001")
	ep3 := newEndpoint("5002")
	ep4 := newEndpoint("5003")

	for _, limit := range []int{0, 10} {
		sm.setEndpoints("proxy", ep4, ep3, ep2, ep1)
		apm := newProxyManager(sm, "proxy", rpc.UseAllProxies, limit)
		apm.updateAvailableProxies(ctx)
		if got, want := len(apm.idle(ctx)), 4; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	apm := newProxyManager(sm, "proxy", rpc.UseAllProxies, 3)
	if got, want := len(apm.idle(ctx)), 0; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	testGrowth(t, apm, true, false)

	sm.setEndpoints("proxy", ep2, ep1)
	apm.updateAvailableProxies(ctx)
	if got, want := len(apm.idle(ctx)), 2; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	sm.setEndpoints("proxy", ep4, ep3, ep2, ep1)
	apm.updateAvailableProxies(ctx)
	idle := apm.idle(ctx)
	if got, want := len(idle), 3; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	testGrowth(t, apm, true, true)
	for _, ep := range idle {
		apm.markActive(ep)
	}
	testGrowth(t, apm, false, true)
	apm.markInActive(idle[0])
	testGrowth(t, apm, true, true)

}

func TestSingleProxyConnections(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	sm := newMockServer()
	ep1 := newEndpoint("5000")

	fpm := newProxyManager(sm, "proxy0", rpc.UseFirstProxy, 10)
	rpm := newProxyManager(sm, "proxy1", rpc.UseRandomProxy, 10)
	apm := newProxyManager(sm, "proxy2", rpc.UseAllProxies, 10)

	for i, pm := range []*proxyManager{fpm, rpm, apm} {
		proxyName := fmt.Sprintf("proxy%v", i)
		sm.setEndpoints(proxyName, ep1)
		ch := make(chan struct{})
		sm.setChan(proxyName, ch)
		pm.updateAvailableProxies(ctx)
		idle := pm.idle(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func(pm *proxyManager) {
			pm.connectToSingleProxy(ctx, proxyName, idle[0])
			wg.Done()
		}(pm)

		for {
			time.Sleep(100 * time.Millisecond)
			if len(sm.listening[proxyName]) > 0 {
				break
			}
		}

		// Blocks until the endpoint is not available.
		nch := make(chan struct{})
		sm.setChan(proxyName, nch)
		sm.setError(proxyName, fmt.Errorf("try again"))
		sm.setEndpoints(proxyName)
		close(ch)
		wg.Wait()

		// Blocks until the context is canceled.
		sm.setEndpoints(proxyName, ep1)
		sm.clearListen(proxyName)
		sm.setChan(proxyName, make(chan struct{}))
		sm.setError(proxyName, nil)
		pm.updateAvailableProxies(ctx)
		cctx, cancel := context.WithCancel(ctx)
		wg.Add(1)
		go func(pm *proxyManager) {
			pm.connectToSingleProxy(cctx, proxyName, idle[0])
			wg.Done()
		}(pm)

		cancel()
		wg.Wait()

		// Blocks until the context is canceled, but this time wait until
		// the connection is listening.
		sm.setEndpoints(proxyName, ep1)
		sm.clearListen(proxyName)
		sm.setChan(proxyName, make(chan struct{}))
		sm.setError(proxyName, nil)
		pm.updateAvailableProxies(ctx)
		cctx, cancel = context.WithCancel(ctx)
		wg.Add(1)
		go func(pm *proxyManager) {
			pm.connectToSingleProxy(cctx, proxyName, idle[0])
			wg.Done()
		}(pm)

		for {
			time.Sleep(100 * time.Millisecond)
			if len(sm.listening[proxyName]) > 0 {
				break
			}
		}
		cancel()
		wg.Wait()
	}
}

func TestMultipleProxyConnections(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	sm := newMockServer()
	ep1 := newEndpoint("5000")
	ep2 := newEndpoint("5001")
	ep3 := newEndpoint("5002")
	eps := []naming.Endpoint{ep1, ep2, ep3}

	fpm := newProxyManager(sm, "proxy0", rpc.UseFirstProxy, 1)
	rpm := newProxyManager(sm, "proxy1", rpc.UseRandomProxy, 1)
	apm := newProxyManager(sm, "proxy2", rpc.UseAllProxies, 2)

	for i, tc := range []struct {
		pm       *proxyManager
		expected int
	}{
		{fpm, 1},
		{rpm, 1},
		{apm, 2},
	} {
		cctx, cancel := context.WithCancel(ctx)
		pm := tc.pm
		// tune down the delays
		pm.resolveDelay = time.Millisecond
		pm.reconnectDelay = time.Millisecond
		proxyName := fmt.Sprintf("proxy%v", i)
		ch := make(chan struct{})
		sm.setChan(proxyName, ch)

		var wg sync.WaitGroup
		wg.Add(1)
		go func(pm *proxyManager) {
			pm.manageProxyConnections(cctx)
			wg.Done()
		}(pm)

		// Let the proxy manager wait before injecting some endpoints.
		time.Sleep(time.Millisecond * 100)
		sm.setEndpoints(proxyName, eps...)

		// Wait for the expected number of connections.
		for {
			time.Sleep(100 * time.Millisecond)
			if len(sm.listening[proxyName]) == tc.expected {
				break
			}
		}

		// Remove the endpoints and finish the current listeners.
		sm.setEndpoints(proxyName)
		sm.clearListen(proxyName)
		sm.setError(proxyName, fmt.Errorf("oops"))
		close(ch)

		// Wait for the expected number of connections to reach 0
		for {
			time.Sleep(10 * time.Millisecond)
			if len(sm.listening[proxyName]) == 0 {
				break
			}
		}

		sm.setError(proxyName, nil)

		ch = make(chan struct{})
		sm.setChan(proxyName, ch)
		sm.clearListen(proxyName)
		sm.setEndpoints(proxyName, ep1, ep2)
		pm.updateAvailableProxies(ctx)

		// Wait for the expected number of connections.
		for {
			time.Sleep(100 * time.Millisecond)
			if len(sm.listening[proxyName]) == tc.expected {
				break
			}
		}

		cancel()
		// Should immediately return if the context is already canceled.
		pm.manageProxyConnections(cctx)

		wg.Wait()
	}

}
