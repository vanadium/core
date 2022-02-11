// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/serialization"
)

type CredentialsStoreOption func(*credentialsStoreOption)

type credentialsStoreOption struct {
	reader         CredentialsStoreReader
	publicKey      security.PublicKey
	writer         CredentialsStoreReadWriter
	signer         serialization.Signer
	updateInterval time.Duration
}

func WithReadonlyStore(store CredentialsStoreReader, key security.PublicKey) CredentialsStoreOption {
	return func(o *credentialsStoreOption) {
		o.reader = store
		o.publicKey = key
	}
}

func WithStore(store CredentialsStoreReadWriter, signer serialization.Signer) CredentialsStoreOption {
	return func(o *credentialsStoreOption) {
		o.writer = store
		o.publicKey = signer.PublicKey()
		o.signer = signer
	}
}

func WithUpdate(interval time.Duration) CredentialsStoreOption {
	return func(o *credentialsStoreOption) {
		o.updateInterval = interval
	}
}

// LoadPrincipalOption represents an option to LoadPrincipalOpts.
type LoadPrincipalOption func(o *principalOptions) error

type principalOptions struct {
	readonly       CredentialsStoreReader
	writeable      CredentialsStoreReadWriter
	interval       time.Duration
	allowPublicKey bool
	passphrase     []byte
}

func LoadFromReadonly(store CredentialsStoreReader) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.writeable != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.readonly = store
		return nil
	}
}

func LoadFrom(store CredentialsStoreReadWriter) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.readonly != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.writeable = store
		return nil
	}
}

func LoadUsingPassphrase(passphrase []byte) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if len(passphrase) > 0 {
			o.passphrase = make([]byte, len(passphrase))
			copy(o.passphrase, passphrase)
			ZeroPassphrase(passphrase)
		}
		return nil
	}
}

func LoadRefreshInterval(interval time.Duration) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.interval = interval
		return nil
	}
}

func LoadAllowPublicKeyPrincipal(allow bool) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.allowPublicKey = allow
		return nil
	}
}

// CreatePrincipalOption represents an option to CreatePrincipalOpts.
type CreatePrincipalOption func(o *createPrincipalOptions) error

type createPrincipalOptions struct {
	signer          security.Signer
	privateKey      crypto.PrivateKey
	passphrase      []byte
	privateKeyBytes []byte
	publicKeyBytes  []byte
	store           CredentialsStoreCreator
	blessingStore   security.BlessingStore
	blessingRoots   security.BlessingRoots
}

func UseStore(store CredentialsStoreCreator) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.store = store
		return nil
	}
}

func UseBlessingsStore(store security.BlessingStore) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingStore = store
		return nil
	}
}

func UseBlessingRoots(roots security.BlessingRoots) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingRoots = roots
		return nil
	}
}

func UseSigner(signer security.Signer) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.signer = signer
		return nil
	}
}

// UsePrivateKey specifies the private key to use when creating the principal.
func UsePrivateKey(key crypto.PrivateKey, passphrase []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if err := o.checkPrivateKey("UsingPrivateKeyBytes"); err != nil {
			return err
		}
		o.privateKey = key
		if len(passphrase) > 0 {
			o.passphrase = make([]byte, len(passphrase))
			copy(o.passphrase, passphrase)
		}
		defer ZeroPassphrase(passphrase)
		api, err := keyRegistrar.APIForKey(key)
		if err != nil {
			return err
		}
		publicKey, err := api.CryptoPublicKey(key)
		if err != nil {
			return err
		}
		o.publicKeyBytes, err = keyRegistrar.MarshalPublicKey(publicKey)
		if err != nil {
			return err
		}
		o.privateKeyBytes, err = keyRegistrar.MarshalPrivateKey(key, passphrase)
		return err
	}
}

// UsePrivateKey specifies the public key bytes to use when creating a
// public-key only principal.
func UsePublicKeyBytes(keyBytes []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if _, err := keyRegistrar.ParsePublicKey(keyBytes); err != nil {
			return err
		}
		o.publicKeyBytes = keyBytes
		return nil
	}
}

// UsePrivateKeyBytes specifies the private key bytes to use when creating
// a principal.
func UsePrivateKeyBytes(ctx context.Context, public, private, passphrase []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if err := o.checkPrivateKey("UsingPrivateKeyBytes"); err != nil {
			return err
		}
		if len(passphrase) > 0 {
			o.passphrase = make([]byte, len(passphrase))
			copy(o.passphrase, passphrase)
		}
		defer ZeroPassphrase(passphrase)
		o.privateKeyBytes = private
		o.publicKeyBytes = public
		return nil
	}
}
