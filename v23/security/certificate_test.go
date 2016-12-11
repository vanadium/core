// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/elliptic"
	"fmt"
	"reflect"
	"testing"
	"time"

	"v.io/v23/verror"
)

func TestCertificateDigest(t *testing.T) {
	// This test generates a bunch of Certificates and Signatures using the reflect package
	// to ensure that ever single field of these two is excercised.
	//
	// Then with this "comprehensive" set of certificates and signatures, it ensures that:
	// (1) No two certificates with different fields have the same message digest
	// (2) No two certificates when hashed with distinct parent signatures have the same message digest.
	// (3) Except, the "Signature" field in the certificates should not be included in the message digest.
	var (
		// Array of Certificate and Signature where the i-th element differs from the (i-1)th in exactly one field.
		certificates = make([]Certificate, 1)
		signatures   = make([]Signature, 1)
		numtested    = 0

		v = func(item interface{}) reflect.Value { return reflect.ValueOf(item) }
		// type of field in Certificate/Signature to a set of values to test against.
		type2values = map[reflect.Type][]reflect.Value{
			reflect.TypeOf(""):         []reflect.Value{v("a"), v("b")},
			reflect.TypeOf(Hash("")):   []reflect.Value{v(SHA256Hash), v(SHA384Hash)},
			reflect.TypeOf([]byte{}):   []reflect.Value{v([]byte{1}), v([]byte{2})},
			reflect.TypeOf([]Caveat{}): []reflect.Value{v([]Caveat{newCaveat(NewMethodCaveat("Method"))}), v([]Caveat{newCaveat(NewExpiryCaveat(time.Now()))})},
		}
		hashfn = SHA256Hash // hash function used to compute the message digest in tests.
	)
	defer func() {
		// Paranoia: Most of the tests are gated by loops on the size of "certificates" and "signatures",
		// so a small bug might cause many loops to be skipped. This sanity check tries to detect such test
		// bugs by counting the expected number of digests that were generated and tested.
		// - len(certificates) = 3 fields * 2 values + empty cert = 7
		//   Thus, number of certificate pairs = 7C2 = 21
		// - len(signatures) = 4 fields * 2 values each + empty = 9
		//   Thus, number of signature pairs = 9C2 = 36
		//
		// Tests:
		// - digests should be different for each Certificate:      21 hash comparisons
		// - digests should depend on the chaining of certificates: 21 hash comparisons
		// - content digests should not depend on the Signature:     8 hash comparisons
		// - digests should depend on the Signature:                36 hash comparisons
		if got, want := numtested, 21+21+36+8; got != want {
			t.Fatalf("Executed %d tests, expected %d", got, want)
		}
	}()

	// Generate a bunch of certificates (adding them to certs), each with one field
	// different from the previous one. No two certificates should have the same
	// digest (since they differ in content). Exclude the Signature field since
	// that does not affect the digest.
	for typ, idx := reflect.TypeOf(Certificate{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		if field.Name == "Signature" {
			continue
		}
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		cert := certificates[len(certificates)-1] // copy of the last certificate
		for _, v := range values {
			reflect.ValueOf(&cert).Elem().Field(idx).Set(v)
			certificates = append(certificates, cert)
		}
	}
	// Similarly, generate a bunch of signatures.
	for typ, idx := reflect.TypeOf(Signature{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		sig := signatures[len(signatures)-1]
		for _, v := range values {
			reflect.ValueOf(&sig).Elem().Field(idx).Set(v)
			signatures = append(signatures, sig)
		}
	}

	// We have generated a bunch of test data: Certificates with all fields.
	// TEST: No two certificates should have the same digests.
	digests, contentDigests := make([][]byte, len(certificates)), make([][]byte, len(certificates))
	for i, cert := range certificates {
		digests[i], contentDigests[i] = cert.chainedDigests(hashfn, nil)
	}
	for i := 0; i < len(digests); i++ {
		for j := i + 1; j < len(digests); j++ {
			numtested++
			if bytes.Equal(digests[i], digests[j]) {
				t.Errorf("Certificates:{%+v} and {%+v} have the same message digest", certificates[i], certificates[j])
			}
			if bytes.Equal(contentDigests[i], contentDigests[j]) {
				t.Errorf("Certificates:{%+v} and {%+v} have the same content digest", certificates[i], certificates[j])
			}
		}
	}

	// TEST: The digests should change with chaining.
	certDigests := digests
	digests, contentDigests = make([][]byte, len(certificates)), make([][]byte, len(certificates))
	cert := certificates[len(certificates)-1] // The last certificate
	for i := 0; i < len(digests); i++ {
		digests[i], contentDigests[i] = cert.chainedDigests(hashfn, certDigests[i])
	}
	for i := 0; i < len(digests); i++ {
		for j := i + 1; j < len(digests); j++ {
			numtested++
			if bytes.Equal(digests[i], digests[j]) {
				t.Errorf("Certificate digest is the same for two different certificate chains - {%v} chained to {%v} and {%v}", cert, certificates[i], certificates[j])
			}
			if bytes.Equal(contentDigests[i], contentDigests[j]) {
				t.Errorf("Content digest is the same for two different certificate chains - {%v} chained to {%v} and {%v}", cert, certificates[i], certificates[j])
			}
		}
	}

	// TEST: The Signature field within a certificate itself should not
	// affect the content digest but will affect the full digest.
	digests, contentDigests = make([][]byte, len(signatures)), make([][]byte, len(signatures))
	for i, sig := range signatures {
		cert := Certificate{Signature: sig}
		digests[i], contentDigests[i] = cert.chainedDigests(hashfn, nil)
	}
	for i := 1; i < len(contentDigests); i++ {
		numtested++
		if !bytes.Equal(contentDigests[i], contentDigests[i-1]) {
			cert1 := Certificate{Signature: signatures[i]}
			cert2 := Certificate{Signature: signatures[i-1]}
			t.Errorf("Certificate{%v} and {%v} which differ only in the signature field have different content digests", cert1, cert2)
		}
	}
	for i := 0; i < len(digests); i++ {
		for j := i + 1; j < len(digests); j++ {
			numtested++
			if bytes.Equal(digests[i], digests[j]) {
				cert1 := Certificate{Signature: signatures[i]}
				cert2 := Certificate{Signature: signatures[j]}
				t.Errorf("Certificate{%v} and {%v} have different signatures but the same digests", cert1, cert2)
			}
		}
	}
}

func TestChainSignatureUsesDigestWithStrengthComparableToSigningKey(t *testing.T) {
	tests := []struct {
		curve  elliptic.Curve
		hash   Hash
		nBytes int
	}{
		{elliptic.P224(), SHA256Hash, 32},
		{elliptic.P256(), SHA256Hash, 32},
		{elliptic.P384(), SHA384Hash, 48},
		{elliptic.P521(), SHA512Hash, 64},
	}
	for idx, test := range tests {
		var cert Certificate
		digest, contentDigest := cert.chainedDigests(test.hash, nil)
		if got, want := len(digest), test.nBytes; got != want {
			t.Errorf("Got digest of %d bytes, want %d for hash function %q", got, want, test.hash)
			continue
		}
		if got, want := len(contentDigest), test.nBytes; got != want {
			t.Errorf("Got content digest of %d bytes, want %d for hash function %q", got, want, test.hash)
			continue
		}
		signer := newECDSASigner(t, test.curve)
		chain, _, err := chainCertificate(signer, nil, cert)
		if err != nil {
			t.Errorf("chainCertificate for test #%d (hash:%q) failed: %v", idx, test.hash, err)
			continue
		}
		cert = chain[0]
		if !cert.Signature.Verify(signer.PublicKey(), contentDigest) {
			t.Errorf("Incorrect hash function used by sign. Test #%d, expected hash:%q", idx, test.hash)
			continue
		}
	}
}

func TestChainMixing(t *testing.T) {
	var (
		// Private and public keys
		sRoot        = newECDSASigner(t, elliptic.P256())
		pRoot, _     = sRoot.PublicKey().MarshalBinary()
		sUser        = newECDSASigner(t, elliptic.P256())
		pUser, _     = sUser.PublicKey().MarshalBinary()
		pDelegate, _ = newECDSASigner(t, elliptic.P256()).PublicKey().MarshalBinary()

		// Individual certificates
		cRoot1    = Certificate{Extension: "alpha", PublicKey: pRoot}
		cRoot2    = Certificate{Extension: "beta", PublicKey: pRoot}
		cUser     = Certificate{Extension: "user", PublicKey: pUser}
		cDelegate = Certificate{Extension: "delegate", PublicKey: pDelegate}

		// Certificate chains
		C1, _, _   = chainCertificate(sRoot, nil, cRoot1)                            // alpha
		C2, _, _   = chainCertificate(sRoot, nil, cRoot2)                            // beta
		C3, _, _   = chainCertificate(sRoot, C1, cUser)                              // alpha:user
		C4, _, _   = chainCertificate(sUser, C3, cDelegate)                          // alpha:user:delegate
		Cbad, _, _ = chainCertificate(sUser, []Certificate{C2[0], C3[1]}, cDelegate) // malformed beta:user:delegate

		validate = func(chain []Certificate, expectedKeyBytes []byte, expectedError verror.ID) error {
			var expectedKey PublicKey
			var err error
			if len(expectedKeyBytes) > 0 {
				if expectedKey, err = UnmarshalPublicKey(expectedKeyBytes); err != nil {
					return err
				}
			}
			// Run all validations twice to account for caching of certificate verifications.
			for i := 1; i <= 2; i++ {
				p, digest, err := validateCertificateChain(chain)
				if !reflect.DeepEqual(expectedKey, p) {
					return fmt.Errorf("Got (%v, %v) wanted (%v, %q) on call #%d to validateCertificateChain", p, err, expectedKey, expectedError, i)
				}
				if got, want := verror.ErrorID(err), expectedError; got != want {
					return fmt.Errorf("Got error %v (id=%q) want error id=%q on call #%d to validateCertificateChain", err, got, want, i)
				}
				if got, want := (err == nil), (len(digest) != 0); got != want {
					return fmt.Errorf("Validation error:%v, Digest:%v on call #%d to validateCertificateChain", got, want, i)
				}
			}
			return nil
		}

		tests = []struct {
			Chain     []Certificate
			PublicKey []byte
			Error     verror.ID
		}{
			{C1, pRoot, ""},
			{C2, pRoot, ""},
			{C3, pUser, ""},
			{C4, pDelegate, ""},
			{[]Certificate{C1[0], C3[1]}, pUser, ""}, // Same as C3
			{Cbad, nil, errBadCertSignature.ID},
		}
	)
	for idx, test := range tests {
		if err := validate(test.Chain, test.PublicKey, test.Error); err != nil {
			t.Errorf("Test #%d: %v", idx, err)
		}
	}
	// And repeat the tests clearing the cache between every test case.
	for idx, test := range tests {
		signatureCache.disable() // clears the cache too
		signatureCache.enable()
		if err := validate(test.Chain, test.PublicKey, test.Error); err != nil {
			t.Errorf("Test #%d: %v", idx, err)
		}
	}
}

func benchmarkDigestsForCertificateChain(b *testing.B, ncerts int) {
	chain := makeBlessings(b, ncerts).chains[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		digestsForCertificateChain(chain)
	}
}

func BenchmarkDigestsForCertificateChain_1Cert(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, 1)
}
func BenchmarkDigestsForCertificateChain_3Certs(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, 3)
}
func BenchmarkDigestsForCertificateChain_4Certs(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, 4)
}
