// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/x509"
	"fmt"
)

func (p *principal) blessSelfX509(host string, x509Cert *x509.Certificate, caveats []Caveat) (Blessings, error) {
	if p.signer == nil {
		return Blessings{}, fmt.Errorf("underlying signer is nil")
	}
	if !CryptoPublicKeyEqual(p.publicKey, x509Cert.PublicKey) {
		return Blessings{}, fmt.Errorf("public key associated with this principal and the x509 certificate differ")
	}

	certs, err := newUnsignedCertificateFromX509(host, x509Cert, caveats)
	if err != nil {
		return Blessings{}, err
	}
	if len(certs) == 0 {
		return Blessings{}, fmt.Errorf("failed to create any certificates based on the supplied x509 certificate")
	}
	chains := make([][]Certificate, len(certs))
	digests := make([][]byte, len(certs))
	for i := range certs {
		chains[i], digests[i], err = chainCertificate(p.signer, nil, certs[i])
		if err != nil {
			return Blessings{}, err
		}
	}
	ret := Blessings{
		chains:    chains,
		publicKey: p.PublicKey(),
		digests:   digests,
	}
	ret.init()
	return ret, nil
}

func newUnsignedCertificateFromX509(host string, x509Cert *x509.Certificate, caveats []Caveat) ([]Certificate, error) {
	cavs := make([]Caveat, len(caveats), len(caveats)+2)
	copy(cavs, caveats)
	notBefore, err := NewNotBeforeCaveat(x509Cert.NotBefore)
	if err != nil {
		return nil, err
	}
	notAfter, err := NewExpiryCaveat(x509Cert.NotAfter)
	if err != nil {
		return nil, err
	}
	pkBytes, err := x509.MarshalPKIXPublicKey(x509Cert.PublicKey)
	if err != nil {
		return nil, err
	}
	if len(host) == 0 {
		certs := make([]Certificate, len(x509Cert.DNSNames))
		for i, name := range x509Cert.DNSNames {
			certs[i] = Certificate{
				Extension: name,
				PublicKey: pkBytes,
				Caveats:   append(cavs, notAfter, notBefore),
				X509Raw:   x509Cert.Raw,
			}
		}
		return certs, nil
	}
	if err := x509Cert.VerifyHostname(host); err != nil {
		return nil, err
	}
	return []Certificate{{
		Extension: host,
		PublicKey: pkBytes,
		Caveats:   append(cavs, notAfter, notBefore),
		X509Raw:   x509Cert.Raw,
	}}, nil
}
