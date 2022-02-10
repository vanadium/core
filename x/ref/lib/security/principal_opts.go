// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"

	"v.io/v23/security"
)

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

	reader := o.readonly
	if o.writeable != nil {
		reader = o.writeable
	}

	var publicKey security.PublicKey
	signer, err := reader.NewSigner(ctx, o.passphrase)
	if err != nil {
		return nil, err
	}
	if signer == nil {
		publicKey, err = reader.NewPublicKey(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		publicKey = signer.PublicKey()
	}

	storeOpt := WithReadonlyStore(reader, publicKey)
	if o.writeable != nil {
		storeOpt = WithStore(o.writeable, &serializationSigner{signer})
	}

	blessingRoots, err := NewBlessingRootsOpts(ctx, storeOpt, WithUpdate(o.interval))
	if err != nil {
		return nil, err
	}

	blessingStore, err := NewBlessingStoreOpts(ctx, publicKey, storeOpt, WithUpdate(o.interval))
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(signer, blessingStore, blessingRoots)
}
