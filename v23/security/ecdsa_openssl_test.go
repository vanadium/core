// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/security"
)

func BenchmarkSign_ECDSA_OpenSSL(b *testing.B) {
	benchmarkSign(opensslECDSA256Key, b)
}

func BenchmarkVerify_ECDSA_OpenSSL(b *testing.B) {
	benchmarkVerify(opensslECDSA256Key, b)
}

// Sanity check: Ensure that the implementations are indeed different.
func sanityCheckSigners(t *testing.T, signers []security.Signer) {
	for i, si := range signers {
		for j := i + 1; j < len(signers); j++ {
			sj := signers[j]
			if pi, pj := si.PublicKey(), sj.PublicKey(); reflect.TypeOf(pi) == reflect.TypeOf(pj) {
				t.Errorf("PublicKey %d and %d have the same type: %T", i, j, pi)
			}
		}
	}
}

// Signatures by any one implementation should be by verifable by all.
func verifySignerInterop(t *testing.T, prefix string, signers []security.Signer) int {
	ntests := 0
	for _, signer := range signers {
		signature, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("%s: Signature with %T failed: %v", prefix, signer.PublicKey(), err)
			continue
		}
		for _, verifier := range signers {
			pub := verifier.PublicKey()
			if !signature.Verify(pub, message) {
				t.Errorf("%s: Messaged signed by %T not verified by %T", prefix, signer, pub)
				continue
			}
			ntests++
		}
	}
	return ntests
}

// All the metadata functions should return the same results.
func verifyMetadata(t *testing.T, prefix string, signers []security.Signer) {
	pub0 := signers[0].PublicKey()
	bin0, err := pub0.MarshalBinary()
	if err != nil {
		t.Errorf("%s: %T.MarshalBinary failed: %v", prefix, pub0, err)
		return
	}
	for _, signer := range signers[1:] {
		pubi := signer.PublicKey()
		if got, want := pubi.String(), pub0.String(); got != want {
			t.Errorf("%s: String for %T and %T do not match (%q vs %q)", prefix, pubi, pub0, got, want)
		}
		bini, err := pubi.MarshalBinary()
		if err != nil {
			t.Errorf("%s: %T.MarshalBinary failed: %v", prefix, pubi, err)
			continue
		}
		if !bytes.Equal(bin0, bini) {
			t.Errorf("%s: MarshalBinary for %T and %T do not match (%v vs %v)", prefix, pubi, pub0, bini, bin0)
		}
		if got, want := security.ExposePublicKeyHashAlgo(pubi), security.ExposePublicKeyHashAlgo(pub0); got != want {
			t.Errorf("%s: hash for %T(=%v) and %T(=%v) do not match", prefix, pubi, got, pub0, want)
		}
	}
}

func TestOpenSSLCompatibilityECDSA(t *testing.T) {
	t.Log(security.OpenSSLVersion())
	ntests := 0
	for curveidx, curve := range []elliptic.Curve{elliptic.P224(), elliptic.P256(), elliptic.P384(), elliptic.P521()} {
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key #%d: %v", curveidx, err)
			continue
		}
		hash := security.ExposeECDSAHash(&key.PublicKey)
		golang, err := security.NewGoStdlibECDSASigner(key, hash)
		if err != nil {
			t.Error(err)
			continue
		}
		openssl, err := security.NewOpenSSLECDSASigner(key)
		if err != nil {
			t.Error(err)
			continue
		}
		signers := []security.Signer{golang, openssl}
		sanityCheckSigners(t, signers)
		ntests += verifySignerInterop(t, fmt.Sprintf("Curve #%d", curveidx), signers)
		verifyMetadata(t, fmt.Sprintf("Curve #%d", curveidx), signers)
	}
	// Silly sanity check to help ensure that these nested loops weren't short circuited
	if got, want := ntests, 16; got != want { // 2 types of keys => 4 tests * 4 curves
		t.Errorf("%d combinations of tests succeeded, expected %d", got, want)
	}
}
