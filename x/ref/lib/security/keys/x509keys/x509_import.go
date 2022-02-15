// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509keys

import (
	"crypto/x509"

	"v.io/x/ref/lib/security/keys/internal"
)

func parseFirstCertificate(pemBytes []byte) (*x509.Certificate, error) {
	block, err := internal.DecodePEM(pemBytes, "CERTIFICATE")
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(block.Bytes)
}

// ImportPublicKeyBytes returns the byte representation for an imported.
// x509/ssl public key.
func ImportPublicKeyBytes(publicKeyBytes []byte) ([]byte, error) {
	// Read the first certificate and then re-encode just that one cert.
	cert, err := parseFirstCertificate(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	return internal.EncodePEM("CERTIFICATE", cert.Raw, nil)
}
