// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"fmt"
	"reflect"
	"sync"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	iflow "v.io/x/ref/runtime/internal/flow"
)

// writeBuffer is used to coalesce the many small writes issued by the vom
// encoder into a single larger one to reduce the number of system calls
// and network packets sent.
type writeBuffer struct {
	wr   *writer
	conn blessingsCon
	buf  []byte
}

func (w *writeBuffer) Write(buf []byte) (int, error) {
	w.buf = append(w.buf, buf...)
	return len(buf), nil
}

func (w *writeBuffer) Flush(ctx *context.T) error {
	if len(w.buf) == 0 {
		return nil
	}
	err := w.conn.writeEncodedBlessings(ctx, w.wr, w.buf)
	// Free the buffer since it's unlikely that a second set of
	// blessings or discharges will be sent.
	w.buf = nil
	return err
}

// blessingsFlow is used to implement the 'special' flow (ID 1) used to
// send/receive blessings from a remote peer. The connection that uses
// this special flow is responsible for writing received blessings data to it
// (via writeMsg). The flow implementation calls the connection via its
// writeEncodedBlessings method to send blessings to the connection's
// remote peer; this is the only method that the flow invokes on the
// connection. Past implementations would close the connection on
// encountering an error but it is preferable to defer this to higher
// levels of the connection code and to avoid having to reason about all
// possible interactions between the two.
//
// blessingsFlow will validate newly received blessings against a specified
// public key (set via setPublicKeyBinding), however, this public key
// is not known until after the first blessing's exchange and hence is specified
// by the connection hosting this flow once that initial exchange has taken
// place. This prevents the remote peer from sending additional blessings
// with a different pubic key over the existing connection.
type blessingsFlow struct {
	writer

	mu sync.Mutex

	conn   blessingsCon
	dec    *vom.Decoder
	enc    *vom.Encoder
	encBuf writeBuffer
	decBuf bytes.Buffer

	binding security.PublicKey

	nextKey  uint64
	incoming inCache
	outgoing outCache
}

// blessingsCon represents the methods that the blessings flow expects of its
// underlying connection. The blessingsFlow needs to send the blessings via
// the connection and the connection will call writeMsg on the blessings
// flow when data is received for it.
type blessingsCon interface {
	writeEncodedBlessings(ctx *context.T, writer *writer, encoded []byte) error
}

func newBlessingsFlow(conn blessingsCon) *blessingsFlow {
	b := &blessingsFlow{
		conn:    conn,
		nextKey: 1,
	}
	b.encBuf = writeBuffer{conn: conn, wr: &b.writer, buf: make([]byte, 0, 4096)}
	b.dec = vom.NewDecoder(&b.decBuf)
	b.enc = vom.NewEncoder(&b.encBuf)
	b.writer.notify = make(chan struct{})
	return b
}

func (b *blessingsFlow) setPublicKeyBinding(pk security.PublicKey) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.binding = pk
}

func (b *blessingsFlow) writeMsg(buf []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.decBuf.Write(buf)
	return err
}

func validateReceivedBlessings(ctx *context.T, pk security.PublicKey, blessings security.Blessings) error {
	if pk != nil && !reflect.DeepEqual(blessings.PublicKey(), pk) {
		return ErrBlessingsNotBound.Errorf(ctx, "blessings not bound to connection remote public key")
	}
	return nil
}

func (b *blessingsFlow) receiveBlessingsLocked(ctx *context.T, bkey uint64, blessings security.Blessings) error {
	// When accepting, make sure the blessings received are bound to the conn's
	// remote public key.
	if err := validateReceivedBlessings(ctx, b.binding, blessings); err != nil {
		return err
	}
	b.incoming.addBlessings(bkey, blessings)
	return nil
}

func (b *blessingsFlow) receiveDischargesLocked(ctx *context.T, bkey, dkey uint64, discharges []security.Discharge) {
	b.incoming.addDischarges(bkey, dkey, discharges)
}

func (b *blessingsFlow) receiveLocked(ctx *context.T, bd BlessingsFlowMessage) error {
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
// references in the Auth message that terminates the auth handshake.
func (b *blessingsFlow) getRemote(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	for {
		blessings, hasB := b.incoming.hasBlessings(bkey)
		if hasB {
			if dkey == 0 {
				return blessings, nil, nil
			}
			discharges, _, hasD := b.incoming.hasDischarges(dkey)
			if hasD {
				return blessings, dischargeMap(discharges), nil
			}
		}
		var received BlessingsFlowMessage
		if err := b.dec.Decode(&received); err != nil {
			return security.Blessings{}, nil, err
		}
		if err := b.receiveLocked(ctx, received); err != nil {
			return security.Blessings{}, nil, err
		}
	}
}

func (b *blessingsFlow) encodeBlessingsLocked(ctx *context.T, blessings security.Blessings, bkey uint64, peers []security.BlessingPattern) error {
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
	b.mu.Lock()
	defer b.mu.Unlock()
<<<<<<< HEAD

	fmt.Printf("send: %v .. %v\n", blessings, len(discharges))

=======
>>>>>>> cos-cleanup-accept-handshake
	buid := string(blessings.UniqueID())
	bkey, hasB := b.outgoing.hasBlessings(buid)
	if !hasB {
		bkey = b.nextKey
		b.nextKey++
		b.outgoing.addBlessings(buid, bkey, blessings)
		if err := b.encodeBlessingsLocked(ctx, blessings, bkey, peers); err != nil {
			return 0, 0, err
		}
	}
	if len(discharges) == 0 {
		err = b.encBuf.Flush(ctx)
		fmt.Printf("send: 2: %v: %v\n", blessings, err)
		return bkey, 0, err
	}
	dlist, dkey, hasD := b.outgoing.hasDischarges(bkey)
	if hasD && equalDischarges(discharges, dlist) {
		return bkey, dkey, nil
	}

	dlist = dischargeList(discharges)
	dkey = b.nextKey
	b.nextKey++
	b.outgoing.addDischarges(bkey, dkey, dlist)
	if err := b.encodeDischargesLocked(ctx, dlist, bkey, dkey, peers); err != nil {
		return 0, 0, err
	}
	err = b.encBuf.Flush(ctx)
	fmt.Printf("send: 3: %v: %v\n", blessings, err)
	return bkey, dkey, err
}

func dischargeList(in map[string]security.Discharge) []security.Discharge {
	if len(in) == 0 {
		return nil
	}
	out := make([]security.Discharge, 0, len(in))
	for _, d := range in {
		out = append(out, d)
	}
	return out
}
func dischargeMap(in []security.Discharge) map[string]security.Discharge {
	if len(in) == 0 {
		return nil
	}
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
