// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"os"
	"path"

	"v.io/v23/security"
	"v.io/v23/verror"
)

const pkgPath = "v.io/x/ref/lib/security"

var (
	errCantCreateSigner      = verror.Register(pkgPath+".errCantCreateSigner", verror.NoRetry, "{1:}{2:} failed to create serialization.Signer{:_}")
	errCantLoadBlessingRoots = verror.Register(pkgPath+".errCantLoadBlessingRoots", verror.NoRetry, "{1:}{2:} failed to load BlessingRoots{:_}")
	errCantLoadBlessingStore = verror.Register(pkgPath+".errCantLoadBlessingStore", verror.NoRetry, "{1:}{2:} failed to load BlessingStore{:_}")
	errCantInitPrivateKey    = verror.Register(pkgPath+".errCantInitPrivateKey", verror.NoRetry, "{1:}{2:} failed to initialize private key{:_}")
	errNotADirectory         = verror.Register(pkgPath+".errNotADirectory", verror.NoRetry, "{1:}{2:} {3} is not a directory{:_}")
	errCantCreate            = verror.Register(pkgPath+".errCantCreate", verror.NoRetry, "{1:}{2:} failed to create {3}{:_}")
	errCantOpenForWriting    = verror.Register(pkgPath+".errCantOpenForWriting", verror.NoRetry, "{1:}{2:} failed to open {3} for writing{:_}")
	errCantGenerateKey       = verror.Register(pkgPath+".errCantGenerateKey", verror.NoRetry, "{1:}{2:} failed to generate private key{:_}")
	errCantSaveKey           = verror.Register(pkgPath+".errCantSaveKey", verror.NoRetry, "{1:}{2:} failed to save private key to {3}{:_}")
)

const (
	blessingStoreDataFile = "blessingstore.data"
	blessingStoreSigFile  = "blessingstore.sig"

	blessingRootsDataFile = "blessingroots.data"
	blessingRootsSigFile  = "blessingroots.sig"

	privateKeyFile = "privatekey.pem"
)

// NewPrincipal mints a new private key and generates a principal based on this
// key, storing its BlessingRoots and BlessingStore in memory.
func NewPrincipal() (security.Principal, error) {
	pub, priv, err := NewPrincipalKey()
	if err != nil {
		return nil, err
	}
	return security.CreatePrincipal(security.NewInMemoryECDSASigner(priv), newInMemoryBlessingStore(pub), newInMemoryBlessingRoots())
}

// PrincipalStateSerializer is used to persist BlessingRoots/BlessingStore state
// for a principal with the provided SerializerReaderWriters.
type PrincipalStateSerializer struct {
	BlessingRoots SerializerReaderWriter
	BlessingStore SerializerReaderWriter
}

// NewPrincipalStateSerializer is a convenience function that returns a
// serializer for BlessingStore and BlessingRoots given a directory location. We
// create the directory if it does not already exist.
func NewPrincipalStateSerializer(dir string) (*PrincipalStateSerializer, error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	return &PrincipalStateSerializer{
		BlessingRoots: NewFileSerializer(path.Join(dir, blessingRootsDataFile), path.Join(dir, blessingRootsSigFile)),
		BlessingStore: NewFileSerializer(path.Join(dir, blessingStoreDataFile), path.Join(dir, blessingStoreSigFile)),
	}, nil
}

// NewPrincipalFromSigner creates a new principal using the provided Signer. If
// previously persisted state is available, we use the serializers to populate
// BlessingRoots/BlessingStore for the Principal. If provided, changes to the
// state are persisted and committed with the same serializers. Otherwise, the
// state (ie: BlessingStore, BlessingRoots) is kept in memory.
func NewPrincipalFromSigner(signer security.Signer, state *PrincipalStateSerializer) (security.Principal, error) {
	if state == nil {
		return security.CreatePrincipal(signer, newInMemoryBlessingStore(signer.PublicKey()), newInMemoryBlessingRoots())
	}
	serializationSigner, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		return nil, verror.New(errCantCreateSigner, nil, err)
	}
	blessingRoots, err := newPersistingBlessingRoots(state.BlessingRoots, serializationSigner)
	if err != nil {
		return nil, verror.New(errCantLoadBlessingRoots, nil, err)
	}
	blessingStore, err := newPersistingBlessingStore(state.BlessingStore, serializationSigner)
	if err != nil {
		return nil, verror.New(errCantLoadBlessingStore, nil, err)
	}
	return security.CreatePrincipal(signer, blessingStore, blessingRoots)
}

// TODO(caprita): LoadPersistentPrincipal and CreatePersistentPrincipal load the
// principal without concern to locking or sharing.  Provide alternatives to
// these in agentlib that lock or use agent, and replace as many existing uses
// of LoadPersistentPrincipal and CreatePersistentPrincipal with the safer
// alternatives.

// LoadPersistentPrincipal reads state for a principal (private key,
// BlessingRoots, BlessingStore) from the provided directory 'dir' and commits
// all state changes to the same directory.
// If private key file does not exist then an error 'err' is returned such that
// os.IsNotExist(err) is true.
// If private key file exists then 'passphrase' must be correct, otherwise
// ErrBadPassphrase will be returned.
func LoadPersistentPrincipal(dir string, passphrase []byte) (security.Principal, error) {
	key, err := loadKeyFromDir(dir, passphrase)
	if err != nil {
		return nil, err
	}
	// Note, dir must exist if we reach here, so the mkdir(dir) in
	// NewPrincipalStateSerializer is a no-op.
	state, err := NewPrincipalStateSerializer(dir)
	if err != nil {
		return nil, err
	}
	return NewPrincipalFromSigner(security.NewInMemoryECDSASigner(key), state)
}

// CreatePersistentPrincipal creates a new principal (private key,
// BlessingRoots, BlessingStore) and commits all state changes to the provided
// directory.
//
// The generated private key is serialized and saved encrypted if the
// 'passphrase' is non-nil, and unencrypted otherwise.
//
// If the directory has any preexisting principal data,
// CreatePersistentPrincipal will return an error.
//
// The specified directory may not exist, in which case it gets created by this
// function.
func CreatePersistentPrincipal(dir string, passphrase []byte) (principal security.Principal, err error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	key, err := initKey(dir, passphrase)
	if err != nil {
		return nil, verror.New(errCantInitPrivateKey, nil, err)
	}
	state, err := NewPrincipalStateSerializer(dir)
	if err != nil {
		return nil, err
	}
	return NewPrincipalFromSigner(security.NewInMemoryECDSASigner(key), state)
}

// SetDefaultBlessings sets the provided blessings as default and shareable with
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

func loadKeyFromDir(dir string, passphrase []byte) (*ecdsa.PrivateKey, error) {
	keyFile := path.Join(dir, privateKeyFile)
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := LoadPEMKey(f, passphrase)
	if err != nil {
		return nil, err
	}
	return key.(*ecdsa.PrivateKey), nil
}

func initKey(dir string, passphrase []byte) (*ecdsa.PrivateKey, error) {
	keyFile := path.Join(dir, privateKeyFile)
	f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, verror.New(errCantOpenForWriting, nil, keyFile, err)
	}
	defer f.Close()
	_, key, err := NewPrincipalKey()
	if err != nil {
		return nil, verror.New(errCantGenerateKey, nil, err)
	}
	if err := SavePEMKey(f, key, passphrase); err != nil {
		return nil, verror.New(errCantSaveKey, nil, keyFile, err)
	}
	return key, nil
}
