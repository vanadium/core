// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys

import (
	"bytes"
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
)

func MustRegisterCommon(r *Registrar) {
	if err := RegisterCommon(r); err != nil {
		panic(err)
	}
}

func RegisterCommon(r *Registrar) error {
	r.RegisterPrivateKeyMarshaler(MarshalBuiltinPrivateKey,
		(*rsa.PrivateKey)(nil), (*ecdsa.PrivateKey)(nil), (ed25519.PrivateKey)(nil))
	r.RegisterPublicKeyMarshaler(MarshalPKIXPublicKey,
		(*rsa.PublicKey)(nil), (*ecdsa.PublicKey)(nil), (ed25519.PublicKey)(nil))

	r.RegisterPublicKeyParser(ParsePKIXPublicKey, "PUBLIC KEY", nil)
	r.RegisterPrivateKeyParser(ParseECPrivateKey, "EC PRIVATE KEY", nil)
	r.RegisterPrivateKeyParser(ParsePKCS8PrivateKey, "PRIVATE KEY", nil)

	r.RegisterDecrypter(DecryptPEMBlock, "EC PRIVATE KEY", x509.IsEncryptedPEMBlock)
	r.RegisterDecrypter(DecryptPEMBlock, "PRIVATE KEY", x509.IsEncryptedPEMBlock)
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
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	case *rsa.PublicKey:
		return security.NewRSAPublicKey(k), nil
	case ed25519.PublicKey:
		return security.NewED25519PublicKey(k), nil
	case *ecdsa.PrivateKey:
		return security.NewECDSAPublicKey(&k.PublicKey), nil
	case *rsa.PrivateKey:
		return security.NewRSAPublicKey(&k.PublicKey), nil
	case ed25519.PrivateKey:
		return security.NewED25519PublicKey(k.Public().(ed25519.PublicKey)), nil
	}
	return nil, fmt.Errorf("keys.PublicKey: unsupported key type %T", key)
}

func MarshalPKIXPublicKey(key crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	return pemEncode("PUBLIC KEY", der)
}

func MarshalBuiltinPrivateKey(key crypto.PrivateKey, passphrase []byte) ([]byte, error) {
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
		return pemEncode("ENCRYPTED PRIVATE KEY", data)
	}
	var der []byte
	var err error
	typ := "PRIVATE KEY"
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		der, err = x509.MarshalECPrivateKey(k)
		typ = "EC PRIVATE KEY"
	case ed25519.PrivateKey:
		der, err = x509.MarshalPKCS8PrivateKey(k)
	case *rsa.PrivateKey:
		der, err = x509.MarshalPKCS8PrivateKey(k)
	default:
		return nil, fmt.Errorf("unsupported type: %T", key)
	}
	if err != nil {
		return nil, err
	}
	return pemEncode(typ, der)
}

func ParsePKIXPublicKey(block *pem.Block) (crypto.PublicKey, error) {
	return x509.ParsePKIXPublicKey(block.Bytes)
}

func ParseECPrivateKey(block *pem.Block) (crypto.PrivateKey, error) {
	return x509.ParseECPrivateKey(block.Bytes)
}

func ParsePKCS8PrivateKey(block *pem.Block) (crypto.PrivateKey, error) {
	return x509.ParsePKCS8PrivateKey(block.Bytes)
}

func pemEncode(typ string, der []byte) ([]byte, error) {
	var out bytes.Buffer
	block := pem.Block{
		Type:  typ,
		Bytes: der,
	}
	if err := pem.Encode(&out, &block); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type ErrPassphraseRequired struct{}

func (*ErrPassphraseRequired) Error() string {
	return "passphrase required"
}

type ErrBadPassphrase struct{}

func (*ErrBadPassphrase) Error() string {
	return "bad passphrase"
}

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
