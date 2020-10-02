// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"bytes"
	"crypto/elliptic"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23/internal/sectest"
	"v.io/v23/security"
)

func defaultFieldValues(t *testing.T) map[reflect.Type][]reflect.Value {
	v := reflect.ValueOf
	// type of field in Certificate/Signature to a set of values to test against.
	return map[reflect.Type][]reflect.Value{
		reflect.TypeOf(""):                  {v("a"), v("b")},
		reflect.TypeOf(security.Hash("")):   {v(security.SHA256Hash), v(security.SHA384Hash)},
		reflect.TypeOf([]byte{}):            {v([]byte{1}), v([]byte{2})},
		reflect.TypeOf([]security.Caveat{}): {v([]security.Caveat{sectest.NewMethodCaveat(t, "Method")}), v([]security.Caveat{sectest.NewExpiryCaveat(t, time.Now())})},
	}
}

// Generate a bunch of certificates (adding them to certs), each with one field
// different from the previous one. No two certificates should have the same
// digest (since they differ in content). Exclude the Signature field since
// that does not affect the digest.
func initCertificates(t *testing.T, type2values map[reflect.Type][]reflect.Value) []security.Certificate {
	certs := make([]security.Certificate, 1)
	for typ, idx := reflect.TypeOf(security.Certificate{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		if field.Name == "Signature" {
			continue
		}
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		cert := certs[len(certs)-1] // copy of the last certificate
		for _, v := range values {
			reflect.ValueOf(&cert).Elem().FieldByName(field.Name).Set(v)
			certs = append(certs, cert)
		}
	}
	return certs
}

// generate a bunch of signatures similarly to initCertificates.
func initSignatures(t *testing.T, type2values map[reflect.Type][]reflect.Value) []security.Signature {
	sigs := make([]security.Signature, 1)
	for typ, idx := reflect.TypeOf(security.Signature{}), 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		values := type2values[field.Type]
		if len(values) == 0 {
			t.Fatalf("No sample values for field %q of type %v", field.Name, field.Type)
		}
		sig := sigs[len(sigs)-1]
		// ECDSA and ED25519 signatures are mutually exclusive.
		nilSlice := reflect.ValueOf([]byte(nil))
		switch field.Name {
		case "Ed25519":
			reflect.ValueOf(&sig).Elem().FieldByName("R").Set(nilSlice)
			reflect.ValueOf(&sig).Elem().FieldByName("S").Set(nilSlice)
		case "R", "S":
			reflect.ValueOf(&sig).Elem().FieldByName("Ed25519").Set(nilSlice)
		}
		for _, v := range values {
			reflect.ValueOf(&sig).Elem().FieldByName(field.Name).Set(v)
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

func testCertDigestsUniqueness(t *testing.T, hashfn security.Hash, certificates []security.Certificate) ([][]byte, int) {
	numtested := 0
	digests, contentDigests := make([][]byte, len(certificates)), make([][]byte, len(certificates))
	for i, cert := range certificates {
		digests[i], contentDigests[i] = security.ExposeChainedDigests(cert, hashfn, nil)
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
	return digests, numtested
}

func testCertDigestsChangeWithChaining(t *testing.T, hashfn security.Hash, certificates []security.Certificate, certDigests [][]byte) int {
	numtested := 0
	digests, contentDigests := make([][]byte, len(certificates)), make([][]byte, len(certificates))
	cert := certificates[len(certificates)-1] // The last certificate
	for i := 0; i < len(digests); i++ {
		digests[i], contentDigests[i] = security.ExposeChainedDigests(cert, hashfn, certDigests[i])
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
	return numtested
}

func TestCertificateDigest(t *testing.T) {
	// This test generates a bunch of Certificates and Signatures using the reflect package
	// to ensure that every single field of these two is excercised.
	//
	// Then with this "comprehensive" set of certificates and signatures, it ensures that:
	// (1) No two certificates with different fields have the same message digest
	// (2) No two certificates when hashed with distinct parent signatures have the same message digest.
	// (3) Except, the "Signature" field in the certificates should not be included in the message digest.
	var (
		numtested = 0

		// type of field in Certificate/Signature to a set of values to test against.
		type2values = defaultFieldValues(t)

		// Array of Certificate and Signature where the i-th element differs from the (i-1)th in exactly one field.

		certificates = initCertificates(t, type2values)
		signatures   = initSignatures(t, type2values)

		hashfn = security.SHA256Hash // hash function used to compute the message digest in tests.
	)

	defer func() {
		// Paranoia: Most of the tests are gated by loops on the size of "certificates" and "signatures",
		// so a small bug might cause many loops to be skipped. This sanity check tries to detect such test
		// bugs by counting the expected number of digests that were generated and tested.
		// - len(certificates) = 3 fields * 2 values + empty cert = 7
		//   Thus, number of certificate pairs = 7C2 = 21
		// - len(signatures) = 5 fields * 2 values each + empty = 11
		//   Thus, number of signature pairs = 11C2 = 55
		//
		// Tests:
		// - digests should be different for each Certificate:      21 hash comparisons
		// - digests should depend on the chaining of certificates: 21 hash comparisons
		// - content digests should not depend on the Signature:    10 hash comparisons
		// - digests should depend on the Signature:                55 hash comparisons
		if got, want := numtested, 21+21+55+10; got != want {
			t.Fatalf("Executed %d tests, expected %d", got, want)
		}
	}()

	// We have generated a bunch of test data: Certificates with all fields.

	// TEST: No two certificates should have the same digests.
	certDigests, n := testCertDigestsUniqueness(t, hashfn, certificates)
	numtested += n

	// TEST: The digests should change with chaining.
	numtested += testCertDigestsChangeWithChaining(t, hashfn, certificates, certDigests)

	// TEST: The Signature field within a certificate itself should not
	// affect the content digest but will affect the full digest.
	digests, contentDigests := make([][]byte, len(signatures)), make([][]byte, len(signatures))
	for i, sig := range signatures {
		cert := security.Certificate{Signature: sig}
		digests[i], contentDigests[i] = security.ExposeChainedDigests(cert, hashfn, nil)
	}
	for i := 1; i < len(contentDigests); i++ {
		numtested++
		if !bytes.Equal(contentDigests[i], contentDigests[i-1]) {
			cert1 := security.Certificate{Signature: signatures[i]}
			cert2 := security.Certificate{Signature: signatures[i-1]}
			t.Errorf("Certificate{%v} and {%v} which differ only in the signature field have different content digests", cert1, cert2)
		}
	}

	for i := 0; i < len(digests); i++ {
		for j := i + 1; j < len(digests); j++ {
			numtested++
			if bytes.Equal(digests[i], digests[j]) {
				cert1 := security.Certificate{Signature: signatures[i]}
				cert2 := security.Certificate{Signature: signatures[j]}
				t.Errorf("Certificate{%v} and {%v} have different signatures but the same digests", cert1, cert2)
			}
		}
	}
}

func TestChainSignatureUsesDigestWithStrengthComparableToSigningKey(t *testing.T) {
	tests := []struct {
		signer security.Signer
		hash   security.Hash
		nBytes int
	}{
		{sectest.NewECDSASigner(t, elliptic.P224()), security.SHA256Hash, 32},
		{sectest.NewECDSASigner(t, elliptic.P256()), security.SHA256Hash, 32},
		{sectest.NewECDSASigner(t, elliptic.P384()), security.SHA384Hash, 48},
		{sectest.NewECDSASigner(t, elliptic.P521()), security.SHA512Hash, 64},
		{sectest.NewED25519Signer(t), security.SHA512Hash, 64},
	}
	for idx, test := range tests {
		var cert security.Certificate
		digest, contentDigest := security.ExposeChainedDigests(cert, test.hash, nil)
		if got, want := len(digest), test.nBytes; got != want {
			t.Errorf("Got digest of %d bytes, want %d for hash function %q", got, want, test.hash)
			continue
		}
		if got, want := len(contentDigest), test.nBytes; got != want {
			t.Errorf("Got content digest of %d bytes, want %d for hash function %q", got, want, test.hash)
			continue
		}
		chain, _, err := security.ExposeChainCertificate(test.signer, nil, cert)
		if err != nil {
			t.Errorf("security.ExposeChainCertificate for test #%d (hash:%q) failed: %v", idx, test.hash, err)
			continue
		}
		cert = chain[0]
		if !cert.Signature.Verify(test.signer.PublicKey(), contentDigest) {
			t.Errorf("Incorrect hash function used by sign. Test #%d, expected hash:%q", idx, test.hash)
			continue
		}
	}
}

func TestChainMixingECDSA(t *testing.T) {
	testChainMixing(t,
		sectest.NewECDSASignerP256(t),
		sectest.NewECDSASignerP256(t),
		sectest.NewECDSASignerP256(t),
	)
}

func TestChainMixingED25519(t *testing.T) {
	testChainMixing(t,
		sectest.NewED25519Signer(t),
		sectest.NewED25519Signer(t),
		sectest.NewED25519Signer(t),
	)
}

func TestChainMixing(t *testing.T) {
	testChainMixing(t,
		sectest.NewED25519Signer(t),
		sectest.NewECDSASignerP256(t),
		sectest.NewECDSASignerP256(t),
	)
	testChainMixing(t,
		sectest.NewECDSASignerP256(t),
		sectest.NewED25519Signer(t),
		sectest.NewECDSASignerP256(t),
	)
	testChainMixing(t,
		sectest.NewECDSASignerP256(t),
		sectest.NewECDSASignerP256(t),
		sectest.NewED25519Signer(t),
	)
}

func testChainMixing(t *testing.T, sRoot, sUser, sDelegate security.Signer) {
	var (
		// Private and public keys
		pRoot, _     = sRoot.PublicKey().MarshalBinary()
		pUser, _     = sUser.PublicKey().MarshalBinary()
		pDelegate, _ = sDelegate.PublicKey().MarshalBinary()

		// Individual certificates
		cRoot1    = security.Certificate{Extension: "alpha", PublicKey: pRoot}
		cRoot2    = security.Certificate{Extension: "beta", PublicKey: pRoot}
		cUser     = security.Certificate{Extension: "user", PublicKey: pUser}
		cDelegate = security.Certificate{Extension: "delegate", PublicKey: pDelegate}

		// Certificate chains
		C1, _, _   = security.ExposeChainCertificate(sRoot, nil, cRoot1)                                     // alpha
		C2, _, _   = security.ExposeChainCertificate(sRoot, nil, cRoot2)                                     // beta
		C3, _, _   = security.ExposeChainCertificate(sRoot, C1, cUser)                                       // alpha:user
		C4, _, _   = security.ExposeChainCertificate(sUser, C3, cDelegate)                                   // alpha:user:delegate
		Cbad, _, _ = security.ExposeChainCertificate(sUser, []security.Certificate{C2[0], C3[1]}, cDelegate) // malformed beta:user:delegate

		validate = func(chain []security.Certificate, expectedKeyBytes []byte, expectedError string) error {
			var expectedKey security.PublicKey
			var err error
			if len(expectedKeyBytes) > 0 {
				if expectedKey, err = security.UnmarshalPublicKey(expectedKeyBytes); err != nil {
					return err
				}
			}
			// Run all validations twice to account for caching of certificate verifications.
			for i := 1; i <= 2; i++ {
				p, digest, err := security.ExposeValidateCertificateChain(chain)
				if !reflect.DeepEqual(expectedKey, p) {
					return fmt.Errorf("Got (%v, %v) wanted (%v, %q) on call #%d to validateCertificateChain", p, err, expectedKey, expectedError, i)
				}
				if len(expectedError) > 0 {
					if err == nil || !strings.Contains(err.Error(), expectedError) {
						return fmt.Errorf("Got error %v want error id=%q on call #%d to validateCertificateChain", err, expectedError, i)
					}
				}
				if got, want := (err == nil), (len(digest) != 0); got != want {
					return fmt.Errorf("Validation error:%v, Digest:%v on call #%d to validateCertificateChain", got, want, i)
				}
			}
			return nil
		}

		tests = []struct {
			Chain     []security.Certificate
			PublicKey []byte
			Error     string
		}{
			{C1, pRoot, ""},
			{C2, pRoot, ""},
			{C3, pUser, ""},
			{C4, pDelegate, ""},
			{[]security.Certificate{C1[0], C3[1]}, pUser, ""}, // Same as C3
			{Cbad, nil, "invalid Signature in certificate"},
		}
	)
	for idx, test := range tests {
		if err := validate(test.Chain, test.PublicKey, test.Error); err != nil {
			t.Errorf("Test #%d: %v", idx, err)
		}
	}
	// And repeat the tests clearing the cache between every test case.
	for idx, test := range tests {
		security.ClearSignatureCache()
		if err := validate(test.Chain, test.PublicKey, test.Error); err != nil {
			t.Errorf("Test #%d: %v", idx, err)
		}
	}
}

func benchmarkDigestsForCertificateChain(b *testing.B, sfn func(testing.TB) security.Signer, ncerts int) {
	blessings := makeBlessings(b, sfn, ncerts)
	chain := security.ExposeCertChains(blessings)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		security.ExposeDigestsForCertificateChain(chain[0])
	}
}

func BenchmarkDigestsForCertificateChain_1CertECDSA(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewECDSASignerP256, 1)
}

func BenchmarkDigestsForCertificateChain_3CertsECDSA(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewECDSASignerP256, 3)
}

func BenchmarkDigestsForCertificateChain_4CertsECDSA(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewECDSASignerP256, 4)
}

func BenchmarkDigestsForCertificateChain_1CertED25519(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewED25519Signer, 1)
}

func BenchmarkDigestsForCertificateChain_3CertsED25519(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewED25519Signer, 3)
}

func BenchmarkDigestsForCertificateChain_4CertsED25519(b *testing.B) {
	benchmarkDigestsForCertificateChain(b, sectest.NewED25519Signer, 4)
}
