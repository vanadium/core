// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sshkeys provides support for using ssh keys with the security/keys
// package, including private keys hosted within an ssh agent. In theory any
// ssh agent can be used, including those that use FIDO keys or other security
// enclaves (eg. Apple's T2) to store and sign keys.
package sshkeys

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
)

const (
	sshECDSA = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBHlUi+WP0MzrWGDvdsn2T719H5tBqT+TMHn09WqsznwrvlCHsZxvkfmtDftZyn5aRr3FOBUQzBMQt7fXwW/zeeg= ecdsa-256
ecdsa-sha2-nistp384`
	sshED25519 = `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICNRiLs+nkzkIzekQc80ntmTh/xr5ok2hGYPnk1IzRA5 ed25519`
	sshRSA     = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDJhwZg9AsyVevAEjCs1wFk502IDrFAqQ2JtTBFsJywqNvuuGVdnUA2hXpGlkd5xLdj21V630C+7YCWHuEpg9iTdlZ92ecUHruPFOa11kdYLyHpaD69C3Ok26D3wXtXhljCxblRRqQqzVeaeHMD9L82J3KUDpHHObRzzkaSRDB30QHWiSItKK9Qd4njGTU9ANSehhb1zRJG08wGXIvAy/Da2+kvq0Tq9Rz+L+OXdkmPVfQjI2P41RQh22Z/ME8szkNzv4U1Fz3NoS6+EpPAs8WBWuIg+cDTjB9o/rP2vjz+BLOmHB2bYgGjDsv9f1I/mYedWB6aJoY8+N+V11bpt/Nj rsa-2048`

	sshPrivateKeyPEMType = "OPENSSH PRIVATE KEY"
)

// MustRegister is like Register but panics on error.
func MustRegister(r *keys.Registrar) {
	if err := Register(r); err != nil {
		panic(err)
	}
}

const indirectHeaderValue = "sshagent-hosted-key"

var marshalHostedKeyIndirect = keys.MarshalFuncForIndirection(indirectHeaderValue)

// Register registers the required functions for handling ssh public and
// private key files as well ssh agent hosted private key files via
// the x/ref/security/keys package.
func Register(r *keys.Registrar) error {
	r.RegisterPublicKeyTextParser(parseAuthorizedKey)
	r.RegisterPrivateKeyParser(parseOpensshPrivateKey, sshPrivateKeyPEMType, nil)
	r.RegisterDecrypter(decryptOpensshPrivateKey, sshPrivateKeyPEMType, nil)

	r.RegisterPrivateKeyMarshaler(marshalHostedKey, (*HostedKey)(nil))
	r.RegisterIndirectPrivateKeyParser(parseSSHAgentPrivateKeyFile, indirectHeaderValue)

	// golang.org/x/crypto/ssh returns private types for ssh public types
	// so we register an api handler for them here. The Signer method will
	// never be called since crypto/ssh returns crypto/ecdsa etc types
	// for the private keys.
	ecdsaKey, _ := parseAuthorizedKey([]byte(sshECDSA))
	ed25519Key, _ := parseAuthorizedKey([]byte(sshED25519))
	rsaKey, _ := parseAuthorizedKey([]byte(sshRSA))
	if err := r.RegisterAPI((*sshkeyAPI)(nil), ecdsaKey, ed25519Key, rsaKey); err != nil {
		return err
	}
	return r.RegisterAPI((*hostedKeyAPI)(nil), (*HostedKey)(nil))
}

func marshalHostedKey(key crypto.PrivateKey, passphrase []byte) ([]byte, error) {
	k, ok := key.(*HostedKey)
	if !ok {
		return nil, fmt.Errorf("sshkeys.marshalHostedKey unsupported type: %T", key)
	}
	b := &bytes.Buffer{}
	b.WriteString(k.publicKey.Type())
	b.WriteByte(' ')
	e := base64.NewEncoder(base64.StdEncoding, b)
	e.Write(k.publicKey.Marshal())
	e.Close()
	b.WriteByte(' ')
	b.WriteString(k.comment)
	b.WriteByte('\n')
	return marshalHostedKeyIndirect(b.Bytes())
}

type sshkeyAPI struct{}

func (*sshkeyAPI) Signer(ctx context.Context, key crypto.PrivateKey) (security.Signer, error) {
	// Note that this method will never get called since it is only
	// registered for ssh public keys.
	return nil, fmt.Errorf("sshkeys.Signer: should never be called, has been called for unsupported key type %T", key)
}

func (*sshkeyAPI) PublicKey(key interface{}) (security.PublicKey, error) {
	return publicKey(key)
}

func (*sshkeyAPI) CryptoPublicKey(key interface{}) (crypto.PublicKey, error) {
	return cryptoPublicKey(key)
}

type hostedKeyAPI struct{}

func (*hostedKeyAPI) Signer(ctx context.Context, key crypto.PrivateKey) (security.Signer, error) {
	if k, ok := key.(*HostedKey); ok {
		return k.Signer(ctx)
	}
	// Note that this method will never get called since it is only
	// registered for ssh public keys.
	return nil, fmt.Errorf("sshagent.Signer: unsupported key type %T", key)
}

func (*hostedKeyAPI) PublicKey(key interface{}) (security.PublicKey, error) {
	if k, ok := key.(*HostedKey); ok {
		return publicKey(k.publicKey)
	}
	return nil, fmt.Errorf("sshagent.PublicKey: unsupported key type %T", key)
}

func (*hostedKeyAPI) CryptoPublicKey(key interface{}) (crypto.PublicKey, error) {
	if k, ok := key.(*HostedKey); ok {
		return cryptoPublicKey(k.publicKey)
	}
	return nil, fmt.Errorf("sshagent.CryptoPublicKey: unsupported key type %T", key)
}

func cryptoPublicKey(key interface{}) (crypto.PublicKey, error) {
	if cp, ok := key.(ssh.CryptoPublicKey); ok {
		switch k := cp.CryptoPublicKey().(type) {
		case *ecdsa.PublicKey:
			return k, nil
		case *rsa.PublicKey:
			return k, nil
		case ed25519.PublicKey:
			return k, nil
		}
	}
	return nil, fmt.Errorf("sshkeys.cryptoPublicKey: unsupported key type %T", key)
}

func publicKey(key interface{}) (security.PublicKey, error) {
	if cp, ok := key.(ssh.CryptoPublicKey); ok {
		switch k := cp.CryptoPublicKey().(type) {
		case *ecdsa.PublicKey:
			return security.NewECDSAPublicKey(k), nil
		case *rsa.PublicKey:
			return security.NewRSAPublicKey(k), nil
		case ed25519.PublicKey:
			return security.NewED25519PublicKey(k), nil
		}
	}
	return nil, fmt.Errorf("sshkeys.publicKey: unsupported key type %T", key)
}

// ParseOpensshPrivateKey parses an openssh private key in pem format,
// ie. pem type "OPENSSH PRIVATE KEY".
func parseOpensshPrivateKey(block *pem.Block) (crypto.PrivateKey, error) {
	var out bytes.Buffer
	if err := pem.Encode(&out, block); err != nil {
		return nil, err
	}
	key, err := ssh.ParseRawPrivateKey(out.Bytes())
	if err != nil {
		if errors.Is(err, &ssh.PassphraseMissingError{}) {
			return nil, &keys.ErrPassphraseRequired{}
		}
		return nil, err
	}
	if k, ok := key.(*ed25519.PrivateKey); ok {
		return *k, nil
	}
	return key, nil
}

// DecryptOpensshPrivateKey decrypts an openssh privat key in pem format,
// ie. pem type "OPENSSH PRIVATE KEY". Note that there is no information
// in the pem block to indicate that the block is encrypted.
func decryptOpensshPrivateKey(block *pem.Block, passphrase []byte) (*pem.Block, crypto.PrivateKey, error) {
	var out bytes.Buffer
	if err := pem.Encode(&out, block); err != nil {
		return nil, nil, err
	}
	key, err := ssh.ParseRawPrivateKeyWithPassphrase(out.Bytes(), passphrase)
	if err != nil {
		if strings.Contains(err.Error(), "ssh: key is not password protected") {
			return block, nil, nil
		}
		if err == x509.IncorrectPasswordError {
			return nil, nil, &keys.ErrBadPassphrase{}
		}
		return nil, nil, err
	}
	if k, ok := key.(*ed25519.PrivateKey); ok {
		return nil, *k, nil
	}
	return nil, key, nil
}

func parseAuthorizedKey(data []byte) (crypto.PublicKey, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, err
	}
	return key, nil
}

type indirection struct {
	hostedKey *HostedKey
	comment   string
}

// Next implements keys.Indirection.
func (i *indirection) Next(ctx context.Context) (crypto.PrivateKey, []byte) {
	return i.hostedKey, nil
}

// String implements keys.Indirection.
func (i *indirection) String() string {
	return fmt.Sprintf("ssh-agent: %v: %v", ssh.FingerprintSHA256(i.hostedKey.publicKey), i.comment)
}

func parseSSHAgentPrivateKeyFile(pem *pem.Block) (crypto.PrivateKey, error) {
	filename := string(pem.Bytes)
	key, comment, _, _, err := ssh.ParseAuthorizedKey(pem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ssh public key from %q: %v: %v", pem.Type, filename, err)
	}
	return &indirection{NewHostedKey(key, comment), comment}, nil
}
