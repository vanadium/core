// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	_ "embed"
)

//go:embed testdata/vanadium.io.ca.pem
var vanadiumCA []byte

//go:embed testdata/*vanadium.io.key
var vanadiumKeys embed.FS

//go:embed testdata/*vanadium.io.crt
var vanadiumCerts embed.FS

// VanadiumSSLData returns a selection of keys and certificates for hosts
// created for a self-signed CA.
// Keys are returned for ecdsa, rsa and ed25519 algorithms.
func VanadiumSSLData() (map[string]crypto.PrivateKey, map[string]*x509.Certificate, x509.VerifyOptions) {
	keys := map[string]crypto.PrivateKey{}
	certs := map[string]*x509.Certificate{}
	for _, host := range []string{"ec256", "rsa2048", "rsa4096", "ed25519"} {
		k, err := keyFromFS(vanadiumKeys, "testdata", host+".vanadium.io.key")
		if err != nil {
			panic(err)
		}
		c, err := certFromFS(vanadiumCerts, "testdata", host+".vanadium.io.crt")
		if err != nil {
			panic(err)
		}
		keys[host] = k
		certs[host] = c[0]
	}
	var cert *x509.Certificate
	for _, c := range certs {
		cert = c
		break
	}
	opts, err := loadCA(cert, vanadiumCA)
	if err != nil {
		panic(err)
	}
	return keys, certs, opts
}
