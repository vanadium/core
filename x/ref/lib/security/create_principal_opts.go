// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"

	"v.io/v23/security"
)

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
	if len(o.publicKeyBytes) == 0 && len(o.privateKeyBytes) == 0 {
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
	if signer, err := o.getSigner(ctx); signer != nil {
		if err != nil {
			return nil, err
		}
		bs, br, err := o.inMemoryStores(ctx, signer.PublicKey())
		if err != nil {
			return nil, err
		}
		return security.CreatePrincipal(signer, bs, br)
	}
	if publicKey, err := publicKeyFromBytes(o.publicKeyBytes); publicKey != nil {
		if err != nil {
			return nil, err
		}
		bs, br, err := o.inMemoryStores(ctx, publicKey)
		if err != nil {
			return nil, err
		}
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
	signer, err := o.getSigner(ctx)
	if err != nil {
		return nil, err
	}
	if len(o.privateKeyBytes) == 0 {
		return nil, fmt.Errorf("cannot create a new persistent principal without a private key")
	}
	var publicKey security.PublicKey
	if signer == nil {
		publicKey, err = publicKeyFromBytes(o.publicKeyBytes)
		if err != nil {
			return nil, err
		}
		// just in case...
		defer ZeroPassphrase(o.privateKeyBytes)
	} else {
		publicKey = signer.PublicKey()
	}

	unlock, err := o.store.Lock(ctx, LockKeyStore)
	if err != nil {
		return nil, err
	}
	defer unlock()

	if err := o.store.WriteKeyPair(ctx, o.publicKeyBytes, o.privateKeyBytes); err != nil {
		return nil, err
	}

	// One of publicKey or signer will be nil.
	bs, br, err := o.setPersistentStores(ctx, publicKey, signer)
	if err != nil {
		return nil, err
	}
	if signer == nil {
		security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return security.CreatePrincipal(signer, bs, br)
}
