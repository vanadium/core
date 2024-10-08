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

type X509KeySetID int

const (
	X509Private X509KeySetID = iota
	X509Encrypted
)

// VanadiumSSLData returns a selection of keys and certificates for hosts
// created for a self-signed CA.
// Keys are returned for ecdsa, rsa and ed25519 algorithms.
func VanadiumSSLData() (map[string]crypto.PrivateKey, map[string]*x509.Certificate, x509.VerifyOptions) {
	keys := map[string]crypto.PrivateKey{}
	certs := map[string]*x509.Certificate{}
	var intermediates [][]byte
	for _, typ := range SupportedKeyAlgos {
		host := typ.String()
		k, err := loadPrivateKey(fileContents(vanadiumSSLKeys, host+".vanadium.io.key"))
		if err != nil {
			panic(err)
		}
		c, intermediate, err := loadCerts(fileContents(vanadiumSSLCerts, host+".vanadium.io.crt"))
		if err != nil {
			panic(err)
		}
		keys[host] = k
		certs[host] = c[0]
		intermediates = append(intermediates, intermediate...)
	}
	var cert *x509.Certificate
	for _, c := range certs {
		cert = c
		break
	}
	opts, err := loadCA(cert, intermediates, [][]byte{vanadiumSSLCA})
	if err != nil {
		panic(err)
	}
	return keys, certs, opts
}

func X509VerifyOptions(typ keys.CryptoAlgo) x509.VerifyOptions {
	host := typ.String()
	cert, intermediates, err := loadCerts(fileContents(vanadiumSSLCerts, host+".vanadium.io.crt"))
	if err != nil {
		panic(err)
	}
	opts, err := loadCA(cert[0], intermediates, [][]byte{vanadiumSSLCA})
	if err != nil {
		panic(err)
	}
	return opts
}

func X509Certificate(typ keys.CryptoAlgo) *x509.Certificate {
	cert, _, err := loadCerts(fileContents(vanadiumSSLCerts, typ.String()+".vanadium.io.crt"))
	if err != nil {
		panic(err)
	}
	return cert[0]
}

func X509PublicKey(typ keys.CryptoAlgo) crypto.PublicKey {
	cert, _, err := loadCerts(fileContents(vanadiumSSLCerts, typ.String()+".vanadium.io.crt"))
	if err != nil {
		panic(err)
	}
	return cert[0].PublicKey
}

func X509PublicKeyBytes(typ keys.CryptoAlgo) []byte {
	return fileContents(vanadiumSSLCerts, typ.String()+".vanadium.io.crt")
}

func X509PrivateKey(typ keys.CryptoAlgo) crypto.PrivateKey {
	key, err := loadPrivateKey(fileContents(vanadiumSSLKeys, typ.String()+".vanadium.io.key"))
	if err != nil {
		panic(err)
	}
	return key
}

func X509PrivateKeyBytes(typ keys.CryptoAlgo, set X509KeySetID) []byte {
	filename := typ.String() + ".vanadium.io.key"
	if set == X509Encrypted {
		filename = "encrypted." + filename
	}
	return fileContents(vanadiumSSLKeys, filename)
}

func X509Signer(typ keys.CryptoAlgo) security.Signer {
	key, err := loadPrivateKey(fileContents(vanadiumSSLKeys, typ.String()+".vanadium.io.key"))
	if err != nil {
		panic(err)
	}
	signer, err := signerFromCryptoKey(key)
	if err != nil {
		panic(err)
	}
	return signer
}
