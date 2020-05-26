// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package revocation

import (
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/discharger"
	"v.io/x/ref/services/identity/internal/dischargerlib"
	"v.io/x/ref/test"
)

func revokerSetup(t *testing.T, ctx *context.T) (dischargerKey security.PublicKey, dischargerEndpoint string, revoker RevocationManager) {
	dischargerServiceStub := discharger.DischargerServer(dischargerlib.NewDischarger())
	ctx, dischargerServer, err := v23.WithNewServer(ctx, "", dischargerServiceStub, nil)
	if err != nil {
		t.Fatalf("r.NewServer: %s", err)
	}
	name := dischargerServer.Status().Endpoints[0].Name()
	return v23.GetPrincipal(ctx).PublicKey(), name, NewMockRevocationManager(ctx)
}

func TestDischargeRevokeDischargeRevokeDischarge(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	dcKey, dc, revoker := revokerSetup(t, ctx)

	discharger := discharger.DischargerClient(dc)
	caveat, err := revoker.NewCaveat(dcKey, dc)
	if err != nil {
		t.Fatalf("failed to create revocation caveat: %s", err)
	}
	tp := caveat.ThirdPartyDetails()
	if tp == nil {
		t.Fatalf("failed to extract third party details from caveat %v", caveat)
	}

	var impetus security.DischargeImpetus
	if _, err := discharger.Discharge(ctx, caveat, impetus); err != nil {
		t.Fatalf("failed to get discharge: %s", err)
	}
	if err := revoker.Revoke(tp.ID()); err != nil {
		t.Fatalf("failed to revoke: %s", err)
	}
	if _, err := discharger.Discharge(ctx, caveat, impetus); err == nil {
		t.Fatalf("got a discharge for a revoked caveat: %s", err)
	}
	if err := revoker.Revoke(tp.ID()); err != nil {
		t.Fatalf("failed to revoke again: %s", err)
	}
	if _, err := discharger.Discharge(ctx, caveat, impetus); err == nil {
		t.Fatalf("got a discharge for a doubly revoked caveat: %s", err)
	}
}
