// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"v.io/v23/security"
)

func BenchmarkSign_ED25519_OpenSSL(b *testing.B) {
	initOpenSSLBenchmarks()
	benchmarkSign(opensslED25519Key, b)
}

func BenchmarkVerify_ED25519_OpenSSL(b *testing.B) {
	initOpenSSLBenchmarks()
	benchmarkVerify(opensslED25519Key, b)
}

func TestOpenSSLCompatibilityED25519(t *testing.T) {
	t.Log(security.OpenSSLVersion())
	ntests := 0

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Failed to generate key", err)
	}
	golang, err := security.NewGoStdlibED25519Signer(privKey)
	if err != nil {
		t.Fatal(err)
	}
	openssl, err := security.NewOpenSSLED25519Signer(privKey)
	if err != nil {
		t.Fatal(err)
	}
	signers := []security.Signer{golang, openssl}
	sanityCheckSigners(t, signers)
	ntests += verifySignerInterop(t, "ed25519", signers)
	verifyMetadata(t, "ed25519", signers)

	// Silly sanity check to help ensure that these nested loops weren't short circuited
	if got, want := ntests, 4; got != want { // 2 types of keys => 4 tests * 1 curve
		t.Errorf("%d combinations of tests succeeded, expected %d", got, want)
	}
}
