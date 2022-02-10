// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"v.io/v23/security"
	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal/lockedfile"
)

const (
	directoryLockfileName = "dir.lock"
	privateKeyFile        = "privatekey.pem"
	publicKeyFile         = "publickey.pem"

	blessingStoreDataFile     = "blessingstore.data"
	blessingStoreSigFile      = "blessingstore.sig"
	blessingStoreLockFilename = "blessings.lock"

	blessingRootsDataFile      = "blessingroots.data"
	blessingRootsSigFile       = "blessingroots.sig"
	blessingsRootsLockFilename = "blessingroots.lock"
)

type LockScope int

const (
	LockKeyStore LockScope = iota
	LockBlessingStore
	LockBlessingRoots
)

type CredentialsStoreReader interface {
	NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error)
	NewPublicKey(ctx context.Context) (security.PublicKey, error)
	RLock(context.Context, LockScope) (func(), error)
	BlessingsReader(context.Context) (SerializerReader, error)
	RootsReader(context.Context) (SerializerReader, error)
}

type CredentialsStoreWriter interface {
	Lock(context.Context, LockScope) (func(), error)
	RootsWriter(context.Context) (SerializerWriter, error)
	BlessingsWriter(context.Context) (SerializerWriter, error)
}

type CredentialsStoreCreator interface {
	CredentialsStoreReadWriter
	// ... private may be nil, public must always be provided.
	WriteKeyPair(ctx context.Context, public, private []byte) error
}

type CredentialsStoreReadWriter interface {
	CredentialsStoreReader
	CredentialsStoreWriter
}

func CreateFilesystemStore(dir string) CredentialsStoreCreator {
	return &writeableStore{fsStoreCommon: newStoreCommon(dir)}
}

func FilesystemStoreWriter(dir string) CredentialsStoreReadWriter {
	return &writeableStore{fsStoreCommon: newStoreCommon(dir)}
}

func FilesystemStoreReader(dir string) CredentialsStoreReader {
	if _, ok := ref.ReadonlyCredentialsDir(); ok {
		return &readonlyFSStore{fsStoreCommon: newStoreCommon(dir)}
	}
	return &readonlyStore{fsStoreCommon: newStoreCommon(dir)}
}

type fsStoreCommon struct {
	dir             string
	keysLock        *lockedfile.Mutex
	blessingsLock   *lockedfile.Mutex
	rootsLock       *lockedfile.Mutex
	blessingsReader SerializerReader
	rootsReader     SerializerReader
	blessingsWriter SerializerWriter
	rootsWriter     SerializerWriter
	publicKeyFile   string
	privateKeyFile  string
}

func newStoreCommon(dir string) fsStoreCommon {
	rootsSerializer := newFileSerializer(
		filepath.Join(dir, blessingRootsDataFile),
		filepath.Join(dir, blessingRootsSigFile))
	blessingsSerializer := newFileSerializer(
		filepath.Join(dir, blessingStoreDataFile),
		filepath.Join(dir, blessingStoreSigFile))
	return fsStoreCommon{
		dir:             dir,
		keysLock:        lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName)),
		blessingsLock:   lockedfile.MutexAt(filepath.Join(dir, blessingStoreLockFilename)),
		blessingsReader: SerializerReader(blessingsSerializer),
		blessingsWriter: SerializerWriter(blessingsSerializer),
		rootsLock:       lockedfile.MutexAt(filepath.Join(dir, blessingsRootsLockFilename)),
		rootsReader:     SerializerReader(rootsSerializer),
		rootsWriter:     SerializerWriter(rootsSerializer),
		publicKeyFile:   publicKeyFile,
		privateKeyFile:  privateKeyFile,
	}
}

func (store *fsStoreCommon) NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error) {
	privKeyBytes, err := os.ReadFile(filepath.Join(store.dir, store.privateKeyFile))
	if err != nil {
		return nil, err
	}
	key, err := keyRegistrar.ParsePrivateKey(ctx, privKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}
	return signerFromKey(ctx, key)
}

func (store *fsStoreCommon) NewPublicKey(ctx context.Context) (security.PublicKey, error) {
	pubKeyBytes, err := os.ReadFile(filepath.Join(store.dir, store.publicKeyFile))
	if err != nil {
		return nil, err
	}
	return publicKeyFromBytes(pubKeyBytes)
}

func (store *fsStoreCommon) RLock(ctx context.Context, scope LockScope) (unlock func(), err error) {
	switch scope {
	case LockKeyStore:
		return store.keysLock.RLock()
	case LockBlessingStore:
		return store.blessingsLock.RLock()
	case LockBlessingRoots:
		return store.rootsLock.RLock()
	}
	return nil, fmt.Errorf("unsupport lock scope: %v", scope)
}

func (store *fsStoreCommon) BlessingsReader(context.Context) (SerializerReader, error) {
	return store.blessingsReader, nil
}

func (store *fsStoreCommon) RootsReader(context.Context) (SerializerReader, error) {
	return store.rootsReader, nil
}

func (store *fsStoreCommon) BlessingsWriter(context.Context) (SerializerWriter, error) {
	return nil, fmt.Errorf("writing not implemented, this is likely a readonly store")
}

func (store *fsStoreCommon) RootsWriter(context.Context) (SerializerWriter, error) {
	return nil, fmt.Errorf("writing not implemented, this is likely a readonly store")
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
	fsStoreCommon
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

func (store *writeableStore) initAndLock() (func(), error) {
	if err := mkDir(store.dir); err != nil {
		return nil, err
	}
	flock := lockedfile.MutexAt(filepath.Join(store.dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	return unlock, nil
}

func (store *writeableStore) writeKeyFile(keyFile string, data []byte) error {
	keyFile = filepath.Join(store.dir, keyFile)
	to, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return fmt.Errorf("failed to open %v for writing: %v", keyFile, err)
	}
	defer to.Close()
	_, err = io.Copy(to, bytes.NewReader(data))
	return err
}

func (store *writeableStore) writeKeyPair(pubBytes, privBytes []byte) error {
	if err := store.writeKeyFile(store.publicKeyFile, pubBytes); err != nil {
		return err
	}
	if len(privBytes) == 0 {
		return nil
	}
	return store.writeKeyFile(store.privateKeyFile, privBytes)
}

func (store *writeableStore) WriteKeyPair(ctx context.Context, public, private []byte) error {
	reason, readonlyFS := ref.ReadonlyCredentialsDir()
	if readonlyFS {
		return fmt.Errorf("process is running on a read-only filesystem: %v", reason)
	}
	if err := writeKeyFile(filepath.Join(store.dir, publicKeyFile), public); err != nil {
		return err
	}
	if len(private) == 0 {
		return nil
	}
	return writeKeyFile(filepath.Join(store.dir, privateKeyFile), private)
}

func (store *writeableStore) Signer(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFile(ctx, store.keysLock, filepath.Join(store.dir, store.privateKeyFile), passphrase)
}

func (store *writeableStore) Lock(ctx context.Context, scope LockScope) (unlock func(), err error) {
	switch scope {
	case LockKeyStore:
		return store.keysLock.Lock()
	case LockBlessingStore:
		return store.blessingsLock.Lock()
	case LockBlessingRoots:
		return store.rootsLock.Lock()
	}
	return nil, fmt.Errorf("unsupport lock scope: %v", scope)
}

func (store *writeableStore) BlessingsWriter(ctx context.Context) (SerializerWriter, error) {
	return store.blessingsWriter, nil
}

func (store *writeableStore) RootsWriter(ctx context.Context) (SerializerWriter, error) {
	return store.rootsWriter, nil
}

func (store *readonlyStore) NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFile(ctx, store.keysLock, filepath.Join(store.dir, store.privateKeyFile), passphrase)
}

func (store *readonlyFSStore) NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFileLocked(ctx, filepath.Join(store.dir, store.privateKeyFile), passphrase)
}

func (store *readonlyStore) NewPublicKey(ctx context.Context) (security.PublicKey, error) {
	return publicKeyFromFile(ctx, store.keysLock, filepath.Join(store.dir, store.publicKeyFile))
}

func (store *readonlyFSStore) NewPublicKey(ctx context.Context) (security.PublicKey, error) {
	return publicKeyFromFileLocked(ctx, filepath.Join(store.dir, store.publicKeyFile))
}

func signerFromFile(ctx context.Context, flock *lockedfile.Mutex, filename string, passphrase []byte) (security.Signer, error) {
	unlock, err := flock.RLock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return signerFromFileLocked(ctx, filename, passphrase)
}

func signerFromFileLocked(ctx context.Context, filename string, passphrase []byte) (security.Signer, error) {
	privBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	private, err := keyRegistrar.ParsePrivateKey(ctx, privBytes, passphrase)
	if err != nil {
		return nil, translatePassphraseError(err)
	}
	return signerFromKey(ctx, private)
}

func publicKeyFromFile(ctx context.Context, flock *lockedfile.Mutex, filename string) (security.PublicKey, error) {
	unlock, err := flock.RLock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return publicKeyFromFileLocked(ctx, filename)
}

func publicKeyFromFileLocked(ctx context.Context, filename string) (security.PublicKey, error) {
	pubBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return publicKeyFromBytes(pubBytes)
}
