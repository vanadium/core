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
	"io"
	"os"
	"time"

	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
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
}

func splitASN1Sig(sig []byte) (R, S []byte) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	_ = input.ReadASN1(&inner, asn1.SEQUENCE) &&
		inner.ReadASN1Bytes(&R, asn1.INTEGER) &&
		inner.ReadASN1Bytes(&S, asn1.INTEGER)
	return
}

func SignatureForX509(cert *x509.Certificate) (security.PublicKey, security.Signature, error) {
	pk, err := NewPublicKey(cert.PublicKey)
	if err != nil {
		return nil, security.Signature{}, fmt.Errorf("failed to create public key of type %T, for cert issued by: %v: %v\n", cert.PublicKey, cert.Issuer, err)
	}
	sig := security.Signature{X509: true}
	switch cert.SignatureAlgorithm {
	case x509.SHA1WithRSA:
		sig.Hash = security.SHA1Hash
		sig.Rsa = cert.Signature
	case x509.SHA256WithRSA:
		sig.Hash = security.SHA256Hash
		sig.Rsa = cert.Signature
	case x509.SHA384WithRSA:
		sig.Hash = security.SHA384Hash
		sig.Rsa = cert.Signature
	case x509.SHA512WithRSA:
		sig.Hash = security.SHA512Hash
		sig.Rsa = cert.Signature
	case x509.ECDSAWithSHA1:
		sig.Hash = security.SHA1Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)
	case x509.ECDSAWithSHA256:
		sig.Hash = security.SHA256Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)

	case x509.ECDSAWithSHA384:
		sig.Hash = security.SHA384Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)

	case x509.ECDSAWithSHA512:
		sig.Hash = security.SHA512Hash
		sig.R, sig.S = splitASN1Sig(cert.Signature)

	case x509.PureEd25519:
		sig.Hash = security.SHA512Hash
		sig.Ed25519 = cert.Signature
	default:
		return nil, security.Signature{}, fmt.Errorf("unsupported signature algorithm for %v for cert issued by: %v: %v\n", cert.SignatureAlgorithm, cert.Issuer, err)
	}
	return pk, sig, nil
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
func NewPublicKey(key crypto.PublicKey) (security.PublicKey, error) {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	case *rsa.PublicKey:
		return security.NewRSAPublicKey(k), nil
	case ed25519.PublicKey:
		return security.NewED25519PublicKey(k), nil
	}
	return nil, fmt.Errorf("%T is an unsupported key type", key)
}
