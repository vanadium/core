// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/naming"
	"v.io/v23/security"
)

func TestDefaultAuthorizerECDSA(t *testing.T) {
	testDefaultAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestDefaultAuthorizerED25519(t *testing.T) {
	testDefaultAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestDefaultAuthorizer(t *testing.T) {
	testDefaultAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testDefaultAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testDefaultAuthorizer(t *testing.T, pali, pbob, pche, pdis security.Principal) { // pdis is the third-party caveat discharger.
	var (
		che, _ = pche.BlessSelf("che")
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		tpcav  = mkThirdPartyCaveat(t, pdis.PublicKey(), "someLocation", security.UnconstrainedUse())
		dis, _ = pdis.MintDischarge(tpcav, security.UnconstrainedUse())
		dismap = map[string]security.Discharge{dis.ID(): dis}

		// bless(ali, bob, "friend") will generate a blessing for ali, calling him "bob:friend".
		bless = func(target, extend security.Blessings, extension string, caveats ...security.Caveat) security.Blessings {
			var p security.Principal
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
				caveats = []security.Caveat{security.UnconstrainedUse()}
			}
			ret, err := p.Bless(target.PublicKey(), extend, extension, caveats[0], caveats[1:]...)
			if err != nil {
				panic(err)
			}
			return ret
		}

		U = func(blessings ...security.Blessings) security.Blessings {
			u, err := security.UnionOfBlessings(blessings...)
			if err != nil {
				panic(err)
			}
			return u
		}

		// Shorthands for getting blessings for Ali and Bob.
		A = func(as security.Blessings, extension string, caveats ...security.Caveat) security.Blessings {
			return bless(ali, as, extension, caveats...)
		}
		B = func(as security.Blessings, extension string, caveats ...security.Caveat) security.Blessings {
			return bless(bob, as, extension, caveats...)
		}

		authorizer = security.DefaultAuthorizer()
	)
	// Make ali, bob (the two ends) recognize all three blessings
	for ip, p := range []security.Principal{pali, pbob} {
		for _, b := range []security.Blessings{ali, bob, che} {
			if err := security.AddToRoots(p, b); err != nil {
				t.Fatalf("%d: %v - %v", ip, b, err)
			}
		}
	}
	// All tests are run as if "ali" is the local end and "bob" is the remote.
	tests := []struct {
		local, remote security.Blessings
		call          security.CallParams
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
			call:       security.CallParams{RemoteDischarges: dismap},
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
			call:       security.CallParams{RemoteDischarges: dismap},
			authorized: true,
		},
	}
	for _, test := range tests {
		test.call.LocalPrincipal = pali
		test.call.LocalBlessings = test.local
		test.call.RemoteBlessings = test.remote
		ctx, cancel := context.RootContext()
		err := authorizer.Authorize(ctx, security.NewCall(&test.call))
		if (err == nil) != test.authorized {
			t.Errorf("call: %v. Got %v", test.call, err)
		}
		cancel()
	}
}

func mkThirdPartyCaveat(t *testing.T, discharger security.PublicKey, location string, c security.Caveat) security.Caveat {
	tpc, err := security.NewPublicKeyCaveat(discharger, location, security.ThirdPartyRequirements{}, c)
	if err != nil {
		t.Fatal(err)
	}
	return tpc
}

func TestAllowEveryoneECDSA(t *testing.T) {
	testAllowEveryone(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestAllowEveryoneED25519(t *testing.T) {
	testAllowEveryone(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestAllowEveryone(t *testing.T) {
	testAllowEveryone(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testAllowEveryone(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testAllowEveryone(t *testing.T, pali, pbob security.Principal) {
	var (
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		ctx, cancel = context.RootContext()
	)
	defer cancel()
	if err := security.AllowEveryone().Authorize(ctx, security.NewCall(&security.CallParams{
		LocalPrincipal:  pali,
		LocalBlessings:  ali,
		RemoteBlessings: bob,
	})); err != nil {
		t.Fatal(err)
	}
}
func TestPublicKeyAuthorizerECDSA(t *testing.T) {
	testPublicKeyAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestPublicKeyAuthorizerED25519(t *testing.T) {
	testPublicKeyAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestPublicKeyAuthorizer(t *testing.T) {
	testPublicKeyAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testPublicKeyAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testPublicKeyAuthorizer(t *testing.T, pali, pbob security.Principal) {
	var (
		ali, _ = pali.BlessSelf("ali")
		bob, _ = pbob.BlessSelf("bob")

		ctx, cancel = context.RootContext()
	)
	defer cancel()
	if err := security.PublicKeyAuthorizer(pbob.PublicKey()).Authorize(ctx, security.NewCall(&security.CallParams{
		RemoteBlessings: bob,
	})); err != nil {
		t.Fatal(err)
	}
	if err := security.PublicKeyAuthorizer(pbob.PublicKey()).Authorize(ctx, security.NewCall(&security.CallParams{
		RemoteBlessings: ali,
	})); err == nil {
		t.Fatal("Expected error")
	}
}

func TestEndpointAuthorizerECDSA(t *testing.T) {
	testEndpointAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestEndpointAuthorizerED25519(t *testing.T) {
	testEndpointAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}
func TestEndpointAuthorizer(t *testing.T) {
	testEndpointAuthorizer(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testEndpointAuthorizer(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testEndpointAuthorizer(t *testing.T, pali, pbob security.Principal) {
	var (
		ali, _       = pali.BlessSelf("ali")
		alifriend, _ = pali.Bless(pbob.PublicKey(), ali, "friend", security.UnconstrainedUse())

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
	if err := security.AddToRoots(pali, ali); err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		err := security.EndpointAuthorizer().Authorize(ctx, security.NewCall(&security.CallParams{
			RemoteEndpoint:  test.ep,
			LocalPrincipal:  pali,
			RemoteBlessings: alifriend,
		}))
		if (err == nil) != test.ok {
			t.Errorf("%v: Got error (%v), want error %v", test.ep, err, !test.ok)
		}
	}
}
