// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"reflect"
	"testing"

	"v.io/v23/verror"
)

func TestLoadSavePEMKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed ecdsa.GenerateKey: %v", err)
	}

	var privateKeyBuf, publicKeyBuf bytes.Buffer
	if err := SavePEMKeyPair(&privateKeyBuf, &publicKeyBuf, key, nil); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}

	loadedKey, err := LoadPEMPrivateKey(&privateKeyBuf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedKey, key) {
		t.Fatalf("Got key %v, but want %v", loadedKey, key)
	}

	loadedKey, err = LoadPEMPublicKey(&publicKeyBuf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedKey, &key.PublicKey) {
		t.Fatalf("Got key %v, but want %v", loadedKey, key)
	}
}

func TestLoadSavePEMKeyWithPassphrase(t *testing.T) {
	pass := []byte("openSesame")
	incorrectPass := []byte("wrongPassphrase")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed ecdsa.GenerateKey: %v", err)
	}
	var buf bytes.Buffer

	// Test incorrect passphrase.
	if err := SavePEMKeyPair(&buf, nil, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	loadedKey, err := LoadPEMPrivateKey(&buf, incorrectPass)
	if loadedKey != nil && err != nil {
		t.Errorf("expected (nil, err != nil) received (%v,%v)", loadedKey, err)
	}

	// Test correct password.
	if err := SavePEMKeyPair(&buf, nil, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	loadedKey, err = LoadPEMPrivateKey(&buf, pass)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loadedKey, key) {
		t.Fatalf("Got key %v, but want %v", loadedKey, key)
	}

	// Test nil passphrase.
	if err := SavePEMKeyPair(&buf, nil, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	if loadedKey, err = LoadPEMPrivateKey(&buf, nil); loadedKey != nil || verror.ErrorID(err) != ErrPassphraseRequired.ID {
		t.Fatalf("expected(nil, ErrPassphraseRequired), instead got (%v, %v)", loadedKey, err)
	}
}
