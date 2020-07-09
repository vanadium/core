// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"

	"v.io/v23/security/internal/ecdsaonly"
	"v.io/v23/vom"
)

// Test that the defined signing purposes work as expected.
func testSigning(signer Signer, messageSizeInBits int) error {
	purposes := [][]byte{[]byte(SignatureForMessageSigning), []byte(SignatureForBlessingCertificates), []byte(SignatureForDischarge)}
	message := []byte("test")
	for _, p := range purposes {
		sig, err := signer.Sign(p, message)
		if err != nil {
			return fmt.Errorf("Failed to generate signature: %v", err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			return fmt.Errorf("Signature verification failed")
		}
	}
	// Sign a message nbits long and then ensure that a larger message does not have the same signature.
	message = make([]byte, messageSizeInBits>>3)
	sig, err := signer.Sign(nil, message)
	if err != nil {
		return fmt.Errorf("Failed to generate signature: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Signature verification failed")
	}
	message = append(message, 1)
	if sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Signature of message of %d bytes incorrectly verified", len(message))
	}
	// Sign a message larger than nbits and verify that switching the last byte does not end up with the same signature.
	if sig, err = signer.Sign(nil, message); err != nil {
		return fmt.Errorf("Failed to generate signature: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Signature verification failed")
	}
	message[len(message)-1] = message[len(message)-1] + 1
	if sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Signature of modified message incorrectly verified")
	}
	// Signing small messages and then extending them.
	nbytes := (messageSizeInBits + 7) / 3
	message = make([]byte, 1, nbytes)
	if sig, err = signer.Sign(nil, message); err != nil {
		return fmt.Errorf("Failed to generate signature: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Failed to verify signature of small message")
	}
	for len(message) < nbytes {
		// Extended message should fail verification
		message = append(message, message...)
		if sig.Verify(signer.PublicKey(), message) {
			return fmt.Errorf("Signature of extended message (%d bytes) incorrectly verified", len(message))
		}
	}
	return nil
}

type signaturePurposeTest struct {
	P, M []byte
}

func testSignaturePurpose(signer Signer, purpose, message []byte, tests []signaturePurposeTest) error {
	sig, err := signer.Sign(purpose, message)
	if err != nil {
		return fmt.Errorf("Unable to compute signature: %v", err)
	}
	if !bytes.Equal(sig.Purpose, purpose) {
		return fmt.Errorf("Got purpose %q, want %q when", sig.Purpose, purpose)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		return fmt.Errorf("Signature verification failed")
	}
	// No other combination of (purpose, message) should have the same signature.
	for _, test := range tests {
		sig.Purpose = test.P
		if sig.Verify(signer.PublicKey(), test.M) {
			return fmt.Errorf("Signature for (%q, %q) is valid for (%q, %q)", purpose, message, test.P, test.M)
		}
	}
	return nil
}

var curvesToTest = []elliptic.Curve{
	elliptic.P224(),
	elliptic.P256(),
	elliptic.P384(),
	elliptic.P521(),
}

func TestECDSASignature(t *testing.T) {
	for _, curve := range curvesToTest {
		nbits := curve.Params().BitSize
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Errorf("Failed to generate key for curve of %d bits: %v", nbits, err)
			continue
		}
		signer, err := NewInMemoryECDSASigner(key)
		if err != nil {
			t.Errorf("Failed to create signer for curve of %d bits: %v", nbits, err)
			continue
		}
		if err := testSigning(signer, nbits); err != nil {
			t.Errorf("signing failed for curve with %d bits: %v", nbits, err)
		}
	}
}

func TestED25519Signature(t *testing.T) {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key for ed25519: %v", err)
	}
	signer, err := NewInMemoryED25519Signer(privKey)
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := testSigning(signer, 512); err != nil {
		t.Errorf("signing failed for ed25519: %v", err)
	}
}

func TestSignaturePurpose(t *testing.T) {
	// Distinct (purpose, message) pairs should have distinct signatures, even if (purpose+message) or (message+purpose) are identical.

	var (
		purpose = []byte{'a'}
		message = []byte{'b', 'c', 'd'}

		samePurposePlusMessage = []signaturePurposeTest{
			{nil, append(purpose, message...)},
			{append(purpose, message...), nil},
			{[]byte{'a', 'b'}, []byte{'c', 'd'}},
		}
		sameMessagePlusPurpose = []signaturePurposeTest{
			{nil, append(message, purpose...)},
			{append(message, purpose...), nil},
			{[]byte{'d', 'a'}, []byte{'b', 'c'}},
		}
		samePurpose = signaturePurposeTest{purpose, []byte{'x', 'y', 'z'}}
		sameMessage = signaturePurposeTest{[]byte{'x'}, message}
		nilPurpose  = signaturePurposeTest{nil, message}
		nilMessage  = signaturePurposeTest{purpose, nil}

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
		signer, err := NewInMemoryECDSASigner(key)
		if err != nil {
			t.Errorf("Failed to creator signer key for curve of %d bits: %v", nbits, err)
			continue
		}
		if err := testSignaturePurpose(signer, purpose, message, tests); err != nil {
			t.Errorf("testSignaturePurpose failed for curve with %d bits: %v", nbits, err)
		}
	}
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key for ed25519: %v", err)
	}
	signer, err := NewInMemoryED25519Signer(privKey)
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	if err := testSignaturePurpose(signer, purpose, message, tests); err != nil {
		t.Errorf("testSignaturePurpose failed ed25519: %v", err)
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key for ed25519: %v", err)
	}
	signer, err := NewInMemoryECDSASigner(key)
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}
	message := []byte("hello there new implementation")
	sig, err := signer.Sign([]byte(SignatureForMessageSigning), message)
	if err != nil {
		t.Errorf("Failed to generate signature: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("Signature verification failed")
	}

	// Make sure that we encode/decode from the old to the new
	// Signature type.
	ecsdaOnlySig := ecdsaonly.Signature{
		Purpose: sig.Purpose,
		Hash:    ecdsaonly.Hash(sig.Hash),
		R:       sig.R,
		S:       sig.S,
	}
	out := &bytes.Buffer{}
	enc := vom.NewEncoder(out)
	if err := enc.Encode(ecsdaOnlySig); err != nil {
		t.Errorf("failed to encode old ecdsaonly Signature: %v", err)
	}
	in := bytes.NewBuffer(out.Bytes())
	dec := vom.NewDecoder(in)
	var nsig Signature
	if err := dec.Decode(&nsig); err != nil {
		t.Errorf("failed to decode old ecdsaonly Signature: %v", err)
	}
	if !nsig.Verify(signer.PublicKey(), message) {
		t.Errorf("Signature re-verification failed")
	}

	// Make sure we can encode/cdecode from the new to the old
	// Signature type.
	out = &bytes.Buffer{}
	enc = vom.NewEncoder(out)
	if err := enc.Encode(nsig); err != nil {
		t.Errorf("failed to encode current Signature: %v", err)
	}
	in = bytes.NewBuffer(out.Bytes())
	dec = vom.NewDecoder(in)

	var nECDSAOnlySignature ecdsaonly.Signature
	if err := dec.Decode(&nECDSAOnlySignature); err != nil {
		t.Errorf("failed to decode old ecdsaonly Signature: %v", err)
	}

	sig = Signature{
		Purpose: nECDSAOnlySignature.Purpose,
		Hash:    Hash(nECDSAOnlySignature.Hash),
		R:       nECDSAOnlySignature.R,
		S:       nECDSAOnlySignature.S,
	}

	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("Signature verification failed")
	}

}
