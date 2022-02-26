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

func signerFromBytes(ctx context.Context, privateKeyBytes, passphrase []byte) (security.Signer, error) {
	privateKey, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}
	return signerFromKey(ctx, privateKey)
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
		return signerFromBytes(ctx, o.privateKeyBytes, o.passphrase)
	}
	return nil, nil
}

func (o createPrincipalOptions) getPublicKey(ctx context.Context) (security.PublicKey, *x509.Certificate, error) {
	if o.signer != nil {
		return o.signer.PublicKey(), nil, nil
	}
	if o.privateKey != nil {
		api, err := keyRegistrar.APIForKey(o.privateKey)
		if err != nil {
			return nil, nil, err
		}
		publicKey, err := api.PublicKey(o.privateKey)
		return publicKey, nil, err
	}
	if len(o.publicKeyBytes) > 0 {
		key, err := keyRegistrar.ParsePublicKey(o.publicKeyBytes)
		if err != nil {
			return nil, nil, err
		}
		cert, _ := key.(*x509.Certificate)
		api, err := keyRegistrar.APIForKey(key)
		if err != nil {
			return nil, nil, err
		}
		publicKey, err := api.PublicKey(key)
		if err != nil {
			return nil, nil, err
		}
		return publicKey, cert, err
	}
	if len(o.privateKeyBytes) > 0 {
		key, err := keyRegistrar.ParsePrivateKey(ctx, o.privateKeyBytes, o.passphrase)
		if err != nil {
			return nil, nil, err
		}
		api, err := keyRegistrar.APIForKey(key)
		if err != nil {
			return nil, nil, err
		}
		publicKey, err := api.PublicKey(key)
		if err != nil {
			return nil, nil, err
		}
		return publicKey, nil, nil
	}
	return nil, nil, fmt.Errorf("no security.PublicKey found in options")
}

func (o createPrincipalOptions) getCryptoPublicKey(ctx context.Context) (crypto.PublicKey, error) {
	if o.privateKey != nil {
		api, err := keyRegistrar.APIForKey(o.privateKey)
		if err != nil {
			return nil, err
		}
		return api.CryptoPublicKey(o.privateKey)
	}
	if len(o.publicKeyBytes) > 0 {
		return keyRegistrar.ParsePublicKey(o.publicKeyBytes)
	}
	if len(o.privateKeyBytes) > 0 {
		key, err := keyRegistrar.ParsePrivateKey(ctx, o.privateKeyBytes, o.passphrase)
		if err != nil {
			return nil, err
		}
		api, err := keyRegistrar.APIForKey(key)
		if err != nil {
			return nil, err
		}
		publicKey, err := api.CryptoPublicKey(key)
		if err != nil {
			return nil, err
		}
		return publicKey, nil
	}
	return nil, fmt.Errorf("no crypto.PublicKey found in options")
}

func (o createPrincipalOptions) getKeyInfo(ctx context.Context) (security.Signer, security.PublicKey, *x509.Certificate, error) {
	signer, err := o.getSigner(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	publicKey, cert, err := o.getPublicKey(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return signer, publicKey, cert, nil
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
	signer, publicKey, cert, err := o.getKeyInfo(ctx)
	if err != nil {
		return nil, err
	}
	bs, br, err := o.inMemoryStores(ctx, publicKey)
	if err != nil {
		return nil, err
	}
	if signer != nil {
		return security.CreateX509Principal(signer, cert, bs, br)
	}
	if publicKey != nil {
		return security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return nil, fmt.Errorf("no signer/private key or public key information provided")
}

func (o createPrincipalOptions) setPersistentStores(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (blessingStore security.BlessingStore, blessingRoots security.BlessingRoots, err error) {
	var blessingsStoreOpt BlessingsStoreOption
	var blessingRootOpt BlessingRootsOption
	if signer != nil {
		blessingsStoreOpt = BlessingsStoreWriteable(o.store, &serializationSigner{signer})
		blessingRootOpt = BlessingRootsWriteable(o.store, &serializationSigner{signer})
		publicKey = signer.PublicKey()
	} else {
		blessingsStoreOpt = BlessingsStoreReadonly(o.store, publicKey)
		blessingRootOpt = BlessingRootsReadonly(o.store, publicKey)
	}

	blessingStore = o.blessingStore
	if blessingStore == nil {
		blessingStore, err = NewBlessingStoreOpts(ctx, publicKey, blessingsStoreOpt)
		if err != nil {
			return
		}
	}
	blessingRoots = o.blessingRoots
	if blessingRoots == nil {
		blessingRoots, err = NewBlessingRootsOpts(ctx, blessingRootOpt)
		if err != nil {
			return
		}
	}
	return
}

func (o createPrincipalOptions) createPersistentPrincipal(ctx context.Context) (security.Principal, error) {
	signer, publicKey, cert, err := o.getKeyInfo(ctx)
	if err != nil {
		return nil, err
	}
	publicKeyBytes, privateKeyBytes := o.publicKeyBytes, o.privateKeyBytes
	if len(privateKeyBytes) == 0 {
		if o.privateKey != nil {
			privateKeyBytes, err = keyRegistrar.MarshalPrivateKey(o.privateKey, o.passphrase)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(privateKeyBytes) == 0 && !o.allowPublicKey {
		return nil, fmt.Errorf("cannot create a new persistent principal without a private key")
	}

	if len(publicKeyBytes) == 0 {
		publicKey, err := o.getCryptoPublicKey(ctx)
		if err != nil {
			return nil, err
		}
		publicKeyBytes, err = keyRegistrar.MarshalPublicKey(publicKey)
		if err != nil {
			return nil, err
		}
	}

	if len(publicKeyBytes) == 0 {
		return nil, fmt.Errorf("cannot create a new persistent principal without a public key")
	}

	unlock, err := o.store.Lock(ctx, LockKeyStore)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if err := o.store.WriteKeyPair(ctx, publicKeyBytes, privateKeyBytes); err != nil {
		return nil, err
	}

	// One of publicKey or signer will be nil.
	bs, br, err := o.setPersistentStores(ctx, publicKey, signer)
	if err != nil {
		return nil, err
	}
	if signer == nil {
		if !o.allowPublicKey {
			return nil, fmt.Errorf("cannot create a public key only principal without using: WithPublicKey(true)")
		}
		return security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return security.CreateX509Principal(signer, cert, bs, br)
}
