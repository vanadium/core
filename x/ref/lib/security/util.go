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
	"os"

	"v.io/v23/security"
	"v.io/v23/verror"
)

// DefaultSSHAgentSockNameFunc can be overriden to return the address of a custom
// ssh agent to use instead of the one specified by SSH_AUTH_SOCK. This is
// primarily intended for tests.
var DefaultSSHAgentSockNameFunc = func() string {
	return os.Getenv("SSH_AUTH_SOCK")
}

// NewPrivateKey creates a new private key of the requested type.
// keyType must be one of ecdsa256, ecdsa384, ecdsa521 or ed25519.
func NewPrivateKey(keyType string) (interface{}, error) {
	switch keyType {
	case "ecdsa256":
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "ecdsa384":
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "ecdsa521":
		return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	case "ed25519":
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		return privateKey, err
	default:
		return nil, fmt.Errorf("unsupported key type: %v", keyType)
	}
}

// NewECDSAKeyPair generates an ECDSA (public, private) key pair.
func NewECDSAKeyPair() (security.PublicKey, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, verror.New(errCantGenerateKey, nil, err)
	}
	return security.NewECDSAPublicKey(&priv.PublicKey), priv, nil
}
