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

// BlessingsStoreOption represents an option to NewBlessingStoreOpts.
type BlessingsStoreOption func(*blessingsStoreOptions)

// BlessingRootsOption represents an option to NewBlessingRootOpts.
type BlessingRootsOption func(*blessingRootsOptions)

type commonStoreOptions struct {
	reader         CredentialsStoreReader
	publicKey      security.PublicKey
	writer         CredentialsStoreReadWriter
	signer         serialization.Signer
	updateInterval time.Duration
	x509Opts       x509.VerifyOptions
}

type blessingsStoreOptions struct {
	commonStoreOptions
}

type blessingRootsOptions struct {
	commonStoreOptions
}

// BlessingStoreReadonly specifies a readonly store from which blessings can be read.
func BlessingsStoreReadonly(store CredentialsStoreReader, key security.PublicKey) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.reader = store
		o.publicKey = key
	}
}

// BlessingsStoreWriteable specifies a writeable store on which blessings
// can be stored.
func BlessingsStoreWriteable(store CredentialsStoreReadWriter, signer serialization.Signer) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.writer = store
		o.publicKey = signer.PublicKey()
		o.signer = signer
	}
}

// BlessingsStoreUpdate specifies that blessings should be periodically
// reloaded to obtain any changes made to them by another entity.
func BlessingsStoreUpdate(interval time.Duration) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.updateInterval = interval
	}
}

// BlessingRootsReadonly specifies a readonly store from which blessings can be read.
func BlessingRootsReadonly(store CredentialsStoreReader, key security.PublicKey) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
		o.reader = store
		o.publicKey = key
	}
}

// BlessingRootsWriteable specifies a writeable store on which blessings
// can be stored.
func BlessingRootsWriteable(store CredentialsStoreReadWriter, signer serialization.Signer) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
		o.writer = store
		o.publicKey = signer.PublicKey()
		o.signer = signer
	}
}

// BlessingRootsUpdate specifies that blessing roots should be periodically
// reloaded to obtain any changes made to them by another entity.
func BlessingRootsUpdate(interval time.Duration) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
		o.updateInterval = interval
	}
}

// BlessingRootsX509VerifyOptions specifies the x509 verification options to use with
// a blessing roots store.
func BlessingRootsX509VerifyOptions(opts x509.VerifyOptions) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
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

// FromReadonly specifies a readonly store from which credentials information
// can be read. This includes keys, blessings and blessing roots.
func FromReadonly(store CredentialsStoreReader) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.writeable != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.readonly = store
		return nil
	}
}

// FromWritable specifies a writeable store from credentials information can
// be read. This includes keys, blessings and blessing roots.
func FromWritable(store CredentialsStoreReadWriter) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if o.readonly != nil {
			return fmt.Errorf("a credentials store option has already been specified")
		}
		o.writeable = store
		return nil
	}
}

// FromPassphrase specifies the passphrase to use for decrypting private
// key information. The supplied passphrase is zeroed.
func FromPassphrase(passphrase []byte) LoadPrincipalOption {
	return func(o *principalOptions) error {
		if len(passphrase) > 0 {
			o.passphrase = make([]byte, len(passphrase))
			copy(o.passphrase, passphrase)
			ZeroPassphrase(passphrase)
		}
		return nil
	}
}

// RefreshInterval specifies that credentials state should be periodically
// reloaed to obtain any changes made to them by another entity.
func RefreshInterval(interval time.Duration) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.interval = interval
		return nil
	}
}

// FromPublicKey specifies whether the principal to be created can be restricted
// to having only a public key. Such a principal can verify credentials but
// not create any of its own.
func FromPublicKey(allow bool) LoadPrincipalOption {
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

// WithStore specifies the credentials store to use for creating a new
// principal. Such a store must support persisting key information.
func WithStore(store CredentialsStoreCreator) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.store = store
		return nil
	}
}

// WithBlessingsStore specifies the security.BlessingStore to use for
// the new principal.
func WithBlessingsStore(store security.BlessingStore) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingStore = store
		return nil
	}
}

// WithBlessingRoots specifies the security.BlessingRoots to use for
// the new principal.
func WithBlessingRoots(roots security.BlessingRoots) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.blessingRoots = roots
		return nil
	}
}

// WithSigner specifies the security.Signer to use for the new principal.
func WithSigner(signer security.Signer) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.signer = signer
		return nil
	}
}

// WithPrivateKey specifies the private key to use for the new principal.
func WithPrivateKey(key crypto.PrivateKey, passphrase []byte) CreatePrincipalOption {
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

// WithPublicKeyBytes specifies the public key bytes to use when creating a
// public-key only principal.
func WithPublicKeyBytes(keyBytes []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if _, err := keyRegistrar.ParsePublicKey(keyBytes); err != nil {
			return err
		}
		o.publicKeyBytes = keyBytes
		return nil
	}
}

// WithPrivateKeyBytes specifies the private key bytes to use when creating
// a principal. The passphrase is zeroed.
func WithPrivateKeyBytes(ctx context.Context, public, private, passphrase []byte) CreatePrincipalOption {
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

// WithX509VerifyOptions specifies the x509 verification options to use with
// when verifying X509 blessing roots.
func WithX509VerifyOptions(opts x509.VerifyOptions) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.x509Opts = opts
		return nil
	}
}
