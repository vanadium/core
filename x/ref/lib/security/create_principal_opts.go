// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/security"
)

// CreatePrincipalOption represents an option to CreatePrincipalOpts.
type CreatePrincipalOption func(o *createPrincipalOptions) error

type createPrincipalOptions struct {
	privateKey           crypto.PrivateKey
	dir                  string
	passPhrase           []byte
	readonly             bool
	privateKeyEncryption PrivateKeyEncryption
	blessingStore        security.BlessingStore
	blessingRoots        security.BlessingRoots
}

func FilesystemPrincipalOption(dir string) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.dir = dir
		return nil
	}
}

// PrivateKeyOpt specifies the private key to use when creating the
// principal. If not specified an ecdsa key with the P256 curve
// will be created and used. The supported key types are:
// *ecdsa.PrivateKey, ed25519.PrivateKey, *rsa.PrivateKey,
// SSHAgentHostedKey, *SSHAgentHostedKey.
func PrivateKeyOpt(pk crypto.PrivateKey) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		switch pk.(type) {
		case *ecdsa.PrivateKey:
		case *rsa.PrivateKey:
		case *ed25519.PrivateKey:
		case SSHAgentHostedKey:
		case *SSHAgentHostedKey:
		default:
			return fmt.Errorf("unsupported key type: %T", pk)
		}
		o.privateKey = pk
		return nil
	}
}

// PrivateKeyEncryption represents the format used for encrypted private keys.
type PrivateKeyEncryption int

const (
	EncryptedPEM   PrivateKeyEncryption = iota
	EncryptedPKCS8                      // This is a placeholder and is not yet implemented.
)

func (pf PrivateKeyEncryption) String() string {
	switch pf {
	case EncryptedPEM:
		return "encrypted-pem"
	case EncryptedPKCS8:
		return "encrypted-pkcs8"
	}
	return "unknown"
}

func PrivateKeyEOpt(format PrivateKeyEncryption) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.privateKeyEncryption = format
		return nil
	}
}

func PrivateKeyPassphrase(passphrase []byte) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		o.passPhrase = passphrase
		return nil
	}
}

/*
// PrivateKeyStoreType represents the type of storage used for both encrypted and
// cleartext private keys.
type PrivateKeyStoreType int

const (
	PrivateKeyStorePEM PrivateKeyStoreType = iota
	PrivateKeyStorePKCS8
	PrivateKeyStoreSSHAgent
)

// PrivateKeyStoreOpt specifies how a private key is to be stored. The default
// is currently PEM, but will be switched to PKCS8 in the future. Other
// storage systems such as AWS' secret store will also be added in the
// near future.
func PrivateKeyStoreOpt(typ PrivateKeyStoreType) CreatePrincipalOption {
	return func(o *createPrincipalOptions) error {
		switch typ {
		case PrivateKeyStorePEM:
		case PrivateKeyStorePKCS8:
		case PrivateKeyStoreSSHAgent:
		default:
			return fmt.Errorf("unsupported storage type: %v", typ)
		}
		o.privateKeyStore = typ
		return nil
	}
}*/

// CreatePrincipalOpts creates a Principal using the specified options. It is
// intended to replace all of the other 'Create' methods provided by this
// package.
// NOTE: this is experimental and not fully tested yet.
func CreatePrincipalOpts(ctx *context.T, opts ...CreatePrincipalOption) (security.Principal, error) {
	var o createPrincipalOptions
	for _, fn := range opts {
		if err := fn(&o); err != nil {
			return nil, err
		}
	}
	if o.privateKey == nil {
		pk, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
		if err != nil {
			return nil, err
		}
		o.privateKey = pk
	}
	if len(o.dir) == 0 {
		return o.createInMemoryPrincipal(ctx)
	}
	return o.createPersistentPrincipal(ctx)
}

func isSSHAgentKey(key crypto.PrivateKey) bool {
	switch key.(type) {
	case SSHAgentHostedKey:
		return true
	case *SSHAgentHostedKey:
		return true
	}
	return false
}

func (o *createPrincipalOptions) copySSHAgentKey() (*SSHAgentHostedKey, bool) {
	switch k := o.privateKey.(type) {
	case SSHAgentHostedKey:
		sk := &SSHAgentHostedKey{}
		*sk = k
		return sk, true
	case *SSHAgentHostedKey:
		sk := &SSHAgentHostedKey{}
		*sk = *k
		return sk, true
	}
	return nil, false
}

func (o *createPrincipalOptions) createInMemoryPrincipal(ctx *context.T) (security.Principal, error) {
	return nil, nil
}

func (o *createPrincipalOptions) createPersistentPrincipal(ctx *context.T) (security.Principal, error) {
	return nil, nil
}
