// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"os"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/passphrase"
)

// PrivateKeyFromFileWithPrompt reads a private key file from the specified file
// and will only prompt for a passphrase if the contents of the file are encrypted.
func PrivateKeyFromFileWithPrompt(ctx context.Context, filename string) (crypto.PrivateKey, error) {
	privKeyBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf("Passphrase required to decrypt encrypted private key file for private key in %v.\nEnter passphrase: ", filename)
	return PrivateKeyWithPrompt(ctx, privKeyBytes, prompt)
}

// PrivateKeyWithPrompt parses the supplied key bytes to obtain a private key
// and will only prompt for a passphrase if those
func PrivateKeyWithPrompt(ctx context.Context, privKeyBytes []byte, prompt string) (crypto.PrivateKey, error) {
	key, err := keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, nil)
	if err == nil || !errors.Is(translatePassphraseError(err), ErrPassphraseRequired) {
		return key, err
	}
	pass, err := passphrase.Get(prompt)
	if err != nil {
		return nil, err
	}
	defer ZeroPassphrase(pass)
	return keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, pass)
}

// ImportPrivateKeyFile returns the byte representation for an imported private
// key file.
func ImportPrivateKeyFile(filename string) ([]byte, error) {
	return indirectkeyfiles.MarshalPrivateKey([]byte(filename))
}

func publicKeyFromBytes(publicKeyBytes []byte) (security.PublicKey, error) {
	if len(publicKeyBytes) == 0 {
		return nil, nil
	}
	key, err := keyRegistrar.ParsePublicKey(publicKeyBytes)
	if err != nil {
		return nil, err
	}
	api, err := keyRegistrar.APIForKey(key)
	if err != nil {
		return nil, err
	}
	publicKey, err := api.PublicKey(key)
	return publicKey, err
}
