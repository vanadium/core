// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/naming"
)

func TestDefaultAuthorizerECDSA(t *testing.T) {
	testDefaultAuthorizer(t,
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
	)
}
func TestDefaultAuthorizerED25519(t *testing.T) {
	testDefaultAuthorizer(t,
		newED25519Principal(t),
		newED25519Principal(t),
		newED25519Principal(t),
		newED25519Principal(t),
	)
}

func TestDefaultAuthorizer(t *testing.T) {
	testDefaultAuthorizer(t,
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
		newED25519Principal(t),
	)
	testDefaultAuthorizer(t,
		newED25519Principal(t),
		newED25519Principal(t),
		newED25519Principal(t),
		newECDSAPrincipal(t),
	)
}

func testDefaultAuthorizer(t *testing.T, pali, pbob, pche, pdis Principal) { // pdis is the third-party caveat discharger.
	var (
		che, _ = pche.BlessSelf("che")
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		tpcav  = mkThirdPartyCaveat(t, pdis.PublicKey(), "someLocation", UnconstrainedUse())
		dis, _ = pdis.MintDischarge(tpcav, UnconstrainedUse())
		dismap = map[string]Discharge{dis.ID(): dis}

		// bless(ali, bob, "friend") will generate a blessing for ali, calling him "bob:friend".
		bless = func(target, extend Blessings, extension string, caveats ...Caveat) Blessings {
			var p Principal
			switch extend.PublicKey() {
			case ali.PublicKey():
				p = pali
			case bob.PublicKey():
				p = pbob
			case che.PublicKey():
				p = pche
			default:
				panic(extend)
			}
			if len(caveats) == 0 {
				caveats = []Caveat{UnconstrainedUse()}
			}
			ret, err := p.Bless(target.PublicKey(), extend, extension, caveats[0], caveats[1:]...)
			if err != nil {
				panic(err)
			}
			return ret
		}

		U = func(blessings ...Blessings) Blessings {
			u, err := UnionOfBlessings(blessings...)
			if err != nil {
				panic(err)
			}
			return u
		}

		// Shorthands for getting blessings for Ali and Bob.
		A = func(as Blessings, extension string, caveats ...Caveat) Blessings {
			return bless(ali, as, extension, caveats...)
		}
		B = func(as Blessings, extension string, caveats ...Caveat) Blessings {
			return bless(bob, as, extension, caveats...)
		}

		authorizer defaultAuthorizer
	)
	// Make ali, bob (the two ends) recognize all three blessings
	for ip, p := range []Principal{pali, pbob} {
		for _, b := range []Blessings{ali, bob, che} {
			if err := AddToRoots(p, b); err != nil {
				t.Fatalf("%d: %v - %v", ip, b, err)
			}
		}
	}
	// All tests are run as if "ali" is the local end and "bob" is the remote.
	tests := []struct {
		local, remote Blessings
		call          CallParams
		authorized    bool
	}{
		{
			local:      ali,
			remote:     ali,
			authorized: true,
		},
		{
			local:      ali,
			remote:     bob,
			authorized: false,
		},
		{
			// ali talking to ali:friend (invalid caveat)
			local:      ali,
			remote:     B(ali, "friend", tpcav),
			authorized: false,
		},
		{
			// ali talking to ali:friend
			local:      ali,
			remote:     B(ali, "friend", tpcav),
			call:       CallParams{RemoteDischarges: dismap},
			authorized: true,
		},
		{
			// bob:friend talking to bob (local blessing has an invalid caveat, but it is not checked)
			local:      A(bob, "friend", tpcav),
			remote:     bob,
			authorized: true,
		},
		{
			// che:friend talking to che:family
			local:      A(che, "friend"),
			remote:     B(che, "family"),
			authorized: false,
		},
		{
			// {ali, bob:friend, che:friend} talking to {bob:friend:spouse, che:family}
			local:      U(ali, A(bob, "friend"), A(che, "friend")),
			remote:     U(B(bob, "friend:spouse", tpcav), B(che, "family")),
			call:       CallParams{RemoteDischarges: dismap},
			authorized: true,
		},
	}
	for _, test := range tests {
		test.call.LocalPrincipal = pali
		test.call.LocalBlessings = test.local
		test.call.RemoteBlessings = test.remote
		ctx, cancel := context.RootContext()
		err := authorizer.Authorize(ctx, NewCall(&test.call))
		if (err == nil) != test.authorized {
			t.Errorf("call: %v. Got %v", test.call, err)
		}
		cancel()
	}
}

func mkThirdPartyCaveat(t *testing.T, discharger PublicKey, location string, c Caveat) Caveat {
	tpc, err := NewPublicKeyCaveat(discharger, location, ThirdPartyRequirements{}, c)
	if err != nil {
		t.Fatal(err)
	}
	return tpc
}

func TestAllowEveryoneECDSA(t *testing.T) {
	testAllowEveryone(t,
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
	)
}
func TestAllowEveryoneED25519(t *testing.T) {
	testAllowEveryone(t,
		newED25519Principal(t),
		newED25519Principal(t),
	)
}

func TestAllowEveryone(t *testing.T) {
	testAllowEveryone(t,
		newECDSAPrincipal(t),
		newED25519Principal(t),
	)
	testAllowEveryone(t,
		newED25519Principal(t),
		newECDSAPrincipal(t),
	)
}

func testAllowEveryone(t *testing.T, pali, pbob Principal) {
	var (
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		ctx, cancel = context.RootContext()
	)
	defer cancel()
	if err := AllowEveryone().Authorize(ctx, NewCall(&CallParams{
		LocalPrincipal:  pali,
		LocalBlessings:  ali,
		RemoteBlessings: bob,
	})); err != nil {
		t.Fatal(err)
	}
}
func TestPublicKeyAuthorizerECDSA(t *testing.T) {
	testPublicKeyAuthorizer(t,
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
	)
}
func TestPublicKeyAuthorizerED25519(t *testing.T) {
	testPublicKeyAuthorizer(t,
		newED25519Principal(t),
		newED25519Principal(t),
	)
}

func TestPublicKeyAuthorizer(t *testing.T) {
	testPublicKeyAuthorizer(t,
		newECDSAPrincipal(t),
		newED25519Principal(t),
	)
	testPublicKeyAuthorizer(t,
		newED25519Principal(t),
		newECDSAPrincipal(t),
	)
}

func testPublicKeyAuthorizer(t *testing.T, pali, pbob Principal) {
	var (
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		ctx, cancel = context.RootContext()
	)
	defer cancel()
	if err := PublicKeyAuthorizer(pbob.PublicKey()).Authorize(ctx, NewCall(&CallParams{
		RemoteBlessings: bob,
	})); err != nil {
		t.Fatal(err)
	}
	if err := PublicKeyAuthorizer(pbob.PublicKey()).Authorize(ctx, NewCall(&CallParams{
		RemoteBlessings: ali,
	})); err == nil {
		t.Fatal("Expected error")
	}
}

func TestEndpointAuthorizerECDSA(t *testing.T) {
	testEndpointAuthorizer(t,
		newECDSAPrincipal(t),
		newECDSAPrincipal(t),
	)
}
func TestEndpointAuthorizerED25519(t *testing.T) {
	testEndpointAuthorizer(t,
		newED25519Principal(t),
		newED25519Principal(t),
	)
}
func TestEndpointAuthorizer(t *testing.T) {
	testEndpointAuthorizer(t,
		newECDSAPrincipal(t),
		newED25519Principal(t),
	)
	testEndpointAuthorizer(t,
		newED25519Principal(t),
		newECDSAPrincipal(t),
	)
}

func testEndpointAuthorizer(t *testing.T, pali, pbob Principal) {
	var (
		ali, _       = pali.BlessSelf("ali")
		alifriend, _ = pali.Bless(pbob.PublicKey(), ali, "friend", UnconstrainedUse())

		tests = []struct {
			ep naming.Endpoint
			ok bool
		}{
			{naming.Endpoint{}, true},
			{naming.Endpoint{}.WithBlessingNames([]string{"foo", "bar"}), false},
			{naming.Endpoint{}.WithBlessingNames([]string{"foo", "ali", "bar"}), true},
		}

		ctx, cancel = context.RootContext()
	)
	defer cancel()
	if err := AddToRoots(pali, ali); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		err := EndpointAuthorizer().Authorize(ctx, NewCall(&CallParams{
			RemoteEndpoint:  test.ep,
			LocalPrincipal:  pali,
			RemoteBlessings: alifriend,
		}))
		if (err == nil) != test.ok {
			t.Errorf("%v: Got error (%v), want error %v", test.ep, err, !test.ok)
		}
	}
}
