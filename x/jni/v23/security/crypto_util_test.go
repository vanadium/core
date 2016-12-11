// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"reflect"
	"testing"
)

func TestCryptoUtilPublicKeyCoder(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("couldn't generate private key: %v", err)
	}
	pub := &priv.PublicKey
	data, err := marshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("couldn't marshal public key: %v", err)
	}
	pubNew, err := parsePKIXPublicKey(data)
	if err != nil {
		t.Fatalf("couldn't parse public key: %v", err)
	}
	if !reflect.DeepEqual(pub, pubNew) {
		t.Fatalf("public keys don't match")
	}
}

func TestCryptoUtilPrivateKeyCoder(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("couldn't generate private key: %v", err)
	}
	data, err := marshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("couldn't marshal private key: %v", err)
	}
	privNew, err := parsePKCS8PrivateKey(data)
	if err != nil {
		t.Fatalf("couldn't parse private key: %v", err)
	}
	if !reflect.DeepEqual(priv, privNew) {
		t.Fatalf("private keys don't match")
	}
}
