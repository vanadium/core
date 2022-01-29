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
		t.Log(cert)
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

		ipub, ipriv, err = x509keys.MarshalForImport(ctx, publicKeyBytes, x509keys.ImportPrivateKeyBytes(privateKeyBytes, nil, nil))
		validatePublicKey(err, kt)

		key, err := keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)

		ipub, ipriv, err = x509keys.MarshalForImport(ctx, publicKeyBytes, x509keys.ImportPrivateKeyFile("some-file-somewhere"))
		validatePublicKey(err, kt)

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

		ipub, ipriv, err = x509keys.MarshalForImport(ctx, publicKeyBytes, x509keys.ImportPrivateKeyFile(filename))
		validatePublicKey(err, kt)

		key, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)
	}
}

func copyPassphrase(pp []byte) []byte {
	n := make([]byte, len(pp))
	copy(n, pp)
	return n
}

func isZero(pp []byte) bool {
	for _, v := range pp {
		if v != 0 {
			return false
		}
	}
	return true
}

// This test is very similar to sshkeys/ssh_import_test.go - make sure to
// mirror changes here to that test.
func TestImportCopy(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		publicKeyBytes := sectestdata.X509PublicKeyBytes(kt)
		privateKeyType, _ := sectestdata.CryptoType(kt)
		for _, tc := range []struct {
			set            sectestdata.X509KeySetID
			origPassphrase []byte
			newPassphrase  []byte
		}{
			{sectestdata.X509Private, nil, []byte("foobar")},
			{sectestdata.X509Encrypted, sectestdata.Password(), nil},
			{sectestdata.X509Encrypted, sectestdata.Password(), []byte("foobar")},
		} {
			privateKeyBytes := sectestdata.X509PrivateKeyBytes(kt, tc.set)

			origPassphrase := copyPassphrase(tc.origPassphrase)
			newPassphrase := copyPassphrase(tc.newPassphrase)

			_, ipriv, err := x509keys.MarshalForImport(ctx, publicKeyBytes,
				x509keys.ImportPrivateKeyBytes(privateKeyBytes, origPassphrase, newPassphrase))
			if err != nil {
				t.Fatalf("%v: %v: passphrase %v - %v: %v", kt, tc.set, len(tc.origPassphrase) > 0, len(tc.newPassphrase) > 0, err)
			}

			if !isZero(origPassphrase) || !isZero(newPassphrase) {
				t.Fatalf("%v: %v: failed to zero passphrase", kt, tc.set)
			}

			if len(tc.newPassphrase) == 0 {
				if got, want := ipriv, privateKeyBytes; !bytes.Equal(got, want) {
					t.Fatalf("%v: %v: passphrase %v - %v: got %s, want %s", kt, tc.set, len(tc.origPassphrase) > 0, len(tc.newPassphrase) > 0, got, want)
				}
			} else {
				if got, want := ipriv, "ENCRYPTED PRIVATE KEY"; !bytes.Contains(got, []byte(want)) {
					t.Fatalf("%v: %v: passphrase %v - %v: got %s, want %s", kt, tc.set, len(tc.origPassphrase) > 0, len(tc.newPassphrase) > 0, got, want)
				}
			}

			passphrase := tc.origPassphrase
			if len(tc.newPassphrase) > 0 {
				passphrase = tc.newPassphrase
			}

			key, err := keyRegistrar.ParsePrivateKey(ctx, ipriv, passphrase)
			if err != nil {
				t.Fatalf("%v: %v: passphrase %v - %v: %v", kt, tc.set, len(tc.origPassphrase) > 0, len(tc.newPassphrase) > 0, err)
			}

			if got, want := reflect.TypeOf(key).String(), privateKeyType; got != want {
				t.Fatalf("%v: %v: passphrase %s: got %s, want %s", kt, tc.set, tc.newPassphrase, got, want)
			}
		}
	}

}
