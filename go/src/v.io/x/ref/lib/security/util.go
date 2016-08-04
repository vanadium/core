// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"

	"v.io/v23/security"
	"v.io/v23/verror"
)

var (
	// ErrBadPassphrase is a possible return error from LoadPEMKey()
	ErrBadPassphrase = verror.Register(pkgPath+".errBadPassphrase", verror.NoRetry, "{1:}{2:} passphrase incorrect for decrypting private key{:_}")
	// ErrPassphraseRequired is a possible return error from LoadPEMKey()
	ErrPassphraseRequired = verror.Register(pkgPath+".errPassphraseRequired", verror.NoRetry, "{1:}{2:} passphrase required for decrypting private key{:_}")

	errNoPEMKeyBlock       = verror.Register(pkgPath+".errNoPEMKeyBlock", verror.NoRetry, "{1:}{2:} no PEM key block read{:_}")
	errPEMKeyBlockBadType  = verror.Register(pkgPath+".errPEMKeyBlockBadType", verror.NoRetry, "{1:}{2:} PEM key block has an unrecognized type{:_}")
	errCantSaveKeyType     = verror.Register(pkgPath+".errCantSaveKeyType", verror.NoRetry, "{1:}{2:} key of type {3} cannot be saved{:_}")
	errCantEncryptPEMBlock = verror.Register(pkgPath+".errCantEncryptPEMBlock", verror.NoRetry, "{1:}{2:} failed to encrypt pem block{:_}")
)

const ecPrivateKeyPEMType = "EC PRIVATE KEY"

// NewPrincipalKey generates an ECDSA (public, private) key pair.
func NewPrincipalKey() (security.PublicKey, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return security.NewECDSAPublicKey(&priv.PublicKey), priv, nil
}

// LoadPEMKey loads a key from 'r'. returns ErrBadPassphrase for incorrect Passphrase.
// If the key held in 'r' is unencrypted, 'passphrase' will be ignored.
func LoadPEMKey(r io.Reader, passphrase []byte) (interface{}, error) {
	pemBlockBytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	if pemBlock == nil {
		return nil, verror.New(errNoPEMKeyBlock, nil)
	}
	var data []byte
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// Assume empty passphrase is disallowed.
		if len(passphrase) == 0 {
			return nil, verror.New(ErrPassphraseRequired, nil)
		}
		data, err = x509.DecryptPEMBlock(pemBlock, passphrase)
		if err != nil {
			return nil, verror.New(ErrBadPassphrase, nil)
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
			return nil, verror.New(ErrBadPassphrase, nil)
		}
		return key, nil
	}
	return nil, verror.New(errPEMKeyBlockBadType, nil, pemBlock.Type)
}

// SavePEMKey marshals 'key', encrypts it using 'passphrase', and saves the bytes to 'w' in PEM format.
// If passphrase is nil, the key will not be encrypted.
//
// For example, if key is an ECDSA private key, it will be marshaled
// in ASN.1, DER format, encrypted, and then written in a PEM block.
func SavePEMKey(w io.Writer, key interface{}, passphrase []byte) error {
	var data []byte
	var err error
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		if data, err = x509.MarshalECPrivateKey(k); err != nil {
			return err
		}
	default:
		return verror.New(errCantSaveKeyType, nil, fmt.Sprintf("%T", k))
	}

	var pemKey *pem.Block
	if passphrase != nil {
		pemKey, err = x509.EncryptPEMBlock(rand.Reader, ecPrivateKeyPEMType, data, passphrase, x509.PEMCipherAES256)
		if err != nil {
			return verror.New(errCantEncryptPEMBlock, nil, err)
		}
	} else {
		pemKey = &pem.Block{
			Type:  ecPrivateKeyPEMType,
			Bytes: data,
		}
	}

	return pem.Encode(w, pemKey)
}
