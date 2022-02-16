// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package keys provides support for working with an extensible set of
// cryptographic keys. Parsing and marshaling of different key formats is
// supported, including the ability to use external agents to host private
// keys and perform key signing operations.
//
// Extensibility is provided via a registration mechanism (keys.Registrar)
// that allows for parsing operations to be associated with PEM block types
// and non-PEM, plain text, data. Marshaling operations are associated with
// types and a common API interface can also be registered for specific types.
//
// A specific PEM type, keys.PrivateKeyPEMType, is used to support 'indirect'
// private keys where an indirect key is one whose PEM encoding refers to
// an external file or agent. This allows for private keys to be hosted
// by ssh agents for example, or indeed other services such as AWS' secrets
// service. Other than support for ssh agents, other signing services are not
// implemented in this package in order to reduce dependencies. Multiple
// indirections are via a PEM header that is used to identify a specific
// parser to invoke.
package keys

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"v.io/v23/security"
)

// CryptoAlgo represents the supported cryptographic algorithms.
type CryptoAlgo int

// Supported key types.
const (
	UnsupportedAlgoType CryptoAlgo = iota
	ECDSA256
	ECDSA384
	ECDSA521
	ED25519
	RSA2048
	RSA4096
)

func (algo CryptoAlgo) String() string {
	switch algo {
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

// NewPrivateKeyForAlgo creates a new private key for the requested algorithm.
func NewPrivateKeyForAlgo(algo CryptoAlgo) (crypto.PrivateKey, error) {
	switch algo {
	case ECDSA256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case ECDSA384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case ECDSA521:
		return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	case ED25519:
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		return privateKey, err
	case RSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case RSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	default:
		return nil, fmt.Errorf("keys.NewPrivateKeyForAlgo: unsupported key type: %T", algo)
	}
}

// PublicKey returns a security.PublicKey for the supplied public or private key.
// It accepts *ecdsa.PublicKey, *rsa.PublicKey, ed25519.PublicKey and
// *ecdsa.PrivateKey, *rsa.PrivateKey, ed25519.PrivateKey.
func PublicKey(key interface{}) (security.PublicKey, error) {
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

// ZeroPassphrase overwrites the passphrase.
func ZeroPassphrase(pass []byte) {
	for i := range pass {
		pass[i] = 0
	}
}
