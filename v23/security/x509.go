// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/x509"
)

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
