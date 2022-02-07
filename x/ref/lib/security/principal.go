// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/passphrase"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPersistentPrincipal()
	ErrBadPassphrase = verror.NewID("errBadPassphrase")
	// ErrPassphraseRequired is a possible return error from LoadPersistentPrincipal()
	ErrPassphraseRequired = verror.NewID("errPassphraseRequired")
)

const (
	blessingStoreDataFile     = "blessingstore.data"
	blessingStoreSigFile      = "blessingstore.sig"
	blessingStoreLockFilename = "blessings.lock"

	blessingRootsDataFile      = "blessingroots.data"
	blessingRootsSigFile       = "blessingroots.sig"
	blessingsRootsLockFilename = "blessingroots.lock"

	directoryLockfileName = "dir.lock"
)

// NewPrincipal mints a new private (ecdsa) key and generates a principal
// based on this key, storing its BlessingRoots and BlessingStore in memory.
func NewPrincipal() (security.Principal, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}
	signer, err := security.NewInMemoryECDSASigner(priv)
	if err != nil {
		return nil, err
	}
	pub := security.NewECDSAPublicKey(&priv.PublicKey)
	return security.CreatePrincipal(signer, NewBlessingStore(pub), NewBlessingRoots())
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
	if !errors.Is(err, ErrPassphraseRequired) {
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
// If passphrase is nil, readonly is true and the private key file is encrypted
// LoadPersistentPrincipalDaemon will not attempt to create a signer and will
// instead just use the principal's public key.
func LoadPersistentPrincipalDaemon(ctx context.Context, dir string, passphrase []byte, readonly bool, update time.Duration) (security.Principal, error) {
	return loadPersistentPrincipal(ctx, dir, passphrase, readonly, update)
}

func loadPersistentPrincipal(ctx context.Context, dir string, passphrase []byte, readonly bool, update time.Duration) (security.Principal, error) {
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	loader := func() error { return nil }
	var unlock func()
	var err error
	if readonly {
		unlock, err = readLockAndLoad(flock, loader)
	} else {
		unlock, err = writeLockAndLoad(flock, loader)
	}
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
	signer, err := signerFromDir(ctx, dir, passphrase)
	if err != nil {
		if !readonly {
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
	publicKey, err := publicKeyFromDir(dir)
	if err != nil {
		return nil, err
	}
	blessingsStore, blessingRoots, err := newStores(ctx, nil, publicKey, dir, true, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipalPublicKeyOnly(publicKey, blessingsStore, blessingRoots)
}

// SetDefault`Blessings `sets the provided blessings as default and shareable with
// all peers on provided principal's BlessingStore, and also adds it as a root
// to the principal's BlessingRoots.
func SetDefaultBlessings(p security.Principal, blessings security.Blessings) error {
	// TODO(cnicolaou): ideally should make this atomic and undo the effects of
	// AddToRoots if SetDefault fails etc, etc.

	// Call AddToRoots first so that any entity waiting for notification
	// of a change to the default blessings (the notification is via the
	// SetDefault method below) is guaranteed to have the new roots installed
	// when they receive the notification.
	if err := security.AddToRoots(p, blessings); err != nil {
		return err
	}
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		return err
	}
	if _, err := p.BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
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
