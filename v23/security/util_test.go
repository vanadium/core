// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains utility functions and types for tests for the security package.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"

	"v.io/v23/context"
)

// EnableSignatureCache exposes signatureCache.enable() to tests.
func EnableSignatureCache() {
	signatureCache.enable()
}

// DisableSignatureCache exposes signatureCache.disable() to tests.
func DisableSignatureCache() {
	signatureCache.disable()
}

// ClearSignatureCache clears the signature cache.
func ClearSignatureCache() {
	signatureCache.disable()
	signatureCache.enable()
}

// EnableDischargesSignatureCache exposes  dischargeSignatureCache.enable()
// to tests.
func EnableDischargesSignatureCache() {
	dischargeSignatureCache.enable()
}

// DisableSignatureCache exposes dischargeSignatureCache.disable() to tests.
func DisableDischargesSignatureCache() {
	dischargeSignatureCache.disable()
}

// ExposeCertChains exposes Blessings.chains to tests.
func ExposeCertChains(b Blessings) [][]Certificate {
	return b.chains
}

// ExposeAppendCertChains exposes appending to Blessings.chains to tests.
func ExposeAppendCertChains(b *Blessings, chains [][]Certificate) {
	b.chains = append(b.chains, chains...)
}

// ExposeVerifySignatiure exposes signature verification to tests.
// ExposeVerifySignatiure verifies the signature of 'b'. Note that it does
// not verify caveats since caveats are always evaluated within the context
// of a security.Call.
func ExposeVerifySignature(b *Blessings) error {
	if len(b.chains) == 0 {
		return fmt.Errorf("empty certificate chain in blessings")
	}
	for _, chain := range b.chains {
		if len(chain) == 0 {
			return fmt.Errorf("empty certificate chain in blessings")
		}
		if _, _, err := validateCertificateChain(chain); err != nil {
			return err
		}
	}
	return nil
}

func SetCaveatValidationForTest(fn func(ctx *context.T, call Call, sets [][]Caveat) []error) func(ctx *context.T, call Call, sets [][]Caveat) []error {
	// For tests we skip the panic on multiple calls, so that we can easily revert
	// to the default validator.
	caveatValidationMu.Lock()
	ofn := caveatValidation
	caveatValidation = fn
	caveatValidationMu.Unlock()
	return ofn
}

// ExposeClaimedName exposes claimedName to tests.
func ExposeClaimedName(chain []Certificate) string {
	return claimedName(chain)
}

// ExposeChainedDigests exposes Certificate.chainedDigests tests.
func ExposeChainedDigests(c Certificate, h Hash, chain []byte) (digest, contentDigest []byte) {
	return c.chainedDigests(cryptoHash(h), chain)
}

// ExposeChainCertificate exposes chainCertificate to tests.
func ExposeChainCertificate(signer Signer, chain []Certificate, cert Certificate) ([]Certificate, []byte, error) {
	return chainCertificate(signer, chain, cert)
}

// ExposeValidateCertificateChain exposes validateCertificateChain to tests.
func ExposeValidateCertificateChain(chain []Certificate) (PublicKey, []byte, error) {
	return validateCertificateChain(chain)
}

// ExposeDigestsForCertificateChain exposes digestsForCertificateChain to tests.
func ExposeDigestsForCertificateChain(chain []Certificate) (digest, contentDigest []byte) {
	return digestsForCertificateChain(chain)
}

// ExposePublicKeyHashAlgo exposes the hashAlgo method on PublicKey, it is
// only required for openssl tests.
func ExposePublicKeyHashAlgo(pk PublicKey) crypto.Hash {
	return pk.hashAlgo()
}

// ExposeCryptoPublicKey exposes the underlying crypto.PublicKey method on PublicKey
// to tests.
func ExposeCryptoPublicKey(pk PublicKey) crypto.PublicKey {
	return pk.cryptoKey()
}

// ExposeECDSAHash exposes the Hash function used for a particular curve, it is
// only required for openssl tests.
func ExposeECDSAHash(key *ecdsa.PublicKey) Hash {
	nbits := key.Curve.Params().BitSize
	switch {
	case nbits <= 160:
		return SHA1Hash
	case nbits <= 256:
		return SHA256Hash
	case nbits <= 384:
		return SHA384Hash
	default:
		return SHA512Hash
	}
}

func ExposeX509Certificate(p Principal) *x509.Certificate {
	return p.(*principal).x509Cert
}

func ExposeFingerPrint(pk interface{}) string {
	return fingerprintCryptoPublicKey(pk)
}
