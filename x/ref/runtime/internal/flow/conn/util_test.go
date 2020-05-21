// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"errors"
	"reflect"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	securitylib "v.io/x/ref/lib/security"
	"v.io/x/ref/runtime/internal/flow/flowtest"
)

type fh chan<- flow.Flow

func (fh fh) HandleFlow(f flow.Flow) error {
	if fh == nil {
		panic("writing to nil flow handler")
	}
	fh <- f
	return nil
}

func setupConns(t *testing.T,
	network, address string,
	dctx, actx *context.T,
	dflows, aflows chan<- flow.Flow,
	dAuth, aAuth []security.BlessingPattern) (dialed, accepted *Conn, derr, aerr error) {
	return setupConnsWithTimeout(t, network, address, dctx, actx, dflows, aflows, dAuth, aAuth, 0, time.Minute, 0)
}

func setupConnsWithTimeout(t *testing.T,
	network, address string,
	dctx, actx *context.T,
	dflows, aflows chan<- flow.Flow,
	dAuth, aAuth []security.BlessingPattern,
	acceptdelay time.Duration,
	handshakeTimeout time.Duration,
	channelTimeout time.Duration) (dialed, accepted *Conn, derr, aerr error) {
	dmrw, amrw := flowtest.Pipe(t, actx, network, address)
	versions := version.RPCVersionRange{Min: 3, Max: 5}
	ridep := naming.Endpoint{Protocol: network, Address: address, RoutingID: naming.FixedRoutingID(191341)}
	ep := naming.Endpoint{Protocol: network, Address: address}
	dch := make(chan *Conn)
	ach := make(chan *Conn)
	derrch := make(chan error)
	aerrch := make(chan error)
	go func() {
		var handler FlowHandler
		dep := ep
		if dflows != nil {
			handler = fh(dflows)
			dep = ridep
		}
		dBlessings, _ := v23.GetPrincipal(dctx).BlessingStore().Default()
		d, _, _, err := NewDialed(dctx, dmrw, dep, ep, versions, peerAuthorizer{dBlessings, dAuth}, false, handshakeTimeout, channelTimeout, handler)
		dch <- d
		derrch <- err
	}()
	go func() {
		var handler FlowHandler
		if aflows != nil {
			handler = fh(aflows)
		}
		if acceptdelay > 0 {
			time.Sleep(acceptdelay)
		}
		a, err := NewAccepted(actx, aAuth, amrw, ridep, versions, time.Minute, channelTimeout, handler)
		ach <- a
		aerrch <- err
	}()
	return <-dch, <-ach, <-derrch, <-aerrch
}

func setupFlow(t *testing.T, network, address string, dctx, actx *context.T, dialFromDialer bool) (dialed flow.Flow, accepted <-chan flow.Flow, close func()) {
	dfs, accepted, ac, dc := setupFlows(t, network, address, dctx, actx, dialFromDialer, 1)
	return dfs[0], accepted, func() { dc.Close(dctx, nil); ac.Close(dctx, nil) }
}

func setupFlows(t *testing.T, network, address string, dctx, actx *context.T, dialFromDialer bool, n int) (dialed []flow.Flow, accepted <-chan flow.Flow, dc, ac *Conn) {
	dialed = make([]flow.Flow, n)
	dflows, aflows := make(chan flow.Flow, n), make(chan flow.Flow, n)
	d, a, derr, aerr := setupConns(t, network, address, dctx, actx, dflows, aflows, nil, nil)
	if derr != nil || aerr != nil {
		t.Fatal(derr, aerr)
	}
	if !dialFromDialer {
		d, a = a, d
		dctx = actx
		aflows = dflows
	}
	for i := 0; i < n; i++ {
		var err error
		if dialed[i], err = d.Dial(dctx, d.LocalBlessings(), nil, naming.Endpoint{}, 0, false); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}
	return dialed, aflows, d, a
}

func oneFlow(t *testing.T, ctx *context.T, dc *Conn, aflows <-chan flow.Flow, channelTimeout time.Duration) (df, af flow.Flow) {
	df, err := dc.Dial(ctx, dc.LocalBlessings(), nil, naming.Endpoint{}, channelTimeout, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	b := []byte{0}
	if _, err := df.WriteMsg(b); err != nil {
		t.Fatalf("Couldn't write.")
	}
	af = <-aflows
	if got, err := af.ReadMsg(); err != nil || !reflect.DeepEqual(b, got) {
		t.Fatalf("Couldn't read %v %v", got, err)
	}
	return df, af
}

type peerAuthorizer struct {
	// blessings are the blessings presented to all peers.
	blessings security.Blessings
	// authorizedPeers are the set of blessing patterns according
	// to which peers are authorized. If the set is nil or empty then
	// all peers are authorized.
	authorizedPeers []security.BlessingPattern
}

func (a peerAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEP, remoteEP naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges map[string]security.Discharge,
) ([]string, []security.RejectedBlessing, error) {
	call := security.NewCall(&security.CallParams{
		Timestamp:        time.Now(),
		LocalPrincipal:   v23.GetPrincipal(ctx),
		LocalEndpoint:    localEP,
		RemoteEndpoint:   remoteEP,
		RemoteBlessings:  remoteBlessings,
		RemoteDischarges: remoteDischarges,
	})
	peerNames, rejectedNames := security.RemoteBlessingNames(ctx, call)
	if len(a.authorizedPeers) == 0 {
		return peerNames, rejectedNames, nil
	}
	for _, p := range a.authorizedPeers {
		if p.MatchedBy(peerNames...) {
			return peerNames, rejectedNames, nil
		}
	}
	return nil, nil, errors.New("peer not authorized")
}

func (a peerAuthorizer) BlessingsForPeer(ctx *context.T, _ []string) (
	security.Blessings, map[string]security.Discharge, error) {
	dis, _ := securitylib.PrepareDischarges(ctx, a.blessings, nil, "", nil)
	return a.blessings, dis, nil
}
