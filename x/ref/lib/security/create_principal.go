// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"

	"v.io/v23/security"
)

// CreatePersistentPrincipal wraps CreatePersistentPrincipalUsingKey to
// creates a new Principal using a newly generated ECSDA key.
func CreatePersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	return CreatePrincipalOpts(context.TODO(), UseStore(CreateFilesystemStore(dir)))
}

// CreatePersistentPrincipalUsingKey creates a new Principal using the supplied
// key and commits all state changes to the provided directory.
//
// The private key is serialized and saved encrypted if the
// 'passphrase' is non-nil, and unencrypted otherwise.
//
// If the directory has any preexisting principal data, an error is returned.
//
// The specified directory may not exist, in which case it will be created.
// The follow key types are supported:
// *ecdsa.PrivateKey, ed25519.PrivateKey, *rsa.PrivateKey and *sshkeys.HostedKey.
func CreatePersistentPrincipalUsingKey(ctx context.Context, key crypto.PrivateKey, dir string, passphrase []byte) (security.Principal, error) {
	return CreatePrincipalOpts(context.TODO(),
		UseStore(CreateFilesystemStore(dir)),
		UsePrivateKey(key, passphrase),
	)
}

/*
func createPrincipalUsingSigner(ctx context.Context, signer security.Signer, dir string) (security.Principal, error) {
	var update time.Duration
	blessingsStore, blessingRoots, err := newStores(ctx, signer, signer.PublicKey(), dir, false, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(signer, blessingsStore, blessingRoots)
}


func initAndLockPrincipalDir(dir string) (func(), error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	return unlock, nil
}

func newStores(ctx context.Context, signer security.Signer, publicKey security.PublicKey, dir string, readonly bool, update time.Duration) (security.BlessingStore, security.BlessingRoots, error) {

	blessingRootsSerializer := newFileSerializer(
		path.Join(dir, blessingRootsDataFile),
		path.Join(dir, blessingRootsSigFile))
	rootsReader, rootsWriter := SerializerReader(blessingRootsSerializer), SerializerWriter(blessingRootsSerializer)

	blessingStoreSerializer := newFileSerializer(
		path.Join(dir, blessingStoreDataFile),
		path.Join(dir, blessingStoreSigFile))
	storeReader, storeWriter := SerializerReader(blessingStoreSerializer), SerializerWriter(blessingStoreSerializer)

	var signerSerialization serialization.Signer
	if readonly {
		rootsWriter, storeWriter = nil, nil
	} else {
		signerSerialization = &serializationSigner{signer}
	}

	blessingRoots, err := NewPersistentBlessingRoots(
		ctx,
		filepath.Join(dir, blessingsRootsLockFilename),
		rootsReader,
		rootsWriter,
		signerSerialization,
		publicKey,
		update,
	)
	if err != nil {
		return nil, nil, err
	}
	blessingsStore, err := NewPersistentBlessingStore(
		ctx,
		filepath.Join(dir, blessingStoreLockFilename),
		storeReader,
		storeWriter,
		signerSerialization,
		publicKey,
		update,
	)
	if err != nil {
		return nil, nil, err
	}
	return blessingsStore, blessingRoots, nil
}
*/
