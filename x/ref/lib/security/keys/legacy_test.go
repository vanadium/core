// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"v.io/x/ref/test/sectestdata"
)

func TestLegacy(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		for _, passphrase := range [][]byte{nil, sectestdata.Password()} {
			set := sectestdata.V23LegacyKeys
			if len(passphrase) > 0 {
				set = sectestdata.V23LegacyEncryptedKeys
			}

			privateKeyType, publicKeyType := sectestdata.CryptoType(kt)
			publicKeyBytes := sectestdata.V23PublicKeyBytes(kt, set)
			publicKey, err := keyRegistrar.ParsePublicKey(publicKeyBytes)
			if err != nil {
				t.Fatalf("%v: %v", kt, err)
			}
			if got, want := reflect.TypeOf(publicKey).String(), publicKeyType; got != want {
				t.Fatalf("%v: got %v, want %v", kt, got, want)
			}
			privateKeyBytes := sectestdata.V23PrivateKeyBytes(kt, set)

			privateKey, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, passphrase)
			if err != nil {
				t.Fatalf("%v: %v", kt, err)
			}
			if got, want := reflect.TypeOf(privateKey).String(), privateKeyType; got != want {
				t.Fatalf("%v: got %v, want %v", kt, got, want)
			}

			if len(passphrase) > 0 {
				_, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, nil)
				if err == nil || !strings.Contains(err.Error(), "passphrase required") {
					t.Fatalf("%v: key was not encrypted: %v", kt, err)
				}
			}

		}
	}

}
