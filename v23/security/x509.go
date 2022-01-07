// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/x509"
	"strings"
)

func sanitizeExtensionString(s string) string {
	for _, illegal := range invalidBlessingSubStrings {
		s = strings.ReplaceAll(s, illegal, "")
	}
	return s
}

// Subject Alternative Name (SAN) mechanism - may return multiple certificates.
// wildcard ssl certs
func newUnsignedCertificateFromX509(x509Cert *x509.Certificate, pkBytes []byte, caveats []Caveat) ([]Certificate, error) {
	cavs := make([]Caveat, len(caveats), len(caveats)+2)
	copy(cavs, caveats)
	notBefore, err := NewNotBeforeCaveat(x509Cert.NotBefore)
	if err != nil {
		return nil, err
	}
	notAfter, _ := NewExpiryCaveat(x509Cert.NotAfter)
	if err != nil {
		return nil, err
	}
	cert := Certificate{
		Extension: sanitizeExtensionString(x509Cert.Subject.CommonName),
		PublicKey: pkBytes,
		Caveats:   append(cavs, notAfter, notBefore),
		X509Raw:   x509Cert.Raw,
	}
	return []Certificate{cert}, nil
}

/*
func splitASN1Sig(sig []byte) (R, S []byte) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	_ = input.ReadASN1(&inner, asn1.SEQUENCE) &&
		inner.ReadASN1Bytes(&R, asn1.INTEGER) &&
		inner.ReadASN1Bytes(&S, asn1.INTEGER)
	return
}

func signatureForX509(cert *x509.Certificate) (PublicKey, Signature, error) {
	pk, err := NewPublicKey(cert.PublicKey)
	if err != nil {
		return nil, Signature{}, fmt.Errorf("failed to create public key of type %T, for cert issued by: %v: %v\n", cert.PublicKey, cert.Issuer, err)
	}
	sig := Signature{
		Purpose: []byte(SignatureForBlessingCertificates),
	}
	switch cert.SignatureAlgorithm {
	case x509.SHA1WithRSA:
		sig.Hash = SHA1Hash
		sig.Rsa = cert.Signature
	case x509.SHA256WithRSA:
		sig.Hash = SHA256Hash
		sig.Rsa = cert.Signature
	case x509.SHA384WithRSA:
		sig.Hash = SHA384Hash
		sig.Rsa = cert.Signature
	case x509.SHA512WithRSA:
		sig.Hash = SHA512Hash
		sig.Rsa = cert.Signature
	case x509.ECDSAWithSHA1:
		sig.Hash = SHA1Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)
	case x509.ECDSAWithSHA256:
		sig.Hash = SHA256Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)
	case x509.ECDSAWithSHA384:
		sig.Hash = SHA384Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)
	case x509.ECDSAWithSHA512:
		sig.Hash = SHA512Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)
	case x509.PureEd25519:
		sig.Hash = SHA512Hash
		sig.Ed25519 = cert.Signature
	default:
		return nil, Signature{}, fmt.Errorf("unsupported signature algorithm for %v for cert issued by: %v: %v\n", cert.SignatureAlgorithm, cert.Issuer, err)
	}
	return pk, sig, nil
}


func x509TBS(der []byte) ([]byte, error) {
	input := cryptobyte.String(der)
	// we read the SEQUENCE including length and tag bytes so that
	// we can populate Certificate.Raw, before unwrapping the
	// SEQUENCE so it can be operated on
	if !input.ReadASN1Element(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed certificate")
	}
	if !input.ReadASN1(&input, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed certificate")
	}

	var tbs cryptobyte.String
	// do the same trick again as above to extract the raw
	// bytes for Certificate.RawTBSCertificate
	if !input.ReadASN1Element(&tbs, cryptobyte_asn1.SEQUENCE) {
		return nil, errors.New("x509: malformed tbs certificate")
	}
	return tbs, nil
}*/

/*
func BlessingFromX509(x509Cert *x509.Certificate) (Blessings, error) {
	unsigned, err := newUnsignedCertificateFromX509(x509Cert)
	if err != nil {
		return Blessings{}, err
	}
	chains := make([][]Certificate, 1, 1)
	chains[0] = make([]Certificate, 1, 1)
	chains[0][0] = cert
	digest := make([][]byte, len(certs))
	digest[0], _ = certs[0].chainedDigests(cryptoHash(certs[0].Signature.Hash)

	blessings := Blessings{
		chains:    chains,
		digests:   chainedDigests,
		publicKey: pk,
	}
	blessings.init()
	return blessings, nil
}

// chain must have been obtained from x509.Certificate.Verify
func chainFromX509(x509Certs []*x509.Certificate) ([]Certificate, error) {
	certs := make([]Certificate, 0, len(x509Certs))
	for _, c := range x509Certs {
		cert, err := certFromX509(c)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func BlessingsForX509Chains(pk PublicKey, x509chains [][]*x509.Certificate) (Blessings, error) {
	chains := make([][]Certificate, 0, len(x509chains))
	chainedDigests := make([][]byte, 0, len(x509chains))
	for _, chain := range x509chains {
		certs, err := chainFromX509(chain)
		if err != nil {
			return Blessings{}, err
		}
		if len(certs) == 0 {
			continue
		}
		digest := make([][]byte, 1)
		digest[0], _ = certs[0].chainedDigests(cryptoHash(certs[0].Signature.Hash), nil)
		for i := 1; i < len(certs); i++ {
			digest[i], _ = certs[i].chainedDigests(cryptoHash(certs[i].Signature.Hash), digest[i-1])
		}
		chains = append(chains, certs)
		chainedDigests = append(chainedDigests, digest[len(digest)-1])
	}
	blessings := Blessings{
		chains:    chains,
		digests:   chainedDigests,
		publicKey: pk,
	}
	blessings.init()
	return blessings, nil
}
*/
