// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dischargerlib

import (
	"fmt"
	"time"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/services/discharger"
)

const dischargeExpiryTime = 24 * time.Hour

// dischargerd issues discharges for all caveats present in the current
// namespace with no additional caveats iff the caveat is valid.
type dischargerd struct{}

func (dischargerd) Discharge(ctx *context.T, call rpc.ServerCall, caveat security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	tp := caveat.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, discharger.NewErrNotAThirdPartyCaveat(ctx, caveat)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", tp, err)
	}
	expiry, err := security.NewExpiryCaveat(time.Now().Add(dischargeExpiryTime))
	if err != nil {
		return security.Discharge{}, fmt.Errorf("unable to create expiration caveat on the discharge: %v", err)
	}
	return call.Security().LocalPrincipal().MintDischarge(caveat, expiry)
}

// NewDischarger returns a discharger service implementation that grants
// discharges using the MintDischarge on the principal receiving the RPC.
//
// Discharges are valid for 24 hours.
func NewDischarger() discharger.DischargerServerMethods {
	return dischargerd{}
}
