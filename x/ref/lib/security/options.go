// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"time"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/serialization"
)

// CredentialsStoreOption represents an option to NewBlessingStoreOpts and
// NewBlessingRootsOpts.
type CredentialsStoreOption func(*credentialsStoreOptions)

type credentialsStoreOptions struct {
	reader         CredentialsStoreReader
	publicKey      security.PublicKey
	writer         CredentialsStoreReadWriter
	signer         serialization.Signer
	updateInterval time.Duration
	x509Opts       x509.VerifyOptions
}

// WithReadonlyStore specifies a readonly store from which blessings or blessing
// roots can be read.
func WithReadonlyStore(store CredentialsStoreReader, key security.PublicKey) CredentialsStoreOption {
	return func(o *credentialsStoreOptions) {
		o.reader = store
		o.publicKey = key
	}
}

// WithStore specifies a writeable store on which blessings or blessing roots
// can be stored.
func WithStore(store CredentialsStoreReadWriter, signer serialization.Signer) CredentialsStoreOption {
	return func(o *credentialsStoreOptions) {
		o.writer = store
		o.publicKey = signer.PublicKey()
		o.signer = signer
	}
}

// WithUpdate specifies that blessings or blessing roots should be periodically
// reloaded to obtain any changes made to them by another entity.
func WithUpdate(interval time.Duration) CredentialsStoreOption {
	return func(o *credentialsStoreOptions) {
		o.updateInterval = interval
	}
}

// WithX509VerifyOptions specifies the x509 verification options to use with
// a blessing roots store.
func WithX509VerifyOptions(opts x509.VerifyOptions) CredentialsStoreOption {
	return func(o *credentialsStoreOptions) {
		o.x509Opts = opts
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

// LoadFromReadonly specifies a readonly store from which credentials information
// can be read. This includes keys, blessings and blessing roots.
func LoadFromReadonly(store CredentialsStoreReader) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.writeable != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.readonly = store
		return nil
	}
}

// LoadFrom specifies a writeable store from credentials information can
// be read. This includes keys, blessings and blessing roots.
func LoadFrom(store CredentialsStoreReadWriter) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.readonly != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.writeable = store
		return nil
	}
}

// LoadUsingPassphrase specifies the passphrase to use for decrypting private
// key information. The supplied passphrase is zeroed.
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

// LoadRefreshInterval specifies that credentials state should be periodically
// reloaed to obtain any changes made to them by another entity.
func LoadRefreshInterval(interval time.Duration) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.interval = interval
		return nil
	}
}

// LoadAllowPublicKeyPrincipal specifies whether the principal to be
// created can be restricted to having only a public key. Such a principal
// can verify credentials but not create any of its own.
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
	x509Opts        x509.VerifyOptions
}

// UseStore specifies the credentials store to use for creating a new
// principal. Such a store must support persisting key information.
func UseStore(store CredentialsStoreCreator) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.store = store
		return nil
	}
}

// UseBlessingsStore specifies the security.BlessingStore to use for
// the new principal.
func UseBlessingsStore(store security.BlessingStore) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingStore = store
		return nil
	}
}

// UseBlessingRoots specifies the security.BlessingRoots to use for
// the new principal.
func UseBlessingRoots(roots security.BlessingRoots) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingRoots = roots
		return nil
	}
}

// UseSigner specifies the security.Signer to use for the new principal.
func UseSigner(signer security.Signer) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.signer = signer
		return nil
	}
}

// UsePrivateKey specifies the private key to use for the new principal.
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

// UsePublicKeyBytes specifies the public key bytes to use when creating a
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
// a principal. The passphrase is zeroed.
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

// UseX509VerifyOptions specifies the x509 verification options to use with
// a blessing roots store.
func UseX509VerifyOptions(opts x509.VerifyOptions) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.x509Opts = opts
		return nil
	}
}
