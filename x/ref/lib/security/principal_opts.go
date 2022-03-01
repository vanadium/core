// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"

	"v.io/v23/security"
)

func (o principalOptions) getBlessingStore(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingStore, error) {
	if o.blessingStore != nil {
		return o.blessingStore, nil
	}
	if o.writeable != nil {
		return NewBlessingStoreOpts(ctx, publicKey,
			BlessingStoreUpdate(o.interval),
			BlessingStoreWriteable(o.writeable, signer))
	}
	return NewBlessingStoreOpts(ctx, publicKey,
		BlessingStoreUpdate(o.interval),
		BlessingStoreReadonly(o.readonly, publicKey))
}

func (o principalOptions) getBlessingRoots(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingRoots, error) {
	if o.blessingRoots != nil {
		return o.blessingRoots, nil
	}
	if o.writeable != nil {
		return NewBlessingRootsOpts(ctx,
			BlessingRootsUpdate(o.interval),
			BlessingRootsWriteable(o.writeable, signer))
	}
	return NewBlessingRootsOpts(ctx,
		BlessingRootsUpdate(o.interval),
		BlessingRootsReadonly(o.readonly, publicKey))
}

// LoadPrincipalOpts loads the state required to create a principal according
// to the specified options. The most common use case is to load a principal
// from a filesystem directory, as in:
//
//     LoadPrincipalOpts(ctx, LoadFrom(FilesystemStoreWriter(dir)))
//
func LoadPrincipalOpts(ctx context.Context, opts ...LoadPrincipalOption) (security.Principal, error) {
	var o principalOptions
	for _, fn := range opts {
		if err := fn(&o); err != nil {
			return nil, err
		}
	}
	defer ZeroPassphrase(o.passphrase)

	if o.writeable == nil && o.readonly == nil {
		return CreatePrincipalOpts(ctx)
	}

	var reader CredentialsStoreReader
	if o.writeable != nil {
		unlock, err := o.writeable.Lock(ctx, LockKeyStore)
		if err != nil {
			return nil, err
		}
		defer unlock()
		reader = o.writeable
	} else {
		unlock, err := o.readonly.RLock(ctx, LockKeyStore)
		if err != nil {
			return nil, err
		}
		defer unlock()
		reader = o.readonly
	}
	signer, err := reader.NewSigner(ctx, o.passphrase)

	if err != nil {
		if !o.allowPublicKey {
			return nil, err
		}
	}
	publicKey, cert, err := reader.NewPublicKey(ctx)
	if err != nil {
		return nil, err
	}

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
	return security.CreateX509Principal(signer, cert, bs, br)
}
