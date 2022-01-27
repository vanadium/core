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
	"strings"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/sectestdata"
)

func TestImport(t *testing.T) {
	ctx := context.Background()
	var (
		publicKeyBytes []byte
		ipub, ipriv    []byte
	)

	validatePublicKey := func(err error, kt keys.CryptoAlgo) {
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
		if got, want := ipub, publicKeyBytes; !bytes.Equal(got, want) {
			t.Fatalf("%v: got %v, want %v", kt, got, want)
		}
	}

	validatePrivateKey := func(err error, kt keys.CryptoAlgo, key crypto.PrivateKey) {
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
		switch key.(type) {
		case *ecdsa.PrivateKey:
		case *rsa.PrivateKey:
		case ed25519.PrivateKey:
		default:
			t.Fatalf("%v: parsed private key is a crypto key: %T", kt, key)
		}

	}

	tmpdir := t.TempDir()

	for _, kt := range sectestdata.SupportedKeyAlgos {
		var err error
		publicKeyBytes = sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)
		privateKeyBytes := sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate)

		ipub, ipriv, err = sshkeys.MarshalForImport(publicKeyBytes, sshkeys.ImportUsingAgent(true))
		validatePublicKey(err, kt)

		key, err := keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		if err != nil {
			t.Fatalf("%v: %v", kt, err)
		}

		if _, ok := key.(*sshkeys.HostedKey); !ok {
			t.Fatalf("%v: parsed private key is not an ssh agent hosted key: %T", kt, key)
		}

		ipub, ipriv, err = sshkeys.MarshalForImport(publicKeyBytes, sshkeys.ImportPrivateKeyBytes(privateKeyBytes))
		validatePublicKey(err, kt)

		key, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)

		ipub, ipriv, err = sshkeys.MarshalForImport(publicKeyBytes, sshkeys.ImportPrivateKeyFile("some-file-somewhere"))
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

		ipub, ipriv, err = sshkeys.MarshalForImport(publicKeyBytes, sshkeys.ImportPrivateKeyFile(filename))
		validatePublicKey(err, kt)

		key, err = keyRegistrar.ParsePrivateKey(ctx, ipriv, nil)
		validatePrivateKey(err, kt, key)
	}
}
