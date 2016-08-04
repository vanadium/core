// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/x/ref/runtime/internal/flow/crypto"
)

// unsafeUnencrypted allows protocol implementors to provide unencrypted
// protocols.  If the underlying connection implements this method, and
// the method returns true, we do not encrypt.
// This is mostly only used for testing.
type unsafeUnencrypted interface {
	UnsafeDisableEncryption() bool
}

// TODO(mattr): Consider cleaning up the ControlCipher library to
// eliminate extraneous functionality and reduce copying.
type messagePipe struct {
	rw       flow.MsgReadWriteCloser
	cipher   crypto.ControlCipher
	writeBuf []byte
}

func newMessagePipe(rw flow.MsgReadWriteCloser) *messagePipe {
	return &messagePipe{
		rw:       rw,
		writeBuf: make([]byte, defaultMtu),
		cipher:   &crypto.NullControlCipher{},
	}
}

func (p *messagePipe) setupEncryption(ctx *context.T, pk, sk, opk *[32]byte) []byte {
	if uu, ok := p.rw.(unsafeUnencrypted); ok && uu.UnsafeDisableEncryption() {
		return p.cipher.ChannelBinding()
	}
	p.cipher = crypto.NewControlCipherRPC11(
		(*crypto.BoxKey)(pk),
		(*crypto.BoxKey)(sk),
		(*crypto.BoxKey)(opk))
	return p.cipher.ChannelBinding()
}

func (p *messagePipe) writeMsg(ctx *context.T, m message.Message) (err error) {
	// TODO(mattr): Because of the API of the underlying crypto library,
	// an enormous amount of copying happens here.
	// TODO(mattr): We allocate many buffers here to hold potentially
	// many copies of the data.  The maximum memory usage per Conn is probably
	// quite high.  We should try to reduce it.
	if p.writeBuf, err = message.Append(ctx, m, p.writeBuf[:0]); err != nil {
		return err
	}
	if needed := len(p.writeBuf) + p.cipher.MACSize(); cap(p.writeBuf) < needed {
		tmp := make([]byte, needed)
		copy(tmp, p.writeBuf)
		p.writeBuf = tmp
	} else {
		p.writeBuf = p.writeBuf[:needed]
	}
	if err = p.cipher.Seal(p.writeBuf); err != nil {
		return err
	}
	if _, err = p.rw.WriteMsg(p.writeBuf); err != nil {
		return err
	}
	var payload [][]byte
	switch msg := m.(type) {
	case *message.Data:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			payload = msg.Payload
		}
	case *message.OpenFlow:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			payload = msg.Payload
		}
	}
	if payload != nil {
		if _, err = p.rw.WriteMsg(payload...); err != nil {
			return err
		}
	}
	if ctx.V(2) {
		ctx.Infof("Wrote low-level message: %T: %v", m, m)
	}
	return nil
}

func (p *messagePipe) readMsg(ctx *context.T) (message.Message, error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	if !p.cipher.Open(msg) {
		return nil, message.NewErrInvalidMsg(ctx, 0, uint64(len(msg)), 0, nil)
	}
	m, err := message.Read(ctx, msg[:len(msg)-p.cipher.MACSize()])
	var payload []byte
	switch msg := m.(type) {
	case *message.Data:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			payload, err = p.rw.ReadMsg()
			msg.Payload = [][]byte{payload}
		}
	case *message.OpenFlow:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			payload, err = p.rw.ReadMsg()
			msg.Payload = [][]byte{payload}
		}
	}
	if err != nil {
		return nil, err
	}
	if ctx.V(2) {
		ctx.Infof("Read low-level message: %T: %v", m, m)
	}
	return m, err
}
