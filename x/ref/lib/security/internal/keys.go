// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	ecPrivateKeyPEMType    = "EC PRIVATE KEY"
	rsaPrivateKeyPEMType   = "RSA PRIVATE KEY"
	pkcs8PrivateKeyPEMType = "PRIVATE KEY"
	pkixPublicKeyPEMType   = "PUBLIC KEY"

	certPEMType = "CERTIFICATE"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPEMPrivateKey()
	ErrBadPassphrase = errors.New("passphrase incorrect for decrypting private key")

	// ErrPassphraseRequired is a possible return error from LoadPEMPrivateKey()
	ErrPassphraseRequired = errors.New("passphrase required for decrypting private key")
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

// WritePEMKeyPair writes a key pair in pem format.
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

// loadPEMPrivateKey loads a key from 'r'. returns ErrBadPassphrase for incorrect Passphrase.
// If the key held in 'r' is unencrypted, 'passphrase' will be ignored.
func LoadPEMPrivateKey(r io.Reader, passphrase []byte) (interface{}, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	for {
		var pemBlock *pem.Block
		pemBlock, pemBlockBytes = pem.Decode(pemBlockBytes)
		if pemBlock == nil {
			return nil, fmt.Errorf("no PEM key block read")
		}

		var data []byte
		// TODO(cnicolaou): migrate away from PEM keys.
		if x509.IsEncryptedPEMBlock(pemBlock) { //nolint:staticcheck
			// Assume empty passphrase is disallowed.
			if len(passphrase) == 0 {
				return nil, ErrPassphraseRequired
			}
			data, err = x509.DecryptPEMBlock(pemBlock, passphrase) //nolint:staticcheck
			if err != nil {
				return nil, ErrBadPassphrase
			}
		} else {
			data = pemBlock.Bytes
		}

		switch pemBlock.Type {
		case ecPrivateKeyPEMType:
			key, err := x509.ParseECPrivateKey(data)
			if err != nil {
				// x509.DecryptPEMBlock may occasionally return random
				// bytes for data with a nil error when the passphrase
				// is invalid; hence, failure to parse data could be due
				// to a bad passphrase.
				return nil, ErrBadPassphrase
			}
			return key, nil
		case rsaPrivateKeyPEMType:
			key, err := x509.ParsePKCS1PrivateKey(data)
			if err != nil {
				return nil, ErrBadPassphrase
			}
			return key, nil
		case pkcs8PrivateKeyPEMType:
			key, err := x509.ParsePKCS8PrivateKey(data)
			if err != nil {
				return nil, ErrBadPassphrase
			}
			return key, nil

		default:
			if strings.Contains(pemBlock.Type, "PARAMETERS") {
				continue
			}
		}
		return nil, fmt.Errorf("PEM key block has an unrecognized type: %v", pemBlock.Type)
	}
}

// LoadPEMPublicKeyFile loads a public key file in PEM PKIX format.
func LoadPEMPublicKeyFile(filename string) (interface{}, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	key, err := LoadPEMPublicKey(f)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key from: %v: %v", filename, err)
	}
	return key, nil
}

// LoadPEMPublicKey loads a public key in PEM PKIX format.
func LoadPEMPublicKey(r io.Reader) (interface{}, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read bytes: %v", err)
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	if pemBlock == nil {
		return nil, fmt.Errorf("no PEM key block read")
	}
	key, err := x509.ParsePKIXPublicKey(pemBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("x509.ParsePKIXPublicKey: failed to parse bytes: %v", err)
	}
	return key, nil
}

// LoadSSHPublicKeyFile loads a public key file in SSH authorized hosts format.
func LoadSSHPublicKeyFile(filename string) (ssh.PublicKey, string, error) {
	if !strings.HasSuffix(filename, ".pub") {
		return nil, "", fmt.Errorf("%v does not have suffix: .pub", filename)
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	key, comment, err := LoadSSHPublicKey(f)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load ssh public key from: %v: %v", filename, err)
	}
	return key, comment, nil
}

// LoadSSHPublicKey loads a public key in SSH authorized hosts format.
func LoadSSHPublicKey(r io.Reader) (ssh.PublicKey, string, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read bytes: %v", err)
	}
	key, comment, _, _, err := ssh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, "", err
	}
	return key, comment, nil
}

// SavePEMKey marshals 'key', encrypts it using 'passphrase', and saves the bytes to 'w' in PEM format.
// If passphrase is nil, the key will not be encrypted.
//
// For example, if key is an ECDSA private key, it will be marshaled
// in ASN.1, DER format, encrypted, and then written in a PEM block.
func SavePEMKeyPair(private, public io.Writer, key crypto.PrivateKey, passphrase []byte) error {
	var privateData []byte
	var err error
	var privatePEMType string
	var publicKey crypto.PublicKey
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		// TODO(cnicolaou): transition to using MarshalPKCS8PrivateKey.
		if privateData, err = x509.MarshalECPrivateKey(k); err != nil {
			return err
		}
		publicKey = k.Public()
		privatePEMType = ecPrivateKeyPEMType
	case ed25519.PrivateKey:
		privateData, err = x509.MarshalPKCS8PrivateKey(k)
		privatePEMType = pkcs8PrivateKeyPEMType
		publicKey = k.Public()
	case *rsa.PrivateKey:
		privateData, err = x509.MarshalPKCS8PrivateKey(k)
		privatePEMType = pkcs8PrivateKeyPEMType
		publicKey = k.Public()
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
	if err := pem.Encode(private, pemKey); err != nil {
		return err
	}

	if public == nil {
		return nil
	}
	return SavePublicKeyPEM(public, publicKey)
}

// SavePublicKeyPEM marshals the public using MarshalPKIXPublicKey and writes
// it to the supplied io.Writer.
func SavePublicKeyPEM(public io.Writer, key crypto.PublicKey) error {
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

// ParseECDSAKey creates an ecdsa.PublicKey from an ssh ECDSA key.
func ParseECDSAKey(key ssh.PublicKey) (*ecdsa.PublicKey, error) {
	var sshWire struct {
		Name string
		ID   string
		Key  []byte
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key type: %v: %v", key.Type(), err)
	}
	pk := new(ecdsa.PublicKey)
	switch sshWire.ID {
	case "nistp256":
		pk.Curve = elliptic.P256()
	case "nistp384":
		pk.Curve = elliptic.P384()
	case "nistp521":
		pk.Curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("uncrecognised ecdsa curve: %v", sshWire.ID)
	}
	pk.X, pk.Y = elliptic.Unmarshal(pk.Curve, sshWire.Key)
	if pk.X == nil || pk.Y == nil {
		return nil, fmt.Errorf("invalid curve point")
	}
	return pk, nil
}

// ParseED25519Key creates an ed25519.PublicKey from an ssh ED25519 key.
func ParseED25519Key(key ssh.PublicKey) (ed25519.PublicKey, error) {
	var sshWire struct {
		Name     string
		KeyBytes []byte
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %v: %v", key.Type(), err)
	}
	return ed25519.PublicKey(sshWire.KeyBytes), nil
}

// ParseRSAKey creates an rsa.PublicKey from an ssh rsa key.
func ParseRSAKey(key ssh.PublicKey) (*rsa.PublicKey, error) {
	var sshWire struct {
		Name string
		E    *big.Int
		N    *big.Int
	}
	if err := ssh.Unmarshal(key.Marshal(), &sshWire); err != nil {
		return nil, fmt.Errorf("failed to unmarshal key %v: %v", key.Type(), err)
	}
	return &rsa.PublicKey{N: sshWire.N, E: int(sshWire.E.Int64())}, nil
}

// CryptoKeyFromSSHKey returns one of *ecdsa.PublicKey or
// ed25519.PublicKey from the the supplied ssh PublicKey.
func CryptoKeyFromSSHKey(pk ssh.PublicKey) (interface{}, error) {
	switch pk.Type() {
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		return ParseECDSAKey(pk)
	case ssh.KeyAlgoED25519:
		return ParseED25519Key(pk)
	case ssh.KeyAlgoRSA, ssh.SigAlgoRSASHA2256, ssh.SigAlgoRSASHA2512:
		return ParseRSAKey(pk)
	default:
		return nil, fmt.Errorf("unsupported ssh key key tyoe %v", pk.Type())
	}
}

// LoadCertificate loads a certificate from 'r'.
func LoadCertificate(r io.Reader) (*x509.Certificate, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	for {
		var pemBlock *pem.Block
		pemBlock, pemBlockBytes = pem.Decode(pemBlockBytes)
		if pemBlock == nil {
			return nil, fmt.Errorf("no PEM certificate block read")
		}
		if pemBlock.Type == certPEMType {
			return x509.ParseCertificate(pemBlock.Bytes)
		}
	}
}
