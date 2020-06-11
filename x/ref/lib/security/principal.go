// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/security/serialization"
	"v.io/x/ref/lib/security/signing/keyfile"
)

const (
	pkgPath = "v.io/x/ref/lib/security"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPersistentPrincipal()
	ErrBadPassphrase = verror.Register(pkgPath+".errBadPassphrase", verror.NoRetry, "{1:}{2:} passphrase incorrect for decrypting private key{:_}")
	// ErrPassphraseRequired is a possible return error from LoadPersistentPrincipal()
	ErrPassphraseRequired = verror.Register(pkgPath+".errPassphraseRequired", verror.NoRetry, "{1:}{2:} passphrase required for decrypting private key{:_}")

	errCantCreateSigner      = verror.Register(pkgPath+".errCantCreateSigner", verror.NoRetry, "{1:}{2:} failed to create serialization.Signer{:_}")
	errCantLoadBlessingRoots = verror.Register(pkgPath+".errCantLoadBlessingRoots", verror.NoRetry, "{1:}{2:} failed to load BlessingRoots{:_}")
	errCantLoadBlessingStore = verror.Register(pkgPath+".errCantLoadBlessingStore", verror.NoRetry, "{1:}{2:} failed to load BlessingStore{:_}")
	errNotADirectory         = verror.Register(pkgPath+".errNotADirectory", verror.NoRetry, "{1:}{2:} {3} is not a directory{:_}")
	errCantCreate            = verror.Register(pkgPath+".errCantCreate", verror.NoRetry, "{1:}{2:} failed to create {3}{:_}")
	errCantGenerateKey       = verror.Register(pkgPath+".errCantGenerateKey", verror.NoRetry, "{1:}{2:} failed to generate private key{:_}")
	errUnsupportedKeyType    = verror.Register(pkgPath+".errUnsupportedKeyType", verror.NoRetry, "{1:}{2:} unsupported key type{:_}")
)

const (
	blessingStoreDataFile     = "blessingstore.data"
	blessingStoreSigFile      = "blessingstore.sig"
	blessingStoreLockFilename = "blessings.lock"

	blessingRootsDataFile      = "blessingroots.data"
	blessingRootsSigFile       = "blessingroots.sig"
	blessingsRootsLockFilename = "blessingroots.lock"

	directoryLockfileName = "dir.lock"
	privateKeyFile        = "privatekey.pem"
	publicKeyFile         = "publickey.pem"
)

// NewPrincipal mints a new private (ecdsa) key and generates a principal
// based on this key, storing its BlessingRoots and BlessingStore in memory.
func NewPrincipal() (security.Principal, error) {
	pub, priv, err := NewECDSAKeyPair()
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(security.NewInMemoryECDSASigner(priv), NewBlessingStore(pub), NewBlessingRoots())
}

// NewPrincipalFromSigner creates a new Principal using the provided
// Signer with in-memory blessing roots and blessings store.
func NewPrincipalFromSigner(signer security.Signer) (security.Principal, error) {
	return security.CreatePrincipal(signer, NewBlessingStore(signer.PublicKey()), NewBlessingRoots())
}

// NewPrincipalFromSignerAndState creates a new Principal using the provided
// Signer with blessing roots and blessings store loaded from the supplied
// state directory.
func NewPrincipalFromSignerAndState(signer security.Signer, dir string) (security.Principal, error) {
	blessingsStore, blessingRoots, err := newStores(context.TODO(), signer, nil, dir, true, time.Duration(0))
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(signer, blessingsStore, blessingRoots)
}

// LoadPersistentPrincipal reads state for a principal (private key,
// BlessingRoots, BlessingStore) from the provided directory 'dir' and commits
// all state changes to the same directory.
// If private key file does not exist then an error 'err' is returned such that
// os.IsNotExist(err) is true.
// If private key file exists then 'passphrase' must be correct, otherwise
// ErrBadPassphrase will be returned.
// The newly loaded is principal's persistent store is locked and the returned
// unlock function must be called to release that lock.
func LoadPersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	return loadPersistentPrincipal(context.TODO(), dir, passphrase, false, time.Duration(0))
}

// LoadPersistentPrincipalWithPassphrasePrompt is like LoadPersistentPrincipal but will
// prompt for a passphrase if one is required.
func LoadPersistentPrincipalWithPassphrasePrompt(dir string) (security.Principal, error) {
	ctx := context.TODO()
	p, err := loadPersistentPrincipal(ctx, dir, nil, false, time.Duration(0))
	if err == nil {
		return p, nil
	}
	if verror.ErrorID(err) != ErrPassphraseRequired.ID {
		return nil, err
	}
	pass, err := passphrase.Get(fmt.Sprintf("Passphrase required to decrypt encrypted private key file for credentials in %v.\nEnter passphrase: ", dir))
	if err != nil {
		return nil, err
	}
	defer ZeroPassphrase(pass)
	return loadPersistentPrincipal(ctx, dir, pass, false, time.Duration(0))
}

// ZeroPassphrase overwrites the passphrase.
func ZeroPassphrase(pass []byte) {
	for i := range pass {
		pass[i] = 0
	}
}

// LoadPersistentPrincipalDaemon is like LoadPersistentPrincipal but is
// intended for use in long running applications which may not need
// to initiate changes to the principal but may need to reload their
// blessings roots and stores. If readonly is true, the principal will
// not write changes to its underlying persistent store. If a non-zero
// update duration is specified then the principal will be reloaded
// at the frequency implied by that duration. In addition, on systems
// that support it, a SIGHUP can be used to request an immediate reload.
// If pwphrase is nil, readonly is true and the private key file is encrypted
// LoadPersistentPrincipalDaemon will not attempt to create a signer and will
// instead just the principal's public key.
func LoadPersistentPrincipalDaemon(ctx context.Context, dir string, passphrase []byte, readonly bool, update time.Duration) (security.Principal, error) {
	return loadPersistentPrincipal(ctx, dir, passphrase, readonly, update)
}

func loadPersistentPrincipal(ctx context.Context, dir string, passphrase []byte, readonly bool, update time.Duration) (security.Principal, error) {
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	defer unlock()
	return newPersistentPrincipal(ctx, dir, passphrase, readonly, update)
}

func newPersistentPrincipal(ctx context.Context, dir string, passphrase []byte, readonly bool, update time.Duration) (security.Principal, error) {
	signer, err := newSigner(ctx, dir, passphrase)
	if err != nil {
		if verror.ErrorID(err) != ErrPassphraseRequired.ID || !readonly || passphrase != nil {
			return nil, err
		}
		return newPersistentPrincipalPublicKeyOnly(ctx, dir, update)
	}
	blessingsStore, blessingRoots, err := newStores(ctx, signer, signer.PublicKey(), dir, readonly, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(signer, blessingsStore, blessingRoots)
}

func newPersistentPrincipalPublicKeyOnly(ctx context.Context, dir string, update time.Duration) (security.Principal, error) {
	publicKey, err := loadPublicKey(ctx, dir)
	if err != nil {
		return nil, err
	}
	blessingsStore, blessingRoots, err := newStores(ctx, nil, publicKey, dir, true, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipalPublicKeyOnly(publicKey, blessingsStore, blessingRoots)
}

// CreatePersistentPrincipal creates a new Principal using a newly generated
// ECSDA key and commits all state changes to the provided directory.
// Use CreatePersistentPrincipalUsingKey to specify a different key
// type or to use an existing key.
//
// The private key is serialized and saved encrypted if the
// 'passphrase' is non-nil, and unencrypted otherwise.
//
// If the directory has any preexisting principal data,
// CreatePersistentPrincipal will return an error.
//
// The specified directory may not exist, in which case it will be created.
func CreatePersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	_, key, err := NewECDSAKeyPair()
	if err != nil {
		return nil, err
	}
	return CreatePersistentPrincipalUsingKey(key, dir, passphrase)
}

// CreatePersistentPrincipalUsingKey is like CreatePersistentPrincipal but
// will use the supplied key instead of creating one.
func CreatePersistentPrincipalUsingKey(key interface{}, dir string, passphrase []byte) (security.Principal, error) {
	if key == nil {
		var err error
		_, key, err = NewECDSAKeyPair()
		if err != nil {
			return nil, err
		}
	}
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	defer unlock()
	return createPersistentPrincipal(context.TODO(), key, dir, passphrase)
}

func createPersistentPrincipal(ctx context.Context, key interface{}, dir string, passphrase []byte) (principal security.Principal, err error) {
	if err := internal.WritePEMKeyPair(
		key,
		path.Join(dir, privateKeyFile),
		path.Join(dir, publicKeyFile),
		passphrase,
	); err != nil {
		return nil, err
	}
	var update time.Duration
	return newPersistentPrincipal(ctx, dir, passphrase, false, update)
}

func newSigner(ctx context.Context, dir string, passphrase []byte) (security.Signer, error) {
	// TODO(cnicolaou): determine when/how to use SSH agent, fido etc.
	svc := keyfile.NewSigningService()
	signer, err := svc.Signer(ctx, filepath.Join(dir, privateKeyFile), passphrase)
	switch {
	case err == nil:
		return signer, nil
	case verror.ErrorID(err) == internal.ErrBadPassphrase.ID:
		return nil, verror.New(ErrBadPassphrase, nil)
	case verror.ErrorID(err) == internal.ErrPassphraseRequired.ID:
		return nil, verror.New(ErrPassphraseRequired, nil)
	case os.IsNotExist(err):
		return nil, err
	default:
		return nil, verror.New(errCantCreateSigner, nil, err)
	}
}

func loadPublicKey(ctx context.Context, dir string) (security.PublicKey, error) {
	f, err := os.Open(filepath.Join(dir, publicKeyFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := internal.LoadPEMPublicKey(f)
	if err != nil {
		return nil, err
	}
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	}
	return nil, verror.New(errUnsupportedKeyType, nil, fmt.Sprintf("%T", key))
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

// SetDefault`Blessings `sets the provided blessings as default and shareable with
// all peers on provided principal's BlessingStore, and also adds it as a root
// to the principal's BlessingRoots.
func SetDefaultBlessings(p security.Principal, blessings security.Blessings) error {
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		return err
	}
	if _, err := p.BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
		return err
	}
	if err := security.AddToRoots(p, blessings); err != nil {
		return err
	}
	return nil
}

// InitDefaultBlessings uses the provided principal to create a self blessing
// for name 'name', sets it as default on the principal's BlessingStore and adds
// it as root to the principal's BlessingRoots.
// TODO(ataly): Get rid this function given that we have SetDefaultBlessings.
func InitDefaultBlessings(p security.Principal, name string) error {
	blessing, err := p.BlessSelf(name)
	if err != nil {
		return err
	}
	return SetDefaultBlessings(p, blessing)
}

func mkDir(dir string) error {
	if finfo, err := os.Stat(dir); err == nil {
		if !finfo.IsDir() {
			return verror.New(errNotADirectory, nil, dir)
		}
	} else if err := os.MkdirAll(dir, 0700); err != nil {
		return verror.New(errCantCreate, nil, dir, err)
	}
	return nil
}
