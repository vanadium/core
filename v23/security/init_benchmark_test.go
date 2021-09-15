// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"sync"

	"v.io/v23/internal/sectest"
	"v.io/v23/security"
)

var (
	ecdsaKey      *bmkey
	ed25519Key    *bmkey
	rsa2048Key    *bmkey
	benchmarkInit sync.Once
)

var (
	purpose, message []byte
)

func init() {
	purpose, message = sectest.GenPurposeAndMessage(5, 100)

}

type bmkey struct {
	signer    security.Signer
	signature security.Signature
}

func newECDSABenchmarkKey(sfn func(*ecdsa.PrivateKey) (security.Signer, error)) *bmkey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	signer, err := sfn(key)
	if err != nil {
		panic(err)
	}
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	return &bmkey{signer, signature}
}

func newED25519BenchmarkKey(sfn func(ed25519.PrivateKey) (security.Signer, error)) *bmkey {
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	signer, err := sfn(privKey)
	if err != nil {
		panic(err)
	}
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	return &bmkey{signer, signature}
}

func newRSABenchmarkKey(bits int, sfn func(*rsa.PrivateKey) (security.Signer, error)) *bmkey {
	privKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		panic(err)
	}
	signer, err := sfn(privKey)
	if err != nil {
		panic(err)
	}
	signature, err := signer.Sign(purpose, message)
	if err != nil {
		panic(err)
	}
	return &bmkey{signer, signature}
}

func initBenchmarks() {
	benchmarkInit.Do(func() {
		rsa2048Key = newRSABenchmarkKey(2048, security.NewInMemoryRSASigner)
		ecdsaKey = newECDSABenchmarkKey(security.NewInMemoryECDSASigner)
		ed25519Key = newED25519BenchmarkKey(security.NewInMemoryED25519Signer)
	})
}
