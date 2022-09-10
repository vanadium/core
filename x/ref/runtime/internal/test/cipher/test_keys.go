// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipher

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/box"
	"v.io/x/ref/runtime/internal/flow/cipher"
	"v.io/x/ref/runtime/internal/flow/cipher/aead"
	"v.io/x/ref/runtime/internal/flow/cipher/naclbox"
)

// API provides an API for testing ciphers used for encrypting
// messages sent by message pipe implementations. This interface is used
// solely for testing and is intended to allow for a single set of tests to
// be used for multiple cipher implementations in the future.
type API interface {
	// Seal returns an authenticated and encrypted version of the supplied
	// data. Seal appends the encrypted data to the supplied buf and the
	// the returned slice points to the newly encrypted data that can be used
	// as the argument to Open (it includes he MAC).
	Seal(buf, data []byte) ([]byte, error)

	// Open authenticates and decrypts a box produced by Seal and returns true
	// on success. The decrypted data is appended to buf and the returned slice
	// points to the newly decrypted data.
	Open(buf, data []byte) ([]byte, bool)

	// ChannelBinding Returns a byte slice that is unique for the the
	// particular cipher (and the parties between which it is operating).
	// Having both parties assert out of the band that they are indeed
	// participating in a connection with that channel binding value is
	// sufficient to authenticate the data received through the cipher.
	ChannelBinding() []byte

	// Overhead is the max difference between the plaintext and ciphertext sizes.
	Overhead() int
}

func NewRPC11Keys() (pk1, sk1, pk2, sk2 *[32]byte, err error) {
	pk1, sk1, err = box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	pk2, sk2, err = box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	return
}

func NewRPC11Ciphers() (c1, c2 API, err error) {
	pk1, sk1, pk2, sk2, err := NewRPC11Keys()
	c1, err = naclbox.NewCipher(pk1, sk1, pk2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	c2, err = naclbox.NewCipher(pk2, sk2, pk1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	return
}

func NewRPC15Keys() (pk1, sk1, pk2, sk2 *[32]byte, err error) {
	pk1, sk1, err = cipher.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	pk2, sk2, err = cipher.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	return
}

func NewRPC15Ciphers() (c1, c2 API, err error) {
	pk1, sk1, pk2, sk2, err := NewRPC15Keys()
	c1, err = aead.NewCipher(pk1, sk1, pk2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	c2, err = aead.NewCipher(pk2, sk2, pk1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	return
}

func NewMixedKeys() (pk1, sk1, pk2, sk2 *[32]byte, err error) {
	// Test with keys generated by box.GenerateKey and the
	// internal cipher.GenerateKey.
	pk1, sk1, err = box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	pk2, sk2, err = cipher.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("can't generate key")
	}
	return
}

func NewMixedCiphers() (c1, c2 API, err error) {
	// Test with keys generated by box.GenerateKey and the
	// internal cipher.GenerateKey.
	pk1, sk1, pk2, sk2, err := NewMixedKeys()
	if err != nil {
		return nil, nil, fmt.Errorf("can't generate key")
	}
	c1, err = naclbox.NewCipher(pk1, sk1, pk2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	c2, err = naclbox.NewCipher(pk2, sk2, pk1)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %v", err)
	}
	return
}
