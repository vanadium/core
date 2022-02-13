// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"crypto"
	"errors"
	"fmt"
	"io"
	"os"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/passphrase"
)

func writeKeyFile(keyfile string, data []byte) error {
	to, err := os.OpenFile(keyfile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return fmt.Errorf("failed to open %v for writing: %v", keyfile, err)
	}
	if err != nil {
		return err
	}
	defer to.Close()
	_, err = io.Copy(to, bytes.NewReader(data))
	return err
}

func marshalKeyPair(private crypto.PrivateKey, passphrase []byte) (pubBytes, privBytes []byte, err error) {
	privBytes, err = keyRegistrar.MarshalPrivateKey(private, passphrase)
	if err != nil {
		err = translatePassphraseError(err)
		return
	}
	api, err := keyRegistrar.APIForKey(private)
	if err != nil {
		return
	}
	pubKey, err := api.CryptoPublicKey(private)
	if err != nil {
		return
	}
	pubBytes, err = keyRegistrar.MarshalPublicKey(pubKey)
	return
}

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
	if err == nil || !errors.Is(err, ErrPassphraseRequired) {
		return key, err
	}
	pass, err := passphrase.Get(prompt)
	if err != nil {
		return nil, err
	}
	defer ZeroPassphrase(pass)
	return keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, pass)
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

func signerFromKey(ctx context.Context, private crypto.PrivateKey) (security.Signer, error) {
	api, err := keyRegistrar.APIForKey(private)
	if err != nil {
		return nil, err
	}
	return api.Signer(ctx, private)
}

func signerFromBytes(ctx context.Context, privateKeyBytes, passphrase []byte) (security.Signer, error) {
	privateKey, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}
	return signerFromKey(ctx, privateKey)
}
