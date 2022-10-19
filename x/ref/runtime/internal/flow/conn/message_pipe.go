// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/rpc/version"
	"v.io/x/ref/runtime/internal/flow/cipher/aead"
	"v.io/x/ref/runtime/internal/flow/cipher/naclbox"
	"v.io/x/ref/runtime/protocols/lib/framer"
)

// unsafeUnencrypted allows protocol implementors to provide unencrypted
// protocols.  If the underlying connection implements this method, and
// the method returns true and encryption is disabled even if enableEncryption
// is called. This is only used for testing and in particular by the debug
// protocol.
type unsafeUnencrypted interface {
	UnsafeDisableEncryption() bool
}

// newMessagePipe returns a new messagePipe instance that may create its
// own frames on the write path if the supplied MsgReadWriteCloser implements
// framing.T. This offers a significant speedup (half the number of system calls)
// and reduced memory usage and associated allocations.
func newMessagePipe(rw flow.MsgReadWriteCloser) *messagePipe {
	if bypass, ok := rw.(framer.T); ok {
		return &messagePipe{
			rw:          rw,
			framer:      bypass,
			frameOffset: bypass.FrameHeaderSize(),
		}
	}
	return &messagePipe{
		rw: rw,
	}
}

// newMessagePipeUseFramer is like newMessagePipe but will always use
// an external framer, it's included primarily for tests.
func newMessagePipeUseFramer(rw flow.MsgReadWriteCloser) *messagePipe {
	return &messagePipe{
		rw: rw,
	}
}

type sealFunc func(out, data []byte) ([]byte, error)
type openFunc func(out, data []byte) ([]byte, bool)

// messagePipe implements messagePipe for RPC11 version and beyond.
type messagePipe struct {
	rw          flow.MsgReadWriteCloser
	framer      framer.T
	frameOffset int

	seal sealFunc
	open openFunc

	// locks are required to serialize access to the read/write operations since
	// the messagePipe may be called by different goroutines when connections
	// are being created or because of the need to send changed blessings
	// asynchronously. Other than these cases there will be no lock
	// contention.
	readMu, writeMu sync.Mutex
}

func (p *messagePipe) isEncapsulated() bool {
	_, ok := p.rw.(*flw)
	return ok
}

func (p *messagePipe) disableEncryptionOnEncapsulatedFlow() {
	if f, ok := p.rw.(*flw); ok {
		f.disableEncryption()
	}
}

func (p *messagePipe) Close() error {
	return p.rw.Close()
}

// enableEncryption enables encryption on the pipe (unless the underlying
// transport reader implements UnsafeDisableEncryption and that
// implementatio nreturns true). The encryption used depends on the RPC version
// being used.
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
	case rpcversion >= version.RPCVersion15:
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

func usedOurBuffer(x, y []byte) bool {
	return &x[0:cap(x)][cap(x)-1] == &y[0:cap(y)][cap(y)-1]
}

type serialize func(ctx *context.T, buf []byte) ([]byte, error)

func (p *messagePipe) createCiphertext(ctx *context.T, fn serialize, plaintextBuf, ciphertextBuf []byte) (wire, framed []byte, err error) {
	if p.seal == nil {
		wire, err = fn(ctx, plaintextBuf[p.frameOffset:p.frameOffset])
		framed = plaintextBuf
		return
	}
	plaintext, err := fn(ctx, plaintextBuf[:0])
	if err != nil {
		return
	}
	wire, err = p.seal(ciphertextBuf[p.frameOffset:p.frameOffset], plaintext)
	framed = ciphertextBuf
	return
}

func (p *messagePipe) writeCiphertext(ctx *context.T, fn serialize, plaintextBuf, ciphertextBuf []byte) error {
	wire, framedWire, err := p.createCiphertext(ctx, fn, plaintextBuf, ciphertextBuf)
	if err != nil {
		return err
	}
	if p.frameOffset > 0 && usedOurBuffer(framedWire, wire) {
		// Write the frame size directly into the buffer we allocated and then
		// write out that buffer in a single write operation.
		if err := p.framer.PutSize(framedWire[:p.frameOffset], len(wire)); err != nil {
			return err
		}
		if _, err := p.framer.Write(framedWire[:len(wire)+p.frameOffset]); err != nil {
			return err
		}
		return nil
	}
	// NOTE that in the case where p.frameOffset > 0 but the returned buffer
	// differs from the one passed in, p.frameOffset bytes are wasted in the
	// buffers used here.
	_, err = p.rw.WriteMsg(wire)
	return err
}

func (p *messagePipe) getBuffers() (*[]byte, *[]byte) {
	return messagePipePool.Get().(*[]byte), messagePipePool.Get().(*[]byte)
}

func (p *messagePipe) putBuffers(ptext, ctext *[]byte) {
	messagePipePool.Put(ptext)
	messagePipePool.Put(ctext)
}

// Handle plaintext payloads which are not serialized by message.Append
// above and are instead written separately in the clear.
func (p *messagePipe) handlePlaintextPayload(flags uint64, payload [][]byte) error {
	if flags&message.DisableEncryptionFlag != 0 {
		if _, err := p.rw.WriteMsg(payload...); err != nil {
			return err
		}
	}
	return nil
}

func (p *messagePipe) writeData(ctx *context.T, m message.Data) error {
	plaintextBuf, ciphertextBuf := p.getBuffers()
	defer p.putBuffers(plaintextBuf, ciphertextBuf)
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := p.writeCiphertext(ctx, m.Append, *plaintextBuf, *ciphertextBuf); err != nil {
		return err
	}
	return p.handlePlaintextPayload(m.Flags, m.Payload)
}

func (p *messagePipe) writeOpenFlow(ctx *context.T, m message.OpenFlow) error {
	plaintextBuf, ciphertextBuf := p.getBuffers()
	defer p.putBuffers(plaintextBuf, ciphertextBuf)
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := p.writeCiphertext(ctx, m.Append, *plaintextBuf, *ciphertextBuf); err != nil {
		return err
	}
	return p.handlePlaintextPayload(m.Flags, m.Payload)
}

func (p *messagePipe) writeRelease(ctx *context.T, m message.Release) error {
	plaintextBuf, ciphertextBuf := p.getBuffers()
	defer p.putBuffers(plaintextBuf, ciphertextBuf)
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.writeCiphertext(ctx, m.Append, *plaintextBuf, *ciphertextBuf)
}

func (p *messagePipe) writeMsg(ctx *context.T, fn serialize) error {
	plaintextBuf, ciphertextBuf := p.getBuffers()
	defer p.putBuffers(plaintextBuf, ciphertextBuf)
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.writeCiphertext(ctx, fn, *plaintextBuf, *ciphertextBuf)
}

func (p *messagePipe) readClearText(ctx *context.T, plaintextBuf []byte) ([]byte, error) {
	p.readMu.Lock()
	defer p.readMu.Unlock()
	if p.open == nil {
		return p.rw.ReadMsg2(plaintextBuf)
	}
	ciphertextBuf := messagePipePool.Get().(*[]byte)
	defer messagePipePool.Put(ciphertextBuf)
	ciphertext, err := p.rw.ReadMsg2(*ciphertextBuf)
	if err != nil {
		return nil, err
	}
	plaintext, ok := p.open(plaintextBuf[:0], ciphertext)
	if !ok {
		return nil, message.NewErrInvalidMsg(ctx, 0, uint64(len(ciphertext)), 0, nil)
	}
	return plaintext, nil
}

func (p *messagePipe) readMsg(ctx *context.T, plaintextBuf []byte) (message.Message, error) {
	plaintext, err := p.readClearText(ctx, plaintextBuf)
	if err != nil {
		return nil, err
	}
	if len(plaintext) == 0 {
		return nil, message.NewErrInvalidMsg(ctx, message.InvalidType, 0, 0, nil)
	}
	return p.readAnyMessage(ctx, plaintext)
}

func (p *messagePipe) readAnyMessage(ctx *context.T, plaintext []byte) (message.Message, error) {
	msgType, from := plaintext[0], plaintext[1:]
	switch msgType {
	case message.DataType:
		return p.handleData(ctx, from)
	case message.OpenFlowType:
		return p.handleOpenFlow(ctx, from)
	}
	return message.ReadNoPayload(ctx, plaintext)
}

func (p *messagePipe) handleData(ctx *context.T, from []byte) (message.Message, error) {
	m, err := message.Data{}.Read(ctx, from)
	if err != nil {
		return nil, err
	}
	msg := m.(message.Data)
	if msg.Flags&message.DisableEncryptionFlag == 0 {
		return m, nil
	}
	payload, err := p.rw.ReadMsg2(nil)
	if err != nil {
		return nil, err
	}
	msg.Payload = [][]byte{payload}
	msg = msg.SetNoCopy(true)
	return msg, nil
}

func (p *messagePipe) handleOpenFlow(ctx *context.T, from []byte) (message.Message, error) {
	m, err := message.OpenFlow{}.Read(ctx, from)
	if err != nil {
		return nil, err
	}
	msg := m.(message.OpenFlow)
	if msg.Flags&message.DisableEncryptionFlag == 0 {
		return m, nil
	}
	payload, err := p.rw.ReadMsg2(nil)
	if err != nil {
		return nil, err
	}
	msg.Payload = [][]byte{payload}
	msg = msg.SetNoCopy(true)
	return msg, nil
}
