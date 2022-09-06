// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package aead uses the aead.GCM block cipher with AES 256 encryption.
package aead

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"

	"golang.org/x/crypto/curve25519"
)

// T wraps crypto/cipher to provide synchronized nonces between the requestor
// and responder assuming they interact in a strict request/response manner.
type T struct {
	gcm            cipher.AEAD
	channelBinding [64]byte
	stream         boxStream
}

// NewCipherRPC15 returns a Cipher for RPC versions greater than or equal to 15.
// The cipher used is cipher.AEAD created by cipher.NewGCM with an AES256
// key derived from the private/publick key pairs using ECDH.
func NewCipherRPC15(myPublicKey, myPrivateKey, theirPublicKey *[32]byte) (*T, error) {
	var c T
	// ECDH multiplication of local private and remote private key to obtain
	// shared session key.
	key, err := curve25519.X25519(myPrivateKey[:], theirPublicKey[:])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.gcm, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if bytes.Compare(myPublicKey[:], theirPublicKey[:]) < 0 {
		c.stream.InitDirection(true)
		copy(c.channelBinding[:], (*myPublicKey)[:])
		copy(c.channelBinding[32:], (*theirPublicKey)[:])
	} else {
		c.stream.InitDirection(false)
		copy(c.channelBinding[:], (*theirPublicKey)[:])
		copy(c.channelBinding[32:], (*myPublicKey)[:])
	}
	return &c, nil
}

func (c *T) Overhead() int {
	return c.gcm.Overhead()
}

func (c *T) Seal(buf, data []byte) ([]byte, error) {
	ret := c.gcm.Seal(buf, c.stream.SealNonce(), data, nil)
	c.stream.SealAdvance()
	return ret, nil
}

func (c *T) Open(buf, data []byte) ([]byte, bool) {
	ret, err := c.gcm.Open(buf, c.stream.OpenNonce(), data, nil)
	if err != nil {
		// Return without advancing the nonce so that the stream remains in sync.
		return nil, false
	}
	c.stream.OpenAdvance()
	return ret, true
}

func (c *T) ChannelBinding() []byte {
	return c.channelBinding[:]
}

// boxStream implements nonce management for a cipher stream.
type boxStream struct {
	sealCounter uint64
	sealNonce   [12]byte
	openCounter uint64
	openNonce   [12]byte
}

// InitDirection must be called to correctly initialize the stream nonces.
// The stream is full-duplex, and we want the directions to use different
// nonces, so we set bit (1 << 64) in one stream, and leave it cleared in
// the other server stream. Advance touches only the first 8 bytes,
// so this change is permanent for the duration of the stream.
func (s *boxStream) InitDirection(seal bool) {
	if seal {
		s.sealNonce[8] = 1
		return
	}
	s.openNonce[8] = 1
}

func (s *boxStream) SealNonce() []byte {
	return s.sealNonce[:]
}

func (s *boxStream) SealAdvance() {
	s.sealCounter++
	binary.LittleEndian.PutUint64(s.sealNonce[:], s.sealCounter)
}

func (s *boxStream) OpenNonce() []byte {
	return s.openNonce[:]
}

func (s *boxStream) OpenAdvance() {
	s.openCounter++
	binary.LittleEndian.PutUint64(s.openNonce[:], s.openCounter)
}
