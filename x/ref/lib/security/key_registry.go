// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/lib/security/keys/x509keys"
)

var keyRegistrar *keys.Registrar

// KeyRegistrar exposes the keys.Registrar used by this package to allow
// for external packages to extend the set of supported key types.
func KeyRegistrar() *keys.Registrar {
	return keyRegistrar
}

func init() {
	keyRegistrar = keys.NewRegistrar()
	keys.MustRegister(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
	sshkeys.MustRegister(keyRegistrar)
	x509keys.MustRegister(keyRegistrar)
}

// APIForKey calls APIForKey on KeyRegistrar().
func APIForKey(key crypto.PrivateKey) (keys.API, error) {
	return keyRegistrar.APIForKey(key)
}

// MarshalPrivateKey calls MarshalPrivateKey on KeyRegistrar().
func MarshalPrivateKey(key crypto.PrivateKey, passphrase []byte) ([]byte, error) {
	return keyRegistrar.MarshalPrivateKey(key, passphrase)
}

// MarshalPublicKey calls MarshalPublicKey on KeyRegistrar().
func MarshalPublicKey(key crypto.PublicKey) ([]byte, error) {
	return keyRegistrar.MarshalPublicKey(key)
}

// ParsePrivateKey calls ParsePrivateKey on KeyRegistrar().
func ParsePrivateKey(ctx context.Context, data, passphrase []byte) (crypto.PrivateKey, error) {
	return keyRegistrar.ParsePrivateKey(ctx, data, passphrase)
}

// ParsePublicKey calls ParsePublicKey on KeyRegistrar().
func ParsePublicKey(data []byte) (crypto.PublicKey, error) {
	return keyRegistrar.ParsePublicKey(data)
}

func translatePassphraseError(err error) error {
	if errors.Is(err, &keys.ErrPassphraseRequired{}) {
		return ErrPassphraseRequired.Errorf(nil, "passphrase required for decrypting private key")
	}
	if errors.Is(err, &keys.ErrBadPassphrase{}) {
		return ErrBadPassphrase.Errorf(nil, "passphrase incorrect for decrypting private key")
	}
	return err
}

func convertToPKCS8(ctx context.Context, keyBytes []byte, passphrase []byte) ([]byte, error) {
	key, err := keyRegistrar.ParsePrivateKey(ctx, keyBytes, passphrase)
	if err != nil {
		return nil, translatePassphraseError(err)
	}
	newKey, err := keys.MarshalPKCS8PrivateKey(key, passphrase)
	if err != nil {
		return nil, translatePassphraseError(err)
	}
	return newKey, err
}

// ConvertPrivateKeyForPrincipal will convert a private key encoded in a PEM block in
// any supported format to a PEM block of type 'PRIVATE KEY' encoded
// as PKCS8. It is intended for updating existing Vanadium principals that
// use 'EC PRIVATE KEY' and PEM encryption to PKCS8 format and encryption.
func ConvertPrivateKeyForPrincipal(ctx context.Context, dir string, passphrase []byte) error {
	defer ZeroPassphrase(passphrase)
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		return fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	defer unlock()
	privFilename := filepath.Join(dir, privateKeyFile)
	keyBytes, err := os.ReadFile(privFilename)
	if err != nil {
		return err
	}
	newKeyBytes, err := convertToPKCS8(ctx, keyBytes, passphrase)
	if err != nil {
		return err
	}
	newPrivFilename := filepath.Join(dir, "new-"+privateKeyFile)
	oldPrivFilename := filepath.Join(dir, "old-"+privateKeyFile)
	if err := os.WriteFile(newPrivFilename, newKeyBytes, 0400); err != nil {
		return err
	}
	if err := os.Rename(privFilename, oldPrivFilename); err != nil {
		return err
	}
	if err := os.Rename(newPrivFilename, privFilename); err != nil {
		return err
	}
	return os.Remove(oldPrivFilename)
}
