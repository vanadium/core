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
	"v.io/x/ref/lib/security/keys"
)

var SupportedKeyAlgos = []keys.CryptoAlgo{
	keys.ECDSA256, keys.ECDSA384, keys.ECDSA521,
	keys.ED25519,
	keys.RSA2048, keys.RSA4096,
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

func loadCerts(data []byte) ([]*x509.Certificate, [][]byte, error) {
	certs := []*x509.Certificate{}
	pemBytes := make([][]byte, 0)
	rest := data
	prev := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, nil, fmt.Errorf("wrong PEM type, expected CERTIFICATE: %v", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse cert: %v", err)
		}
		certs = append(certs, cert)
		pemBytes = append(pemBytes, prev[:len(prev)-len(rest)])
		prev = rest
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no public certs")
	}
	return certs, pemBytes, nil
}

func loadCA(cert *x509.Certificate, intermediates [][]byte, roots [][]byte) (x509.VerifyOptions, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return x509.VerifyOptions{}, err
	}
	opts := x509.VerifyOptions{
		Roots: certPool,
	}
	if cert != nil {
		opts.CurrentTime = cert.NotBefore.Add(48 * time.Hour)
	}
	for _, root := range roots {
		if !certPool.AppendCertsFromPEM(root) {
			return x509.VerifyOptions{}, fmt.Errorf("failed to add root cert")
		}
	}
	if len(intermediates) > 1 {
		opts.Intermediates = x509.NewCertPool()
		for _, intermediate := range intermediates[1:] {
			if !opts.Intermediates.AppendCertsFromPEM(intermediate) {
				return x509.VerifyOptions{}, fmt.Errorf("failed to add intermediate cert: %s", intermediate)
			}
		}
	}
	return opts, nil
}

func prepopulatedDir(fs embed.FS, suffix, from string) (string, error) {
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

func mustBytesFromFS(fs embed.FS, dir, name string) []byte {
	data, err := fs.ReadFile(path.Join(dir, name))
	if err != nil {
		panic(fmt.Sprintf("failed to load %v/%v: %v", dir, name, err))
	}
	return data
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
	default:
		return nil, fmt.Errorf("unsupported key type: %v: %T", key, key)
	}
}

// CryptoSignerType eturns the types of the public keys associated with a signer.
// These will differ from the CryptoTypes when openssl is used to implement
// the signer.
func CryptoSignerType(kd keys.CryptoAlgo) (public string) {
	return publicSignerKeyTypes[kd]
}

func Password() []byte {
	return []byte("password")
}
