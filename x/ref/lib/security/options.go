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
func BlessingStoreReadonly(store CredentialsStoreReader, publicKey security.PublicKey) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.reader = store
		o.publicKey = publicKey
	}
}

// BlessingStoreWriteable specifies a writeable store on which blessings
// can be stored.
func BlessingStoreWriteable(store CredentialsStoreReadWriter, signer security.Signer) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.writer = store
		o.signer = &serializationSigner{signer}
		o.publicKey = signer.PublicKey()
	}
}

// BlessingStoreUpdate specifies that blessings should be periodically
// reloaded to obtain any changes made to them by another entity.
func BlessingStoreUpdate(interval time.Duration) BlessingsStoreOption {
	return func(o *blessingsStoreOptions) {
		o.updateInterval = interval
	}
}

// BlessingRootsReadonly specifies a readonly store from which blessings can be read.
func BlessingRootsReadonly(store CredentialsStoreReader, publicKey security.PublicKey) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
		o.reader = store
		o.publicKey = publicKey
	}
}

// BlessingRootsWriteable specifies a writeable store on which blessings
// can be stored.
func BlessingRootsWriteable(store CredentialsStoreReadWriter, signer security.Signer) BlessingRootsOption {
	return func(o *blessingRootsOptions) {
		o.writer = store
		o.signer = &serializationSigner{signer}
		o.publicKey = signer.PublicKey()
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
	blessingRoots  security.BlessingRoots
	blessingStore  security.BlessingStore
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

// FromPublicKeyOnly specifies whether the principal to be created can be restricted
// to having only a public key. Such a principal can verify credentials but
// not create any of its own.
func FromPublicKeyOnly(allow bool) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.allowPublicKey = allow
		return nil
	}
}

// FromBlessingStore specifies a security.BlessingStore to use with the new principal.
// If not specified, a security.BlessingStore will be created by LoadPrincipalOpts.
func FromBlessingStore(store security.BlessingStore) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.blessingStore = store
		return nil
	}
}

// FromBlessingRoots specifies a security.BlessingRoots to use with the new principal.
// If not specified, a security.BlessingRoots will be created by LoadPrincipalOpts.
func FromBlessingRoots(store security.BlessingRoots) LoadPrincipalOption {
	return func(o *principalOptions) error {
		o.blessingRoots = store
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
	allowPublicKey  bool
	x509Opts        x509.VerifyOptions
	x509Cert        *x509.Certificate
}

func (o createPrincipalOptions) checkPrivateKey(msg string) error {
	if len(o.passphrase) > 0 {
		return fmt.Errorf("%s: a private key with a passphrase has already been specified as an option", msg)
	}
	if o.privateKey != nil {
		return fmt.Errorf("%s: a private key has already been specified as an option", msg)
	}
	if len(o.privateKeyBytes) > 0 {
		return fmt.Errorf("%s: a marshaled private key (as bytes) has already been specified as an option", msg)
	}
	return nil
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
// WithSigner takes precedence over WithPrivateKey or WithPrivateKeyBytes.
func WithSigner(signer security.Signer) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.signer = signer
		return nil
	}
}

func (o *createPrincipalOptions) copyPassphrase(passphrase []byte) {
	if len(passphrase) > 0 {
		o.passphrase = make([]byte, len(passphrase))
		copy(o.passphrase, passphrase)

	}
	defer ZeroPassphrase(passphrase)
}

// WithPrivateKey specifies the private key to use for the new principal.
// WithPrivateKey takes precedence over WithPrivateKeyBytes.
// Passphrase is zeroed.
func WithPrivateKey(key crypto.PrivateKey, passphrase []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if err := o.checkPrivateKey("UsingPrivateKeyBytes"); err != nil {
			return err
		}
		o.privateKey = key
		o.copyPassphrase(passphrase)
		return nil
	}
}

// WithPublicKeyBytes specifies the public key bytes to use when creating a
// public-key only principal.
// If the public key bytes encode a CERTIFICATE PEM block then that Certificate
// will be retained and associated with the principal as opposed to just the public
// key portion of the certificate.
func WithPublicKeyBytes(keyBytes []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if _, err := keyRegistrar.ParsePublicKey(keyBytes); err != nil {
			return err
		}
		o.publicKeyBytes = keyBytes
		return nil
	}
}

// WithPrivateKeyBytes specifies the public and private key bytes to use when creating
// a principal. The passphrase is zeroed.
// If publicKeyBytes are nil then the public key will be derived from the
// private key. If not, the public key will be parsed from the supplied bytes.
// If the public key bytes encode a CERTIFICATE PEM block then that Certificate
// will be retained and associated with the principal as opposed to just the public
// key portion of the certificate.
func WithPrivateKeyBytes(ctx context.Context, publicKeyBytes, privateKeyBytes, passphrase []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		if err := o.checkPrivateKey("UsingPrivateKeyBytes"); err != nil {
			return err
		}
		o.copyPassphrase(passphrase)
		o.privateKeyBytes = privateKeyBytes
		o.publicKeyBytes = publicKeyBytes
		return nil
	}
}

// WithPublicKeyOnly specifies whether the principal to be created can be restricted
// to having only a public key. Such a principal can verify credentials but
// not create any of its own.
func WithPublicKeyOnly(allow bool) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.allowPublicKey = allow
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

// WithX509Certificate specifices the x509 certificate to associate with
// this principal. It's public key must match the public key already
// set for this principal if one has already been set via a private key,
// a signer or as bytes. Note that if the public key bytes specified
// via WithPublicKeyBytes is a PEM CERTIFICATE block then the x509
// Certificate will be used from that also.
func WithX509Certificate(cert *x509.Certificate) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.x509Cert = cert
		return nil
	}
}
