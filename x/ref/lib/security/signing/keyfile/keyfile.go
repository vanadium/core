// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package keyfile provides a signing service that uses files to store
// keys. Currently .pem and ssh private key file formats are supported.
package keyfile

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"fmt"
	"os"
	"strings"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing"
)

// TODO(cnicolaou): this implementation anticipates forthcoming changes
//   to use context.Context and to simplify verror. The TODO is to transition
//   to their replacements, for now use context.Context and fmt.Errorf
//   directly in some cases.

type keyfile struct {
}

func NewSigningService() signing.Service {
	return &keyfile{}
}

func determineSigner(key interface{}) (security.Signer, error) {
	switch v := key.(type) {
	case *ecdsa.PrivateKey:
		return security.NewInMemoryECDSASigner(v)
	case ed25519.PrivateKey:
		return security.NewInMemoryED25519Signer(v)
	default:
		return nil, fmt.Errorf("unsupported signing key type %T", key)
	}
}

// Signer implements v.io/ref/lib/security.SigningService.
// The suffix for keyFile determines how the file is parsed:
//   - .pem for PEM files
func (kf *keyfile) Signer(ctx context.Context, keyFile string, passphrase []byte) (security.Signer, error) {
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if strings.HasSuffix(keyFile, ".pem") {
		key, err := internal.LoadPEMPrivateKey(f, passphrase)
		if err != nil {
			return nil, err
		}
		return determineSigner(key)
	}
	return nil, fmt.Errorf("unrecognised file suffix: %v, currently only pem files are supported", keyFile)
}

// Close implements v.io/ref/lib/security.SigningService.
func (kf *keyfile) Close(ctx context.Context) error {
	return nil
}
