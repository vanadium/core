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

/*
// X509CertificateInfo represents a subset of the information in an x509
// certificate.
type X509CertificateInfo struct {
	PublicKey           security.PublicKey
	NotBefore, NotAfter time.Time
	Issuer, Subject     string
	X509Certificate     *x509.Certificate
}

// ParseX509CertificateFile parses an ssl/tls public key from the specified file.
func ParseX509CertificateFile(filename string, verifyOpts x509.VerifyOptions) (X509CertificateInfo, error) {
	f, err := os.Open(filename)
	if err != nil {
		return X509CertificateInfo{}, err
	}
	defer f.Close()
	return ParseX509Certificate(f, verifyOpts)
}

// ParseX509Certificate parses an ssl/tls public key from the specified io.Reader.
func ParseX509Certificate(rd io.Reader, verifyOpts x509.VerifyOptions) (X509CertificateInfo, error) {
	c, err := internal.LoadCertificate(rd)
	if err != nil {
		return X509CertificateInfo{}, fmt.Errorf("failed to parse x509.Certificate: %v", err)
	}
	chains, err := c.Verify(verifyOpts)
	if err != nil {
		return X509CertificateInfo{}, fmt.Errorf("failed to verify x509.Certificate: %v", err)
	}
	if len(chains) == 0 || len(chains[0]) == 0 {
		return X509CertificateInfo{}, fmt.Errorf("no verified chains in x509.Certificate")
	}
	cert := chains[0][0]
	if len(cert.Issuer.CommonName) == 0 {
		return X509CertificateInfo{}, fmt.Errorf("x509 certificate Issuer has no Common Name")
	}
	if len(cert.Subject.CommonName) == 0 {
		return X509CertificateInfo{}, fmt.Errorf("x509 certificate Subject has no Common Name")
	}
	if cert.PublicKey == nil {
		return X509CertificateInfo{}, fmt.Errorf("x509 certificate has no public key")
	}
	pk, err := NewPublicKey(cert.PublicKey)
	if err != nil {
		return X509CertificateInfo{}, fmt.Errorf("x509 public key type is not supported: %v", err)
	}

	return X509CertificateInfo{
		PublicKey:       pk,
		NotBefore:       cert.NotBefore,
		NotAfter:        cert.NotAfter,
		Issuer:          cert.Issuer.CommonName,
		Subject:         cert.Subject.CommonName,
		X509Certificate: cert,
	}, nil
}*/

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
