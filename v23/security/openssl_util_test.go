// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
)

func NewOpenSSLECDSASigner(key *ecdsa.PrivateKey) (Signer, error) {
	return newOpenSSLECDSASigner(key, ecdsaHash(&key.PublicKey))
}
func NewOpenSSLED25519Signer(key ed25519.PrivateKey) (Signer, error) {
	return newOpenSSLED25519Signer(key, SHA512Hash)
}
func NewOpenSSLRSASigner(key *rsa.PrivateKey) (Signer, error) {
	return newOpenSSLRSASigner(key, SHA512Hash)
}

func NewGoStdlibECDSASigner(key *ecdsa.PrivateKey) (Signer, error) {
	return newGoStdlibECDSASigner(key, ecdsaHash(&key.PublicKey))
}
func NewGoStdlibED25519Signer(key ed25519.PrivateKey) (Signer, error) {
	return newGoStdlibED25519Signer(key, SHA512Hash)
}
func NewGoStdlibRSASigner(key *rsa.PrivateKey) (Signer, error) {
	return newGoStdlibRSASigner(key, SHA512Hash)
}

func OpenSSLVersion() string {
	return openssl_version()
}
