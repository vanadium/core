// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	gocontext "context"
	"fmt"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/x/ref/lib/stats"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/debug/debuglib"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type fakeServer struct{}

func (*fakeServer) Foo(ctx *context.T, call rpc.ServerCall) error { return nil }

func TestWithNewServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	if _, s, err := v23.WithNewServer(ctx, "", &fakeServer{}, nil); err != nil || s == nil {
		t.Fatalf("Could not create server: %v", err)
	}
}

func TestPrincipal(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p2 := testutil.NewPrincipal()
	c2, err := v23.WithPrincipal(ctx, p2)
	if err != nil {
		t.Fatalf("Could not attach principal: %v", err)
	}
	if !c2.Initialized() {
		t.Fatal("Got uninitialized context.")
	}
	if p2 != v23.GetPrincipal(c2) {
		t.Fatal("The new principal should be attached to the context, but it isn't")
	}

	// Stats should die with the context.
	c3, cancel := context.WithCancel(ctx)
	c3, err = v23.WithPrincipal(c3, testutil.NewPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	hasStats := func() error {
		prefix := fmt.Sprintf("security/principal/%v", v23.GetPrincipal(c3).PublicKey())
		// A counter is used to generate the variable name, so try a few times.
		for i := 0; i < 100; i++ {
			store := fmt.Sprintf("%v/blessingstore/%d", prefix, i)
			roots := fmt.Sprintf("%v/blessingroots/%d", prefix, i)
			_, e1 := stats.Value(store)
			_, e2 := stats.Value(roots)
			if (e1 == nil) && (e2 == nil) {
				return nil
			}
		}
		return fmt.Errorf("did not find stats for blessing store and roots")
	}
	if err := hasStats(); err != nil {
		t.Error(err)
	}
	cancel()
	// Eventually, the stats should go away.
	for i := 0; i < 10; i++ {
		if err := hasStats(); err != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if err := hasStats(); err == nil {
		t.Errorf("Found blessing store and blessing roots stats even after context was killed, stats are likely holding on to a dead principal")
	}
}

func TestClient(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	orig := v23.GetClient(ctx)

	c2, client, err := v23.WithNewClient(ctx)
	if err != nil || client == nil {
		t.Fatalf("Could not create client: %v", err)
	}
	if !c2.Initialized() {
		t.Fatal("Got uninitialized context.")
	}
	if client == orig {
		t.Fatal("Should have replaced the client but didn't")
	}
	if client != v23.GetClient(c2) {
		t.Fatal("The new client should be attached to the context, but it isn't")
	}
}

func TestNamespace(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	orig := v23.GetNamespace(ctx)
	orig.CacheCtl(naming.DisableCache(true))

	newroots := []string{"/newroot1", "/newroot2"}
	c2, ns, err := v23.WithNewNamespace(ctx, newroots...)
	if err != nil || ns == nil {
		t.Fatalf("Could not create namespace: %v", err)
	}
	if !c2.Initialized() {
		t.Fatal("Got uninitialized context.")
	}
	if ns == orig {
		t.Fatal("Should have replaced the namespace but didn't")
	}
	if ns != v23.GetNamespace(c2) {
		t.Fatal("The new namespace should be attached to the context, but it isn't")
	}
	newrootmap := map[string]bool{"/newroot1": true, "/newroot2": true}
	for _, root := range ns.Roots() {
		if !newrootmap[root] {
			t.Errorf("root %s found in ns, but we expected: %v", root, newroots)
		}
	}
	opts := ns.CacheCtl()
	if len(opts) != 1 {
		t.Fatalf("Expected one option for cache control, got %v", opts)
	}
	if disable, ok := opts[0].(naming.DisableCache); !ok || !bool(disable) {
		t.Errorf("expected a disable(true) message got %#v", opts[0])
	}
}

func TestBackgroundContext(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	bgctx := v23.GetBackgroundContext(ctx)

	if bgctx == ctx {
		t.Error("The background context should not be the same as the context")
	}

	bgctx2 := v23.GetBackgroundContext(bgctx)
	if bgctx != bgctx2 {
		t.Error("Calling GetBackgroundContext a second time should return the same context.")
	}
}

func TestReservedNameDispatcher(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	oldDebugDisp := v23.GetReservedNameDispatcher(ctx)
	newDebugDisp := debuglib.NewDispatcher(nil)

	nctx := v23.WithReservedNameDispatcher(ctx, newDebugDisp)
	debugDisp := v23.GetReservedNameDispatcher(nctx)

	if debugDisp != newDebugDisp || debugDisp == oldDebugDisp {
		t.Error("WithNewDebugDispatcher didn't update the context properly")
	}

}

func TestFlowManager(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	oldman, err := v23.NewFlowManager(ctx, 0)
	if err != nil || oldman == nil {
		t.Errorf("NewFlowManager failed: %v, %v", oldman, err)
	}
	newman, err := v23.NewFlowManager(ctx, 0)
	if err != nil || newman == nil || newman == oldman {
		t.Fatalf("NewFlowManager failed: %v, %v", newman, err)
	}
}

func TestContextInitialization(t *testing.T) {
	_, shutdown := test.V23Init()
	defer shutdown()
	ctx := context.FromGoContext(gocontext.Background())
	_, _, err := v23.WithNewNamespace(ctx, "/dummy")
	if err == nil || !strings.Contains(err.Error(), "context not initialized") {
		t.Fatalf("expected a specific error: %v", err)
	}
	_, _, err = v23.WithNewClient(ctx)
	if err == nil || !strings.Contains(err.Error(), "context not initialized") {
		t.Fatalf("expected a specific error: %v", err)
	}
	_, err = v23.NewDiscovery(ctx)
	if err == nil || !strings.Contains(err.Error(), "context not initialized") {
		t.Fatalf("expected a specific error: %v", err)
	}
}
