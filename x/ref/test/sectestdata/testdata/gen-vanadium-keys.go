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

func main() {
	passphrase := []byte("password")
	for _, kd := range sectestdata.SupportedKeyAlgos {
		for _, set := range []string{"-a-", "-b-", "-c-", "-d-", "-e-"} {
			pk, err := keys.NewPrivateKeyForAlgo(kd)
			if err != nil {
				panic(err)
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
				panic(err)
			}
			pemKey.Bytes = data
			encoded := &bytes.Buffer{}
			if err := pem.Encode(encoded, &pemKey); err != nil {
				panic(err)
			}
			err = os.WriteFile("v23-private"+set+kd.String()+".key", encoded.Bytes(), 0600)
			if err != nil {
				panic(err)
			}

			// Write encrypted private key.
			encoded.Reset()
			encryptedPemKey, err := x509.EncryptPEMBlock(rand.Reader, pemKey.Type, data, passphrase, x509.PEMCipherAES256) //nolint:staticcheck
			if err != nil {
				panic(err)
			}
			if err := pem.Encode(encoded, encryptedPemKey); err != nil {
				panic(err)
			}

			if err := os.WriteFile("v23-encrypted"+set+kd.String()+".key", encoded.Bytes(), 0600); err != nil {
				panic(err)
			}

			// Write public key.
			data, err = x509.MarshalPKIXPublicKey(publicKey)
			if err != nil {
				panic(err)
			}
			pemKey = pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: data,
			}
			encoded.Reset()
			if err := pem.Encode(encoded, &pemKey); err != nil {
				panic(err)
			}
			if err := os.WriteFile("v23-public"+set+kd.String()+".key", encoded.Bytes(), 0600); err != nil {
				panic(err)
			}

		}
	}
}
