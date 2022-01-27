// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"context"
	"reflect"
	"testing"

	"v.io/x/ref/test/sectestdata"
)

func TestParsingV23Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		privateKeyType, publicKeyType := sectestdata.CryptoType(kt)
		publicKey, plainPrivateKey, decryptedPrivateKey := testParsing(
			ctx, t, kt,
			sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23keySetA),
			sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23keySetAEncrypted),
			sectestdata.V23PublicKeyBytes(kt, sectestdata.V23keySetA))

		if got, want := reflect.TypeOf(plainPrivateKey).String(), privateKeyType; got != want {
			t.Errorf("%v: got %v, want %v", kt, got, want)
		}
		if got, want := reflect.TypeOf(decryptedPrivateKey).String(), privateKeyType; got != want {
			t.Errorf("%v: got %v, want %v", kt, got, want)
		}
		if got, want := reflect.TypeOf(publicKey).String(), publicKeyType; got != want {
			t.Errorf("%v: got %v, want %v", kt, got, want)
		}
	}
}

func TestPasswordsV23Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testPasswordEncryption(
			ctx, t, kt,
			sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23keySetA),
			sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23keySetAEncrypted),
			sectestdata.V23PublicKeyBytes(kt, sectestdata.V23keySetA),
		)
	}
}
