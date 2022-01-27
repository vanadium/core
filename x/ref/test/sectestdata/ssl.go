// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	_ "embed"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
)

//go:embed testdata/vanadium.io.ca.pem
var vanadiumSSLCA []byte

//go:embed testdata/*vanadium.io.key
var vanadiumSSLKeys embed.FS

//go:embed testdata/*vanadium.io.crt
var vanadiumSSLCerts embed.FS

// VanadiumSSLData returns a selection of keys and certificates for hosts
// created for a self-signed CA.
// Keys are returned for ecdsa, rsa and ed25519 algorithms.
func VanadiumSSLData() (map[string]crypto.PrivateKey, map[string]*x509.Certificate, x509.VerifyOptions) {
	keys := map[string]crypto.PrivateKey{}
	certs := map[string]*x509.Certificate{}
	for _, typ := range SupportedKeyAlgos {
		host := typ.String()
		k, err := keyFromFS(vanadiumSSLKeys, "testdata", host+".vanadium.io.key")
		if err != nil {
			panic(err)
		}
		c, err := certFromFS(vanadiumSSLCerts, "testdata", host+".vanadium.io.crt")
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
	opts, err := loadCA(cert, vanadiumSSLCA)
	if err != nil {
		panic(err)
	}
	return keys, certs, opts
}

func X509PublicKey(typ keys.CryptoAlgo) crypto.PublicKey {
	cert, err := certFromFS(vanadiumSSLCerts, "testdata", typ.String()+".vanadium.io.crt")
	if err != nil {
		panic(err)
	}
	return cert[0].PublicKey
}

func X509PrivateKey(typ keys.CryptoAlgo) crypto.PrivateKey {
	key, err := keyFromFS(vanadiumSSLKeys, "testdata", typ.String()+".vanadium.io.key")
	if err != nil {
		panic(err)
	}
	return key
}

func X509PrivateKeyBytes(typ keys.CryptoAlgo) []byte {
	return fileContents(vanadiumSSLKeys, typ.String()+".vanadium.io.key")
}

func X509Signer(typ keys.CryptoAlgo) security.Signer {
	key, err := keyFromFS(vanadiumSSLKeys, "testdata", typ.String()+".vanadium.io.key")
	if err != nil {
		panic(err)
	}
	signer, err := signerFromCryptoKey(key)
	if err != nil {
		panic(err)
	}
	return signer
}
