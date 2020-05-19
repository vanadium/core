// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blesser

import (
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

type rpcCall struct {
	rpc.ServerCall
	secCall security.Call
}

func (c rpcCall) Security() security.Call         { return c.secCall }
func (c rpcCall) LocalEndpoint() naming.Endpoint  { return naming.Endpoint{} }
func (c rpcCall) RemoteEndpoint() naming.Endpoint { return naming.Endpoint{} }

func fakeCall(provider, user security.Principal) rpc.ServerCall {
	return rpcCall{secCall: security.NewCall(&security.CallParams{
		LocalPrincipal:  provider,
		LocalBlessings:  blessSelf(provider, "provider"),
		RemoteBlessings: blessSelf(user, "self-signed-user"),
	})}
}

func blessSelf(p security.Principal, name string) security.Blessings {
	b, err := p.BlessSelf(name)
	if err != nil {
		panic(err)
	}
	return b
}

func newCaveat(c security.Caveat, err error) security.Caveat {
	if err != nil {
		panic(err)
	}
	return c
}
