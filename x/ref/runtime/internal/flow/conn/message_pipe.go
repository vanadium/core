// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/rpc/version"
	"v.io/x/ref/runtime/internal/flow/cipher/aead"
	"v.io/x/ref/runtime/internal/flow/cipher/naclbox"
)

// unsafeUnencrypted allows protocol implementors to provide unencrypted
// protocols.  If the underlying connection implements this method, and
// the method returns true and encryption is disabled even if enableEncryption
// is called. This is only used for testing and in particular by the debug
// protocol.
type unsafeUnencrypted interface {
	UnsafeDisableEncryption() bool
}

func newMessagePipe(rw flow.MsgReadWriteCloser) *messagePipe {
	return &messagePipe{
		rw: rw,
		// On the write side it's safe to use a passthrough
		// like this for when encryption is not enabled.
		// This is not true for reading where the buffer management
		// is more involved.
		seal: func(out, data []byte) ([]byte, error) {
			return data, nil
		},
	}
}

type sealFunc func(out, data []byte) ([]byte, error)
type openFunc func(out, data []byte) ([]byte, bool)

// messagePipe implements messagePipe for RPC11 version and beyond.
type messagePipe struct {
	rw   flow.MsgReadWriteCloser
	seal sealFunc
	open openFunc
}

func (p *messagePipe) flow() flow.MsgReadWriteCloser {
	return p.rw
}

// enableEncryption enables encryption on the pipe (unless the underlying
// transport reader implements UnsafeDisableEncryption and that
// implementatio nreturns true).
func (p *messagePipe) enableEncryption(ctx *context.T, publicKey, secretKey, remotePublicKey *[32]byte, rpcversion version.RPCVersion) ([]byte, error) {
	if uu, ok := p.rw.(unsafeUnencrypted); ok && uu.UnsafeDisableEncryption() {
		return nil, nil
	}
	switch {
	case rpcversion >= version.RPCVersion11 && rpcversion < version.RPCVersion15:
		cipher, err := naclbox.NewCipher(publicKey, secretKey, remotePublicKey)
		if err != nil {
			return nil, err
		}
		p.seal = cipher.Seal
		p.open = cipher.Open
		return cipher.ChannelBinding(), nil
	case rpcversion == version.RPCVersion15:
		cipher, err := aead.NewCipher(publicKey, secretKey, remotePublicKey)
		if err != nil {
			return nil, err
		}
		p.seal = cipher.Seal
		p.open = cipher.Open
		return cipher.ChannelBinding(), nil
	}
	return nil, ErrRPCVersionMismatch.Errorf(ctx, "conn.message_pipe: %v is not supported", rpcversion)
}

func (p *messagePipe) writeMsg(ctx *context.T, m message.Message) (err error) {
	writePoolBuf := messagePipeWritePool.Get().(*writeBuffers)
	defer messagePipeWritePool.Put(writePoolBuf)

	plaintext, err := message.Append(ctx, m, writePoolBuf.plaintext[:0])
	if err != nil {
		return err
	}
	enc, err := p.seal(writePoolBuf.ciphertext[:0], plaintext)
	if err != nil {
		return err
	}
	if _, err = p.rw.WriteMsg(enc); err != nil {
		return err
	}

	// Handle plaintext payloads which are not serialized by message.Append
	// above and are instead written separately in the clear.
	if payload := hasPlaintextPayload(m); payload != nil {
		if _, err = p.rw.WriteMsg(payload...); err != nil {
			return err
		}
	}
	if ctx.V(2) {
		ctx.Infof("Wrote low-level message: %T: %v", m, m)
	}
	return nil
}

func (p *messagePipe) readMsg(ctx *context.T, plaintextBuf []byte) (message.Message, error) {
	var m message.Message
	var plaintext []byte
	var err error
	if p.open == nil {
		// For the plaintext case we can use the supplied buffer to read from
		// the underlying connection directly.
		plaintext, err = p.rw.ReadMsg2(plaintextBuf)
		if err != nil {
			return nil, err
		}
		m, err = message.Read(ctx, plaintext)
	} else {
		// For the encrypted case we need to allocate a new buffer for the
		// ciphertext that is only used temporarily.
		readBuf := messagePipeReadPool.Get().(*[]byte)
		defer messagePipeReadPool.Put(readBuf)
		var ciphertext []byte
		ciphertext, err = p.rw.ReadMsg2((*readBuf)[:])
		if err != nil {
			return nil, err
		}
		plaintext, ok := p.open(plaintextBuf, ciphertext)
		if !ok {
			return nil, message.NewErrInvalidMsg(ctx, 0, uint64(len(ciphertext)), 0, nil)
		}
		m, err = message.Read(ctx, plaintext)
	}
	if err != nil {
		return nil, err
	}

	if needsPlaintextPayload(m) {
		// todo: test this carefully.... want to reuse buffer for plaintext
		// payload.
		payload, err := p.rw.ReadMsg2(plaintextBuf[len(plaintext):])
		if err != nil {
			return nil, err
		}
		setPlaintextPayload(m, payload)
	}

	if ctx.V(2) {
		ctx.Infof("Read low-level message: %T: %v", m, m)
	}
	return m, err
}

func hasPlaintextPayload(m message.Message) [][]byte {
	switch msg := m.(type) {
	case *message.Data:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			return msg.Payload
		}
	case *message.OpenFlow:
		if msg.Flags&message.DisableEncryptionFlag != 0 {
			return msg.Payload
		}
	}
	return nil
}

func needsPlaintextPayload(m message.Message) bool {
	switch msg := m.(type) {
	case *message.Data:
		return msg.Flags&message.DisableEncryptionFlag != 0
	case *message.OpenFlow:
		return msg.Flags&message.DisableEncryptionFlag != 0
	}
	return false
}

func setPlaintextPayload(m message.Message, payload []byte) {
	switch msg := m.(type) {
	case *message.Data:
		msg.Payload = [][]byte{payload}
	case *message.OpenFlow:
		msg.Payload = [][]byte{payload}
	}
}
