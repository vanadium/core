// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blesser

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"v.io/x/ref/services/identity"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/util"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vom"
)

type macaroonBlesser struct {
}

// NewMacaroonBlesserServer provides an identity.MacaroonBlesser Service that generates blessings
// after unpacking a BlessingMacaroon.
func NewMacaroonBlesserServer() identity.MacaroonBlesserServerStub {
	return identity.MacaroonBlesserServer(&macaroonBlesser{})
}

func (b *macaroonBlesser) Bless(ctx *context.T, call rpc.ServerCall, macaroon string) (security.Blessings, error) {
	secCall := call.Security()
	var empty security.Blessings
	inputs, err := util.Macaroon(macaroon).Decode(v23.GetPrincipal(ctx))
	if err != nil {
		return empty, err
	}
	var m oauth.BlessingMacaroon
	if err := vom.Decode(inputs, &m); err != nil {
		return empty, err
	}
	if time.Now().After(m.Creation.Add(time.Minute * 5)) {
		return empty, fmt.Errorf("macaroon has expired")
	}
	if secCall.LocalPrincipal() == nil {
		return empty, fmt.Errorf("server misconfiguration: no authentication happened")
	}
	macaroonPublicKey, err := security.UnmarshalPublicKey(m.PublicKey)
	if err != nil {
		return empty, fmt.Errorf("failed to unmarshal public key in macaroon: %v", err)
	}
	if !reflect.DeepEqual(secCall.RemoteBlessings().PublicKey(), macaroonPublicKey) {
		return empty, errors.New("remote end's public key does not match public key in macaroon")
	}
	if len(m.Caveats) == 0 {
		m.Caveats = []security.Caveat{security.UnconstrainedUse()}
	}
	return secCall.LocalPrincipal().Bless(secCall.RemoteBlessings().PublicKey(),
		secCall.LocalBlessings(), m.Name, m.Caveats[0], m.Caveats[1:]...)
}
