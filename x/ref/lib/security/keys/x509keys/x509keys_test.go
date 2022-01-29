// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509keys_test

import (
	"context"
	"crypto/x509"
	"reflect"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/x509keys"
	"v.io/x/ref/test/sectestdata"
)

var keyRegistrar = keys.NewRegistrar()

func init() {
	keys.MustRegisterCommon(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
	x509keys.MustRegister(keyRegistrar)
}

func TestX509Keys(t *testing.T) {
	ctx := context.Background()

	for _, kt := range sectestdata.SupportedKeyAlgos {
		publicKeyBytes, privateKeyBytes, err := x509keys.MarshalForImport(ctx,
			sectestdata.X509PublicKeyBytes(kt),
			x509keys.ImportPrivateKeyBytes(sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private), nil, nil),
		)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		publicKey, err := keyRegistrar.ParsePublicKey(publicKeyBytes)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		if _, ok := publicKey.(*x509.Certificate); !ok {
			t.Fatalf("%v: %v", kt, err)
		}

		api, err := keyRegistrar.APIForKey(publicKey)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		pk, err := api.PublicKey(publicKey)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		if got, want := reflect.TypeOf(pk).String(), sectestdata.CryptoSignerType(kt); got != want {
			t.Fatalf("%v: got %v, want %v", kt, got, want)
		}

		privateKey, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, nil)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		privateKeyType, _ := sectestdata.CryptoType(keys.CryptoAlgo(kt))
		if got, want := reflect.TypeOf(privateKey).String(), privateKeyType; got != want {
			t.Fatalf("%v: got %v, want %v", kt, got, want)
		}

	}
}
