// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/serialization"
	"v.io/x/ref/lib/security/signing/sshagent"
)

// CreatePersistentPrincipal wraps CreatePersistentPrincipalUsingKey to
// creates a new Principal using a newly generated ECSDA key.
func CreatePersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}
	return CreatePersistentPrincipalUsingKey(context.TODO(), key, dir, passphrase)
}

// SSHAgentHostedKey represents a private key hosted by an ssh agent. The public
// key file must be accessible and is used to identify the private key hosted
// by the ssh agent. Currently ecdsa and ed25519 key types are supported.
type SSHAgentHostedKey struct {
	PublicKeyFile string
	Agent         *sshagent.Client
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
// *ecdsa.PrivateKey, ed25519.PrivateKey and SSHAgentHostedKey.
func CreatePersistentPrincipalUsingKey(ctx context.Context, key interface{}, dir string, passphrase []byte) (security.Principal, error) {
	unlock, err := initAndLockPrincipalDir(dir)
	if err != nil {
		return nil, err
	}
	defer unlock()

	// Handle ssh keys where the private key is stored in an agent and
	// we only have the public key.
	switch sshkey := key.(type) {
	case SSHAgentHostedKey:
		return createSSHAgentPrincipal(ctx, sshkey.Agent, sshkey.PublicKeyFile, dir, passphrase)
	case *SSHAgentHostedKey:
		return createSSHAgentPrincipal(ctx, sshkey.Agent, sshkey.PublicKeyFile, dir, passphrase)
	}

	if err := internal.WritePEMKeyPair(
		key,
		path.Join(dir, privateKeyFile),
		path.Join(dir, publicKeyFile),
		passphrase,
	); err != nil {
		return nil, err
	}
	signer, err := newFileSigner(ctx, path.Join(dir, privateKeyFile), passphrase)
	if err != nil {
		return nil, err
	}
	return createPrincipalUsingSigner(ctx, signer, dir)
}

func createSSHAgentPrincipal(ctx context.Context, service *sshagent.Client, sshPublicKeyFile, dir string, passphrase []byte) (security.Principal, error) {
	from, to := sshPublicKeyFile, filepath.Join(dir, filepath.Base(sshPublicKeyFile))
	if _, _, err := internal.LoadSSHPublicKeyFile(from); err != nil {
		return nil, err
	}
	if err := internal.CopyKeyFile(from, to); err != nil {
		return nil, fmt.Errorf("failed to copy ssh public key file: %v to %v: %v", from, to, err)
	}
	signer, err := service.Signer(ctx, to, passphrase)
	if err != nil {
		return nil, err
	}
	return createPrincipalUsingSigner(ctx, signer, dir)
}

func createPrincipalUsingSigner(ctx context.Context, signer security.Signer, dir string) (security.Principal, error) {
	var update time.Duration
	blessingsStore, blessingRoots, err := newStores(ctx, signer, signer.PublicKey(), dir, false, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(signer, blessingsStore, blessingRoots)
}

func mkDir(dir string) error {
	if finfo, err := os.Stat(dir); err == nil {
		if !finfo.IsDir() {
			return fmt.Errorf("%v is not a directory", dir)
		}
	} else if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create: %v: %v", dir, err)
	}
	return nil
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
