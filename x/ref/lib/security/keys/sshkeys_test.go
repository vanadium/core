// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"context"
	"crypto"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/test/sectestdata"
)

func validateSSHTypes(t *testing.T, kt keys.CryptoAlgo, publicKey, plainPrivateKey, decryptedPrivateKey crypto.PublicKey) {
	privateKeyType, _ := sectestdata.CryptoType(kt)
	if got, want := reflect.TypeOf(plainPrivateKey).String(), privateKeyType; got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
	if got, want := reflect.TypeOf(decryptedPrivateKey).String(), privateKeyType; got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}
	if got, want := reflect.TypeOf(publicKey).String(), sectestdata.SSHPublicKeyType(kt); got != want {
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

func TestParsingSSHKeys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		publicKey, plainPrivateKey, decryptedPrivateKey := testParsing(
			ctx, t, kt,
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate),
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyEncrypted),
			sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic),
		)
		validateSSHTypes(t, kt, publicKey, plainPrivateKey, decryptedPrivateKey)
	}
}

func TestPasswordsSSHKeys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testPasswordEncryption(
			ctx, t, kt,
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate),
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyEncrypted),
			sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic),
		)
	}
}

func setupIndirection(t *testing.T, plain, encrypted []byte) ([]byte, []byte) {
	tmpdir := t.TempDir()
	plaintextFile := filepath.Join(tmpdir, "plain")
	err := os.WriteFile(plaintextFile, plain, 0600)
	if err != nil {
		t.Fatalf("writefile: %v", err)
	}
	encryptedFile := filepath.Join(tmpdir, "encrypted")
	err = os.WriteFile(encryptedFile, encrypted, 0600)
	if err != nil {
		t.Fatalf("writefile: %v", err)
	}
	iplain, err := indirectkeyfiles.MarshalPrivateKey([]byte(plaintextFile))
	if err != nil {
		t.Fatalf("MarshalIndirectPrivateKeyFile: %v", err)
	}
	iencrypted, err := indirectkeyfiles.MarshalPrivateKey([]byte(encryptedFile))
	if err != nil {
		t.Fatalf("MarshalIndirectPrivateKeyFile: %v", err)
	}
	return iplain, iencrypted
}

func TestIndirectSSHKeys(t *testing.T) {
	ctx := context.Background()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		plain, encrypted := setupIndirection(t,
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate),
			sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyEncrypted),
		)
		publicKey, plainPrivateKey, decryptedPrivateKey := testParsing(
			ctx, t, kt, plain, encrypted,
			sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic),
		)
		validateSSHTypes(t, kt, publicKey, plainPrivateKey, decryptedPrivateKey)
	}
}
