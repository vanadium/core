// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naclbox

import (
	"bytes"
	"encoding/binary"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/salsa20/salsa"
)

// T wraps go.crypto/nacl/{box,secretbox} to provide synchronized
// nonces between the requestor and responder assuming they interact in a
// strict request/response manner.
type T struct {
	sharedKey      [32]byte
	channelBinding [64]byte
	stream         boxStream
}

var zeros [16]byte

func precompute(sharedKey, peersPublicKey, privateKey *[32]byte) error {
	// ECDH multipilication, but avoiding the deprecated curve25519.ScalarMult
	// used in box.Precompute.
	key, err := curve25519.X25519(privateKey[:], peersPublicKey[:])
	if err != nil {
		return err
	}
	copy(sharedKey[:], key)
	// Derive the key to use with the nacl/secretbox cipher.
	salsa.HSalsa20(sharedKey, &zeros, sharedKey, &salsa.Sigma)
	return nil
}

// NewCipherRPC11 returns a Cipher for RPC versions greater than or equal to 11.
// The underlying cipher is nacl/secretbox.
func NewCipherRPC11(myPublicKey, myPrivateKey, theirPublicKey *[32]byte) (*T, error) {
	var c T
	if err := precompute(&c.sharedKey, theirPublicKey, myPrivateKey); err != nil {
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
	return box.Overhead
}

func (c *T) Seal(buf, data []byte) ([]byte, error) {
	ret := secretbox.Seal(buf, data, c.stream.SealNonce(), &c.sharedKey)
	c.stream.SealAdvance()
	return ret, nil
}

func (c *T) Open(buf, data []byte) ([]byte, bool) {
	ret, ok := secretbox.Open(buf, data, c.stream.OpenNonce(), &c.sharedKey)
	if !ok {
		// Return without advancing the nonce so that the enc/dec
		// remain in sync.
		return nil, false
	}
	c.stream.OpenAdvance()
	return ret, ok
}

func (c *T) ChannelBinding() []byte {
	return c.channelBinding[:]
}

// boxStream implements nonce management for a cipher stream.
type boxStream struct {
	sealCounter uint64
	sealNonce   [24]byte
	openCounter uint64
	openNonce   [24]byte
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

func (s *boxStream) SealNonce() *[24]byte {
	return &s.sealNonce
}

func (s *boxStream) SealAdvance() {
	s.sealCounter++
	binary.LittleEndian.PutUint64(s.sealNonce[:], s.sealCounter)
}

func (s *boxStream) OpenNonce() *[24]byte {
	return &s.openNonce
}

func (s *boxStream) OpenAdvance() {
	s.openCounter++
	binary.LittleEndian.PutUint64(s.openNonce[:], s.openCounter)
}
