// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

type bmkey struct {
	signer    Signer
	signature Signature
}

var (
	ecdsaKey   *bmkey
	ed25519Key *bmkey
	message    = []byte("over the mountain and under the bridge")
	purpose    = []byte("benchmarking")
)

func init() {
	// ECDSA signer.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	signer := NewInMemoryECDSASigner(key)
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	ecdsaKey = &bmkey{signer, signature}

	// ED25519 signer.
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	signer = NewInMemoryED25519Signer(privKey)

	signature, err = signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	ed25519Key = &bmkey{signer, signature}
}

func benchmarkSign(k *bmkey, b *testing.B) {
	signer := k.signer
	for i := 0; i < b.N; i++ {
		if _, err := signer.Sign(purpose, message); err != nil {
			b.Fatalf("%T: %v", signer, err)
		}
	}
}

func benchmarkVerify(k *bmkey, b *testing.B) {
	pub := k.signer.PublicKey()
	for i := 0; i < b.N; i++ {
		if !k.signature.Verify(pub, message) {
			b.Fatalf("%T: Verification failed", pub)
		}
	}
}

func BenchmarkSign_ECDSA(b *testing.B) {
	benchmarkSign(ecdsaKey, b)
}

func BenchmarkVerify_ECDSA(b *testing.B) {
	benchmarkVerify(ecdsaKey, b)
}

func BenchmarkSign_ED25519(b *testing.B) {
	benchmarkSign(ed25519Key, b)
}

func BenchmarkVerify_ED25519(b *testing.B) {
	benchmarkVerify(ed25519Key, b)
}
