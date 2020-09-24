// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/rpc"
	"v.io/v23/security"

	"v.io/x/ref/services/discharger"
)

func init() {
	security.RegisterCaveatValidator(LoggingCaveat, func(ctx *context.T, _ security.Call, params []string) error {
		ctx.Infof("Params: %#v", params)
		return nil
	})

}

type dischargerImpl struct {
	serverConfig *serverConfig
}

func (dischargerImpl) Discharge(ctx *context.T, call rpc.ServerCall, caveat security.Caveat, impetus security.DischargeImpetus) (security.Discharge, error) {
	details := caveat.ThirdPartyDetails()
	if details == nil {
		return security.Discharge{}, discharger.ErrorfNotAThirdPartyCaveat(ctx, "discharges are not required for non-third-party caveats (id: %v)", caveat)
	}
	if err := details.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, err
	}
	// TODO(rthellend,ashankar): Do proper logging when the API allows it.
	ctx.Infof("Discharge() impetus: %#v", impetus)

	expiry, err := security.NewExpiryCaveat(time.Now().Add(5 * time.Minute))
	if err != nil {
		return security.Discharge{}, useOrCreateErrInternal(ctx, err)
	}
	// Bind the discharge to precisely the purpose the requestor claims it will be used.
	method, err := security.NewMethodCaveat(impetus.Method)
	if err != nil {
		return security.Discharge{}, useOrCreateErrInternal(ctx, err)
	}
	peer, err := security.NewCaveat(security.PeerBlessingsCaveat, impetus.Server)
	if err != nil {
		return security.Discharge{}, useOrCreateErrInternal(ctx, err)
	}
	discharge, err := v23.GetPrincipal(ctx).MintDischarge(caveat, expiry, method, peer)
	if err != nil {
		return security.Discharge{}, useOrCreateErrInternal(ctx, err)
	}
	return discharge, nil
}

//nolint:golint // API change required.
func (d *dischargerImpl) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	return globChildren(ctx, call, d.serverConfig, m)
}
