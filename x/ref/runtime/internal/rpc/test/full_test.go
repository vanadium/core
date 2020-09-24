// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/nacl/box"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/i18n"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/lib/ibe"
	"v.io/x/lib/netstate"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/bcrypter"
	"v.io/x/ref/runtime/internal/flow/crypto"
	"v.io/x/ref/runtime/protocols/debug"
	"v.io/x/ref/runtime/protocols/lib/tcputil"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func TestAddRemoveName(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	_, s, err := v23.WithNewServer(ctx, "one", &testServer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitForNames(t, ctx, true, "one")
	s.AddName("two")   //nolint:errcheck
	s.AddName("three") //nolint:errcheck
	waitForNames(t, ctx, true, "one", "two", "three")
	s.RemoveName("one")
	waitForNames(t, ctx, false, "one")
	s.RemoveName("two")
	s.RemoveName("three")
	waitForNames(t, ctx, false, "one", "two", "three")
}

func TestCallWithNilContext(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	call, err := v23.GetClient(ctx).StartCall(nil, "foo", "bar", []interface{}{})
	if call != nil {
		t.Errorf("Expected nil interface got: %#v", call)
	}
	if !errors.Is(err, verror.ErrBadArg) {
		t.Errorf("Expected a BadArg error, got: %s", err.Error())
	}
}

func TestRPC(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})
	testRPC(t, ctx, true)
}

func TestRPCWithWebsocket(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"ws", "127.0.0.1:0"}},
	})
	testRPC(t, ctx, true)
}

// TestCloseSendOnFinish tests that Finish informs the server that no more
// inputs will be sent by the client if CloseSend has not already done so.
func TestRPCCloseSendOnFinish(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})
	testRPC(t, ctx, false)
}

func TestRPCCloseSendOnFinishWithWebsocket(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"ws", "127.0.0.1:0"}},
	})
	testRPC(t, ctx, false)
}

func testRPC(t *testing.T, ctx *context.T, shouldCloseSend bool) {
	ctx = i18n.WithLangID(ctx, "foolang")
	type v []interface{}
	type testcase struct {
		name       string
		method     string
		args       v
		streamArgs v
		startErr   error
		results    v
		finishErr  error
	}
	var (
		tests = []testcase{
			{"mountpoint/server/suffix", "Closure", nil, nil, nil, nil, nil},
			{"mountpoint/server/suffix", "Error", nil, nil, nil, nil, errMethod},

			{"mountpoint/server/suffix", "Echo", v{"foo"}, nil, nil, v{`method:"Echo",suffix:"suffix",arg:"foo"`}, nil},
			{"mountpoint/server/suffix/abc", "Echo", v{"bar"}, nil, nil, v{`method:"Echo",suffix:"suffix/abc",arg:"bar"`}, nil},

			{"mountpoint/server/suffix", "EchoUser", v{"foo", userType("bar")}, nil, nil, v{`method:"EchoUser",suffix:"suffix",arg:"foo"`, userType("bar")}, nil},
			{"mountpoint/server/suffix/abc", "EchoUser", v{"baz", userType("bla")}, nil, nil, v{`method:"EchoUser",suffix:"suffix/abc",arg:"baz"`, userType("bla")}, nil},
			{"mountpoint/server/suffix", "Stream", v{"foo"}, v{userType("bar"), userType("baz")}, nil, v{`method:"Stream",suffix:"suffix",arg:"foo" bar baz`}, nil},
			{"mountpoint/server/suffix/abc", "Stream", v{"123"}, v{userType("456"), userType("789")}, nil, v{`method:"Stream",suffix:"suffix/abc",arg:"123" 456 789`}, nil},
			{"mountpoint/server/suffix", "EchoBlessings", nil, nil, nil, v{"[test-blessing:server]", "[test-blessing:client]"}, nil},
			{"mountpoint/server/suffix", "EchoAndError", v{"bugs bunny"}, nil, nil, v{`method:"EchoAndError",suffix:"suffix",arg:"bugs bunny"`}, nil},
			{"mountpoint/server/suffix", "EchoAndError", v{"error"}, nil, nil, nil, errMethod},
			{"mountpoint/server/suffix", "EchoLang", nil, nil, nil, v{"foolang"}, nil},
		}
		name = func(t testcase) string {
			return fmt.Sprintf("%s.%s(%v)", t.name, t.method, t.args)
		}
		cctx = withPrincipal(t, ctx, "client")
		sctx = withPrincipal(t, ctx, "server")
	)
	_, _, err := v23.WithNewDispatchingServer(sctx, "mountpoint/server", &testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	client := v23.GetClient(cctx)
	for _, test := range tests {
		cctx.VI(1).Infof("%s client.StartCall", name(test))
		call, err := client.StartCall(cctx, test.name, test.method, test.args)
		if err != test.startErr {
			t.Errorf(`%s client.StartCall got error "%v", want "%v"`,
				name(test), err, test.startErr)
			continue
		}
		for _, sarg := range test.streamArgs {
			cctx.VI(1).Infof("%s client.Send(%v)", name(test), sarg)
			if err := call.Send(sarg); err != nil {
				t.Errorf(`%s call.Send(%v) got unexpected error "%v"`, name(test), sarg, err)
			}
			var u userType
			if err := call.Recv(&u); err != nil {
				t.Errorf(`%s call.Recv(%v) got unexpected error "%v"`, name(test), sarg, err)
			}
			if !reflect.DeepEqual(u, sarg) {
				t.Errorf("%s call.Recv got value %v, want %v", name(test), u, sarg)
			}
		}
		if shouldCloseSend {
			cctx.VI(1).Infof("%s call.CloseSend", name(test))
			// When the method does not involve streaming
			// arguments, the server gets all the arguments in
			// StartCall and then sends a response without
			// (unnecessarily) waiting for a CloseSend message from
			// the client.  If the server responds before the
			// CloseSend call is made at the client, the CloseSend
			// call will fail.  Thus, only check for errors on
			// CloseSend if there are streaming arguments to begin
			// with (i.e., only if the server is expected to wait
			// for the CloseSend notification).
			if err := call.CloseSend(); err != nil && len(test.streamArgs) > 0 {
				t.Errorf(`%s call.CloseSend got unexpected error "%v"`, name(test), err)
			}
		}
		cctx.VI(1).Infof("%s client.Finish", name(test))
		results := makeResultPtrs(test.results)
		err = call.Finish(results...)
		if got, want := err, test.finishErr; (got == nil) != (want == nil) {
			t.Errorf(`%s call.Finish got error "%v", want "%v'`, name(test), got, want)
		} else if want != nil && verror.ErrorID(got) != verror.ErrorID(want) {
			t.Errorf(`%s call.Finish got error "%v", want "%v"`, name(test), got, want)
		}
		checkResultPtrs(t, name(test), results, test.results)

		// Calling Finish a second time should result in a useful error.
		err = call.Finish(results...)
		if !matchesErrorPattern(err, verror.ErrBadState, "Finish has already been called") {
			t.Fatalf(`got "%v", want "%v"`, err, verror.ErrBadState)
		}
	}
}

func TestStreamReadTerminatedByServer(t *testing.T) {
	// TODO(suharshs): Fix conn/readq to block for outstanding reads to ensure
	// that io.EOF is returned when q.close is called before the context is cancelled.
	t.Skip(`There is a race in flow.close that causes this test to flake.`)
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	cctx := withPrincipal(t, ctx, "client")
	sctx := withPrincipal(t, ctx, "server")

	s := &streamRecvInGoroutineServer{c: make(chan error, 1)}
	_, _, err := v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{s})
	if err != nil {
		t.Fatal(err)
	}

	call, err := v23.GetClient(cctx).StartCall(cctx, "mountpoint/server/suffix", "RecvInGoroutine", []interface{}{})
	if err != nil {
		t.Fatalf("StartCall failed: %v", err)
	}

	c := make(chan error, 1)
	go func() {
		for i := 0; true; i++ {
			if err := call.Send(i); err != nil {
				c <- err
				return
			}
		}
	}()

	// The goroutine at the server executing "Recv" should have terminated
	// with EOF.
	if err := <-s.c; err != io.EOF {
		t.Errorf("Got %v at server, want io.EOF", err)
	}
	// The client Send should have failed since the RPC has been
	// terminated.
	if err := <-c; err == nil {
		t.Errorf("Client Send should fail as the server should have closed the flow")
	}
}

func TestPreferredAddress(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	pa := netstate.AddressChooserFunc(func(string, []net.Addr) ([]net.Addr, error) {
		return []net.Addr{netstate.NewNetAddr("tcp", "1.1.1.1")}, nil
	})
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs{{"tcp", ":0"}},
		AddressChooser: pa,
	})
	_, server, err := v23.WithNewServer(sctx, "", &testServer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	iep := server.Status().Endpoints[0]
	host, _, err := net.SplitHostPort(iep.Address)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if got, want := host, "1.1.1.1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreferredAddressErrors(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	paerr := netstate.AddressChooserFunc(func(_ string, a []net.Addr) ([]net.Addr, error) {
		return nil, fmt.Errorf("oops")
	})
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{
		Addrs:          rpc.ListenAddrs{{"tcp", ":0"}},
		AddressChooser: paerr,
	})
	_, server, err := v23.WithNewServer(sctx, "", &testServer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	status := server.Status()
	if got, want := len(status.Endpoints), 1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got, want := len(status.ListenErrors), 1; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	addr := v23.GetListenSpec(sctx).Addrs[0]
	if got, want := status.ListenErrors[addr].Error(), "oops"; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGranter(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	cctx := withPrincipal(t, ctx, "client")
	sctx := withPrincipal(t, ctx, "server")
	_, _, err := v23.WithNewDispatchingServer(sctx, "mountpoint/server", &testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		granter                       rpc.Granter
		startErrID, finishErrID       verror.IDAction
		blessing, starterr, finisherr string
	}{
		{blessing: ""},
		{granter: granter{}, blessing: "test-blessing:client:blessed"},
		{
			granter:  granter{b: bless(t, cctx, sctx, "blessed")},
			blessing: "test-blessing:client:blessed",
		},
		{
			granter:    granter{err: errors.New("hell no")},
			startErrID: verror.ErrNotTrusted,
			starterr:   "hell no",
		},
		{
			granter:     granter{b: defaultBlessings(cctx)},
			finishErrID: verror.ErrNoAccess,
			finisherr:   "blessing granted not bound to this server",
		},
	}
	for i, test := range tests {
		call, err := v23.GetClient(cctx).StartCall(cctx,
			"mountpoint/server/suffix",
			"EchoGrantedBlessings",
			[]interface{}{"argument"},
			test.granter)
		if !matchesErrorPattern(err, test.startErrID, test.starterr) {
			t.Errorf("%d: %+v: StartCall returned error %v", i, test, err)
		}
		if err != nil {
			continue
		}
		var result, blessing string
		if err = call.Finish(&result, &blessing); !matchesErrorPattern(err, test.finishErrID, test.finisherr) {
			t.Errorf("%+v: Finish returned error %v", test, err)
		}
		if err != nil {
			continue
		}
		if result != "argument" || blessing != test.blessing {
			t.Errorf("%+v: Got (%q, %q)", test, result, blessing)
		}
	}
}

func TestRPCClientAuthorization(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	type v []interface{}
	var (
		cctx                = withPrincipal(t, ctx, "client")
		sctx                = withPrincipal(t, ctx, "server")
		now                 = time.Now()
		serverName          = "mountpoint/server"
		dischargeServerName = "mountpoint/dischargeserver"

		// Caveats on blessings to the client: First-party caveats
		cavOnlyEcho = mkCaveat(security.NewMethodCaveat("Echo"))
		cavExpired  = mkCaveat(security.NewExpiryCaveat(now.Add(-1 * time.Second)))
		// Caveats on blessings to the client: Third-party caveats
		cavTPValid = mkThirdPartyCaveat(
			v23.GetPrincipal(ctx).PublicKey(),
			dischargeServerName,
			mkCaveat(security.NewExpiryCaveat(now.Add(24*time.Hour))))
		cavTPExpired = mkThirdPartyCaveat(v23.GetPrincipal(ctx).PublicKey(),
			dischargeServerName,
			mkCaveat(security.NewExpiryCaveat(now.Add(-1*time.Second))))

		// Client blessings that will be tested.
		bServerClientOnlyEcho  = bless(t, sctx, cctx, "onlyecho", cavOnlyEcho)
		bServerClientExpired   = bless(t, sctx, cctx, "expired", cavExpired)
		bServerClientTPValid   = bless(t, sctx, cctx, "dischargeable_third_party_caveat", cavTPValid)
		bServerClientTPExpired = bless(t, sctx, cctx, "expired_third_party_caveat", cavTPExpired)
		bClient, _             = v23.GetPrincipal(cctx).BlessingStore().Default()
		bRandom, _             = v23.GetPrincipal(cctx).BlessSelf("random")

		tests = []struct {
			blessings  security.Blessings // Blessings used by the client
			name       string             // object name on which the method is invoked
			method     string
			args       v
			results    v
			authorized bool // Whether or not the RPC should be authorized by the server.
		}{
			// There are three different authorization policies
			// (security.Authorizer implementations) used by the server,
			// depending on the suffix (see testServerDisp.Lookup):
			//
			// - nilAuth suffix: the default authorization policy (only
			// delegates of or delegators of the server can call RPCs)
			//
			// - aclAuth suffix: the AccessList only allows blessings
			// matching the patterns "server" or "client"
			//
			// - other suffixes: testServerAuthorizer allows any principal
			// to call any method except "Unauthorized"
			//
			// Expired blessings should fail nilAuth and aclAuth (which care
			// about names), but should succeed on other suffixes (which
			// allow all blessings), unless calling the Unauthorized method.
			{bServerClientExpired, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, false},
			{bServerClientExpired, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, false},
			{bServerClientExpired, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},
			{bServerClientExpired, "mountpoint/server/suffix", "Unauthorized", nil, v{""}, false},

			// Same for blessings that should fail to obtain a discharge for
			// the third party caveat.
			{bServerClientTPExpired, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, false},
			{bServerClientTPExpired, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, false},
			{bServerClientTPExpired, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},
			{bServerClientTPExpired, "mountpoint/server/suffix", "Unauthorized", nil, v{""}, false},

			// The "server:client" blessing (with MethodCaveat("Echo"))
			// should satisfy all authorization policies when "Echo" is
			// called.
			{bServerClientOnlyEcho, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, true},
			{bServerClientOnlyEcho, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, true},
			{bServerClientOnlyEcho, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},

			// The "server:client" blessing (with MethodCaveat("Echo"))
			// should satisfy no authorization policy when any other method
			// is invoked, except for the testServerAuthorizer policy (which
			// will not recognize the blessing "server:onlyecho", but it
			// would authorize anyone anyway).
			{bServerClientOnlyEcho, "mountpoint/server/nilAuth", "Closure", nil, nil, false},
			{bServerClientOnlyEcho, "mountpoint/server/aclAuth", "Closure", nil, nil, false},
			{bServerClientOnlyEcho, "mountpoint/server/suffix", "Closure", nil, nil, true},

			// The "client" blessing doesn't satisfy the default
			// authorization policy, but does satisfy the AccessList and the
			// testServerAuthorizer policy.
			{bClient, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, false},
			{bClient, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, true},
			{bClient, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},
			{bClient, "mountpoint/server/suffix", "Unauthorized", nil, v{""}, false},

			// The "random" blessing does not satisfy either the default
			// policy or the AccessList, but does satisfy
			// testServerAuthorizer.
			{bRandom, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, false},
			{bRandom, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, false},
			{bRandom, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},
			{bRandom, "mountpoint/server/suffix", "Unauthorized", nil, v{""}, false},

			// The "server:dischargeable_third_party_caveat" blessing satisfies all policies.
			// (the discharges should be fetched).
			{bServerClientTPValid, "mountpoint/server/nilAuth", "Echo", v{"foo"}, v{""}, true},
			{bServerClientTPValid, "mountpoint/server/aclAuth", "Echo", v{"foo"}, v{""}, true},
			{bServerClientTPValid, "mountpoint/server/suffix", "Echo", v{"foo"}, v{""}, true},
			{bServerClientTPValid, "mountpoint/server/suffix", "Unauthorized", nil, v{""}, false},
		}
	)
	// Start the discharge server.
	_, _, err := v23.WithNewServer(ctx, dischargeServerName, &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}

	// Start the main server.
	_, _, err = v23.WithNewDispatchingServer(sctx, serverName, testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}

	// The server should recognize the client principal as an authority
	// on "random" blessings.
	err = security.AddToRoots(v23.GetPrincipal(sctx), bRandom)
	if err != nil {
		t.Fatal(err)
	}

	// Set a blessing on the client's blessing store to be presented to
	// the discharge server.
	if _, err := v23.GetPrincipal(cctx).BlessingStore().Set(defaultBlessings(cctx), "test-blessing:$"); err != nil {
		t.Fatal(err)
	}

	// testutil.NewPrincipal sets up a principal that shares blessings
	// with all servers, undo that.
	if _, err := v23.GetPrincipal(cctx).BlessingStore().Set(
		security.Blessings{}, security.AllPrincipals); err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		name := fmt.Sprintf("#%d: %q.%s(%v) by %v", i, test.name, test.method, test.args, test.blessings)
		client := v23.GetClient(cctx)

		_, err = v23.GetPrincipal(cctx).BlessingStore().Set(test.blessings, "test-blessing:server")
		if err != nil {
			t.Fatal(err)
		}
		err = client.Call(cctx, test.name, test.method, test.args, makeResultPtrs(test.results))
		switch {
		case err != nil && test.authorized:
			t.Errorf(`%s client.Call got error: "%v", wanted the RPC to succeed`, name, err)
		case err == nil && !test.authorized:
			t.Errorf("%s call.Finish succeeded, expected authorization failure", name)
		case !test.authorized && !errors.Is(err, verror.ErrNoAccess):
			t.Errorf("%s. call.Finish returned error %v(%v), wanted %v", name, verror.ErrorID(verror.Convert(verror.ErrNoAccess, nil, err)), err, verror.ErrNoAccess)
		}
	}
}

func TestRPCServerAuthorization(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	const (
		publicKeyErr        = "not the authorized public key"
		missingDischargeErr = "missing discharge"
		expiryErr           = "is after expiry"
		allowedErr          = "do not match any allowed server patterns"
	)
	type O []rpc.CallOpt // shorthand
	var (
		sctx    = withPrincipal(t, ctx, "server")
		now     = time.Now()
		noErrID verror.IDAction

		// Third-party caveats on blessings presented by server.
		cavTPValid = mkThirdPartyCaveat(
			v23.GetPrincipal(ctx).PublicKey(),
			"mountpoint/dischargeserver",
			mkCaveat(security.NewExpiryCaveat(now.Add(24*time.Hour))))

		cavTPExpired = mkThirdPartyCaveat(
			v23.GetPrincipal(ctx).PublicKey(),
			"mountpoint/dischargeserver",
			mkCaveat(security.NewExpiryCaveat(now.Add(-1*time.Second))))

		// Server blessings.
		bServer          = bless(t, ctx, sctx, "server")
		bServerExpired   = bless(t, ctx, sctx, "expiredserver", mkCaveat(security.NewExpiryCaveat(time.Now().Add(-1*time.Second))))
		bServerTPValid   = bless(t, ctx, sctx, "serverWithTPCaveats", cavTPValid)
		bServerTPExpired = bless(t, ctx, sctx, "serverWithExpiredTPCaveats", cavTPExpired)
		bOther           = bless(t, ctx, sctx, "other")
		bTwoBlessings, _ = security.UnionOfBlessings(bServer, bOther)

		ACL = func(patterns ...string) security.Authorizer {
			var acl access.AccessList
			for _, p := range patterns {
				acl.In = append(acl.In, security.BlessingPattern(p))
			}
			return acl
		}

		tests = []struct {
			server security.Blessings // blessings presented by the server to the client.
			name   string             // name provided by the client to StartCall
			opts   O                  // options provided to StartCall.
			errID  verror.IDAction
			err    string
		}{
			// Client accepts talking to the server only if the
			// server presents valid blessings (and discharges)
			// consistent with the ones published in the endpoint.
			{bServer, "mountpoint/server", nil, noErrID, ""},
			{bServerTPValid, "mountpoint/server", nil, noErrID, ""},

			// Client will not talk to a server that presents
			// expired blessings or is missing discharges.
			{bServerExpired, "mountpoint/server", nil, verror.ErrNotTrusted, expiryErr},
			{bServerTPExpired, "mountpoint/server", nil, verror.ErrNotTrusted, missingDischargeErr},

			// Test the ServerAuthorizer option.
			{
				bOther,
				"mountpoint/server",
				O{options.ServerAuthorizer{
					Authorizer: security.PublicKeyAuthorizer(bOther.PublicKey()),
				}},
				noErrID,
				"",
			},
			{
				bOther,
				"mountpoint/server",
				O{options.ServerAuthorizer{
					Authorizer: security.PublicKeyAuthorizer(testutil.NewPrincipal("irrelevant").PublicKey())}},
				verror.ErrNotTrusted,
				publicKeyErr,
			},

			// Test the "paranoid" names, where the pattern is provided in the name.
			{
				bServer,
				"__(test-blessing:server)/mountpoint/server",
				nil,
				noErrID,
				"",
			},
			{
				bServer,
				"__(test-blessing:other)/mountpoint/server",
				nil,
				verror.ErrNotTrusted,
				allowedErr,
			},
			{
				bTwoBlessings,
				"__(test-blessing:server)/mountpoint/server",
				O{options.ServerAuthorizer{
					Authorizer: ACL("test-blessing:other"),
				}},
				noErrID,
				"",
			},
		}
	)
	// Start the discharge server.
	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range tests {
		scctx, cancel := context.WithCancel(sctx)
		name := fmt.Sprintf("(#%d: Name:%q, Server:%q, opts:%v)",
			i, test.name, test.server, test.opts)
		if err := vsecurity.SetDefaultBlessings(v23.GetPrincipal(sctx), test.server); err != nil {
			t.Fatal(err)
		}
		_, s, err := v23.WithNewDispatchingServer(scctx, "mountpoint/server", &testServerDisp{&testServer{}})
		if err != nil {
			t.Fatal(err, defaultBlessings(scctx))
		}
		call, err := v23.GetClient(ctx).StartCall(ctx, test.name, "Method", nil, test.opts...)
		if !matchesErrorPattern(err, test.errID, test.err) {
			t.Errorf(`%s: client.StartCall: got error "%v", want to match "%v"`,
				name, err, test.err)
		} else if call != nil {
			blessings, proof := call.RemoteBlessings()
			if proof.IsZero() {
				t.Errorf("%s: Returned zero value for remote blessings", name)
			}
			// Currently all tests are configured so that the only
			// blessings presented by the server that are
			// recognized by the client match the pattern
			// "test-blessing"
			if len(blessings) < 1 || !security.BlessingPattern("test-blessing").MatchedBy(blessings...) {
				t.Errorf("%s: Client sees server as %v, expected a single blessing matching test-blessing", name, blessings)
			}
		}
		cancel()
		<-s.Closed()
	}
}

func TestServerManInTheMiddleAttack(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	// Test scenario: A server mounts itself, but then some other service
	// somehow "takes over" the network endpoint (a naughty router
	// perhaps), thus trying to steal traffic.
	var (
		cctx = withPrincipal(t, ctx, "client")
		actx = withPrincipal(t, ctx, "attacker")
	)
	name := "mountpoint/server"
	_, aserver, err := v23.WithNewDispatchingServer(actx, "", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}

	// The legitimate server would have mounted the same endpoint on the
	// namespace, but with different blessings.
	ep := aserver.Status().Endpoints[0].WithBlessingNames([]string{"test-blessings/server"})
	if err := v23.GetNamespace(actx).Mount(ctx, name, ep.Name(), time.Hour); err != nil {
		t.Fatal(err)
	}

	// The RPC call should fail because the blessings presented by the
	// (attacker's) server are not consistent with the ones registered in
	// the mounttable trusted by the client.
	if _, err := v23.GetClient(cctx).StartCall(cctx, "mountpoint/server", "Closure", nil); !errors.Is(err, verror.ErrNotTrusted) {
		t.Errorf("Got error %v (errorid=%v), want errorid=%v", err, verror.ErrorID(err), verror.ErrNotTrusted.ID)
	}
	// But the RPC should succeed if the client explicitly
	// decided to skip server authorization.
	if err := v23.GetClient(cctx).Call(cctx, "mountpoint/server", "Closure", nil, nil, options.ServerAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
		t.Errorf("Unexpected error(%v) when skipping server authorization", err)
	}
}

func TestDischargeImpetusAndContextPropagation(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	sctx := withPrincipal(t, ctx, "server")
	cctx := withPrincipal(t, ctx, "client")

	// Setup the client so that it shares a blessing with a third-party caveat with the server.
	setClientBlessings := func(req security.ThirdPartyRequirements) {
		cav, err := security.NewPublicKeyCaveat(
			v23.GetPrincipal(ctx).PublicKey(),
			"mountpoint/discharger",
			req,
			security.UnconstrainedUse())
		if err != nil {
			t.Fatalf("Failed to create ThirdPartyCaveat(%+v): %v", req, err)
		}
		b, err := v23.GetPrincipal(cctx).BlessSelf("client_for_server", cav)
		if err != nil {
			t.Fatalf("BlessSelf failed: %v", err)
		}
		if _, err := v23.GetPrincipal(cctx).BlessingStore().Set(b, "test-blessing:server"); err != nil {
			t.Fatal(err)
		}
	}

	// Setup the discharge server.
	var tester dischargeTestServer
	_, _, err := v23.WithNewServer(ctx, "mountpoint/discharger", &tester, &testServerAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	// Setup the application server.
	sctx = v23.WithListenSpec(sctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})
	object := "mountpoint/object"
	_, _, err = v23.WithNewServer(sctx, object, &testServer{}, &testServerAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Requirements security.ThirdPartyRequirements
		Impetus      security.DischargeImpetus
	}{
		{ // No requirements, no impetus
			Requirements: security.ThirdPartyRequirements{},
			Impetus:      security.DischargeImpetus{},
		},
		{ // Require everything
			Requirements: security.ThirdPartyRequirements{ReportServer: true, ReportMethod: true, ReportArguments: true},
			Impetus:      security.DischargeImpetus{Server: []security.BlessingPattern{"test-blessing:server"}, Method: "Method", Arguments: []*vom.RawBytes{vom.RawBytesOf("argument")}},
		},
		{ // Require only the method name
			Requirements: security.ThirdPartyRequirements{ReportMethod: true},
			Impetus:      security.DischargeImpetus{Method: "Method"},
		},
	}

	for _, test := range tests {
		setClientBlessings(test.Requirements)
		tid := vtrace.GetSpan(cctx).Trace()
		// StartCall should fetch the discharge, do not worry about finishing the RPC - do not care about that for this test.
		if _, err := v23.GetClient(cctx).StartCall(cctx, object, "Method", []interface{}{"argument"}); err != nil {
			t.Errorf("StartCall(%+v) failed: %v", test.Requirements, err)
			continue
		}
		impetus, traceid := tester.Release()
		// There should have been exactly 1 attempt to fetch discharges when making
		// the RPC to the remote object.
		if len(impetus) != 1 || len(traceid) != 1 {
			t.Errorf("Test %+v: Got (%d, %d) (#impetus, #traceid), wanted exactly one", test.Requirements, len(impetus), len(traceid))
			continue
		}
		// VC creation does not have any "impetus", it is established without
		// knowledge of the context of the RPC. So ignore that.
		//
		// TODO(ashankar): Should the impetus of the RPC that initiated the
		// VIF/VC creation be propagated?
		if got, want := vdl.ValueOf(impetus[len(impetus)-1]), vdl.ValueOf(test.Impetus); !vdl.EqualValue(got, want) {
			t.Errorf("Test %+v: Got impetus %#v, want %#v", test.Requirements, got, want)
		}
		// But the context used for all of this should be the same
		// (thereby allowing debug traces to link VIF/VC creation with
		// the RPC that initiated them).
		for idx, got := range traceid {
			if !reflect.DeepEqual(got, tid) {
				t.Errorf("Test %+v: %d - Got trace id %q, want %q", test.Requirements, idx, hex.EncodeToString(got[:]), hex.EncodeToString(tid[:]))
			}
		}
	}
}

func TestRPCClientBlessingsPublicKey(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	var (
		sctx    = withPrincipal(t, ctx, "server")
		bserver = defaultBlessings(sctx)
		cctx    = withPrincipal(t, ctx, "client")
		bclient = defaultBlessings(cctx)
	)
	cctx, err := v23.WithPrincipal(cctx,
		vsecurity.MustForkPrincipal(
			v23.GetPrincipal(cctx),
			&singleBlessingStore{pk: v23.GetPrincipal(cctx).PublicKey()},
			vsecurity.ImmutableBlessingRoots(v23.GetPrincipal(ctx).Roots())))
	if err != nil {
		t.Fatal(err)
	}

	bvictim := defaultBlessings(withPrincipal(t, ctx, "victim"))

	_, s, err := v23.WithNewDispatchingServer(sctx, "", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	object := naming.Join(s.Status().Endpoints[0].Name(), "suffix")

	tests := []struct {
		blessings security.Blessings
		errID     verror.IDAction
		err       bool
	}{
		{blessings: bclient},
		// server disallows clients from authenticating with blessings not bound to
		// the client principal's public key
		{blessings: bvictim, errID: verror.ErrNoAccess, err: true},
		{blessings: bserver, errID: verror.ErrNoAccess, err: true},
	}
	for i, test := range tests {
		name := fmt.Sprintf("%d: Client RPCing with blessings %v", i, test.blessings)
		if _, err := v23.GetPrincipal(cctx).BlessingStore().Set(test.blessings, "test-blessings"); err != nil {
			t.Fatal(err)
		}
		if err := v23.GetClient(cctx).Call(cctx, object, "Closure", nil, nil); test.err && err == nil {
			t.Errorf("%v: client.Call returned error %v", name, err)
			continue
		}
	}
}

func TestServerLocalBlessings(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	tpCav := mkThirdPartyCaveat(
		v23.GetPrincipal(ctx).PublicKey(),
		"mountpoint/dischargeserver",
		mkCaveat(security.NewExpiryCaveat(time.Now().Add(time.Hour))))
	sctx := withPrincipal(t, ctx, "server", tpCav)
	cctx := withPrincipal(t, ctx, "client")

	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}

	var gotServer, gotClient string
	if err := v23.GetClient(cctx).Call(cctx, "mountpoint/server/suffix", "EchoBlessings", nil, []interface{}{&gotServer, &gotClient}); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if wantServer, wantClient := "[test-blessing:server]", "[test-blessing:client]"; gotServer != wantServer || gotClient != wantClient {
		t.Fatalf("EchoBlessings: got %v, %v want %v, %v", gotServer, gotClient, wantServer, wantClient)
	}
}

func TestDischargesToMounttable(t *testing.T) {
	// This test duplicates the underlying problem in
	// https://github.com/vanadium/issues/issues/1049
	//
	// The mounttable is used once to resolve the name of the discharger
	// and then again to mount. The first time discharges aren't sent (as
	// they aren't available). The second time they must be sent.
	ctx, shutdown := test.V23InitWithMounttable()
	name := "mountpoint/name"
	defer shutdown()

	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	// Set an ACL on the mounttable so that only test-blessing:client authorizes a mount.
	perms := make(access.Permissions)
	perms.Add("test-blessing:client", string(access.Write))
	if err := v23.GetNamespace(ctx).SetPermissions(ctx, name, perms, ""); err != nil {
		t.Fatal(err)
	}

	tpCav := mkThirdPartyCaveat(
		v23.GetPrincipal(ctx).PublicKey(),
		"mountpoint/dischargeserver",
		mkCaveat(security.NewExpiryCaveat(time.Now().Add(time.Hour))))
	cctx := withPrincipal(t, ctx, "client", tpCav)
	if err := v23.GetNamespace(cctx).Mount(cctx, name, "localhost:14141", time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestDischargePurgeFromCache(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	// Client is blessed with a third-party caveat. The discharger
	// service issues discharges with a fakeTimeCaveat.  This blessing
	// is presented to "server".
	cctx := withPrincipal(t, ctx, "client",
		mkThirdPartyCaveat(
			v23.GetPrincipal(ctx).PublicKey(),
			"mountpoint/dischargeserver",
			security.UnconstrainedUse()))

	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}

	call := func() error {
		var got string
		if err := v23.GetClient(cctx).Call(cctx, "mountpoint/server/aclAuth", "Echo", []interface{}{"batman"}, []interface{}{&got}); err != nil {
			return err
		}
		if want := `method:"Echo",suffix:"aclAuth",arg:"batman"`; got != want {
			return verror.Convert(verror.ErrBadArg, nil, fmt.Errorf("Got [%v] want [%v]", got, want))
		}
		return nil
	}

	// First call should succeed
	if err := call(); err != nil {
		t.Fatal(err)
	}
	// Advance virtual clock, which will invalidate the discharge
	clock.Advance(1)
	if err, want := call(), "not authorized"; !matchesErrorPattern(err, verror.ErrNoAccess, want) {
		t.Errorf("Got error [%v] wanted to match pattern %q", err, want)
	}
	// But retrying will succeed since the discharge should be purged
	// from cache and refreshed
	if err := call(); err != nil {
		t.Fatal(err)
	}
}

func TestReplayAttack(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	cctx := withPrincipal(t, ctx, "client")

	_, s, err := v23.WithNewDispatchingServer(sctx, "", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	ep := s.Status().Endpoints[0]
	addr := ep.Addr()

	// Dial the server.
	tcp := tcputil.TCP{}
	conn, err := tcp.Dial(cctx, addr.Network(), addr.String(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Read the setup message from the server.
	b, err := conn.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}
	m, err := message.Read(cctx, b)
	if err != nil {
		t.Fatal(err)
	}
	rSetup, ok := m.(*message.Setup)
	if !ok {
		t.Fatalf("got %#v, want *message.Setup", m)
	}
	rpk := rSetup.PeerNaClPublicKey

	// Send our setup message to the server.
	pk, sk, err := box.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	lSetup := &message.Setup{
		Versions:          rSetup.Versions,
		PeerLocalEndpoint: rSetup.PeerLocalEndpoint,
		PeerNaClPublicKey: pk,
	}
	b, err = message.Append(ctx, lSetup, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = conn.WriteMsg(b); err != nil {
		t.Fatal(err)
	}

	// Setup encryption to the server.
	cipher := crypto.NewControlCipherRPC11(
		(*crypto.BoxKey)(pk),
		(*crypto.BoxKey)(sk),
		(*crypto.BoxKey)(rpk))

	// Read the auth message from the server.
	for auth := false; !auth; {
		b, err = conn.ReadMsg()
		if err != nil {
			t.Fatal(err)
		}
		if !cipher.Open(b) {
			t.Fatal("failed to decrypt message")
		}
		m, err = message.Read(ctx, b[:len(b)-cipher.MACSize()])
		if err != nil {
			t.Fatal(err)
		}
		switch m.(type) {
		case *message.Auth:
			auth = true
		case *message.Data:
		default:
			continue
		}
		if b, err = message.Append(ctx, m, nil); err != nil {
			t.Fatal(err)
		}
		tmp := make([]byte, len(b)+cipher.MACSize())
		copy(tmp, b)
		b = tmp
		if err = cipher.Seal(b); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.WriteMsg(b); err != nil {
			t.Fatal(err)
		}

	}

	// The server should send a tearDown message complaining about the channel binding.
	b, err = conn.ReadMsg()
	if err != nil {
		t.Fatal(err)
	}
	if !cipher.Open(b) {
		t.Fatal("failed to decrypt message")
	}
	m, err = message.Read(ctx, b[:len(b)-cipher.MACSize()])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok = m.(*message.TearDown); !ok {
		t.Fatalf("got %#v, want *message.TearDown", m)
	}
}

func TestPrivateServer(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// The following is so that we have a known default blessing name
	// for the root context.
	ctx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("root"))
	if err != nil {
		t.Fatal(err)
	}
	master, err := ibe.SetupBB1()
	if err != nil {
		t.Fatal(err)
	}
	root := bcrypter.NewRoot("root", master)

	// Derive the client and server contexts
	cctx := withPrincipal(t, ctx, "client")
	cctx = bcrypter.WithCrypter(cctx, bcrypter.NewCrypter())

	sctx := withPrincipal(t, ctx, "server")
	sctx = bcrypter.WithCrypter(sctx, bcrypter.NewCrypter())

	// Add the root params to the server's crypter so that the
	// server can use IBE to keep its blessings private.
	if err := bcrypter.GetCrypter(sctx).AddParams(sctx, root.Params()); err != nil {
		t.Fatal(err)
	}

	// Test that creating a server with an empty set of server peers
	// fails
	_, _, err = v23.WithNewDispatchingServer(sctx, "", &testServerDisp{&testServer{}}, options.ServerPeers([]security.BlessingPattern{}))
	if err == nil {
		t.Fatal("WithNewDispatchingServer unexpectedly passed with empty set of server peers")
	}

	// Test that creating a server with a non-empty set of server peers and
	// a non-empty name fails.
	_, _, err = v23.WithNewDispatchingServer(sctx, "foo", &testServerDisp{&testServer{}}, options.ServerPeers([]security.BlessingPattern{}))
	if err == nil {
		t.Fatal("WithNewDispatchingServer unexpectedly passed with a non-empty set of server peers and a non-empty name")
	}

	// Create a server that only reveals its blesings to peers with blessings
	// matching the pattern "root:client".
	//
	// Since a mounttable server always learns a server's blessings, this also
	// restricts the set of mounttable servers that the server can talk to. In
	// particular the mounttable server must have a blessing matching the pattern
	// "root:client".
	//
	// Having said that, the common case for private mutual authentication would
	// be when there are no mounttables involved and the server's endpoint is
	// discovered using a peer-to-peer discovery mechanism (e.g., mdns).
	_, server, err := v23.WithNewDispatchingServer(sctx, "", &testServerDisp{&testServer{}}, options.ServerPeers([]security.BlessingPattern{"root:client"}))
	if err != nil {
		t.Fatal(err)
	}
	serverEPName := server.Status().Endpoints[0].Name()
	client := v23.GetClient(cctx)

	// Test that an RPC to the server fails as the client has no blessing private keys.
	_, err = client.StartCall(cctx, serverEPName, "Closure", nil)
	if err == nil {
		t.Error("RPC by client with no blessing private key unexpectedly succeeded")
	}

	// Add a private key for a blessing that does not match the server's policy
	// (pattern: "root:client") and test that the RPC still fails.
	if err := bcrypter.GetCrypter(cctx).AddKey(cctx, extractKey(t, ctx, root, "root:otherclient")); err != nil {
		t.Fatal(err)
	}
	_, err = client.StartCall(cctx, serverEPName, "Closure", nil)
	if err == nil {
		t.Error("RPC by client with no matching blessing private key unexpectedly succeeded")
	}

	// Add a private key for a blessing matching the server's policy and test that
	// the RPC succceeds and that the client learns the server's blessing ("root:server").
	if err := bcrypter.GetCrypter(cctx).AddKey(cctx, extractKey(t, ctx, root, "root:client")); err != nil {
		t.Fatal(err)
	}
	call, err := client.StartCall(cctx, serverEPName, "Closure", nil, options.ServerAuthorizer{Authorizer: access.AccessList{In: []security.BlessingPattern{"root:server:$"}}})
	if err != nil {
		t.Error(verror.DebugString(err))
	} else {
		results := makeResultPtrs(nil)
		if err := call.Finish(results...); err != nil {
			t.Errorf("RPC by client with a matching blessing private key failed: %v", err)
		}
	}
}

type publicKeyAuth struct {
	pkey security.PublicKey
}

func (a *publicKeyAuth) Authorize(ctx *context.T, call security.Call) error {
	pkey := call.RemoteBlessings().PublicKey()
	if !reflect.DeepEqual(a.pkey, pkey) {
		return fmt.Errorf("public key mismatch: %v != %v", a.pkey, pkey)
	}
	return nil
}

func TestNamelessClientBlessings(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p, err := vsecurity.NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	clientCtx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	clt := v23.GetClient(clientCtx)

	_, server, err := v23.WithNewServer(ctx, "", &testServer{}, &publicKeyAuth{p.PublicKey()})
	if err != nil {
		t.Fatal(err)
	}
	name := server.Status().Endpoints[0].Name()

	if err := clt.Call(clientCtx, name, "Closure", nil, nil, options.ServerAuthorizer{Authorizer: security.AllowEveryone()}); err != nil {
		t.Fatal(err)
	}
}

func TestNamelessServerBlessings(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	client := v23.GetClient(ctx)

	p, err := vsecurity.NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	sctx, err := v23.WithPrincipal(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	_, server, err := v23.WithNewDispatchingServer(sctx, "", &testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	name := server.Status().Endpoints[0].Name()

	if err := client.Call(ctx, name, "Closure", nil, nil, options.ServerAuthorizer{Authorizer: &publicKeyAuth{p.PublicKey()}}); err != nil {
		t.Fatal(err)
	}
}

func TestServerRefreshDischarges(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server", mkThirdPartyCaveat(
		v23.GetPrincipal(ctx).PublicKey(),
		"mountpoint/dischargeserver",
		security.UnconstrainedUse()))
	cctx := withPrincipal(t, ctx, "client")

	ed := &expiryDischarger{}
	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", ed, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	sctx, cancel := context.WithCancel(sctx)
	_, server, err := v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { <-server.Closed() }()
	defer cancel()

	// Make a call to create a connection. We don't care if the call succeeds,
	// we just want to make sure that we fetch discharges more than once.
	var got string
	cctx, cancel = context.WithCancel(cctx)
	defer cancel()
	//nolint:errcheck
	v23.GetClient(cctx).Call(cctx, "mountpoint/server/aclAuth", "Echo", []interface{}{"batman"}, []interface{}{&got}, options.NoRetry{})
	for {
		ed.mu.Lock()
		count := ed.count
		ed.mu.Unlock()
		if count > 1 {
			break
		}
		t.Logf("waiting for discharger to be called multiple times")
		time.Sleep(50 * time.Millisecond)
	}
}

func TestClientRefreshDischarges(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	cctx := withPrincipal(t, ctx, "client", mkThirdPartyCaveat(
		v23.GetPrincipal(ctx).PublicKey(),
		"mountpoint/dischargeserver",
		security.UnconstrainedUse()))

	d := &dischargeServer{}
	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", d, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	sctx, cancel := context.WithCancel(sctx)
	_, server, err := v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { <-server.Closed() }()
	defer cancel()

	// Make a call to create a server connection. We don't care if the call succeeds,
	// we just want to make sure that we fetch discharges only once when we are a client.
	var got string
	cctx, cancel = context.WithCancel(cctx)
	defer cancel()
	if err := v23.GetClient(cctx).Call(cctx, "mountpoint/server/aclAuth", "Echo", []interface{}{"batman"}, []interface{}{&got}, options.NoRetry{}); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	if d.count != 1 {
		t.Errorf("discharger should have been called exactly once, got %v", d.count)
	}
	d.mu.Unlock()
}

func TestBidirectionalRefreshDischarges(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	sctx := withPrincipal(t, ctx, "server")
	cctx := withPrincipal(t, ctx, "client", mkThirdPartyCaveat(
		v23.GetPrincipal(ctx).PublicKey(),
		"mountpoint/dischargeserver",
		security.UnconstrainedUse()))

	ed := &expiryDischarger{}
	_, _, err := v23.WithNewServer(ctx, "mountpoint/dischargeserver", ed, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	sctx, cancel := context.WithCancel(sctx)
	defer cancel()
	_, _, err = v23.WithNewDispatchingServer(sctx, "mountpoint/server", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	cctx, cancel = context.WithCancel(cctx)
	cctx, server, err := v23.WithNewDispatchingServer(cctx, "mountpoint/clientserver", testServerDisp{&testServer{}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { <-server.Closed() }()
	defer cancel()

	// Make a call to create a connection. We don't care if the call succeeds,
	// we just want to make sure that we fetch discharges more than once.	var got string
	var got string
	if err := v23.GetClient(cctx).Call(cctx, "mountpoint/server/aclAuth", "Echo", []interface{}{"batman"}, []interface{}{&got}, options.NoRetry{}); err != nil {
		t.Fatal(err)
	}
	for {
		ed.mu.Lock()
		count := ed.count
		ed.mu.Unlock()
		if count > 1 {
			break
		}
		t.Logf("waiting for discharger to be called multiple times")
		time.Sleep(50 * time.Millisecond)
	}
}

type manInMiddleConn struct {
	flow.Conn
	ctx *context.T
}

// manInMiddleConn changes the versions in any setup message sent over the wire.
func (c *manInMiddleConn) ReadMsg() ([]byte, error) {
	b, err := c.Conn.ReadMsg()
	if len(b) > 0 {
		m, _ := message.Read(c.ctx, b)
		if msg, ok := m.(*message.Setup); ok {
			// The malicious man in the middle changes the max version to a bad version.
			msg.Versions = version.RPCVersionRange{Min: version.RPCVersion10, Max: 100}
			b, err = message.Append(c.ctx, msg, nil)
		}
	}
	return b, err
}

func TestSetupAttack(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	ctx = debug.WithFilter(ctx, func(c flow.Conn) flow.Conn {
		return &manInMiddleConn{c, ctx}
	})

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{Protocol: "debug", Address: "tcp/127.0.0.1:0"}},
	})
	ctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(ctx, "mountpoint/server", &testServer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { <-server.Closed() }()
	defer cancel()
	// Connection establishment should fail during the RPC because the channel binding
	// check should fail since the Setup message has been altered.
	if err := v23.GetClient(ctx).Call(ctx, "mountpoint/server", "Closure", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("expected error but got <nil>")
	}
}

func defaultBlessings(ctx *context.T) security.Blessings {
	b, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	return b
}

type closeServer struct {
	ch chan struct{}
}

func (s *closeServer) Close(*context.T, rpc.ServerCall) ([20000]byte, error) {
	defer close(s.ch)
	return [20000]byte{}, nil
}

func TestImmediateClose(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	ch := make(chan struct{})
	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewServer(sctx, "", &closeServer{ch}, nil)
	if err != nil {
		t.Fatal(err)
	}
	name := server.Status().Endpoints[0].Name()
	done := make(chan struct{})
	go func() {
		var b [20000]byte
		if err := v23.GetClient(ctx).Call(ctx, name, "Close", nil, []interface{}{&b}); err != nil {
			t.Error(err)
		}
		close(done)
	}()

	// Once the server responds to a request close the server.
	<-ch
	cancel()
	<-server.Closed()

	// Wait for the client to finish up.
	<-done
}
