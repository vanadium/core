// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crypto_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"golang.org/x/crypto/nacl/box"

	"v.io/x/ref/runtime/internal/flow/crypto"
)

// Add space for a MAC.
func newMessage(s string) []byte {
	b := make([]byte, len(s)+box.Overhead)
	copy(b, s)
	return b
}

type testCipherVersion int

const (
	cipherRPC11 testCipherVersion = iota
)

func newCipher(ver testCipherVersion) (c1, c2 crypto.ControlCipher, err error) {
	pk1, sk1, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, errors.New("can't generate key")
	}
	pk2, sk2, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, errors.New("can't generate key")
	}
	if ver == cipherRPC11 {
		c1 = crypto.NewControlCipherRPC11((*crypto.BoxKey)(pk1), (*crypto.BoxKey)(sk1), (*crypto.BoxKey)(pk2))
		c2 = crypto.NewControlCipherRPC11((*crypto.BoxKey)(pk2), (*crypto.BoxKey)(sk2), (*crypto.BoxKey)(pk1))
	}
	return
}

func testCipherOpenSeal(t *testing.T, ver testCipherVersion) { //nolint:gocyclo
	c1, c2, err := newCipher(ver)
	if err != nil {
		t.Fatalf("can't create cipher: %v", err)
	}

	msg1 := newMessage("hello")
	if err := c1.Seal(msg1); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	msg2 := newMessage("world")
	if err := c1.Seal(msg2); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	msg3 := newMessage("hello")
	if err := c1.Seal(msg3); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if bytes.Equal(msg1, msg3) {
		t.Errorf("message should differ: %q, %q", msg1, msg3)
	}

	// Check that the client does not encrypt the same.
	msg4 := newMessage("hello")
	if err := c2.Seal(msg4); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if bytes.Equal(msg4, msg1) {
		t.Errorf("messages should differ %q vs. %q", msg4, msg1)
	}

	// Corrupted message should not decrypt.
	msg1[0] ^= 1
	if ok := c2.Open(msg1); ok {
		t.Errorf("expected error")
	}

	// Fix the message and try again.
	msg1[0] ^= 1
	if ok := c2.Open(msg1); !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(msg1[:5], []byte("hello")) {
		t.Errorf("got %q, expected %q", msg1[:5], "hello")
	}

	// msg3 should not decrypt.
	if ok := c2.Open(msg3); ok {
		t.Errorf("expected error")
	}

	// Resume.
	if ok := c2.Open(msg2); !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(msg2[:5], []byte("world")) {
		t.Errorf("got %q, expected %q", msg2[:5], "world")
	}
	if ok := c2.Open(msg3); !ok {
		t.Errorf("Open failed")
	}
	if !bytes.Equal(msg3[:5], []byte("hello")) {
		t.Errorf("got %q, expected %q", msg3[:5], "hello")
	}
}
func TestCipherOpenSealRPC11(t *testing.T) { testCipherOpenSeal(t, cipherRPC11) }

func testCipherXORKeyStream(t *testing.T, ver testCipherVersion) {
	c1, c2, err := newCipher(ver)
	if err != nil {
		t.Fatalf("can't create cipher: %v", err)
	}

	msg1 := []byte("hello")
	msg2 := []byte("world")
	msg3 := []byte("hello")
	c1.Encrypt(msg1)
	c1.Encrypt(msg2)
	c1.Encrypt(msg3)
	if bytes.Equal(msg1, msg3) {
		t.Errorf("messages should differ: %q, %q", msg1, msg3)
	}

	c2.Decrypt(msg1)
	c2.Decrypt(msg2)
	c2.Decrypt(msg3)
	s1 := string(msg1)
	s2 := string(msg2)
	s3 := string(msg3)
	if s1 != "hello" {
		t.Errorf("got %q, expected 'hello'", s1)
	}
	if s2 != "world" {
		t.Errorf("got %q, expected 'world'", s2)
	}
	if s3 != "hello" {
		t.Errorf("got %q, expected 'hello'", s3)
	}
}
func TestCipherXORKeyStreamRPC11(t *testing.T) { testCipherXORKeyStream(t, cipherRPC11) }

func TestCipherChannelBinding(t *testing.T) {
	values := make([][]byte, 100)
	for i := 0; i < len(values); i++ {
		c1, c2, err := newCipher(cipherRPC11)
		if err != nil {
			t.Fatalf("can't create cipher: %v", err)
		}
		if !bytes.Equal(c1.ChannelBinding(), c2.ChannelBinding()) {
			t.Fatalf("Two ends of the crypter ended up with different channel bindings (iteration #%d)", i)
		}
		values[i] = c1.ChannelBinding()
	}
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if bytes.Equal(values[i], values[j]) {
				t.Fatalf("Same ChannelBinding seen on multiple channels (%d and %d)", i, j)
			}
		}
	}
}
