// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

var curvesToTest = []elliptic.Curve{
	elliptic.P224(),
	elliptic.P256(),
	elliptic.P384(),
	elliptic.P521(),
}

func TestSignature(t *testing.T) {
	for _, curve := range curvesToTest {
		nbits := curve.Params().BitSize
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key for curve of %d bits: %v", nbits, err)
			continue
		}
		// Test that the defined signing purposes work as expected.
		purposes := [][]byte{[]byte(SignatureForMessageSigning), []byte(SignatureForBlessingCertificates), []byte(SignatureForDischarge)}
		message := []byte("test")
		signer := NewInMemoryECDSASigner(key)
		for _, p := range purposes {
			sig, err := signer.Sign(p, message)
			if err != nil {
				t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
				continue
			}
			if !sig.Verify(signer.PublicKey(), message) {
				t.Errorf("Signature verification failed with curve of %d bits", nbits)
				continue
			}
		}
		// Sign a message nbits long and then ensure that a larger message does not have the same signature.
		message = make([]byte, nbits>>3)
		sig, err := signer.Sign(nil, message)
		if err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature verification failed with curve of %d bits", nbits)
			continue
		}
		message = append(message, 1)
		if sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature of message of %d bytes incorrectly verified with curve of %d bits", len(message), nbits)
			continue
		}
		// Sign a message larger than nbits and verify that switching the last byte does not end up with the same signature.
		if sig, err = signer.Sign(nil, message); err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature verification failed with curve of %d bits", nbits)
			continue
		}
		message[len(message)-1] = message[len(message)-1] + 1
		if sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature of modified message incorrectly verified with curve of %d bits", nbits)
			continue
		}
		// Signing small messages and then extending them.
		nbytes := (nbits + 7) / 3
		message = make([]byte, 1, nbytes)
		if sig, err = signer.Sign(nil, message); err != nil {
			t.Errorf("Failed to generate signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Failed to verify signature of small message with curve of %d bits", nbits)
			continue
		}
		for len(message) < nbytes {
			// Extended message should fail verification
			message = append(message, message...)
			if sig.Verify(signer.PublicKey(), message) {
				t.Errorf("Signature of extended message (%d bytes) incorrectly verified with curve of %d bits", len(message), nbits)
				break
			}
		}
	}

}

func TestSignaturePurpose(t *testing.T) {
	// Distinct (purpose, message) pairs should have distinct signatures, even if (purpose+message) or (message+purpose) are identical.
	type T struct {
		P, M []byte
	}
	var (
		purpose = []byte{'a'}
		message = []byte{'b', 'c', 'd'}

		samePurposePlusMessage = []T{
			{nil, append(purpose, message...)},
			{append(purpose, message...), nil},
			T{[]byte{'a', 'b'}, []byte{'c', 'd'}},
		}
		sameMessagePlusPurpose = []T{
			{nil, append(message, purpose...)},
			{append(message, purpose...), nil},
			{[]byte{'d', 'a'}, []byte{'b', 'c'}},
		}
		samePurpose = T{purpose, []byte{'x', 'y', 'z'}}
		sameMessage = T{[]byte{'x'}, message}
		nilPurpose  = T{nil, message}
		nilMessage  = T{purpose, nil}

		tests = append(append(samePurposePlusMessage, sameMessagePlusPurpose...),
			samePurpose,
			sameMessage,
			nilPurpose,
			nilMessage)
	)
	// Apologies for the paranoia, but making sure that the testdata is setup as expected!
	// (This test primarily checks for signatures not being valid for other (purpose, message) pairs,
	// so must ensure that they aren't valid for the right reasons!)
	for _, test := range samePurposePlusMessage {
		if !bytes.Equal(append(purpose, message...), append(test.P, test.M...)) {
			t.Errorf("Invalid test data (%q+%q) != (%q+%q)", purpose, message, test.P, test.M)
		}
	}
	for _, test := range sameMessagePlusPurpose {
		if !bytes.Equal(append(message, purpose...), append(test.M, test.P...)) {
			t.Errorf("Invalid test data (%q+%q) != (%q+%q)", message, purpose, test.M, test.P)
		}
	}
	if test := samePurpose; !bytes.Equal(purpose, test.P) {
		t.Fatalf("Invalid test data, purpose is not identical")
	}
	if test := sameMessage; !bytes.Equal(message, test.M) {
		t.Fatalf("Invalid test data, message is not identitical")
	}
	for _, curve := range curvesToTest {
		nbits := curve.Params().BitSize
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key for curve of %d bits: %v", nbits, err)
			continue
		}
		signer := NewInMemoryECDSASigner(key)
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("Unable to compute signature with curve of %d bits: %v", nbits, err)
			continue
		}
		if !bytes.Equal(sig.Purpose, purpose) {
			t.Errorf("Got purpose %q, want %q when signing with curve of %d bits", sig.Purpose, purpose, nbits)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("Signature verification failed with curve of %d bits", nbits)
		}
		// No other combination of (purpose, message) should have the same signature.
		for _, test := range tests {
			sig.Purpose = test.P
			if sig.Verify(signer.PublicKey(), test.M) {
				t.Errorf("Signature for (%q, %q) is valid for (%q, %q) (curve: %d bits)", purpose, message, test.P, test.M, nbits)
			}
		}
	}
}
