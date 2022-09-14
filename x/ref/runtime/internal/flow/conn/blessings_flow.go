// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
	iflow "v.io/x/ref/runtime/internal/flow"
)

func (c *Conn) newFlowForBlessingsLocked(ctx *context.T) *flw {
	// TODO(cnicolaou): create a simplified flow implementation for
	// blessings as follows:
	//   - flow control is essentially disabled for these flows, so
	//     there's no need for a readq
	//   - there is only ever one writer so there's no need for the
	//     writerq/writech.
	//   - the flow's are always pre-opened and never closed so no
	//     need for state to track that.
	f := &flw{
		id:          blessingsFlowID,
		conn:        c,
		opened:      true,
		borrowing:   true,
		sideChannel: true,
		writeCh:     make(chan struct{}, 1),
	}
	f.q = newReadQ(f.release)
	f.next, f.prev = f, f
	f.ctx, f.cancel = context.WithCancel(ctx)
	c.flows[f.id] = f
	f.releaseLocked(DefaultBytesBufferedPerFlow)
	return f
}

// writeBuffer is used to coalesce the many small writes issued by the vom
// encoder into a single larger one to reduce the number of system calls
// and network packets sent.
type writeBuffer struct {
	*flw
	buf []byte
}

func (w *writeBuffer) Write(buf []byte) (int, error) {
	w.buf = append(w.buf, buf...)
	return len(buf), nil
}

func (w *writeBuffer) Flush() (int, error) {
	if len(w.buf) == 0 {
		return 0, nil
	}
	n, err := w.flw.Write(w.buf)
	// can free the buffer since it's unlikely that a second set of
	// blessings or discharges will be sent.
	w.buf = nil
	return n, err
}

type blessingsFlow struct {
	mu sync.Mutex

	f      *flw
	dec    *vom.Decoder
	enc    *vom.Encoder
	encBuf writeBuffer

	nextKey  uint64
	incoming inCache
	outgoing outCache
}

func newBlessingsFlow(f *flw) *blessingsFlow {
	b := &blessingsFlow{
		f:       f,
		nextKey: 1,
		dec:     vom.NewDecoder(f),
		encBuf:  writeBuffer{flw: f, buf: make([]byte, 0, 4096)},
	}
	b.enc = vom.NewEncoder(&b.encBuf)
	return b
}

func (b *blessingsFlow) receiveBlessingsLocked(ctx *context.T, bkey uint64, blessings security.Blessings) error {
	b.f.useCurrentContext(ctx)
	// When accepting, make sure the blessings received are bound to the conn's
	// remote public key.
	if err := b.f.validateReceivedBlessings(ctx, blessings); err != nil {
		return err
	}
	b.incoming.addBlessings(bkey, blessings)
	return nil
}

func (b *blessingsFlow) receiveDischargesLocked(ctx *context.T, bkey, dkey uint64, discharges []security.Discharge) {
	b.incoming.addDischarges(bkey, dkey, discharges)
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
// references in the Auth message that terminates the auth handshake.
func (b *blessingsFlow) getRemote(ctx *context.T, bkey, dkey uint64) (security.Blessings, map[string]security.Discharge, error) {
	defer b.mu.Unlock()
	b.mu.Lock()
	b.f.useCurrentContext(ctx)
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
			b.f.internalClose(ctx, false, false, err)
			return security.Blessings{}, nil, err
		}
	}
}

func (b *blessingsFlow) encodeBlessingsLocked(ctx *context.T, enc *vom.Encoder, blessings security.Blessings, bkey uint64, peers []security.BlessingPattern) error {
	b.f.useCurrentContext(ctx)
	if len(peers) == 0 {
		// blessings can be encoded in plaintext
		return enc.Encode(BlessingsFlowMessageBlessings{Blessings{
			BKey:      bkey,
			Blessings: blessings,
		}})
	}
	ciphertexts, err := encrypt(ctx, peers, blessings)
	if err != nil {
		return ErrCannotEncryptBlessings.Errorf(ctx, "cannot encrypt blessings for peer: %v: %v", peers, err)
	}
	return enc.Encode(BlessingsFlowMessageEncryptedBlessings{EncryptedBlessings{
		BKey:        bkey,
		Ciphertexts: ciphertexts,
	}})
}

func (b *blessingsFlow) encodeDischargesLocked(ctx *context.T, enc *vom.Encoder, discharges []security.Discharge, bkey, dkey uint64, peers []security.BlessingPattern) error {
	b.f.useCurrentContext(ctx)
	if len(peers) == 0 {
		// discharges can be encoded in plaintext
		return enc.Encode(BlessingsFlowMessageDischarges{Discharges{
			Discharges: discharges,
			DKey:       dkey,
			BKey:       bkey,
		}})
	}
	ciphertexts, err := encrypt(ctx, peers, discharges)
	if err != nil {
		return ErrCannotEncryptDischarges.Errorf(ctx, "cannot encrypt discharges for peers: %v: %v", peers, err)
	}
	return enc.Encode(BlessingsFlowMessageEncryptedDischarges{EncryptedDischarges{
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
	b.f.useCurrentContext(ctx)

	buid := string(blessings.UniqueID())
	bkey, hasB := b.outgoing.hasBlessings(buid)
	if !hasB {
		bkey = b.nextKey
		b.nextKey++
		b.outgoing.addBlessings(buid, bkey, blessings)
		if err := b.encodeBlessingsLocked(ctx, b.enc, blessings, bkey, peers); err != nil {
			return 0, 0, err
		}
	}
	if len(discharges) == 0 {
		_, err = b.encBuf.Flush()
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
	if err := b.encodeDischargesLocked(ctx, b.enc, dlist, bkey, dkey, peers); err != nil {
		return 0, 0, err
	}
	_, err = b.encBuf.Flush()
	return bkey, dkey, err
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
