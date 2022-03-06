// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509keys_test

import (
	"context"
	"crypto/x509"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/x509keys"
	"v.io/x/ref/test/sectestdata"
)

var keyRegistrar = keys.NewRegistrar()

func init() {
	keys.MustRegister(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
	x509keys.MustRegister(keyRegistrar)
}

func TestX509Keys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		privateKeyBytes := sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private)
		publicKeyBytes, err := x509keys.ImportPublicKeyBytes(sectestdata.X509PublicKeyBytes(kt))
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

		privateKeyType, _ := sectestdata.CryptoType(kt)
		if got, want := reflect.TypeOf(privateKey).String(), privateKeyType; got != want {
			t.Fatalf("%v: got %v, want %v", kt, got, want)
		}
	}
}

func TestVanadiumSSLKeys(t *testing.T) {
	ctx := context.Background()
	purpose, message := []byte("testing"), []byte("a message")
	keys, certs, opts := sectestdata.VanadiumSSLData()
	for host, key := range keys {
		cert := certs[host]

		api, err := keyRegistrar.APIForKey(key)
		if err != nil {
			t.Errorf("failed to API for key: %T: %v", key, err)
		}
		signer, err := api.Signer(ctx, key)
		if err != nil {
			t.Errorf("failed to create signer: %v", err)
		}
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("failed to sign using %v: %v", host, err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("failed to verify signature using %v: %v", host, err)
		}
		if _, err := cert.Verify(opts); err != nil {
			t.Errorf("failed to verify cert for %v: %v", host, err)
		}
	}
}

func TestLetsEncryptKeys(t *testing.T) {
	cpriv, _, opts := sectestdata.LetsEncryptData(sectestdata.SingleHostCert)
	ctx := context.Background()
	purpose, message := []byte("testing"), []byte("another message")

	api, err := keyRegistrar.APIForKey(cpriv)
	if err != nil {
		t.Errorf("failed to API for key: %T: %v", cpriv, err)
	}
	signer, err := api.Signer(ctx, cpriv)
	if err != nil {
		t.Errorf("failed to create signer: %v", err)
	}

	sig, err := signer.Sign(purpose, message)
	if err != nil {
		t.Errorf("failed to sign: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("failed to verify signature: %v", err)
	}

	letsencryptDir, err := sectestdata.LetsEncryptDir(sectestdata.SingleHostCert)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(letsencryptDir)

	for _, tc := range []struct {
		filename    string
		fingerprint string
	}{
		{filepath.Join(letsencryptDir, "www.labdrive.io.letsencrypt"),
			// openssl x509 -in testdata/lwww.labdrive.io.letsencrypt --pubkey --noout |
			// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
			"b4:1c:fc:66:5a:60:66:ea:e1:c5:46:76:59:8c:fc:6a"},
		{filepath.Join(letsencryptDir, "letsencrypt-stg-int-e1.pem"),
			// openssl x509 -in testdata/letsencrypt-stg-int-e1.pem --pubkey --noout |
			// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
			"8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf"},
	} {

		data, err := os.ReadFile(tc.filename)
		if err != nil {
			t.Fatalf("%v: %v", tc.filename, err)
		}
		key, err := keyRegistrar.ParsePublicKey(data)
		if err != nil {
			t.Fatalf("%v: %v", tc.filename, err)
		}
		cert := key.(*x509.Certificate)
		pk, err := security.NewPublicKey(cert)
		if err != nil {
			t.Fatalf("%v: %v", tc.filename, err)
		}
		if _, err := cert.Verify(opts); err != nil {
			t.Fatalf("%v: %v", tc.filename, err)
		}

		if got, want := pk.String(), tc.fingerprint; got != want {
			t.Errorf("%v: got %v, want %v", tc.filename, got, want)
		}
	}
}
