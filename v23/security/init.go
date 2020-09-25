// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"time"

	"v.io/v23/context"
)

func init() {
	RegisterCaveatValidator(ConstCaveat, func(ctx *context.T, _ Call, isValid bool) error {
		if isValid {
			return nil
		}
		return ErrorfConstCaveatValidation(ctx, "false const caveat always fails validation")
	})

	RegisterCaveatValidator(ExpiryCaveat, func(ctx *context.T, call Call, expiry time.Time) error {
		now := call.Timestamp()
		if now.After(expiry) {
			return ErrorfExpiryCaveatValidation(ctx, "now(%v) is after expiry(%v)", now, expiry)
		}
		return nil
	})

	RegisterCaveatValidator(MethodCaveat, func(ctx *context.T, call Call, methods []string) error {
		for _, m := range methods {
			if call.Method() == m {
				return nil
			}
		}
		return ErrorfMethodCaveatValidation(ctx, "method %v not in list %v", call.Method(), methods)
	})

	RegisterCaveatValidator(PeerBlessingsCaveat, func(ctx *context.T, call Call, patterns []BlessingPattern) error {
		lnames := LocalBlessingNames(ctx, call)
		for _, p := range patterns {
			if p.MatchedBy(lnames...) {
				return nil

			}
		}
		return ErrorfPeerBlessingsCaveatValidation(ctx, "patterns in peer blessings caveat %v not matched by the peer %v", lnames, patterns)
	})

	RegisterCaveatValidator(PublicKeyThirdPartyCaveat, func(ctx *context.T, call Call, params publicKeyThirdPartyCaveatParam) error {
		discharge, ok := call.RemoteDischarges()[params.ID()]
		if !ok {
			return fmt.Errorf("missing discharge for third party caveat(id=%v)", params.ID())
		}
		// Must be of the valid type.
		var d *PublicKeyDischarge
		switch v := discharge.wire.(type) {
		case WireDischargePublicKey:
			d = &v.Value
		default:
			return fmt.Errorf("invalid discharge(%T) for caveat(%T)", v, params)
		}
		// Must be signed by the principal designated by c.DischargerKey
		key, err := params.discharger(ctx)
		if err != nil {
			return err
		}
		if err := d.verify(ctx, key); err != nil {
			return err
		}
		// And all caveats on the discharge must be met.
		for _, cav := range d.Caveats {
			if err := cav.Validate(ctx, call); err != nil {
				return fmt.Errorf("a caveat(%v) on the discharge failed to validate: %v", cav, err)
			}
		}
		return nil
	})
}
