// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package keyfile provides a signing service that uses keys read from files
// keys.
package keyfile

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing"
)

type keyfile struct{}

func NewSigningService() signing.Service {
	return &keyfile{}
}

func signerFromKey(key interface{}) (security.Signer, error) {
	switch v := key.(type) {
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(v)
	case ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(v)
	case *rsa.PrivateKey:
		return security.NewInMemoryRSASigner(v)
	default:
		return nil, fmt.Errorf("unsupported signing key type %T", key)
	}
}

// Signer implements v.io/ref/lib/security.SigningService.
// It expects the keyBytes to be a PEM block.
func (kf *keyfile) Signer(ctx context.Context, pemBytes []byte, passphrase []byte) (security.Signer, error) {
	key, err := internal.ParsePEMPrivateKey(pemBytes, passphrase)
	if err != nil {
		return nil, err
	}
	return signerFromKey(key)
}

// Close implements v.io/ref/lib/security.SigningService.
func (kf *keyfile) Close(ctx context.Context) error {
	return nil
}
