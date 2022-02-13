// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"

	"v.io/v23/security"
)

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

	var publicKey security.PublicKey
	if err != nil {
		if !o.allowPublicKey {
			return nil, err
		}
		publicKey, err = reader.NewPublicKey(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		publicKey = signer.PublicKey()
	}

	storeOpts := []CredentialsStoreOption{WithUpdate(o.interval)}
	if o.writeable != nil {
		storeOpts = append(storeOpts, WithStore(o.writeable, &serializationSigner{signer}))
	} else {
		storeOpts = append(storeOpts, WithReadonlyStore(o.readonly, publicKey))
	}

	br, err := NewBlessingRootsOpts(ctx, storeOpts...)
	if err != nil {
		return nil, err
	}

	bs, err := NewBlessingStoreOpts(ctx, publicKey, storeOpts...)
	if err != nil {
		return nil, err
	}

	if signer == nil {
		return security.CreatePrincipalPublicKeyOnly(publicKey, bs, br)
	}
	return security.CreatePrincipal(signer, bs, br)
}
