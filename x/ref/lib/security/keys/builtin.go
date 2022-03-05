// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/youmark/pkcs8"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys/internal"
)

// MustRegister is like Register but panics on error.
func MustRegister(r *Registrar) {
	if err := Register(r); err != nil {
		panic(err)
	}
}

// Register registers the required functions for handling the Vanadium built
// in key types and commonly used formats. These are currently the crypto/ecsda,
// crypto/ed25519 and crypto/rsa algorithms with parsers provided for variety
// of PEM block types. PEM and PKCS8 decryption are supported but PCKS8 is
// is used for encryption.
func Register(r *Registrar) error {
	r.RegisterPrivateKeyMarshaler(MarshalPKCS8PrivateKey,
		(*rsa.PrivateKey)(nil), (*ecdsa.PrivateKey)(nil), (ed25519.PrivateKey)(nil))
	r.RegisterPublicKeyMarshaler(MarshalPKIXPublicKey,
		(*rsa.PublicKey)(nil), (*ecdsa.PublicKey)(nil), (ed25519.PublicKey)(nil))

	r.RegisterPublicKeyParser(ParsePKIXPublicKey, "PUBLIC KEY", nil)
	r.RegisterPrivateKeyParser(ParseECPrivateKey, "EC PRIVATE KEY", nil)
	r.RegisterPrivateKeyParser(ParsePKCS8PrivateKey, "PRIVATE KEY", nil)

	r.RegisterDecrypter(DecryptPEMBlock, "EC PRIVATE KEY", x509.IsEncryptedPEMBlock) //nolint:staticcheck
	r.RegisterDecrypter(DecryptPEMBlock, "PRIVATE KEY", x509.IsEncryptedPEMBlock)    //nolint:staticcheck
	r.RegisterDecrypter(DecryptPKCS8Block, "ENCRYPTED PRIVATE KEY", func(*pem.Block) bool { return true })

	return r.RegisterAPI((*api)(nil),
		(*rsa.PrivateKey)(nil), (*ecdsa.PrivateKey)(nil), (ed25519.PrivateKey)(nil),
		(*rsa.PublicKey)(nil), (*ecdsa.PublicKey)(nil), (ed25519.PublicKey)(nil),
	)
}

type api struct{}

func (*api) Signer(ctx context.Context, key crypto.PrivateKey) (security.Signer, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(k)
	case *rsa.PrivateKey:
		return security.NewInMemoryRSASigner(k)
	case ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(k)
	}
	return nil, fmt.Errorf("Signer: unsupported key type %T", key)
}

func (*api) PublicKey(key interface{}) (security.PublicKey, error) {
	return security.NewPublicKey(key)
}

func (*api) CryptoPublicKey(key interface{}) (crypto.PublicKey, error) {
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		return k.Public(), nil
	case *rsa.PrivateKey:
		return k.Public(), nil
	case ed25519.PrivateKey:
		return k.Public(), nil
	case *ecdsa.PublicKey:
		return key, nil
	case *rsa.PublicKey:
		return key, nil
	case ed25519.PublicKey:
		return key, nil
	}
	return nil, fmt.Errorf("CryptoPublicKey: unsupported key type %T", key)
}

// MarshalPKIXPublicKey uses MarshalPKIXPublicKey to marshal the key
// to PEM block of type 'PUBLIC 'KEY'.
func MarshalPKIXPublicKey(key crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	return internal.EncodePEM("PUBLIC KEY", der, nil)
}

// MarshalPKCS8PrivateKey uses x509.MarshalPKCS8PrivateKey to marshal
// the key to PEM block type of 'PRIVATE KEY'. If a passphrase is provided
// the key will be encrypted using PKCS8 rather than PEM..
func MarshalPKCS8PrivateKey(key crypto.PrivateKey, passphrase []byte) ([]byte, error) {
	if len(passphrase) != 0 {
		data, err := pkcs8.MarshalPrivateKey(key, passphrase, &pkcs8.Opts{
			Cipher: pkcs8.AES256CBC,
			KDFOpts: pkcs8.PBKDF2Opts{
				SaltSize:       8,
				IterationCount: 10000,
				HMACHash:       crypto.SHA256,
			},
		})
		if err != nil {
			return nil, err
		}
		return internal.EncodePEM("ENCRYPTED PRIVATE KEY", data, nil)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return internal.EncodePEM("PRIVATE KEY", der, nil)
}

// ParsePKIXPublicKey calls x509.ParsePKIXPublicKey with block.Bytes.
func ParsePKIXPublicKey(block *pem.Block) (crypto.PublicKey, error) {
	return x509.ParsePKIXPublicKey(block.Bytes)
}

// ParseECPrivateKey calls x509.ParseECPrivateKey with block.Bytes.
func ParseECPrivateKey(block *pem.Block) (crypto.PrivateKey, error) {
	return x509.ParseECPrivateKey(block.Bytes)
}

// ParsePKCS8PrivateKey calls x509.ParsePKCS8PrivateKey with block.Bytes.
func ParsePKCS8PrivateKey(block *pem.Block) (crypto.PrivateKey, error) {
	return x509.ParsePKCS8PrivateKey(block.Bytes)
}

// ErrPassphraseRequired is returned when an attempt is made to read
// an encrypted block without a passphrase.
type ErrPassphraseRequired struct{}

func (*ErrPassphraseRequired) Error() string {
	return "passphrase required"
}

// ErrBadPassphrase is returned when the supplied passphrase is unable to
// decrypt the key.
type ErrBadPassphrase struct{}

func (*ErrBadPassphrase) Error() string {
	return "bad passphrase"
}

// DecryptPEMBlock decrypts an encrypted PEM block.
// Deprecated: use PKCS8 encryption instead.
func DecryptPEMBlock(block *pem.Block, passphrase []byte) (*pem.Block, crypto.PrivateKey, error) {
	if !x509.IsEncryptedPEMBlock(block) { //nolint:staticcheck
		return block, nil, nil
	}
	if len(passphrase) == 0 {
		return nil, nil, &ErrPassphraseRequired{}
	}
	data, err := x509.DecryptPEMBlock(block, passphrase) //nolint:staticcheck
	if err != nil {
		return nil, nil, &ErrBadPassphrase{}
	}
	pb := &pem.Block{
		Type:    block.Type,
		Bytes:   data,
		Headers: block.Headers,
	}
	return pb, nil, nil
}

// DecryptPKCS8Block decrypts a private key encrypted using PKCS8. The block's
// PEM type must be "ENCRYPTED PRIVATE KEY".
func DecryptPKCS8Block(block *pem.Block, passphrase []byte) (*pem.Block, crypto.PrivateKey, error) {
	if len(passphrase) == 0 && block.Type == "ENCRYPTED PRIVATE KEY" {
		return nil, nil, &ErrPassphraseRequired{}
	}
	key, _, err := pkcs8.ParsePrivateKey(block.Bytes, passphrase)
	if err != nil && strings.Contains(err.Error(), "incorrect password") {
		return nil, nil, &ErrBadPassphrase{}
	}
	if err != nil {
		return nil, nil, err
	}
	return nil, key, nil
}
