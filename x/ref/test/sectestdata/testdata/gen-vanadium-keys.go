// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func genkey(keyAlgo keys.CryptoAlgo) error {
	passphrase := []byte("password")
	for _, set := range []string{"-a-", "-b-", "-c-", "-d-", "-e-"} {
		pk, err := keys.NewPrivateKeyForAlgo(keyAlgo)
		if err != nil {
			return err
		}
		// Write private key.
		var (
			data      []byte
			publicKey crypto.PublicKey
			pemKey    pem.Block
		)
		pemKey.Type = "PRIVATE KEY"
		switch v := pk.(type) {
		case *ecdsa.PrivateKey:
			publicKey = v.Public()
			data, err = x509.MarshalECPrivateKey(v)
			pemKey.Type = "EC PRIVATE KEY"
		case *rsa.PrivateKey:
			publicKey = v.Public()
			data, err = x509.MarshalPKCS8PrivateKey(v)
		case ed25519.PrivateKey:
			publicKey = v.Public()
			data, err = x509.MarshalPKCS8PrivateKey(v)
		}
		if err != nil {
			return err
		}
		pemKey.Bytes = data
		encoded := &bytes.Buffer{}
		if err := pem.Encode(encoded, &pemKey); err != nil {
			return err
		}
		keyName := set + keyAlgo.String() + ".key"
		err = os.WriteFile("v23-private"+keyName, encoded.Bytes(), 0600)
		if err != nil {
			return err
		}

		// Write encrypted private key.
		encoded.Reset()
		encryptedPemKey, err := x509.EncryptPEMBlock(rand.Reader, pemKey.Type, data, passphrase, x509.PEMCipherAES256) //nolint:staticcheck
		if err != nil {
			return err
		}
		if err := pem.Encode(encoded, encryptedPemKey); err != nil {
			return err
		}

		if err := os.WriteFile("v23-encrypted"+keyName, encoded.Bytes(), 0600); err != nil {
			return err
		}

		// Write public key.
		data, err = x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			return err
		}
		pemKey = pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: data,
		}
		encoded.Reset()
		if err := pem.Encode(encoded, &pemKey); err != nil {
			return err
		}
		if err := os.WriteFile("v23-public"+keyName, encoded.Bytes(), 0600); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	for _, kd := range sectestdata.SupportedKeyAlgos {
		if err := genkey(kd); err != nil {
			panic(err)
		}
	}
}
