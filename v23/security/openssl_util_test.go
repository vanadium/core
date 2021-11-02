// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security

var (
	NewOpenSSLECDSASigner   = newOpenSSLECDSASigner
	NewOpenSSLED25519Signer = newOpenSSLED25519Signer
	NewOpenSSLRSASigner     = newOpenSSLRSASigner

	NewGoStdlibECDSASigner   = newGoStdlibECDSASigner
	NewGoStdlibED25519Signer = newGoStdlibED25519Signer
	NewGoStdlibRSASigner     = newGoStdlibRSASigner
)

func OpenSSLVersion() string {
	return openssl_version()
}
