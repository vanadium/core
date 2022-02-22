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

	blessingRootsDataFile     = "blessingroots.data"
	blessingRootsSigFile      = "blessingroots.sig"
	blessingRootsLockFilename = "blessingroots.lock"
)

// LockScope represents the scope of a read/write or read-only lock on
// a credentials store.
type LockScope int

const (
	// LockKeyStore requests a lock on the key information.
	LockKeyStore LockScope = iota
	// LockBlessingStore requests a lock on the blessings store.
	LockBlessingStore
	// LockBlessingRoots requests a lock on the blessings roots.
	LockBlessingRoots
)

// CredentialsStoreReader represents the read-only operations on a credentials
// store. The CredentialsStore interfaces allow for alternative implementations
// of credentials stores to be used with the rest of this package. For example,
// a store that uses AWS S3 could simply implement these APIs and then be usable
// by the existing blessings store and blessing roots implementations.
//
// All operations must be guarded by a read-only lock as obtained via
// RLock for the appropriate lock scope. NewSigner and NewPublicKey should
// be guarded by a LockKeyStore scope, BlessingsReader by LockBlessingStore and
// RootsReader by LockBlessingRoots.
type CredentialsStoreReader interface {
	RLock(context.Context, LockScope) (func(), error)
	NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error)
	NewPublicKey(ctx context.Context) (security.PublicKey, error)
	BlessingsReader(context.Context) (SerializerReader, error)
	RootsReader(context.Context) (SerializerReader, error)
}

// CredentialsStoreWriter represents the write operations on a credentials
// store.
//
// All operations must be guarded by a as obtained via LLock for the
// appropriate lock scope. BlessingsWriter should be guarded by LockBlessingStore
// and RootsWriter by LockBlessingRoots.
type CredentialsStoreWriter interface {
	Lock(context.Context, LockScope) (func(), error)
	BlessingsWriter(context.Context) (SerializerWriter, error)
	RootsWriter(context.Context) (SerializerWriter, error)
}

// CredentialsStoreCreator represents the operations to create a new
// credentials store.
type CredentialsStoreCreator interface {
	CredentialsStoreReadWriter
	// WriteKeyPair writes the specified key information to the store.
	// Note the public key bytes must always be provided but the private
	// key bytes may be nil.
	//
	// WriteKeyPair must be guarded by a lock of scope LockKeyStore.
	WriteKeyPair(ctx context.Context, public, private []byte) error
}

// CredentialsStoreReadWriter represents a mutable credentials store.
type CredentialsStoreReadWriter interface {
	CredentialsStoreReader
	CredentialsStoreWriter
}

// CreateFilesystemStore returns a store hosted on the local filesystem
// that can be used to create a new credentials store (and hence principal).
func CreateFilesystemStore(dir string) (CredentialsStoreCreator, error) {
	store := &writeableStore{fsStoreCommon: newStoreCommon(dir)}
	if err := store.initLocks(); err != nil {
		return nil, err
	}
	return store, nil
}

// FilesystemStoreWriter returns a CredentialsStoreReadWriter for an
// existing local file system credentials store.
func FilesystemStoreWriter(dir string) CredentialsStoreReadWriter {
	return &writeableStore{fsStoreCommon: newStoreCommon(dir)}
}

// FilesystemStoreReader returns a CredentialsStoreReader for an
// existing local file system credentials store.
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
		rootsLock:       lockedfile.MutexAt(filepath.Join(dir, blessingRootsLockFilename)),
		rootsReader:     SerializerReader(rootsSerializer),
		rootsWriter:     SerializerWriter(rootsSerializer),
		publicKeyFile:   publicKeyFile,
		privateKeyFile:  privateKeyFile,
	}
}

func (store *fsStoreCommon) NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFileLocked(ctx, filepath.Join(store.dir, store.privateKeyFile), passphrase)
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

func (store *writeableStore) initLocks() error {
	if err := mkDir(store.dir); err != nil {
		return err
	}
	for _, lock := range []*lockedfile.Mutex{store.keysLock, store.blessingsLock, store.rootsLock} {
		unlock, err := lock.Lock()
		if err != nil {
			return err
		}
		unlock()
	}
	return nil
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

func (store *writeableStore) WriteKeyPair(ctx context.Context, public, private []byte) error {
	reason, readonlyFS := ref.ReadonlyCredentialsDir()
	if readonlyFS {
		return fmt.Errorf("process is running on a read-only filesystem: %v", reason)
	}
	if err := store.writeKeyFile(publicKeyFile, public); err != nil {
		return err
	}
	if len(private) == 0 {
		return nil
	}
	return store.writeKeyFile(privateKeyFile, private)
}

func (store *writeableStore) Signer(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFileLocked(ctx, filepath.Join(store.dir, store.privateKeyFile), passphrase)
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
	return signerFromFileLocked(ctx, filepath.Join(store.dir, store.privateKeyFile), passphrase)
}

func (store *readonlyFSStore) NewSigner(ctx context.Context, passphrase []byte) (security.Signer, error) {
	return signerFromFileLocked(ctx, filepath.Join(store.dir, store.privateKeyFile), passphrase)
}

func (store *readonlyStore) NewPublicKey(ctx context.Context) (security.PublicKey, error) {
	return publicKeyFromFileLocked(ctx, filepath.Join(store.dir, store.publicKeyFile))
}

func (store *readonlyFSStore) NewPublicKey(ctx context.Context) (security.PublicKey, error) {
	return publicKeyFromFileLocked(ctx, filepath.Join(store.dir, store.publicKeyFile))
}

func (store *readonlyFSStore) RLock(ctx context.Context, scope LockScope) (unlock func(), err error) {
	switch scope {
	case LockKeyStore, LockBlessingStore, LockBlessingRoots:
		// read-locks always work for read-only filesystems.
		return func() {}, nil
	}
	return nil, fmt.Errorf("unsupport lock scope: %v", scope)
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

func publicKeyFromFileLocked(ctx context.Context, filename string) (security.PublicKey, error) {
	pubBytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return publicKeyFromBytes(pubBytes)
}
