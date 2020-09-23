// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

type testService struct{}

func (testService) EchoBlessings(ctx *context.T, call rpc.ServerCall) ([]string, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	return b, nil
}

func (testService) Foo(*context.T, rpc.ServerCall) error {
	return nil
}

func newCtxPrincipal(rootCtx *context.T) *context.T {
	ctx, err := v23.WithPrincipal(rootCtx, testutil.NewPrincipal("defaultBlessings"))
	if err != nil {
		panic(err)
	}
	return ctx
}

func union(blessings ...security.Blessings) security.Blessings {
	var ret security.Blessings
	var err error
	for _, b := range blessings {
		if ret, err = security.UnionOfBlessings(ret, b); err != nil {
			panic(err)
		}
	}
	return ret
}

func mkCaveat(cav security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return cav
}

func mkBlessings(blessings security.Blessings, err error) security.Blessings {
	if err != nil {
		panic(err)
	}
	return blessings
}

func mkThirdPartyCaveat(discharger security.PublicKey, location string, caveats ...security.Caveat) security.Caveat {
	if len(caveats) == 0 {
		caveats = []security.Caveat{security.UnconstrainedUse()}
	}
	tpc, err := security.NewPublicKeyCaveat(discharger, location, security.ThirdPartyRequirements{}, caveats[0], caveats[1:]...)
	if err != nil {
		panic(err)
	}
	return tpc
}

func TestClientServerBlessings(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var (
		rootAlpha, rootBeta  = testutil.NewIDProvider("alpha"), testutil.NewIDProvider("beta")
		clientCtx, serverCtx = newCtxPrincipal(ctx), newCtxPrincipal(ctx)
		pclient              = v23.GetPrincipal(clientCtx)
		pserver              = v23.GetPrincipal(serverCtx)

		// A bunch of blessings
		alphaClient = mkBlessings(rootAlpha.NewBlessings(pclient, "client"))
		betaClient  = mkBlessings(rootBeta.NewBlessings(pclient, "client"))

		alphaServer = mkBlessings(rootAlpha.NewBlessings(pserver, "server"))
		betaServer  = mkBlessings(rootBeta.NewBlessings(pserver, "server"))
	)
	// Setup the client's blessing store
	if _, err := pclient.BlessingStore().Set(alphaClient, "alpha:server"); err != nil {
		t.Fatal(err)
	}
	if _, err := pclient.BlessingStore().Set(betaClient, "beta"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		server security.Blessings // Blessings presented by the server.

		// Expected output
		wantServer []string // Client's view of the server's blessings
		wantClient []string // Server's view of the client's blessings
	}{
		{
			server:     alphaServer,
			wantServer: []string{"alpha:server"},
			wantClient: []string{"alpha:client"},
		},
		{
			server:     union(alphaServer, betaServer),
			wantServer: []string{"alpha:server", "beta:server"},
			wantClient: []string{"alpha:client", "beta:client"},
		},
	}

	// Have the client and server both trust both the root principals.
	for _, ctx := range []*context.T{clientCtx, serverCtx} {
		for _, b := range []security.Blessings{alphaClient, betaClient} {
			p := v23.GetPrincipal(ctx)
			if err := security.AddToRoots(p, b); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Let it rip!
	for _, test := range tests {
		if err := pserver.BlessingStore().SetDefault(test.server); err != nil {
			t.Errorf("pserver.SetDefault(%v) failed: %v", test.server, err)
			continue
		}
		sCtx, cancel := context.WithCancel(serverCtx)
		_, server, err := v23.WithNewServer(sCtx, "", testService{}, security.AllowEveryone())
		if err != nil {
			t.Fatal(err)
		}
		serverObjectName := server.Status().Endpoints[0].Name()
		ctx, client, err := v23.WithNewClient(clientCtx)
		if err != nil {
			panic(err)
		}

		var gotClient []string
		if call, err := client.StartCall(ctx, serverObjectName, "EchoBlessings", nil); err != nil {
			t.Errorf("client.StartCall failed: %v", err)
		} else if err = call.Finish(&gotClient); err != nil {
			t.Errorf("call.Finish failed: %v", err)
		} else if !reflect.DeepEqual(gotClient, test.wantClient) {
			t.Errorf("%v: Got %v, want %v for client blessings", test.server, gotClient, test.wantServer)
		} else if gotServer, _ := call.RemoteBlessings(); !reflect.DeepEqual(gotServer, test.wantServer) {
			t.Errorf("%v: Got %v, want %v for server blessings", test.server, gotServer, test.wantClient)
		}

		cancel()
		<-server.Closed()
		client.Close()
	}
}

func TestServerEndpointBlessingNames(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ctx, _ = v23.WithPrincipal(ctx, testutil.NewPrincipal())
	var (
		p         = v23.GetPrincipal(ctx)
		blessings = union(
			mkBlessings(p.BlessSelf("dev.v.io:users:foo@bar.com:devices:phone:applications:app")),
			mkBlessings(p.BlessSelf("otherblessing")))
		want = []string{
			"dev.v.io:users:foo@bar.com:devices:phone:applications:app",
			"otherblessing",
		}
	)
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(p, blessings); err != nil {
		t.Fatal(err)
	}
	_, server, err := v23.WithNewServer(ctx, "", testService{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := server.Status()
	if len(status.Endpoints) == 0 {
		t.Fatalf("No point in this test if there are no endpoints! Status: %+v", server.Status())
	}
	for idx, ep := range status.Endpoints {
		got := ep.BlessingNames()
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Got %v want %v for endpoint #%d/%d: %v", got, want, idx, len(status.Endpoints), ep)
		}
	}
}

type dischargeService struct {
	called int
	mu     sync.Mutex
}

func (ds *dischargeService) Discharge(ctx *context.T, call rpc.StreamServerCall, cav security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	tp := cav.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, fmt.Errorf("discharger: not a third party caveat (%v)", cav)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", tp, err)
	}
	// If its the first time being called, add an expiry caveat and a MethodCaveat for "EchoBlessings".
	// Otherwise, just add a MethodCaveat for "Foo".
	ds.mu.Lock()
	called := ds.called
	ds.mu.Unlock()
	caveat := security.UnconstrainedUse()
	if called == 0 {
		caveat = mkCaveat(security.NewExpiryCaveat(time.Now().Add(-1 * time.Second)))
	}

	return call.Security().LocalPrincipal().MintDischarge(cav, caveat)
}

func TestServerDischarges(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var (
		dischargerCtx, clientCtx, serverCtx = newCtxPrincipal(ctx), newCtxPrincipal(ctx), newCtxPrincipal(ctx)
		pdischarger                         = v23.GetPrincipal(dischargerCtx)
		pclient                             = v23.GetPrincipal(clientCtx)
		pserver                             = v23.GetPrincipal(serverCtx)
		root                                = testutil.NewIDProvider("root")
	)

	// Setup the server's and discharger's blessing store and blessing roots, and
	// start the server and discharger.
	if err := root.Bless(pdischarger, "discharger"); err != nil {
		t.Fatal(err)
	}
	ds := &dischargeService{}
	_, server, err := v23.WithNewServer(dischargerCtx, "", ds, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	dischargeServerName := server.Status().Endpoints[0].Name()

	if err := root.Bless(pserver, "server", mkThirdPartyCaveat(pdischarger.PublicKey(), dischargeServerName)); err != nil {
		t.Fatal(err)
	}
	serverCtx, server, err = v23.WithNewServer(serverCtx, "", testService{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	serverName := server.Status().Endpoints[0].Name()

	// Setup up the client's blessing store so that it can talk to the server.
	rootClient := mkBlessings(root.NewBlessings(pclient, "client"))
	if _, err := pclient.BlessingStore().Set(security.Blessings{}, security.AllPrincipals); err != nil {
		t.Fatal(err)
	}
	if _, err := pclient.BlessingStore().Set(rootClient, "root:server"); err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(pclient, rootClient); err != nil {
		t.Fatal(err)
	}

	// Test that the client and server can communicate with the expected set of blessings
	// when server provides appropriate discharges.
	wantClient := []string{"root:client"}
	wantServer := []string{"root:server"}
	var gotClient []string
	// This opt ensures that if the Blessings do not match the pattern, StartCall will fail.
	allowedServers := options.ServerAuthorizer{Authorizer: access.AccessList{In: []security.BlessingPattern{"root:server"}}}

	// Create a new client.
	clientCtx, client, err := v23.WithNewClient(clientCtx)
	if err != nil {
		t.Fatal(err)
	}
	makeCall := func() error {
		if call, err := client.StartCall(clientCtx, serverName, "EchoBlessings", nil, allowedServers); err != nil {
			return err
		} else if err = call.Finish(&gotClient); err != nil {
			return fmt.Errorf("call.Finish failed: %v", err)
		} else if !reflect.DeepEqual(gotClient, wantClient) {
			return fmt.Errorf("Got %v, want %v for client blessings", gotClient, wantClient)
		} else if gotServer, _ := call.RemoteBlessings(); !reflect.DeepEqual(gotServer, wantServer) {
			return fmt.Errorf("Got %v, want %v for server blessings", gotServer, wantServer)
		}
		return nil
	}

	if err := makeCall(); !errors.Is(err, verror.ErrNotTrusted) {
		t.Fatalf("got error %v, expected %v", err, verror.ErrNotTrusted.ID)
	}
	ds.mu.Lock()
	ds.called++
	ds.mu.Unlock()
	// makeCall should eventually succeed because a valid discharge should be refreshed.
	start := time.Now()
	for err := makeCall(); err != nil; time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 10*time.Second {
			t.Fatalf("Discharge not refreshed in 10 seconds")
		}
		err = makeCall()
	}

	// Test that the client fails to talk to server that does not present appropriate discharges.
	// Setup a new client so that there are no cached VCs.
	clientCtx, client, err = v23.WithNewClient(clientCtx)
	if err != nil {
		t.Fatal(err)
	}

	rootServerInvalidTPCaveat := mkBlessings(root.NewBlessings(pserver, "server", mkThirdPartyCaveat(pdischarger.PublicKey(), dischargeServerName, mkCaveat(security.NewExpiryCaveat(time.Now().Add(-1*time.Second))))))
	if err := pserver.BlessingStore().SetDefault(rootServerInvalidTPCaveat); err != nil {
		t.Fatal(err)
	}
	_, server, err = v23.WithNewServer(serverCtx, "", testService{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	serverName = server.Status().Endpoints[0].Name()

	call, err := client.StartCall(clientCtx, serverName, "EchoBlessings", nil)
	if err == nil {
		remote, _ := call.RemoteBlessings()
		t.Errorf("client.StartCall passed unexpectedly with remote end authenticated as: %v", remote)
		call.Finish() //nolint:errcheck
	}
}
