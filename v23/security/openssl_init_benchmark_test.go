// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openssl
// +build openssl

package security_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"sync"

	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

var (
	opensslECDSA256Key   *bmkey
	opensslED25519Key    *bmkey
	opensslRSA2048Key    *bmkey
	opensslRSA4096Key    *bmkey
	opensslBenchmarkInit sync.Once
)

func newOpenSSLBMKey(kt keys.CryptoAlgo) *bmkey {
	ctx := context.Background()
	pkBytes := sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23KeySetA)
	key, err := seclib.ParsePrivateKey(ctx, pkBytes, nil)
	if err != nil {
		panic(err)
	}
	var signer security.Signer
	// Explicitly use the OpenSSL variants.
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		signer, err = security.NewOpenSSLECDSASigner(k)
	case ed25519.PrivateKey:
		signer, err = security.NewOpenSSLED25519Signer(k)
	case *rsa.PrivateKey:
		signer, err = security.NewOpenSSLRSASigner(k)
	}
	if err != nil {
		panic(err)
	}
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	return &bmkey{signer, signature}
}

func init() {
	opensslRSA2048Key = newOpenSSLBMKey(keys.RSA2048)
	opensslRSA4096Key = newOpenSSLBMKey(keys.RSA4096)
	opensslECDSA256Key = newOpenSSLBMKey(keys.ECDSA256)
	opensslED25519Key = newOpenSSLBMKey(keys.ED25519)
}
