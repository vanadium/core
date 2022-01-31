// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/sectestdata"
)

func TestImport(t *testing.T) {
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
		if got, want := ipub, publicKeyBytes; !bytes.Equal(got, want) {
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
		publicKeyBytes = sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)
		privateKeyBytes := sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate)

		ipub, ipriv, err = sshkeys.MarshalForImport(ctx, publicKeyBytes,
			sshkeys.ImportUsingAgent(true))
		validatePublicKey(err, kt)

		key, err := keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		if _, ok := key.(*sshkeys.HostedKey); !ok {
			t.Fatalf("%v: parsed private key is not an ssh agent hosted key: %T", kt, key)
		}

		ipub, ipriv, err = sshkeys.MarshalForImport(ctx, publicKeyBytes,
			sshkeys.ImportPrivateKeyBytes(privateKeyBytes, nil, nil))
		validatePublicKey(err, kt)

		key, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)

		ipub, ipriv, err = sshkeys.MarshalForImport(ctx, publicKeyBytes,
			sshkeys.ImportPrivateKeyFile("some-file-somewhere"))
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

		ipub, ipriv, err = sshkeys.MarshalForImport(ctx, publicKeyBytes,
			sshkeys.ImportPrivateKeyFile(filename))
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

// This test is very similar to x509keys/x509_import_test.go - make sure to
// mirror changes here to that test.
func TestImportCopy(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		publicKeyBytes := sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)
		privateKeyType, _ := sectestdata.CryptoType(kt)
		for _, tc := range []struct {
			set            sectestdata.SSHKeySetID
			origPassphrase []byte
			newPassphrase  []byte
		}{
			{sectestdata.SSHKeyPrivate, nil, []byte("foobar")},
			{sectestdata.SSHKeyEncrypted, sectestdata.Password(), nil},
			{sectestdata.SSHKeyEncrypted, sectestdata.Password(), []byte("foobar")},
		} {
			privateKeyBytes := sectestdata.SSHPrivateKeyBytes(kt, tc.set)

			origPassphrase := copyPassphrase(tc.origPassphrase)
			newPassphrase := copyPassphrase(tc.newPassphrase)

			_, ipriv, err := sshkeys.MarshalForImport(ctx, publicKeyBytes,
				sshkeys.ImportPrivateKeyBytes(privateKeyBytes, origPassphrase, newPassphrase))
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
