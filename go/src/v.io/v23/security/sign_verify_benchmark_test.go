// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

type bmkey struct {
	signer    Signer
	signature Signature
}

var (
	ecdsaKey *bmkey
	message  = []byte("over the mountain and under the bridge")
	purpose  = []byte("benchmarking")
)

func init() {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	signer, err := newGoStdlibSigner(key)
	if err != nil {
		panic(err)
	}
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	ecdsaKey = &bmkey{signer, signature}
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
