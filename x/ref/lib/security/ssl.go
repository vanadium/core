// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"regexp"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
)

// ParsePEMPrivateKeyFile parses an pem format private key from the specified file.
func ParsePEMPrivateKeyFile(keyFile string, passphrase []byte) (crypto.PrivateKey, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := internal.LoadPEMPrivateKey(f, passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to load: %v: %v", keyFile, err)
	}
	return key, nil
}

// ParseX509CertificateFile parses an ssl/tls public key from the specified file.
func ParseX509CertificateFile(filename string) ([]*x509.Certificate, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pemBlocks, err := internal.ReadPEMBlocks(f, regexp.MustCompile("^CERTIFICATE$"))
	if err != nil {
		return nil, err
	}
	certs := make([]*x509.Certificate, 0, 1)
	for _, pemBlock := range pemBlocks {
		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

// NewInMemorySigner creates a new security.Signer that stores its
// private key in memory using the security.NewInMemory{ECDSA,ED25519,RSA}Signer
// methods.
func NewInMemorySigner(key crypto.PrivateKey) (security.Signer, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(k)
	case *rsa.PrivateKey:
		return security.NewInMemoryRSASigner(k)
	case ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(k)
	case *ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(*k)
	}
	return nil, fmt.Errorf("%T is an unsupported key type", key)
}

// NewPublicKey creates a new security.PublicKey for the supplied
// crypto.PublicKey.
var NewPublicKey = security.NewPublicKey
