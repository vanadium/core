// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"fmt"

	"v.io/v23/security"
)

// CreatePrincipalOpts creates a Principal using the specified options. It is
// intended to replace the other 'Create' methods provided by this package.
// If no private key was specified via an option then a plaintext ecdsa key
// with the P256 curve will be created and used.
func CreatePrincipalOpts(ctx context.Context, opts ...CreatePrincipalOption) (security.Principal, error) {
	var o createPrincipalOptions
	for _, fn := range opts {
		if err := fn(&o); err != nil {
			return nil, err
		}
	}
	defer ZeroPassphrase(o.passphrase)
	if o.noKeyInfo() {
		pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		WithPrivateKey(pk, nil)(&o)
	}
	if o.store == nil {
		return o.createInMemoryPrincipal(ctx)
	}
	return o.createPersistentPrincipal(ctx)
}

func (o createPrincipalOptions) noKeyInfo() bool {
	return o.signer == nil && o.privateKey == nil && len(o.publicKeyBytes) == 0 && len(o.privateKeyBytes) == 0
}

func signerFromKey(ctx context.Context, private crypto.PrivateKey) (security.Signer, error) {
	api, err := keyRegistrar.APIForKey(private)
	if err != nil {
		return nil, err
	}
	return api.Signer(ctx, private)
}

func (o createPrincipalOptions) getSigner(ctx context.Context) (security.Signer, error) {
	if o.signer != nil {
		return o.signer, nil
	}
	if o.privateKey != nil {
		signer, err := signerFromKey(ctx, o.privateKey)
		return signer, err
	}
	if len(o.privateKeyBytes) > 0 {
		privateKey, err := keyRegistrar.ParsePrivateKey(ctx, o.privateKeyBytes, o.passphrase)
		if err != nil {
			return nil, err
		}
		return signerFromKey(ctx, privateKey)
	}
	return nil, nil
}

func getPublicKeyInfoFromPrivateKey(privateKey crypto.PrivateKey) (crypto.PublicKey, security.PublicKey, error) {
	api, err := keyRegistrar.APIForKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := api.PublicKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	cryptoPublicKey, err := api.CryptoPublicKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	return cryptoPublicKey, publicKey, err
}

func getCryptoPublicKey(publicKey security.PublicKey) (crypto.PublicKey, error) {
	der, err := publicKey.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return x509.ParsePKIXPublicKey(der)
}

func getPublicKey(publicKey crypto.PublicKey) (security.PublicKey, error) {
	api, err := keyRegistrar.APIForKey(publicKey)
	if err != nil {
		return nil, err
	}
	return api.PublicKey(publicKey)
}

func (o createPrincipalOptions) getPublicKeyInfo(ctx context.Context) (cryptoPublicKey crypto.PublicKey, x509cert *x509.Certificate, publicKey security.PublicKey, err error) {
	x509cert = o.x509Cert
	if o.signer != nil {
		publicKey = o.signer.PublicKey()
		cryptoPublicKey, err = getCryptoPublicKey(publicKey)
		return
	}
	privateKey := o.privateKey
	if privateKey != nil {
		cryptoPublicKey, publicKey, err = getPublicKeyInfoFromPrivateKey(privateKey)
		return
	}
	if len(o.privateKeyBytes) > 0 {
		privateKey, err = keyRegistrar.ParsePrivateKey(ctx, o.privateKeyBytes, o.passphrase)
		if err != nil {
			return
		}
		cryptoPublicKey, publicKey, err = getPublicKeyInfoFromPrivateKey(privateKey)
		return
	}
	if len(o.publicKeyBytes) > 0 {
		cryptoPublicKey, err = keyRegistrar.ParsePublicKey(o.publicKeyBytes)
		if err != nil {
			return
		}
		if x509cert == nil {
			if nc, ok := cryptoPublicKey.(*x509.Certificate); ok {
				cryptoPublicKey = nc
			}
		}
		publicKey, err = getPublicKey(cryptoPublicKey)
		return
	}
	err = fmt.Errorf("no security.PublicKey found in options")
	return
}

// getKeyInfo derives key/signer information from the speficied options in order
// of precedence: signer, private key, private key bytes, public key bytes.
// The x509 certificate is derived either directly from an option or from
// public key bytes since there's no other way of obtaining it.
func (o createPrincipalOptions) getKeyInfo(ctx context.Context) (signer security.Signer, cryptoPublicKey crypto.PublicKey, x509Cert *x509.Certificate, publicKey security.PublicKey, err error) {
	signer, err = o.getSigner(ctx)
	if err != nil {
		return
	}
	cryptoPublicKey, x509Cert, publicKey, err = o.getPublicKeyInfo(ctx)
	return
}

func (o createPrincipalOptions) inMemoryStores(ctx context.Context, publicKey security.PublicKey) (blessingStore security.BlessingStore, blessingRoots security.BlessingRoots, err error) {
	blessingStore, blessingRoots = o.blessingStore, o.blessingRoots
	if blessingStore == nil {
		blessingStore = NewBlessingStore(publicKey)
	}
	if blessingRoots == nil {
		blessingRoots, err = NewBlessingRootsOpts(ctx)
	}
	return
}

func (o createPrincipalOptions) createInMemoryPrincipal(ctx context.Context) (security.Principal, error) {
	signer, _, x509Cert, publicKey, err := o.getKeyInfo(ctx)
	if err != nil {
		return nil, err
	}
	bs, br, err := o.inMemoryStores(ctx, publicKey)
	if err != nil {
		return nil, err
	}
	if signer != nil {
		return security.CreateX509Principal(signer, x509Cert, bs, br)
	}
	if publicKey != nil {
		return security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return nil, fmt.Errorf("no signer/private key or public key information provided")
}

func (o createPrincipalOptions) getBlessingStore(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingStore, error) {
	if o.blessingStore != nil {
		return o.blessingStore, nil
	}
	if signer != nil {
		return NewBlessingStoreOpts(ctx,
			publicKey,
			BlessingStoreWriteable(o.store, signer))
	}
	return NewBlessingStoreOpts(ctx,
		publicKey,
		BlessingStoreReadonly(o.store, publicKey))
}

func (o createPrincipalOptions) getBlessingRoots(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingRoots, error) {
	if o.blessingRoots != nil {
		return o.blessingRoots, nil
	}
	if signer != nil {
		return NewBlessingRootsOpts(ctx,
			BlessingRootsWriteable(o.store, signer))
	}
	return NewBlessingRootsOpts(ctx,
		BlessingRootsReadonly(o.store, publicKey))
}

func (o createPrincipalOptions) createPersistentPrincipal(ctx context.Context) (security.Principal, error) {

	// Derive key information from the specified options.
	signer, cryptoPublicKey, x509Cert, publicKey, err := o.getKeyInfo(ctx)
	if err != nil {
		return nil, err
	}

	// Always write out the public and private keys as specified via an
	// option regardless of any other options.
	publicKeyBytes, privateKeyBytes := o.publicKeyBytes, o.privateKeyBytes
	if len(privateKeyBytes) == 0 && o.privateKey != nil {
		privateKeyBytes, err = keyRegistrar.MarshalPrivateKey(o.privateKey, o.passphrase)
		if err != nil {
			return nil, err
		}
	}

	if len(privateKeyBytes) == 0 && !o.allowPublicKey {
		return nil, fmt.Errorf("cannot create a new persistent principal without a private key")
	}

	if len(publicKeyBytes) == 0 {
		if x509Cert != nil {
			publicKeyBytes, err = keyRegistrar.MarshalPublicKey(x509Cert)
		} else {
			publicKeyBytes, err = keyRegistrar.MarshalPublicKey(cryptoPublicKey)
		}
		if err != nil {
			return nil, err
		}
	}

	unlock, err := o.store.Lock(ctx, LockKeyStore)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if err := o.store.WriteKeyPair(ctx, publicKeyBytes, privateKeyBytes); err != nil {
		return nil, err
	}

	// signer may be nil for a public key only principal.
	bs, err := o.getBlessingStore(ctx, publicKey, signer)
	if err != nil {
		return nil, err
	}
	br, err := o.getBlessingRoots(ctx, publicKey, signer)
	if err != nil {
		return nil, err
	}
	if signer == nil {
		return security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return security.CreateX509Principal(signer, x509Cert, bs, br)
}
