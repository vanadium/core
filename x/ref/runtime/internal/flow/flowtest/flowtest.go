// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flowtest

import (
	"fmt"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/security"
)

// Pipe returns a connection pair dialed on against a listener using
// the given network and address.
func Pipe(t testing.TB, ctx *context.T, network, address string) (dialed, accepted flow.Conn) {
	var err error
	dialed, accepted, err = NewPipe(ctx, network, address)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// NewPipe returns a connection pair dialed on against a listener using
// the given network and address.
func NewPipe(ctx *context.T, network, address string) (dialed, accepted flow.Conn, err error) {
	local, _ := flow.RegisteredProtocol(network)
	if local == nil {
		return nil, nil, fmt.Errorf("No registered protocol %s", network)
	}
	l, err := local.Listen(ctx, network, address)
	if err != nil {
		return nil, nil, err
	}
	d, err := local.Dial(ctx, l.Addr().Network(), l.Addr().String(), time.Second)
	if err != nil {
		return nil, nil, err
	}
	a, err := l.Accept(ctx)
	if err != nil {
		return nil, nil, err
	}
	l.Close()
	return d, a, nil
}

type AllowAllPeersAuthorizer struct{}

func (AllowAllPeersAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEndpoint, remoteEndpoint naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges security.Discharges,
) ([]string, []security.RejectedBlessing, error) {
	return nil, nil, nil
}

func (AllowAllPeersAuthorizer) BlessingsForPeer(ctx *context.T, _ []string) (security.Blessings, security.Discharges, error) {
	b, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	return b, nil, nil
}

type peersAuthorizer []string

func NewPeerAuthorizer(names []string) flow.PeerAuthorizer {
	if len(names) == 0 {
		return AllowAllPeersAuthorizer{}
	}
	return peersAuthorizer(names)
}

func (p peersAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEndpoint, remoteEndpoint naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges security.Discharges,
) ([]string, []security.RejectedBlessing, error) {
	call := security.NewCall(&security.CallParams{
		Timestamp:        time.Now(),
		LocalPrincipal:   v23.GetPrincipal(ctx),
		LocalEndpoint:    localEndpoint,
		RemoteBlessings:  remoteBlessings,
		RemoteDischarges: remoteDischarges,
		RemoteEndpoint:   remoteEndpoint,
	})
	peerNames, rejectedPeerNames := security.RemoteBlessingNames(ctx, call)
	for _, pattern := range p {
		if security.BlessingPattern(pattern).MatchedBy(peerNames...) {
			return peerNames, rejectedPeerNames, nil
		}
	}
	return peerNames, rejectedPeerNames, fmt.Errorf("not authorized")
}

func (peersAuthorizer) BlessingsForPeer(ctx *context.T, _ []string) (security.Blessings, security.Discharges, error) {
	b, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	return b, nil, nil
}
