// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"

	"v.io/v23/security"
	"v.io/v23/vom"
)

// Circuitious route to obtain the certificate chain because the use
// of security.MarshalBlessings is discouraged.
func RootCertificateDetails(b security.Blessings) (string, []byte, error) {
	data, err := vom.Encode(b)
	if err != nil {
		return "", nil, fmt.Errorf("malformed Blessings: %v", err)
	}
	var wire security.WireBlessings
	if err := vom.Decode(data, &wire); err != nil {
		return "", nil, fmt.Errorf("malformed WireBlessings: %v", err)
	}
	cert := wire.CertificateChains[0][0]
	return cert.Extension, cert.PublicKey, nil
}
