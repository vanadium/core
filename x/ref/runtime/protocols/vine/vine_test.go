// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vine_test

import (
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/lib/discovery/testutil"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/runtime/protocols/vine"
	"v.io/x/ref/test"
)

func TestOutgoingReachable(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ctx, shutdown, err := vine.Init(ctx, "vineserver", security.AllowEveryone(), "client", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	// Create reachable and unreachable server, ensure they have corresponding tags set.
	ctx, cancel := context.WithCancel(ctx)
	rctx := vine.WithLocalTag(ctx, "reachable")
	uctx := vine.WithLocalTag(ctx, "unreachable")
	_, reachServer, err := v23.WithNewServer(rctx, "reachable", &testService{}, security.AllowEveryone())
	if err != nil {
		t.Error(err)
	}
	_, unreachServer, err := v23.WithNewServer(uctx, "unreachable", &testService{}, security.AllowEveryone())
	if err != nil {
		t.Error(err)
	}
	defer func() {
		cancel()
		<-reachServer.Closed()
		<-unreachServer.Closed()
	}()
	// Before we set any connection behaviors, a client should not be able to talk
	// to either of the servers.
	client := v23.GetClient(ctx)
	if err := client.Call(ctx, "reachable", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	if err := client.Call(ctx, "unreachable", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}

	// Now, we set connection behaviors that say that "client" can reach "reachable"
	// but cannot reach unreachable.
	vineClient := vine.VineClient("vineserver")
	if err := vineClient.SetBehaviors(ctx, map[vine.PeerKey]vine.PeerBehavior{
		{"client", "reachable"}:   {Reachable: true},
		{"client", "unreachable"}: {Reachable: false},
	}); err != nil {
		t.Error(err)
	}
	// The call to reachable should succeed since the cached connection still exists.
	if err := client.Call(ctx, "reachable", "Foo", nil, nil); err != nil {
		t.Error(err)
	}
	// the call to unreachable should fail, since the cached connection should be closed
	// and the new attempt to create a connection fails as well.
	if err := client.Call(ctx, "unreachable", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	// Create new clients to avoid using cached connections.
	if ctx, _, err = v23.WithNewClient(ctx); err != nil {
		t.Error(err)
	}
	// Now, a call to reachable should still work even without a cached connection.
	if err := client.Call(ctx, "reachable", "Foo", nil, nil); err != nil {
		t.Error(err)
	}
	// Now, we remove the entries from the configuration and nothing should be reachable.
	if err := vineClient.SetBehaviors(ctx, nil); err != nil {
		t.Error(err)
	}
	if err := client.Call(ctx, "reachable", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	if err := client.Call(ctx, "unreachable", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
}

func TestIncomingReachable(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ctx, shutdown, err := vine.Init(ctx, "vineserver", security.AllowEveryone(), "client", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	denyCtx := vine.WithLocalTag(ctx, "denyClient")
	if denyCtx, _, err = v23.WithNewClient(denyCtx); err != nil {
		t.Fatal(err)
	}

	sctx := vine.WithLocalTag(ctx, "server")
	sctx, cancel := context.WithCancel(sctx)
	_, server, err := v23.WithNewServer(sctx, "server", &testService{}, security.AllowEveryone())
	if err != nil {
		t.Error(err)
	}
	defer func() {
		cancel()
		<-server.Closed()
	}()

	// Before setting a policy all calls should fail.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	if err := v23.GetClient(denyCtx).Call(denyCtx, "server", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}

	// Set a policy that allows "server" to accept connections from "client" but
	// denies all connections from "denyClient".
	vineClient := vine.VineClient("vineserver")
	if err := vineClient.SetBehaviors(ctx, map[vine.PeerKey]vine.PeerBehavior{
		{"client", "server"}:     {Reachable: true},
		{"denyClient", "server"}: {Reachable: false},
	}); err != nil {
		t.Error(err)
	}

	// Now, the call from client to server should work, since the connection is still cached.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Foo", nil, nil); err != nil {
		t.Error(err)
	}
	// but the call from denyclient to server should fail, since the cached connection
	// should be closed and the new call should also fail.
	if err := v23.GetClient(denyCtx).Call(denyCtx, "server", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	// Create new clients to avoid using cached connections.
	if ctx, _, err = v23.WithNewClient(ctx); err != nil {
		t.Error(err)
	}
	// Now, a call with "client" should still work even without a cached connection.
	if err := v23.GetClient(ctx).Call(ctx, "server", "Foo", nil, nil); err != nil {
		t.Error(err)
	}
	// After removing all policies all calls should fail again.
	if err := vineClient.SetBehaviors(ctx, nil); err != nil {
		t.Error(err)
	}
	if err := v23.GetClient(ctx).Call(ctx, "server", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
	if err := v23.GetClient(denyCtx).Call(denyCtx, "server", "Foo", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("wanted call to fail")
	}
}

func TestDiscovery(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	actx, shutdown, err := vine.Init(ctx, "advertiser", security.AllowEveryone(), "advertiser", 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	dctx, shutdown, err := vine.Init(ctx, "scanner", security.AllowEveryone(), "scanner", 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	a, err := v23.NewDiscovery(actx)
	if err != nil {
		t.Fatal(err)
	}
	d, err := v23.NewDiscovery(dctx)
	if err != nil {
		t.Fatal(err)
	}

	ad := &discovery.Advertisement{
		Id:            discovery.AdId{1, 2, 3},
		InterfaceName: "v.io/x",
		Addresses:     []string{"/@6@wsh@v1.com@@/x"},
	}
	astop, err := testutil.Advertise(actx, a, nil, ad)
	if err != nil {
		t.Fatal(err)
	}
	defer astop()
	// Before setting any behaviors scanner should not be able to discover advertiser.
	if err := testutil.ScanAndMatch(dctx, d, ""); err != nil {
		t.Error(err)
	}

	// Set a policy that says scanner can find advertiser.
	vineClient := vine.VineClient("advertiser")
	if err := vineClient.SetBehaviors(ctx, map[vine.PeerKey]vine.PeerBehavior{
		{"advertiser", "scanner"}: {Discoverable: true},
	}); err != nil {
		t.Error(err)
	}
	// we should find ad when scanning.
	if err := testutil.ScanAndMatch(dctx, d, "", *ad); err != nil {
		t.Error(err)
	}

	// Set a policy that says scanner can't find advertiser anymore.
	if err := vineClient.SetBehaviors(ctx, map[vine.PeerKey]vine.PeerBehavior{
		{"advertiser", "scanner"}: {Discoverable: false},
	}); err != nil {
		t.Error(err)
	}
	// nothing should be found when scanning.
	if err := testutil.ScanAndMatch(dctx, d, ""); err != nil {
		t.Error(err)
	}
}

type testService struct{}

func (*testService) Foo(*context.T, rpc.ServerCall) error {
	return nil
}
