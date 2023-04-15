// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl

package security_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"

	"v.io/v23/security"
)

func BenchmarkSign_RSA2048_OpenSSL(b *testing.B) {
	benchmarkSign(opensslRSA2048Key, b)
}
func BenchmarkSign_RSA4096_OpenSSL(b *testing.B) {
	benchmarkSign(opensslRSA4096Key, b)
}

func BenchmarkVerify_RSA2048_OpenSSL(b *testing.B) {
	benchmarkVerify(opensslRSA2048Key, b)
}

func BenchmarkVerify_RSA2096_OpenSSL(b *testing.B) {
	benchmarkVerify(opensslRSA4096Key, b)
}

func TestOpenSSLCompatibilityRSA(t *testing.T) {
	t.Log(security.OpenSSLVersion())
	ntests := 0

	keySizes := []int{2048}
	for _, keySize := range keySizes {
		key, err := rsa.GenerateKey(rand.Reader, keySize)
		if err != nil {
			t.Errorf("Failed to generate key #%d: %v", keySize, err)
			continue
		}
		golang, err := security.NewGoStdlibRSASigner(key, security.SHA512Hash)
		if err != nil {
			t.Error(err)
			continue
		}
		openssl, err := security.NewOpenSSLRSASigner(key)
		if err != nil {
			t.Error(err)
			continue
		}
		signers := []security.Signer{golang, openssl}
		sanityCheckSigners(t, signers)
		ntests += verifySignerInterop(t, fmt.Sprintf("RSA %d bits", keySize), signers)
		verifyMetadata(t, fmt.Sprintf("RSA %d bits", keySize), signers)

		if !security.CryptoPublicKeyEqual(golang.PublicKey(), openssl.PublicKey()) {
			t.Errorf("keys should be equal [%v] [%v]", golang.PublicKey(), openssl.PublicKey())
		}
	}
	// Silly sanity check to help ensure that these nested loops weren't short circuited
	if got, want := ntests, 4*len(keySizes); got != want { // 2 types of keys => 4 tests * n sizes
		t.Errorf("%d combinations of tests succeeded, expected %d", got, want)
	}
}
