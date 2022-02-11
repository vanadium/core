// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"errors"
	"fmt"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/passphrase"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPersistentPrincipal()
	ErrBadPassphrase = verror.NewID("errBadPassphrase")
	// ErrPassphraseRequired is a possible return error from LoadPersistentPrincipal()
	ErrPassphraseRequired = verror.NewID("errPassphraseRequired")
)

// NewPrincipal mints a new private (ecdsa) key and generates a principal
// based on this key, storing its BlessingRoots and BlessingStore in memory.
func NewPrincipal() (security.Principal, error) {
	return CreatePrincipalOpts(context.TODO())
}

// NewPrincipalFromSigner creates a new Principal using the provided
// Signer with in-memory blessing roots and blessings store.
func NewPrincipalFromSigner(signer security.Signer) (security.Principal, error) {
	return CreatePrincipalOpts(context.TODO(), UseSigner(signer))
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
	return LoadPrincipalOpts(context.TODO(),
		LoadFrom(FilesystemStoreWriter(dir)),
		LoadUsingPassphrase(passphrase))
}

// LoadPersistentPrincipalWithPassphrasePrompt is like LoadPersistentPrincipal but will
// prompt for a passphrase if one is required.
func LoadPersistentPrincipalWithPassphrasePrompt(dir string) (security.Principal, error) {
	ctx := context.TODO()
	store := FilesystemStoreWriter(dir)
	p, err := LoadPrincipalOpts(ctx, LoadFrom(store))
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
	return LoadPrincipalOpts(ctx, LoadFrom(store), LoadUsingPassphrase(pass))
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
	opts := []LoadPrincipalOption{}
	if readonly {
		opts = append(opts, LoadFromReadonly(FilesystemStoreReader(dir)))
	} else {
		opts = append(opts, LoadFrom(FilesystemStoreWriter(dir)))
	}
	opts = append(opts,
		LoadUsingPassphrase(passphrase),
		LoadRefreshInterval(update),
		LoadAllowPublicKeyPrincipal(true))
	return LoadPrincipalOpts(ctx, opts...)
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
