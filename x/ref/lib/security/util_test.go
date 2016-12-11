// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

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

	var buf bytes.Buffer
	if err := SavePEMKey(&buf, key, nil); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}

	loadedKey, err := LoadPEMKey(&buf, nil)
	if !reflect.DeepEqual(loadedKey, key) {
		t.Fatalf("Got key %v, but want %v", loadedKey, key)
	}
}

func TestLoadSavePEMKeyWithPassphrase(t *testing.T) {
	pass := []byte("openSesame")
	incorrect_pass := []byte("wrongPassphrase")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed ecdsa.GenerateKey: %v", err)
	}
	var buf bytes.Buffer

	// Test incorrect passphrase.
	if err := SavePEMKey(&buf, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	loadedKey, err := LoadPEMKey(&buf, incorrect_pass)
	if loadedKey != nil && err != nil {
		t.Errorf("expected (nil, err != nil) received (%v,%v)", loadedKey, err)
	}

	// Test correct password.
	if err := SavePEMKey(&buf, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	loadedKey, err = LoadPEMKey(&buf, pass)
	if !reflect.DeepEqual(loadedKey, key) {
		t.Fatalf("Got key %v, but want %v", loadedKey, key)
	}

	// Test nil passphrase.
	if err := SavePEMKey(&buf, key, pass); err != nil {
		t.Fatalf("Failed to save ECDSA private key: %v", err)
	}
	if loadedKey, err = LoadPEMKey(&buf, nil); loadedKey != nil || verror.ErrorID(err) != ErrPassphraseRequired.ID {
		t.Fatalf("expected(nil, ErrPassphraseRequired), instead got (%v, %v)", loadedKey, err)
	}
}
