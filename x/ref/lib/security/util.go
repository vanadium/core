// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"

	"v.io/v23/security"
	"v.io/v23/verror"
)

// NewPrivateKey creates a new security.Signer of the requested type.
// keyType must be one of ecdsa256, ecdsa384, ecdsa521 or ed25519.
func NewPrivateKey(keyType string) (interface{}, error) {
	var privateKey interface{}
	var err error
	switch keyType {
	case "ecdsa256":
		privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "ecdsa384":
		privateKey, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "ecdsa521":
		privateKey, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	case "ed25519":
		_, privateKey, err = ed25519.GenerateKey(rand.Reader)
	default:
		return nil, fmt.Errorf("unsupported key type: %v", keyType)
	}
	return privateKey, err
}

// NewECDSAKeyPair generates an ECDSA (public, private) key pair.
func NewECDSAKeyPair() (security.PublicKey, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, verror.New(errCantGenerateKey, nil, err)
	}
	return security.NewECDSAPublicKey(&priv.PublicKey), priv, nil
}
