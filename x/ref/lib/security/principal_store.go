// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"fmt"
	"io"
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/internal/lockedfile"
)

type UseCredentialsStore interface {
	signer(ctx *context.T, passphrase []byte) (security.Signer, error)
	openBlessingsReader() (io.ReadCloser, error)
	openBessingsWriter() (io.ReadWriteCloser, error)
	openRootsReader() (io.ReadCloser, error)
	openRootsWriter() (io.ReadWriteCloser, error)
}

type CreateCredentialsStore interface {
	writePrivateKey(ctx *context.T, key crypto.PrivateKey, passphrase []byte, format PrivateKeyEncryption) error
	writePublicKey(ctx *context.T, key crypto.PublicKey) error
	UseCredentialsStore
}

func CreateFilesystemStore(dir string) CreateCredentialsStore {
	return &writeableStore{fsStoreCommon: common}
}

func LoadFilesystemStore(dir string, readonly bool) UseCredentialsStore {
	common := newStoreCommon(dir)

	return &readonlyStore{fsStoreCommon: common}
}

type fsStoreCommon struct {
	dir            string
	storeLock      *lockedfile.Mutex
	blessingsLock  *lockedfile.Mutex
	rootsLock      *lockedfile.Mutex
	publicKeyFile  string
	privateKeyFile string
}

func unexpectedReadonlyFS() error {
	reason, readonlyFS := ref.ReadonlyCredentialsDir()
	if readonlyFS {
		return fmt.Errorf("process is running on a read-only filesystem: %v", reason)
	}
	return nil
}

func newStoreCommon(dir string) fsStoreCommon {
	return fsStoreCommon{
		dir:           dir,
		storeLock:     lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName)),
		blessingsLock: lockedfile.MutexAt(filepath.Join(dir, blessingStoreLockFilename)),
		rootsLock:     lockedfile.MutexAt(filepath.Join(dir, blessingsRootsLockFilename)),
	}
}

// Writeable filesystem store.
type writeableStore struct {
	fsStoreCommon
}

// Only allow read-only operations on the filesystem store.
type readonlyStore struct {
	fsStoreCommon
}

// Hosted on a readonly filesystem.
type readonlyFSStore struct {
	dir string
}

func (store *writeableStore) WriteKeyPair(ctx *context.T, key crypto.PrivateKey, passphrase []byte, format PrivateKeyEncryption) error {
	if err := unexpectedReadonlyFS(); err != nil {
		return err
	}
	unlock, err := initAndLockPrincipalDir(store.dir)
	if err != nil {
		return err
	}
	defer unlock()
	publicKey, err := publicKeyFromPrivate(key)
	if err != nil {
		return err
	}
	store.pubKey = filepath.Join(store.dir, publicKeyFile)
	if err := internal.WritePublicKeyPEM(publicKey, store.pubKey); err != nil {
		return err
	}
	if isSSHAgentKey(key) {
		return nil
	}
	store.privKey = filepath.Join(store.dir, privateKeyFile)
	switch format {
	case EncryptedPEM:
		return internal.WritePrivateKeyPEM(key, store.privKey, passphrase)
	default:
		// TODO(cnicolaou): add pkcs8
		return fmt.Errorf("unsupported private key format: %v", format)
	}
	return nil
}

func (store *writeableStore) WritePublicKey(ctx *context.T, key crypto.PublicKey, passphrase []byte, format PrivateKeyEncryption) error {
	if err := unexpectedReadonlyFS(); err != nil {
		return err
	}
	unlock, err := initAndLockPrincipalDir(store.dir)
	if err != nil {
		return err
	}
	defer unlock()

	store.pubKey = filepath.Join(store.dir, publicKeyFile)
	if err := internal.WritePublicKeyPEM(publicKey, store.pubKey); err != nil {
		return err
	}
	if isSSHAgentKey(key) {
		return nil
	}
	store.privKey = filepath.Join(store.dir, privateKeyFile)
	switch format {
	case EncryptedPEM:
		return internal.WritePrivateKeyPEM(key, store.privKey, passphrase)
	default:
		// TODO(cnicolaou): add pkcs8
		return fmt.Errorf("unsupported private key format: %v", format)
	}
	return nil
}

func (store *writeableStore) Signer(ctx *context.T, passphrase []byte) (security.Signer, error) {
	unlock, err := lockedfile.MutexAt(store.lockfile).RLock()
	if err != nil {
		return nil, err
	}
	defer unlock()

	// what about ssh keys??
	return newFileSigner(ctx, filepath.Join(store.privKey), passphrase)
}

func (fs *fsStore) Stores(ctx *context.T) (security.BlessingStore, security.BlessingRoots, error) {

}

/*
func (fs *fsStore) InitializeAndLock(ctx *context.T) (StoreUnlock, error) {
	ul, err := initAndLockPrincipalDir(fs.dir)
	if err != nil {
		return nil, err
	}
	return func(ctx *context.T) { ul() }, nil
}

func (fs *fsStore) Lock(ctx *context.T) (StoreUnlock, error) {
	flock := lockedfile.MutexAt(filepath.Join(fs.dir, directoryLockfileName))
	ul, err := flock.Lock()
	if err != nil {
		return nil, err
	}
	return func(ctx *context.T) { ul() }, nil
}*/
