// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/passphrase"
	"v.io/x/ref/lib/security/signing/keyfile"
	"v.io/x/ref/lib/security/signing/sshagent"
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
	privateKeyFile        = "privatekey.pem"
	publicKeyFile         = "publickey.pem"
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
// instead just the principal's public key.
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
	signer, err := newSignerFromState(ctx, dir, passphrase)
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

// newSignerFromState will create a signer based on the keys found in the
// supplied state directory. It looks for a privatekey.pem or an ssh
// .pub file which it will use to lookup the matching private key in
// its accessible ssh agent.
func newSignerFromState(ctx context.Context, dir string, passphrase []byte) (security.Signer, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var privatePEM, publicSSH string
	for _, file := range files {
		name := file.Name()
		switch {
		case name == privateKeyFile:
			privatePEM = name
		case strings.HasSuffix(name, ".pub"):
			publicSSH = name
		}
	}
	if len(privatePEM) > 0 && len(publicSSH) > 0 {
		return nil, fmt.Errorf("multiple key files found: %v and %v", privatePEM, publicSSH)
	}
	switch {
	case len(privatePEM) > 0:
		return newFileSigner(ctx, filepath.Join(dir, privatePEM), passphrase)
	case len(publicSSH) > 0:
		return newSSHAgentSigner(ctx, filepath.Join(dir, publicSSH), passphrase)
	}
	return nil, fmt.Errorf("failed to find an appropriate private or public key file in %v", dir)
}

func handleSignerError(signer security.Signer, err error) (security.Signer, error) {
	switch {
	case err == nil:
		return signer, nil
	case errors.Is(err, internal.ErrBadPassphrase):
		return nil, ErrBadPassphrase.Errorf(nil, "passphrase incorrect for decrypting private key")
	case errors.Is(err, internal.ErrPassphraseRequired):
		return nil, ErrPassphraseRequired.Errorf(nil, "passphrase required for decrypting private key")
	case os.IsNotExist(err):
		return nil, err
	default:
		return nil, fmt.Errorf("failed to create serialization.Signer: %v", err)
	}
}

func newFileSigner(ctx context.Context, filename string, passphrase []byte) (security.Signer, error) {
	svc := keyfile.NewSigningService()
	signer, err := svc.Signer(ctx, filename, passphrase)
	return handleSignerError(signer, err)
}

func newSSHAgentSigner(ctx context.Context, filename string, passphrase []byte) (security.Signer, error) {
	svc := sshagent.NewSigningService()
	svc.(*sshagent.Client).SetAgentSockName(DefaultSSHAgentSockNameFunc())
	signer, err := svc.Signer(ctx, filename, passphrase)
	return handleSignerError(signer, err)
}

func newPersistentPrincipalPublicKeyOnly(ctx context.Context, dir string, update time.Duration) (security.Principal, error) {
	publicKey, err := newPublicKeyFromState(ctx, dir)
	if err != nil {
		return nil, err
	}
	blessingsStore, blessingRoots, err := newStores(ctx, nil, publicKey, dir, true, update)
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipalPublicKeyOnly(publicKey, blessingsStore, blessingRoots)
}

// newPublicKeyFromState looks for a public key file in the specified
// state directory. It will accept either a publickey.pem file or an ssh
// .pub file.
func newPublicKeyFromState(ctx context.Context, dir string) (security.PublicKey, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var publicPEM, publicSSH string
	for _, file := range files {
		name := file.Name()
		switch {
		case name == publicKeyFile:
			publicPEM = name
		case strings.HasSuffix(name, ".pub"):
			publicSSH = name
		}
	}
	if len(publicPEM) > 0 && len(publicSSH) > 0 {
		return nil, fmt.Errorf("multiple key files found: %v and %v", publicPEM, publicSSH)
	}
	var key interface{}
	switch {
	case len(publicPEM) > 0:
		key, err = internal.LoadPEMPublicKeyFile(filepath.Join(dir, publicPEM))
	case len(publicSSH) > 0:
		var sshKey ssh.PublicKey
		sshKey, _, err = internal.LoadSSHPublicKeyFile(filepath.Join(dir, publicSSH))
		if err == nil {
			key, err = internal.CryptoKeyFromSSHKey(sshKey)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %v", err)
	}
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return security.NewECDSAPublicKey(k), nil
	case ed25519.PublicKey:
		return security.NewED25519PublicKey(k), nil
	}
	return nil, fmt.Errorf("unsupported key type %T", key)
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
