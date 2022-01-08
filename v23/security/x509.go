// Copyright 2021 The Vanadium Authors. All rights reserved.
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
