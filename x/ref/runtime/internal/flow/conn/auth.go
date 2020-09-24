// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/rand"
	"reflect"
	"sync"
	"time"

	"golang.org/x/crypto/nacl/box"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	iflow "v.io/x/ref/runtime/internal/flow"
)

var (
	authDialerTag   = []byte("AuthDial\x00")
	authAcceptorTag = []byte("AuthAcpt\x00")
)

func (c *Conn) dialHandshake(
	ctx *context.T,
	versions version.RPCVersionRange,
	auth flow.PeerAuthorizer) (names []string, rejected []security.RejectedBlessing, rtt time.Duration, err error) {
	binding, remoteEndpoint, rttstart, err := c.setup(ctx, versions, true)
	if err != nil {
		return nil, nil, 0, err
	}
	dialedEP := c.remote
	c.remote.RoutingID = remoteEndpoint.RoutingID
	bflow := c.newFlowLocked(
		ctx,
		blessingsFlowID,
		security.Blessings{},
		security.Blessings{},
		nil,
		nil,
		naming.Endpoint{},
		true,
		true,
		0,
		true)
	bflow.releaseLocked(DefaultBytesBufferedPerFlow)
	c.blessingsFlow = newBlessingsFlow(bflow)

	rBlessings, rDischarges, rttend, err := c.readRemoteAuth(ctx, binding, true)
	if err != nil {
		return nil, nil, 0, err
	}
	rtt = rttend.Sub(rttstart)
	if rBlessings.IsZero() {
		err = ErrAcceptorBlessingsMissing.Errorf(ctx, "acceptor did not send blessings")
		return nil, nil, rtt, err
	}
	if c.MatchesRID(dialedEP) {
		// If we hadn't reached the endpoint we expected we would have treated this connection as
		// a proxy, and proxies aren't authorized.  In this case we didn't find a proxy, so go ahead
		// and authorize the connection.
		names, rejected, err = auth.AuthorizePeer(ctx, c.local, c.remote, rBlessings, rDischarges)
		if err != nil {
			return names, rejected, rtt, iflow.MaybeWrapError(verror.ErrNotTrusted, ctx, err)
		}
	}
	signedBinding, err := v23.GetPrincipal(ctx).Sign(append(authDialerTag, binding...))
	if err != nil {
		return names, rejected, rtt, err
	}
	lAuth := &message.Auth{ChannelBinding: signedBinding}
	// The client sends its blessings without any blessing-pattern encryption to the
	// server as it has already authorized the server. Thus the 'peers' argument to
	// blessingsFlow.send is nil.
	if lAuth.BlessingsKey, _, err = c.blessingsFlow.send(ctx, c.localBlessings, nil, nil); err != nil {
		return names, rejected, rtt, err
	}
	c.mu.Lock()
	err = c.sendMessageLocked(ctx, true, expressPriority, lAuth)
	c.mu.Unlock()
	return names, rejected, rtt, err
}

// MatchesRID returns true if the given endpoint matches the routing
// ID of the remote server.  Also returns true if the given ep has
// the null routing id (in which case it is assumed that any connected
// server must be the target since nothing has been specified).
func (c *Conn) MatchesRID(ep naming.Endpoint) bool {
	return ep.RoutingID == naming.NullRoutingID ||
		c.remote.RoutingID == ep.RoutingID
}

func (c *Conn) acceptHandshake(
	ctx *context.T,
	versions version.RPCVersionRange,
	authorizedPeers []security.BlessingPattern) (rtt time.Duration, err error) {
	binding, remoteEndpoint, _, err := c.setup(ctx, versions, false)
	if err != nil {
		return rtt, err
	}
	c.remote = remoteEndpoint
	bflw := c.newFlowLocked(
		ctx,
		blessingsFlowID,
		security.Blessings{},
		security.Blessings{},
		nil,
		nil,
		naming.Endpoint{},
		true,
		true,
		0,
		true)
	c.blessingsFlow = newBlessingsFlow(bflw)
	signedBinding, err := v23.GetPrincipal(ctx).Sign(append(authAcceptorTag, binding...))
	if err != nil {
		return rtt, err
	}
	lAuth := &message.Auth{
		ChannelBinding: signedBinding,
	}

	lAuth.BlessingsKey, lAuth.DischargeKey, err = c.blessingsFlow.send(
		ctx, c.localBlessings, c.localDischarges, authorizedPeers)
	if err != nil {
		return rtt, err
	}

	c.mu.Lock()
	rttstart := time.Now()
	err = c.sendMessageLocked(ctx, true, expressPriority, lAuth)
	c.mu.Unlock()
	if err != nil {
		return rtt, err
	}
	_, _, rttend, err := c.readRemoteAuth(ctx, binding, false)
	rtt = rttend.Sub(rttstart)
	return rtt, err
}

func (c *Conn) setup(ctx *context.T, versions version.RPCVersionRange, dialer bool) ([]byte, naming.Endpoint, time.Time, error) { //nolint:gocyclo
	var rttstart time.Time
	pk, sk, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, naming.Endpoint{}, rttstart, err
	}
	lSetup := &message.Setup{
		Versions:          versions,
		PeerLocalEndpoint: c.local,
		PeerNaClPublicKey: pk,
		Mtu:               defaultMtu,
		SharedTokens:      DefaultBytesBufferedPerFlow,
	}
	if !c.remote.IsZero() {
		lSetup.PeerRemoteEndpoint = c.remote
	}
	ch := make(chan error, 1)
	go func() {
		c.mu.Lock()
		rttstart = time.Now()
		ch <- c.sendMessageLocked(ctx, true, expressPriority, lSetup)
		c.mu.Unlock()
	}()
	msg, err := c.mp.readMsg(ctx)
	if err != nil {
		<-ch
		return nil, naming.Endpoint{}, rttstart, ErrRecv.Errorf(ctx, "recv: %v", err)
	}
	rSetup, valid := msg.(*message.Setup)
	if !valid {
		<-ch
		return nil, naming.Endpoint{}, rttstart, ErrUnexpectedMsg.Errorf(ctx, "unexpected message type: %T", msg)
	}
	if err := <-ch; err != nil {
		remoteStr := ""
		if !c.remote.IsZero() {
			remoteStr = c.remote.String()
		}
		return nil, naming.Endpoint{}, rttstart, ErrSend.Errorf(ctx, "send: setup: remote %v: %v", remoteStr, err)
	}
	if c.version, err = version.CommonVersion(ctx, lSetup.Versions, rSetup.Versions); err != nil {
		return nil, naming.Endpoint{}, rttstart, err
	}
	if c.local.IsZero() {
		c.local = rSetup.PeerRemoteEndpoint
	}
	if rSetup.Mtu != 0 {
		c.mtu = rSetup.Mtu
	} else {
		c.mtu = defaultMtu
	}
	c.lshared = lSetup.SharedTokens
	if rSetup.SharedTokens != 0 && rSetup.SharedTokens < c.lshared {
		c.lshared = rSetup.SharedTokens
	}
	if rSetup.PeerNaClPublicKey == nil {
		return nil, naming.Endpoint{}, rttstart, ErrMissingSetupOption.Errorf(ctx, "missing required setup option: peerNaClPublicKey")
	}
	c.mu.Lock()
	binding := c.mp.setupEncryption(ctx, pk, sk, rSetup.PeerNaClPublicKey)
	c.mu.Unlock()
	if c.version >= version.RPCVersion14 {
		// We include the setup messages in the channel binding to prevent attacks
		// where a man in the middle changes fields in the Setup message (e.g. a
		// downgrade attack wherein a MITM attacker changes the Version field of
		// the Setup message to a lower-security version.)
		// We always put the dialer first in the binding.
		if dialer {
			if binding, err = message.Append(ctx, lSetup, nil); err != nil {
				return nil, naming.Endpoint{}, rttstart, err
			}
			if binding, err = message.Append(ctx, rSetup, binding); err != nil {
				return nil, naming.Endpoint{}, rttstart, err
			}
		} else {
			if binding, err = message.Append(ctx, rSetup, nil); err != nil {
				return nil, naming.Endpoint{}, rttstart, err
			}
			if binding, err = message.Append(ctx, lSetup, binding); err != nil {
				return nil, naming.Endpoint{}, rttstart, err
			}
		}
	}
	// if we're encapsulated in another flow, tell that flow to stop
	// encrypting now that we've started.
	if f, ok := c.mp.rw.(*flw); ok {
		f.disableEncryption()
	}
	return binding, rSetup.PeerLocalEndpoint, rttstart, nil
}

func (c *Conn) readRemoteAuth(ctx *context.T, binding []byte, dialer bool) (security.Blessings, map[string]security.Discharge, time.Time, error) {
	tag := authDialerTag
	if dialer {
		tag = authAcceptorTag
	}
	var (
		rauth       *message.Auth
		err         error
		rttend      time.Time
		rBlessings  security.Blessings
		rDischarges map[string]security.Discharge
	)
	for {
		msg, err := c.mp.readMsg(ctx)
		if err != nil {
			remote := ""
			if !c.remote.IsZero() {
				remote = c.remote.String()
			}
			return security.Blessings{}, nil, rttend, ErrRecv.Errorf(ctx, "error reading from %v: %v", remote, err)
		}
		if rauth, _ = msg.(*message.Auth); rauth != nil {
			rttend = time.Now()
			break
		}
		switch m := msg.(type) {
		case *message.TearDown:
			// A teardown message may be sent by the client if it decides
			// that it doesn't trust the server. We handle it here and return
			// a connection closed error rather than waiting for the readMsg
			// above to fail when it tries to read from the closed connection.
			if err := c.handleMessage(ctx, msg); err != nil {
				vlog.Infof("readRemoteAuth: handleMessage teardown: failed: %v", err)
			}
			return security.Blessings{}, nil, rttend, ErrConnectionClosed.Errorf(ctx, "connection closed")
		case *message.OpenFlow:
			// If we get an OpenFlow message here it needs to be handled
			// asynchronously since it will call the flow handler
			// which will block until NewAccepted (which calls
			// this method) returns. OpenFlow is generally expected
			// to be handled by readLoop.
			go func() {
				if err := c.handleMessage(ctx, msg); err != nil {
					vlog.Infof("readRemoteAuth: handleMessage for openFlow for flow %v: failed: %v", m.ID, err)
				}
			}()
			continue
		}
		if err = c.handleMessage(ctx, msg); err != nil {
			return security.Blessings{}, nil, rttend, err
		}
	}
	// Only read the blessings if we were the dialer. Any blessings from the dialer
	// will be sent later.
	if rauth.BlessingsKey != 0 {
		rBlessings, rDischarges, err = c.blessingsFlow.getRemote(
			ctx, rauth.BlessingsKey, rauth.DischargeKey)
		if err != nil {
			return security.Blessings{}, nil, rttend, err
		}
		c.mu.Lock()
		c.rPublicKey = rBlessings.PublicKey()
		c.remoteBlessings = rBlessings
		c.remoteDischarges = rDischarges
		c.remoteValid = make(chan struct{})
		c.mu.Unlock()
	}
	if c.rPublicKey == nil {
		return security.Blessings{}, nil, rttend, ErrNoPublicKey.Errorf(ctx, "No public key  received")
	}
	if !rauth.ChannelBinding.Verify(c.rPublicKey, append(tag, binding...)) {
		return security.Blessings{}, nil, rttend, ErrInvalidChannelBinding.Errorf(ctx, "The channel binding was invalid")
	}
	return rBlessings, rDischarges, rttend, nil
}

type blessingsFlow struct {
	enc *vom.Encoder
	dec *vom.Decoder
	f   *flw

	mu       sync.Mutex
	nextKey  uint64
	incoming *inCache
	outgoing *outCache
}

// inCache keeps track of incoming blessings, discharges, and keys.
type inCache struct {
	dkeys      map[uint64]uint64               // bkey -> dkey of the latest discharges.
	blessings  map[uint64]security.Blessings   // keyed by bkey
	discharges map[uint64][]security.Discharge // keyed by dkey
}

// outCache keeps track of outgoing blessings, discharges, and keys.
type outCache struct {
	bkeys map[string]uint64 // blessings uid -> bkey

	dkeys      map[uint64]uint64               // blessings bkey -> dkey of latest discharges
	blessings  map[uint64]security.Blessings   // keyed by bkey
	discharges map[uint64][]security.Discharge // keyed by dkey
}

func newBlessingsFlow(f *flw) *blessingsFlow {
	b := &blessingsFlow{
		f:       f,
		enc:     vom.NewEncoder(f),
		dec:     vom.NewDecoder(f),
		nextKey: 1,
		incoming: &inCache{
			blessings:  make(map[uint64]security.Blessings),
			dkeys:      make(map[uint64]uint64),
			discharges: make(map[uint64][]security.Discharge),
		},
		outgoing: &outCache{
			bkeys:      make(map[string]uint64),
			dkeys:      make(map[uint64]uint64),
			blessings:  make(map[uint64]security.Blessings),
			discharges: make(map[uint64][]security.Discharge),
		},
	}
	return b
}

func (b *blessingsFlow) receiveBlessingsLocked(ctx *context.T, bkey uint64, blessings security.Blessings) error {
	b.f.useCurrentContext(ctx)
	// When accepting, make sure the blessings received are bound to the conn's
	// remote public key.
	b.f.conn.mu.Lock()
	if pk := b.f.conn.rPublicKey; pk != nil && !reflect.DeepEqual(blessings.PublicKey(), pk) {
		b.f.conn.mu.Unlock()
		return ErrBlessingsNotBound.Errorf(ctx, "Blessings not bound to connection remote public key")
	}
	b.f.conn.mu.Unlock()
	b.incoming.blessings[bkey] = blessings
	return nil
}

func (b *blessingsFlow) receiveDischargesLocked(ctx *context.T, bkey, dkey uint64, discharges []security.Discharge) {
	b.incoming.discharges[dkey] = discharges
	b.incoming.dkeys[bkey] = dkey
}

func (b *blessingsFlow) receiveLocked(ctx *context.T, bd BlessingsFlowMessage) error {
	b.f.useCurrentContext(ctx)
	switch bd := bd.(type) {
	case BlessingsFlowMessageBlessings:
		bkey, blessings := bd.Value.BKey, bd.Value.Blessings
		if err := b.receiveBlessingsLocked(ctx, bkey, blessings); err != nil {
			return err
		}
	case BlessingsFlowMessageEncryptedBlessings:
		bkey, ciphertexts := bd.Value.BKey, bd.Value.Ciphertexts
		var blessings security.Blessings
		if err := decrypt(ctx, ciphertexts, &blessings); err != nil {
			// TODO(ataly): This error should not be returned if the
			// client has explicitly set the peer authorizer to nil.
			// In that case, the client does not care whether the server's
			// blessings can be decrypted or not. Ideally we should just
			// pass this error to the peer authorizer and handle it there.
			return iflow.MaybeWrapError(verror.ErrNotTrusted, ctx, ErrCannotDecryptBlessings.Errorf(ctx, "cannot decrypt the encrypted blessings sent by peer: %v", err))
		}
		if err := b.receiveBlessingsLocked(ctx, bkey, blessings); err != nil {
			return err
		}
	case BlessingsFlowMessageDischarges:
		bkey, dkey, discharges := bd.Value.BKey, bd.Value.DKey, bd.Value.Discharges
		b.receiveDischargesLocked(ctx, bkey, dkey, discharges)
	case BlessingsFlowMessageEncryptedDischarges:
		bkey, dkey, ciphertexts := bd.Value.BKey, bd.Value.DKey, bd.Value.Ciphertexts
		var discharges []security.Discharge
		if err := decrypt(ctx, ciphertexts, &discharges); err != nil {
			return iflow.MaybeWrapError(verror.ErrNotTrusted, ctx, ErrCannotDecryptDischarges.Errorf(ctx, "cannot decrypt the encrypted discharges sent by peer: %v", err))
		}
		b.receiveDischargesLocked(ctx, bkey, dkey, discharges)
	}
	return nil
}

// getRemote gets the remote blessings and discharges associated with the given
// bkey and dkey. We will read messages from the wire until we receive the
// looked for blessings. This method is normally called from the read loop of
// the conn, so all the packets for the desired blessing must have been received
// and buffered before calling this function.  This property is guaranteed since
// we always send blessings and discharges before sending their bkey/dkey
// references in a control message.
func (b *blessingsFlow) getRemote(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	b.f.useCurrentContext(ctx)
	for {
		blessings, hasB := b.incoming.blessings[bkey]
		if hasB {
			if dkey == 0 {
				return blessings, nil, nil
			}
			discharges, hasD := b.incoming.discharges[dkey]
			if hasD {
				return blessings, dischargeMap(discharges), nil
			}
		}

		var received BlessingsFlowMessage
		err := b.dec.Decode(&received)
		if err != nil {
			return security.Blessings{}, nil, err
		}
		if err := b.receiveLocked(ctx, received); err != nil {
			b.f.conn.internalClose(ctx, false, false, err)
			return security.Blessings{}, nil, err
		}
	}
}

func (b *blessingsFlow) encodeBlessingsLocked(ctx *context.T, blessings security.Blessings, bkey uint64, peers []security.BlessingPattern) error {
	b.f.useCurrentContext(ctx)
	if len(peers) == 0 {
		// blessings can be encoded in plaintext
		return b.enc.Encode(BlessingsFlowMessageBlessings{Blessings{
			BKey:      bkey,
			Blessings: blessings,
		}})
	}
	ciphertexts, err := encrypt(ctx, peers, blessings)
	if err != nil {
		return ErrCannotEncryptBlessings.Errorf(ctx, "cannot encrypt blessings for peer: %v: %v", peers, err)
	}
	return b.enc.Encode(BlessingsFlowMessageEncryptedBlessings{EncryptedBlessings{
		BKey:        bkey,
		Ciphertexts: ciphertexts,
	}})
}

func (b *blessingsFlow) encodeDischargesLocked(ctx *context.T, discharges []security.Discharge, bkey, dkey uint64, peers []security.BlessingPattern) error {
	b.f.useCurrentContext(ctx)
	if len(peers) == 0 {
		// discharges can be encoded in plaintext
		return b.enc.Encode(BlessingsFlowMessageDischarges{Discharges{
			Discharges: discharges,
			DKey:       dkey,
			BKey:       bkey,
		}})
	}
	ciphertexts, err := encrypt(ctx, peers, discharges)
	if err != nil {
		return ErrCannotEncryptDischarges.Errorf(ctx, "cannot encrypt discharges for peers: %v: %v", peers, err)
	}
	return b.enc.Encode(BlessingsFlowMessageEncryptedDischarges{EncryptedDischarges{
		DKey:        dkey,
		BKey:        bkey,
		Ciphertexts: ciphertexts,
	}})
}

func (b *blessingsFlow) send(
	ctx *context.T,
	blessings security.Blessings,
	discharges map[string]security.Discharge,
	peers []security.BlessingPattern) (bkey, dkey uint64, err error) {
	if blessings.IsZero() {
		return 0, 0, nil
	}
	defer b.mu.Unlock()
	b.mu.Lock()
	b.f.useCurrentContext(ctx)

	buid := string(blessings.UniqueID())
	bkey, hasB := b.outgoing.bkeys[buid]
	if !hasB {
		bkey = b.nextKey
		b.nextKey++
		b.outgoing.bkeys[buid] = bkey
		b.outgoing.blessings[bkey] = blessings
		if err := b.encodeBlessingsLocked(ctx, blessings, bkey, peers); err != nil {
			return 0, 0, err
		}
	}
	if len(discharges) == 0 {
		return bkey, 0, nil
	}
	dkey, hasD := b.outgoing.dkeys[bkey]
	if hasD && equalDischarges(discharges, b.outgoing.discharges[dkey]) {
		return bkey, dkey, nil
	}
	dlist := dischargeList(discharges)
	dkey = b.nextKey
	b.nextKey++
	b.outgoing.dkeys[bkey] = dkey
	b.outgoing.discharges[dkey] = dlist
	return bkey, dkey, b.encodeDischargesLocked(ctx, dlist, bkey, dkey, peers)
}

func (b *blessingsFlow) close(ctx *context.T, err error) {
	b.f.useCurrentContext(ctx)
	b.f.close(ctx, false, err)
}

func dischargeList(in map[string]security.Discharge) []security.Discharge {
	out := make([]security.Discharge, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}
func dischargeMap(in []security.Discharge) map[string]security.Discharge {
	out := make(map[string]security.Discharge, len(in))
	for _, d := range in {
		out[d.ID()] = d
	}
	return out
}

func equalDischarges(m map[string]security.Discharge, s []security.Discharge) bool {
	if len(m) != len(s) {
		return false
	}
	for _, d := range s {
		inm, ok := m[d.ID()]
		if !ok || !d.Equivalent(inm) {
			return false
		}
	}
	return true
}
