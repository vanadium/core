// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package access_test

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
)

func TestAccessTagCaveatECDSA(t *testing.T) {
	testAccessTagCaveat(t,
		sectest.NewECDSAPrincipalP256TrustAllRoots(t),
		sectest.NewED25519PrincipalTrustAllRoots(t))
}

func TestAccessTagCaveatED25519(t *testing.T) {
	testAccessTagCaveat(t,
		sectest.NewED25519PrincipalTrustAllRoots(t),
		sectest.NewECDSAPrincipalP256TrustAllRoots(t))
}

func testAccessTagCaveat(t *testing.T, server, other security.Principal) {
	var (
		bserver, _ = server.BlessSelf("server")
		caveat, _  = access.NewAccessTagCaveat(access.Debug, access.Resolve)
		bclient, _ = server.Bless(other.PublicKey(), bserver, "debugger", caveat)
		tests      = []struct {
			MethodTags []*vdl.Value
			OK         bool
		}{
			{nil, false},
			{[]*vdl.Value{vdl.ValueOf(access.Debug)}, true},
			{[]*vdl.Value{vdl.ValueOf(access.Resolve)}, true},
			{[]*vdl.Value{vdl.ValueOf(access.Read), vdl.ValueOf(access.Debug)}, true},
			{[]*vdl.Value{vdl.ValueOf(access.Read), vdl.ValueOf(access.Write)}, false},
			{[]*vdl.Value{vdl.ValueOf("Debug"), vdl.ValueOf("Resolve")}, false},
		}
	)
	if err := security.AddToRoots(server, bserver); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	for idx, test := range tests {
		call := security.NewCall(&security.CallParams{
			MethodTags:      test.MethodTags,
			LocalPrincipal:  server,
			RemoteBlessings: bclient,
		})
		got, rejected := security.RemoteBlessingNames(ctx, call)
		if test.OK {
			if len(got) != 1 || got[0] != "server:debugger" {
				t.Errorf("Got (%v, %v), wanted ([%q], nil) for method tags %v (test case #%d)", got, rejected, "server:debugger", test.MethodTags, idx)
			}
		}
		if !test.OK && len(got) != 0 {
			t.Errorf("Got (%v, %v), wanted all blessings to be rejected for method tags %v (test case #%d)", got, rejected, test.MethodTags, idx)
		}
	}
}
