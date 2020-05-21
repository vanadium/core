// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"testing"

	v23 "v.io/v23"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/ref/test/testutil"
)

func TestServerAuthorizer(t *testing.T) {
	var (
		pclient = testutil.NewPrincipal()
		pserver = testutil.NewPrincipal()
		pother  = testutil.NewPrincipal()

		ali, _      = pserver.BlessSelf("ali")
		bob, _      = pserver.BlessSelf("bob")
		che, _      = pserver.BlessSelf("che")
		otherAli, _ = pother.BlessSelf("ali")

		ctx, shutdown = initForTest()

		U = func(blessings ...security.Blessings) security.Blessings {
			u, err := security.UnionOfBlessings(blessings...)
			if err != nil {
				t.Fatal(err)
			}
			return u
		}

		ACL = func(patterns ...string) security.Authorizer {
			var acl access.AccessList
			for _, p := range patterns {
				acl.In = append(acl.In, security.BlessingPattern(p))
			}
			return acl
		}
	)
	defer shutdown()
	ctx, _ = v23.WithPrincipal(ctx, pclient)
	// Make client recognize ali, bob and otherAli blessings
	for _, b := range []security.Blessings{ali, bob, otherAli} {
		if err := security.AddToRoots(pclient, b); err != nil {
			t.Fatal(err)
		}
	}
	// All tests are run as if pclient is the client end and pserver is remote end.
	tests := []struct {
		serverBlessingNames []string
		auth                security.Authorizer
		authorizedServers   []security.Blessings
		unauthorizedServers []security.Blessings
	}{
		{
			// No blessings in the endpoint means that all servers are authorized.
			nil,
			newServerAuthorizer(""),
			[]security.Blessings{ali, otherAli, bob, che},
			[]security.Blessings{},
		},
		{
			// Endpoint sets the expectations for "ali" and "bob".
			// Since no options.ServerAuthorizer was provided, use the blessing names in the endpoint.
			[]string{"ali", "bob"},
			newServerAuthorizer(""),
			[]security.Blessings{ali, otherAli, bob, U(ali, che), U(bob, che)},
			[]security.Blessings{che},
		},
		{
			// Still only ali, otherAli and bob are authorized (che is not
			// authorized since it is not recognized by the client)
			[]string{"ali", "bob", "che"},
			newServerAuthorizer(""),
			[]security.Blessings{ali, otherAli, bob, U(ali, che), U(bob, che)},
			[]security.Blessings{che},
		},
		{

			// Only ali and otherAli are authorized (since the
			// policy does not allow "bob")
			[]string{"ali", "bob", "che"},
			newServerAuthorizer("", options.ServerAuthorizer{
				Authorizer: ACL("ali", "dan")}),
			[]security.Blessings{ali, otherAli, U(ali, che), U(ali, bob)},
			[]security.Blessings{bob, che},
		},
		{
			// Only otherAli is authorized (since only pother's public key is
			// authorized)
			[]string{"ali"},
			newServerAuthorizer("", options.ServerAuthorizer{
				Authorizer: security.PublicKeyAuthorizer(pother.PublicKey())}),
			[]security.Blessings{otherAli},
			[]security.Blessings{ali, bob, che},
		},
		{
			// Blessings in endpoint can be ignored by security.AllowEveryone.
			[]string{"ali"},
			newServerAuthorizer("", options.ServerAuthorizer{
				Authorizer: security.AllowEveryone()}),
			[]security.Blessings{ali, bob, che, otherAli},
			nil,
		},
		{
			// Pattern provied to newServerAuthorizer is respected.
			nil,
			newServerAuthorizer("bob"),
			[]security.Blessings{bob, U(ali, bob)},
			[]security.Blessings{ali, otherAli, che},
		},
		{
			// And if the intersection of the policy and the
			// pattern is empty, then so be it.
			[]string{"ali", "bob", "che"},
			newServerAuthorizer("bob", options.ServerAuthorizer{
				Authorizer: ACL("ali", "che")}),
			[]security.Blessings{U(ali, bob), U(ali, bob, che)},
			[]security.Blessings{ali, otherAli, bob, che, U(ali, che)},
		},
	}
	for _, test := range tests {
		call := &mockCall{
			p:   pclient,
			rep: naming.Endpoint{}.WithBlessingNames(test.serverBlessingNames),
		}
		for _, s := range test.authorizedServers {
			call.r = s
			if err := test.auth.Authorize(ctx, call); err != nil {
				t.Errorf("serverAuthorizer: %#v failed to authorize server: %v", test.auth, s)
			}
		}
		for _, s := range test.unauthorizedServers {
			call.r = s
			if err := test.auth.Authorize(ctx, call); err == nil {
				t.Errorf("serverAuthorizer: %#v authorized server: %v", test.auth, s)
			}
		}
	}
}
