// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/salsa20/salsa"
)

// cbox implements a ControlCipher using go.crypto/nacl/box.
type cbox struct {
	sharedKey      [32]byte
	channelBinding []byte
	enc            cboxStream
	dec            cboxStream
}

// cboxStream implements one stream of encryption or decryption.
type cboxStream struct {
	counter uint64
	nonce   [24]byte
	// buffer is a temporary used for in-place crypto.
	buffer []byte
}

const (
	cboxMACSize = box.Overhead
)

type BoxKey [32]byte

func (s *cboxStream) alloc(n int) []byte {
	if len(s.buffer) < n {
		s.buffer = make([]byte, n*2)
	}
	return s.buffer[:0]
}

func (s *cboxStream) currentNonce() *[24]byte {
	return &s.nonce
}

func (s *cboxStream) advanceNonce() {
	s.counter++
	binary.LittleEndian.PutUint64(s.nonce[:], s.counter)
}

// setupXSalsa20 produces a sub-key and Salsa20 counter given a nonce and key.
//
// See, "Extending the Salsa20 nonce," by Daniel J. Bernsten, Department of
// Computer Science, University of Illinois at Chicago, 2008.
func setupXSalsa20(subKey *[32]byte, counter *[16]byte, nonce *[24]byte, key *[32]byte) {
	// We use XSalsa20 for encryption so first we need to generate a
	// key and nonce with HSalsa20.
	var hNonce [16]byte
	copy(hNonce[:], nonce[:])
	salsa.HSalsa20(subKey, &hNonce, key, &salsa.Sigma)

	// The final 8 bytes of the original nonce form the new nonce.
	copy(counter[:], nonce[16:])
}

// NewControlCipher returns a ControlCipher for RPC versions greater than or equal to 11.
func NewControlCipherRPC11(myPublicKey, myPrivateKey, theirPublicKey *BoxKey) ControlCipher {
	var c cbox
	box.Precompute(&c.sharedKey, (*[32]byte)(theirPublicKey), (*[32]byte)(myPrivateKey))
	// The stream is full-duplex, and we want the directions to use different
	// nonces, so we set bit (1 << 64) in one stream, and leave it cleared in
	// the other server stream. advanceNone touches only the first 8 bytes,
	// so this change is permanent for the duration of the stream.
	if bytes.Compare(myPublicKey[:], theirPublicKey[:]) < 0 {
		c.enc.nonce[8] = 1
		//nolint:gocritic // append result not assigned to the same slice
		c.channelBinding = append(myPublicKey[:], theirPublicKey[:]...)
	} else {
		c.dec.nonce[8] = 1
		//nolint:gocritic // append result not assigned to the same slice
		c.channelBinding = append(theirPublicKey[:], myPublicKey[:]...)
	}
	return &c
}

// MACSize implements the ControlCipher method.
func (c *cbox) MACSize() int {
	return cboxMACSize
}

// Seal implements the ControlCipher method.
func (c *cbox) Seal(data []byte) error {
	n := len(data)
	if n < cboxMACSize {
		return fmt.Errorf("control cipher: message is too short")
	}
	tmp := c.enc.alloc(n)
	nonce := c.enc.currentNonce()
	out := box.SealAfterPrecomputation(tmp, data[:n-cboxMACSize], nonce, &c.sharedKey)
	c.enc.advanceNonce()
	copy(data, out)
	return nil
}

// Open implements the ControlCipher method.
func (c *cbox) Open(data []byte) bool {
	n := len(data)
	if n < cboxMACSize {
		return false
	}
	tmp := c.dec.alloc(n - cboxMACSize)
	nonce := c.dec.currentNonce()
	out, ok := box.OpenAfterPrecomputation(tmp, data, nonce, &c.sharedKey)
	if !ok {
		return false
	}
	c.dec.advanceNonce()
	copy(data, out)
	return true
}

// Encrypt implements the ControlCipher method.
func (c *cbox) Encrypt(data []byte) {
	var subKey [32]byte
	var counter [16]byte
	nonce := c.enc.currentNonce()
	setupXSalsa20(&subKey, &counter, nonce, &c.sharedKey)
	c.enc.advanceNonce()
	salsa.XORKeyStream(data, data, &counter, &subKey)
}

// Decrypt implements the ControlCipher method.
func (c *cbox) Decrypt(data []byte) {
	var subKey [32]byte
	var counter [16]byte
	nonce := c.dec.currentNonce()
	setupXSalsa20(&subKey, &counter, nonce, &c.sharedKey)
	c.dec.advanceNonce()
	salsa.XORKeyStream(data, data, &counter, &subKey)
}

func (c *cbox) ChannelBinding() []byte {
	return c.channelBinding
}
