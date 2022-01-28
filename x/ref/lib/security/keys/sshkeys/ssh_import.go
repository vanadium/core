// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"context"
	"fmt"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
)

// ImportOption represents an option to MarshalForImport.
type ImportOption func(o *importOptions)

type importOptions struct {
	keyBytes       []byte
	origPassphrase []byte
	newPassphrase  []byte
	keyFilename    string
	agent          bool
}

// ImportPrivateKeyBytes will result in the supplied bytes being used
// as the private key, that is, those bytes will be copied to the
// Vanadium principal.
func ImportPrivateKeyBytes(keyBytes []byte, origPassphrase, newPassphrase []byte) ImportOption {
	return func(o *importOptions) {
		o.keyBytes = keyBytes
		o.origPassphrase = origPassphrase
		o.newPassphrase = newPassphrase
	}
}

// ImportPrivateKeyFile will result in the private key in the specified
// file being used. The Vanadium principal will refer to that file and
// not copy it.
func ImportPrivateKeyFile(filename string) ImportOption {
	return func(o *importOptions) {
		o.keyFilename = filename

	}
}

// ImportUsingAgent requests that the private key is hosted by an ssh agent.
// The
func ImportUsingAgent(v bool) ImportOption {
	return func(o *importOptions) {
		o.agent = v
	}
}

// MarshalForImport will marshal the supplied public key and options to
// an appropriate format for use by a Vanadium principal.
func MarshalForImport(ctx context.Context, publicKeyBytes []byte, options ...ImportOption) (importedPublicKeyBytes, importedPrivateKeyBytes []byte, err error) {
	opts := importOptions{}
	for _, fn := range options {
		fn(&opts)
	}
	defer func() {
		keys.ZeroPassphrase(opts.origPassphrase)
		keys.ZeroPassphrase(opts.newPassphrase)
	}()

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey(publicKeyBytes)
	if err != nil {
		return
	}

	if opts.agent {
		hostedKey := NewHostedKey(publicKey, comment)
		privKeyBytes, err := marshalHostedKey(hostedKey, nil)
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	if filename := opts.keyFilename; len(filename) > 0 {
		privKeyBytes, err := indirectkeyfiles.MarshalPrivateKey([]byte(filename))
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	if keyBytes := opts.keyBytes; len(keyBytes) > 0 {
		privKeyBytes, err := importPrivateKeyBytes(ctx, keyBytes,
			opts.origPassphrase, opts.newPassphrase)
		if err != nil {
			return nil, nil, err
		}
		return publicKeyBytes, privKeyBytes, nil
	}

	return nil, nil, fmt.Errorf("no options were specified for how to import the private key")
}

// Make the key functions local to this package available to this package.
var keyRegistrar = keys.NewRegistrar()

func init() {
	MustRegister(keyRegistrar)
}

func importPrivateKeyBytes(ctx context.Context, keyBytes, origPassphrase, newPassphrase []byte) ([]byte, error) {
	if len(newPassphrase) == 0 {
		// downstream code can cope with cleartext or encrypted keys.
		return keyBytes, nil
	}
	// We need to encrypt the imported key bytes, so obtain the original
	// key (which may also be encrypted) and then reencrypt it.
	privKey, err := keyRegistrar.ParsePrivateKey(ctx, keyBytes, origPassphrase)
	if err != nil {
		return nil, err
	}
	// Note that the encrypted key will always be in PCKS8 format.
	return keys.MarshalBuiltinPrivateKey(privKey, newPassphrase)
}
