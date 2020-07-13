// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"reflect"
	"testing"
)

var opensslED25519Key *bmkey

func init() {
	opensslED25519Key = newED25519BenchmarkKey(newOpenSSLED25519Signer)
}

func BenchmarkSign_ED25519_OpenSSL(b *testing.B) {
	benchmarkSign(opensslED25519Key, b)
}

func BenchmarkVerify_ED25519_OpenSSL(b *testing.B) {
	benchmarkVerify(opensslED25519Key, b)
}

func TestOpenSSLCompatibilityED25519(t *testing.T) {
	t.Log(openssl_version())
	var (
		purpose = make([]byte, 5)
		message = make([]byte, 100)
		ntests  = 0
	)
	if _, err := rand.Reader.Read(purpose); err != nil {
		t.Error(err)
	}
	if _, err := rand.Reader.Read(message); err != nil {
		t.Error(err)
	}

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal("Failed to generate key", err)
	}
	golang, err := newGoStdlibED25519Signer(privKey)
	if err != nil {
		t.Fatal(err)
	}
	openssl, err := newOpenSSLED25519Signer(privKey)
	if err != nil {
		t.Fatal(err)
	}
	signers := []Signer{golang, openssl}
	// Sanity check: Ensure that the implementations are indeed different.
	for i := 0; i < len(signers); i++ {
		si := signers[i]
		for j := i + 1; j < len(signers); j++ {
			sj := signers[j]
			if pi, pj := si.PublicKey(), sj.PublicKey(); reflect.TypeOf(pi) == reflect.TypeOf(pj) {
				t.Errorf("PublicKey %d and %d have the same type: %T", i, j, pi)
			}
		}
	}
	// Signatures by any one implementation should be by all.
	for _, signer := range signers {
		signature, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("Signature with %T failed: %v", signer.PublicKey(), err)
			continue
		}
		for _, verifier := range signers {
			pub := verifier.PublicKey()
			if !signature.Verify(pub, message) {
				t.Errorf("Messaged signed by %T not verified by %T", signer, pub)
				continue
			}
			ntests++
		}
	}
	// All the metadata functions should return the same results.
	pub0 := signers[0].PublicKey()
	bin0, err := pub0.MarshalBinary()
	if err != nil {
		t.Fatalf("%T.MarshalBinary failed: %v", pub0, err)
	}
	for _, signer := range signers[1:] {
		pubi := signer.PublicKey()
		if got, want := pubi.String(), pub0.String(); got != want {
			t.Errorf("String for %T and %T do not match (%q vs %q)", pubi, pub0, got, want)
		}
		bini, err := pubi.MarshalBinary()
		if err != nil {
			t.Errorf("%T.MarshalBinary failed: %v", pubi, err)
			continue
		}
		if !bytes.Equal(bin0, bini) {
			t.Errorf("MarshalBinary for %T and %T do not match (%v vs %v)", pubi, pub0, bini, bin0)
		}
		if got, want := pubi.hash(), pub0.hash(); got != want {
			t.Errorf("hash for %T(=%v) and %T(=%v) do not match", pubi, got, pub0, want)
		}
	}

	// Silly sanity check to help ensure that these nested loops weren't short circuited
	if got, want := ntests, 4; got != want { // 2 types of keys => 4 tests * 1 curve
		t.Errorf("%d combinations of tests succeeded, expected %d", got, want)
	}
}
