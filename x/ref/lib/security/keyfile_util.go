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
	"path/filepath"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/passphrase"
)

const (
	privateKeyFile = "privatekey.pem"
	publicKeyFile  = "publickey.pem"
)

func openKeyFile(keyFile string) (*os.File, error) {
	f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return nil, fmt.Errorf("failed to open %v for writing: %v", keyFile, err)
	}
	return f, nil
}

func writeKeyFile(keyfile string, data []byte) error {
	to, err := openKeyFile(keyfile)
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

func signerFromKey(ctx context.Context, private crypto.PrivateKey) (security.Signer, error) {
	api, err := keyRegistrar.APIForKey(private)
	if err != nil {
		return nil, err
	}
	return api.Signer(ctx, private)
}

func signerFromDir(ctx context.Context, dir string, passphrase []byte) (security.Signer, error) {
	privBytes, err := os.ReadFile(filepath.Join(dir, privateKeyFile))
	if err != nil {
		return nil, err
	}
	private, err := keyRegistrar.ParsePrivateKey(ctx, privBytes, passphrase)
	if err != nil {
		return nil, translatePassphraseError(err)
	}
	return signerFromKey(ctx, private)
}

func publicKeyFromDir(dir string) (security.PublicKey, error) {
	pubBytes, err := os.ReadFile(filepath.Join(dir, publicKeyFile))
	if err != nil {
		return nil, err
	}
	key, err := keyRegistrar.ParsePublicKey(pubBytes)
	if err != nil {
		return nil, err
	}
	return keys.PublicKey(key)
}

func writeKeyPairUsingPrivateKey(dir string, private crypto.PrivateKey, passphrase []byte) error {
	pubBytes, privBytes, err := marshalKeyPair(private, passphrase)
	if err != nil {
		return err
	}
	if err := writeKeyFile(filepath.Join(dir, publicKeyFile), pubBytes); err != nil {
		return err
	}
	return writeKeyFile(filepath.Join(dir, privateKeyFile), privBytes)
}

func writeKeyPairUsingBytes(dir string, pubBytes, privBytes []byte) error {
	if err := writeKeyFile(filepath.Join(dir, publicKeyFile), pubBytes); err != nil {
		return err
	}
	return writeKeyFile(filepath.Join(dir, privateKeyFile), privBytes)
}

func PrivateKeyFromFileWithPrompt(ctx context.Context, filename string) (crypto.PrivateKey, error) {
	privKeyBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	key, err := keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, nil)
	if err == nil || !errors.Is(err, ErrPassphraseRequired) {
		return key, err
	}
	pass, err := passphrase.Get(fmt.Sprintf("Passphrase required to decrypt encrypted private key file for private key in %v.\nEnter passphrase: ", filename))
	if err != nil {
		return nil, err
	}
	defer ZeroPassphrase(pass)
	return keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, pass)
}
