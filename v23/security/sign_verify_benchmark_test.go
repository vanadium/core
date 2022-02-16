// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"testing"
)

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
func BenchmarkSign_RSA2048(b *testing.B) {
	benchmarkSign(rsa2048Key, b)
}

func BenchmarkVerify_RSA2048(b *testing.B) {
	benchmarkVerify(rsa2048Key, b)
}
