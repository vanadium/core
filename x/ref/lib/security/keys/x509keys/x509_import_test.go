// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x509keys_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/x509keys"
	"v.io/x/ref/test/sectestdata"
)

func TestX509Import(t *testing.T) {
	ctx := context.Background()
	tmpdir := t.TempDir()

	var (
		publicKeyBytes []byte
		ipub, ipriv    []byte
	)

	validatePublicKey := func(err error, kt keys.CryptoAlgo) {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Fatalf("line %v: %v: %v", line, kt, err)
		}
		key, err := keyRegistrar.ParsePublicKey(ipub)
		if err != nil {
			t.Fatalf("line %v: %v: %v", line, kt, err)
		}
		cert := key.(*x509.Certificate)
		_, publicKeyType := sectestdata.CryptoType(kt)
		if got, want := reflect.TypeOf(cert.PublicKey).String(), publicKeyType; got != want {
			t.Fatalf("line %v: %v: got %v, want %v", line, kt, got, want)
		}
	}

	validatePrivateKey := func(err error, kt keys.CryptoAlgo, key crypto.PrivateKey) {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Fatalf("line: %v: %v: %v", line, kt, err)
		}
		switch key.(type) {
		case *ecdsa.PrivateKey:
		case *rsa.PrivateKey:
		case ed25519.PrivateKey:
		default:
			t.Fatalf("line: %v: %v: parsed private key is a crypto key: %T", line, kt, key)
		}

	}

	for _, kt := range sectestdata.SupportedKeyAlgos {
		var err error
		publicKeyBytes = sectestdata.X509PublicKeyBytes(kt)
		privateKeyBytes := sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private)

		ipub, err = x509keys.ImportPublicKeyBytes(publicKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		validatePublicKey(err, kt)

		if got, want := bytes.Count(ipub, []byte("BEGIN CERTIFICATE")), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		key, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, nil)
		validatePrivateKey(err, kt, key)

		ipriv, err = seclib.ImportPrivateKeyFile("some-file-somewhere")

		// This will fail since the file is non-existent.
		_, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
			t.Fatalf("%v: missing or unexpected error: %v", kt, err)
		}

		// Try again with a valid file and it should succeed.
		filename := filepath.Join(tmpdir, kt.String()+".key")
		if err := os.WriteFile(filename, privateKeyBytes, 0600); err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		ipriv, err = seclib.ImportPrivateKeyFile(filename)
		validatePublicKey(err, kt)

		key, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)
	}
}

// This test is very similar to sshkeys/ssh_import_test.go - make sure to
// mirror changes here to that test.
func TestImportCopy(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		privateKeyType, _ := sectestdata.CryptoType(kt)
		for _, tc := range []struct {
			set           sectestdata.X509KeySetID
			newPassphrase []byte
		}{
			{sectestdata.X509Private, []byte("foobar")},
			{sectestdata.X509Private, nil},
		} {
			privateKeyBytes := sectestdata.X509PrivateKeyBytes(kt, tc.set)
			privateKey, err := seclib.ParsePrivateKey(ctx, privateKeyBytes, nil)

			ipriv, err := seclib.MarshalPrivateKey(privateKey, tc.newPassphrase)
			if err != nil {
				t.Fatalf("%v: %v: passphrase: %v", kt, tc.set, err)
			}

			if len(tc.newPassphrase) == 0 {
				if got, want := ipriv, "PRIVATE KEY"; !bytes.Contains(got, []byte(want)) {
					t.Fatalf("%v: %v: passphrase: got %s, want %s", kt, tc.set, got, want)
				}
			} else {
				if got, want := ipriv, "ENCRYPTED PRIVATE KEY"; !bytes.Contains(got, []byte(want)) {
					t.Fatalf("%v: %v: passphrase: got %s, want %s", kt, tc.set, got, want)
				}
			}

			key, err := keyRegistrar.ParsePrivateKey(ctx, ipriv, tc.newPassphrase)
			if err != nil {
				t.Fatalf("%v: %v: passphrase: %v", kt, tc.set, err)
			}

			if got, want := reflect.TypeOf(key).String(), privateKeyType; got != want {
				t.Fatalf("%v: %v: passphrase %s: got %s, want %s", kt, tc.set, tc.newPassphrase, got, want)
			}
		}
	}
}
