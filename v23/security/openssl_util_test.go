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

func NewOpenSSLECDSASigner(k *ecdsa.PrivateKey) (Signer, error) {
	return newOpenSSLECDSASigner(k, ecdsaHash(&k.PublicKey))
}
func NewOpenSSLED25519Signer(k ed25519.PrivateKey) (Signer, error) {
	return newOpenSSLED25519Signer(k, SHA512Hash)
}
func NewOpenSSLRSASigner(k *rsa.PrivateKey) (Signer, error) {
	return newOpenSSLRSASigner(k, SHA512Hash)
}

var (
	NewGoStdlibECDSASigner   = newGoStdlibECDSASigner
	NewGoStdlibED25519Signer = newGoStdlibED25519Signer
	NewGoStdlibRSASigner     = newGoStdlibRSASigner
)

func OpenSSLVersion() string {
	return openssl_version()
}
