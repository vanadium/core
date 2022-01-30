// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"context"
	"crypto"
	"reflect"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func validateX509Types(t *testing.T, kt keys.CryptoAlgo, publicKey, plainPrivateKey, decryptedPrivateKey crypto.PublicKey) {
	privateKeyType, _ := sectestdata.CryptoType(kt)
	if got, want := reflect.TypeOf(plainPrivateKey).String(), privateKeyType; got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
	if got, want := reflect.TypeOf(decryptedPrivateKey).String(), privateKeyType; got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
	if got, want := reflect.TypeOf(publicKey).String(), "*x509.Certificate"; got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
	api, err := keyRegistrar.APIForKey(publicKey)
	if err != nil {
		t.Errorf("APIForKey: %v", err)
	}
	pk, err := api.PublicKey(publicKey)
	if err != nil {
		t.Errorf("PublicKey: %v", err)
	}
	if got, want := reflect.TypeOf(pk).String(), sectestdata.CryptoSignerType(kt); got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
}

func TestParsingX509Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		publicKey, plainPrivateKey, decryptedPrivateKey := testParsing(
			ctx, t, kt,
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private),
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Encrypted),
			sectestdata.X509PublicKeyBytes(kt),
		)
		validateX509Types(t, kt, publicKey, plainPrivateKey, decryptedPrivateKey)
	}
}

func TestPasswordsX509Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testPasswordEncryption(
			ctx, t, kt,
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private),
			sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Encrypted),
			sectestdata.X509PublicKeyBytes(kt),
		)
	}
}
