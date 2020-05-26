// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

func NewDischargeServer(ctx *context.T) security.Caveat {
	_, server, err := v23.WithNewServer(ctx, "", &dischargeServer{}, security.AllowEveryone())
	if err != nil {
		ctx.Fatalf("WithNewServer failed: %v", err)
	}
	serverAddr := server.Status().Endpoints[0].Name()
	return mkThirdPartyCaveat(v23.GetPrincipal(ctx).PublicKey(), serverAddr, security.UnconstrainedUse())
}

type dischargeServer struct{}

func (ds *dischargeServer) Discharge(ctx *context.T, call rpc.StreamServerCall, cav security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	tp := cav.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, fmt.Errorf("discharger: %v does not represent a third-party caveat", cav)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", cav, err)
	}
	return call.Security().LocalPrincipal().MintDischarge(cav, security.UnconstrainedUse())
}

func mkThirdPartyCaveat(discharger security.PublicKey, location string, c security.Caveat) security.Caveat {
	tpc, err := security.NewPublicKeyCaveat(discharger, location, security.ThirdPartyRequirements{}, c)
	if err != nil {
		panic(err)
	}
	return tpc
}
