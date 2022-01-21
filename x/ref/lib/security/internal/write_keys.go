// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/ssh"
)

func openKeyFile(keyFile string) (*os.File, error) {
	f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0400)
	if err != nil {
		return nil, fmt.Errorf("failed to open %v for writing: %v", keyFile, err)
	}
	return f, nil
}

// CopyKeyFile copies a keyfile, it will fail if it can't overwrite
// an existing file.
func CopyKeyFile(fromFile, toFile string) error {
	to, err := openKeyFile(toFile)
	if err != nil {
		return err
	}
	defer to.Close()
	from, err := os.Open(fromFile)
	if err != nil {
		return err
	}
	defer from.Close()
	_, err = io.Copy(to, from)
	return err
}

// WritePrivateKeyPEM a private key in PEM format.
func WritePrivateKeyPEM(key crypto.PrivateKey, keyfile string, passphrase []byte) error {
	wr, err := openKeyFile(keyfile)
	if err != nil {
		return err
	}
	defer wr.Close()
	if err := EncodePrivateKeyPEM(wr, key, passphrase); err != nil {
		return fmt.Errorf("failed to save private key to %v: %v", keyfile, err)
	}
	return nil
}

func EncodePrivateKeyPEM(private io.Writer, key crypto.PrivateKey, passphrase []byte) error {
	var privateData []byte
	var err error
	var privatePEMType string
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		if privateData, err = x509.MarshalECPrivateKey(k); err != nil {
			return err
		}
		privatePEMType = ecPrivateKeyPEMType
	case ed25519.PrivateKey:
		privateData, err = x509.MarshalPKCS8PrivateKey(k)
		privatePEMType = pkcs8PrivateKeyPEMType
	case *rsa.PrivateKey:
		privateData, err = x509.MarshalPKCS8PrivateKey(k)
		privatePEMType = pkcs8PrivateKeyPEMType
	default:
		return fmt.Errorf("key of type %T cannot be saved", k)
	}
	if err != nil {
		return err
	}
	var pemKey *pem.Block
	if passphrase != nil {
		// TODO(cnicolaou): migrate away from pem keys.
		pemKey, err = x509.EncryptPEMBlock(rand.Reader, privatePEMType, privateData, passphrase, x509.PEMCipherAES256) //nolint:staticcheck
		if err != nil {
			return fmt.Errorf("failed to encrypt pem block: %v", err)
		}
	} else {
		pemKey = &pem.Block{
			Type:  privatePEMType,
			Bytes: privateData,
		}
	}
	return pem.Encode(private, pemKey)
}

// WritePublicKeyPEM a public key in PEM format.
func WritePublicKeyPEM(key crypto.PublicKey, keyfile string) error {
	wr, err := openKeyFile(keyfile)
	if err != nil {
		return err
	}
	defer wr.Close()
	if err := EncodePublicKeyPEM(wr, key); err != nil {
		return fmt.Errorf("failed to save public key to %v: %v", keyfile, err)
	}
	return nil
}

// EncodePublicKeyPEM encodes the public key to PEM format using
// x509.MarshalPKIXPublicKey. It should not be used to marshal ssh public keys.
func EncodePublicKeyPEM(public io.Writer, key crypto.PublicKey) error {
	publicData, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}
	pemKey := &pem.Block{
		Type:  pkixPublicKeyPEMType,
		Bytes: publicData,
	}
	return pem.Encode(public, pemKey)
}

// EncodeSSHPublicKeyPEM encodes ssh public keys to PEM format as per:
// https://www.openssh.com/txt/draft-ietf-secsh-publickeyfile-02.txt
func EncodeSSHPublicKeyPEM(public io.Writer, key ssh.PublicKey) error {

	
	pemKey := &pem.Block{
		Type:  ssh2PEMType,

		Bytes: publicData,
	}
}

// WritePEMKeyPair writes a key pair in pem format.
// Deprecated: remove when it is no longer required by lib/security.
func WritePEMKeyPair(key interface{}, privateKeyFile, publicKeyFile string, passphrase []byte) error {
	private, err := openKeyFile(privateKeyFile)
	if err != nil {
		return err
	}
	defer private.Close()
	public, err := openKeyFile(publicKeyFile)
	if err != nil {
		return err
	}
	defer public.Close()
	if err := SavePEMKeyPair(private, public, key, passphrase); err != nil {
		return fmt.Errorf("failed to save private key to %v, %v: %v", privateKeyFile, publicKeyFile, err)
	}
	return nil
}
