// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"v.io/v23/security"
)

var SupportedKeyTypes = []KeyType{
	ECDSA256, ECDSA384, ECDSA521,
	ED25519,
	RSA2048, RSA4096,
}

// KeyType represents the key types supported by this package, the equivalent
// type in lib/security is not used to avoid import cycles.
type KeyType int

// Supported key types.
const (
	UnsupportedKeyType KeyType = iota
	ECDSA256
	ECDSA384
	ECDSA521
	ED25519
	RSA2048
	RSA4096
)

func (kt KeyType) String() string {
	switch kt {
	case ECDSA256:
		return "ecdsa-256"
	case ECDSA384:
		return "ecdsa-384"
	case ECDSA521:
		return "ecdsa-521"
	case ED25519:
		return "ed25519"
	case RSA2048:
		return "rsa-2048"
	case RSA4096:
		return "rsa-4096"
	}
	return "unknown"
}

func loadPrivateKey(data []byte) (crypto.PrivateKey, error) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(block.Bytes)
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(block.Bytes)
		case "PRIVATE KEY":
			return x509.ParsePKCS8PrivateKey(block.Bytes)
		default:
			if strings.Contains(block.Type, "PARAMETERS") {
				continue
			}
			return nil, fmt.Errorf("wrong PEM type, expected EC PRIVATE KEY: %v", block.Type)
		}
	}
	return nil, fmt.Errorf("failed to load private key")
}

func loadCerts(data []byte) ([]*x509.Certificate, error) {
	certs := []*x509.Certificate{}
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			panic(fmt.Sprintf("wrong PEM type, expected CERTIFICATE: %v", block.Type))
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			panic(err)
		}
		certs = append(certs, cert)
	}

	if len(certs) == 0 {
		return nil, fmt.Errorf("no public certs")
	}
	return certs, nil
}

func loadCA(cert *x509.Certificate, data []byte) (x509.VerifyOptions, error) {
	block, _ := pem.Decode(data)
	if block.Type != "CERTIFICATE" {
		return x509.VerifyOptions{}, fmt.Errorf("wrong PEM type, expected CERTIFICATE: %v", block.Type)
	}
	rootCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return x509.VerifyOptions{}, err
	}
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return x509.VerifyOptions{}, err
	}
	certPool.AddCert(rootCert)
	opts := x509.VerifyOptions{
		Roots: certPool,
	}
	if cert != nil {
		opts.CurrentTime = cert.NotBefore.Add(48 * time.Hour)
	}
	return opts, nil
}

func prepopulatedDir(suffix, from string, fs embed.FS) (string, error) {
	to, err := os.MkdirTemp("", suffix)
	if err != nil {
		return "", err
	}
	if err := copyFS(fs, from, to); err != nil {
		os.RemoveAll(to)
		return "", err
	}
	return to, nil
}

func copyFS(fs embed.FS, fromDir, toDir string) error {
	names, err := fs.ReadDir(fromDir)
	if err != nil {
		return err
	}
	for _, name := range names {
		from := path.Join(fromDir, name.Name())
		to := filepath.Join(toDir, name.Name())
		contents, err := fs.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, contents, 0600); err != nil {
			return err
		}
	}
	return nil
}

func keyFromFS(fs embed.FS, dir, name string) (crypto.PrivateKey, error) {
	data, err := fs.ReadFile(path.Join(dir, name))
	if err != nil {
		return nil, err
	}
	return loadPrivateKey(data)
}

func certFromFS(fs embed.FS, dir, name string) ([]*x509.Certificate, error) {
	data, err := fs.ReadFile(path.Join(dir, name))
	if err != nil {
		return nil, err
	}
	return loadCerts(data)
}

func fileContents(fs embed.FS, filename string) []byte {
	data, err := fs.ReadFile(path.Join("testdata", filename))
	if err != nil {
		panic(err)
	}
	return data
}

func signerFromCryptoKey(key crypto.PrivateKey) (security.Signer, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		return security.NewInMemoryRSASigner(k)
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(k)
	case ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(k)
	case *ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(*k)
	default:
		return nil, fmt.Errorf("unsupported key type: %v: %T", key, key)
	}
}
