// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build openssl

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"reflect"
	"testing"
)

var opensslECDSAKey *bmkey

func init() {
	opensslECDSAKey = newECDSABenchmarkKey(newOpenSSLECDSASigner)
}

func BenchmarkSign_ECDSA_OpenSSL(b *testing.B) {
	benchmarkSign(opensslECDSAKey, b)
}

func BenchmarkVerify_ECDSA_OpenSSL(b *testing.B) {
	benchmarkVerify(opensslECDSAKey, b)
}

func TestOpenSSLCompatibilityECDSA(t *testing.T) {
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
	for curveidx, curve := range []elliptic.Curve{elliptic.P224(), elliptic.P256(), elliptic.P384(), elliptic.P521()} {
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key #%d: %v", curveidx, err)
			continue
		}
		golang, err := newGoStdlibECDSASigner(key)
		if err != nil {
			t.Error(err)
			continue
		}
		openssl, err := newOpenSSLECDSASigner(key)
		if err != nil {
			t.Error(err)
			continue
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
				t.Errorf("Curve #%d: Signature with %T failed: %v", curveidx, signer.PublicKey(), err)
				continue
			}
			for _, verifier := range signers {
				pub := verifier.PublicKey()
				if !signature.Verify(pub, message) {
					t.Errorf("Curve #%d: Messaged signed by %T not verified by %T", curveidx, signer, pub)
					continue
				}
				ntests++
			}
		}
		// All the metadata functions should return the same results.
		pub0 := signers[0].PublicKey()
		bin0, err := pub0.MarshalBinary()
		if err != nil {
			t.Errorf("Curve #%d: %T.MarshalBinary failed: %v", curveidx, pub0, err)
			continue
		}
		for _, signer := range signers[1:] {
			pubi := signer.PublicKey()
			if got, want := pubi.String(), pub0.String(); got != want {
				t.Errorf("Curve #%d: String for %T and %T do not match (%q vs %q)", curveidx, pubi, pub0, got, want)
			}
			bini, err := pubi.MarshalBinary()
			if err != nil {
				t.Errorf("Curve #%d: %T.MarshalBinary failed: %v", curveidx, pubi, err)
				continue
			}
			if !bytes.Equal(bin0, bini) {
				t.Errorf("Curve #%d: MarshalBinary for %T and %T do not match (%v vs %v)", curveidx, pubi, pub0, bini, bin0)
			}
			if got, want := pubi.hash(), pub0.hash(); got != want {
				t.Errorf("Curve #%d: hash for %T(=%v) and %T(=%v) do not match", curveidx, pubi, got, pub0, want)
			}
		}
	}
	// Silly sanity check to help ensure that these nested loops weren't short circuited
	if got, want := ntests, 16; got != want { // 2 types of keys => 4 tests * 4 curves
		t.Errorf("%d combinations of tests succeeded, expected %d", got, want)
	}
}
