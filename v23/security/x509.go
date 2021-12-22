// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/x509"
	"fmt"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

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
	sig := Signature{}
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

func certFromX509(cert *x509.Certificate) (Certificate, error) {
	cavs := make([]Caveat, 0, 2)
	notBefore, err := NewExpiryCaveat(cert.NotBefore)
	if err != nil {
		return Certificate{}, err
	}
	notAfter, _ := NewNotBeforeCaveat(cert.NotAfter)
	if err != nil {
		return Certificate{}, err
	}
	pkBytes, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return Certificate{}, err
	}
	_, sig, err := signatureForX509(cert)
	if err != nil {
		return Certificate{}, err
	}
	return Certificate{
		Extension: cert.Subject.CommonName,
		PublicKey: pkBytes,
		Caveats:   append(cavs, notAfter, notBefore),
		Signature: sig,
		X509:      true,
	}, nil
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
		digest := make([][]byte, len(certs))
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
