// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
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
	if loadedKey, err = LoadPEMPrivateKey(&buf, nil); loadedKey != nil || !errors.Is(err, ErrPassphraseRequired) {
		t.Fatalf("expected(nil, ErrPassphraseRequired), instead got (%v, %v)", loadedKey, err)
	}
}

func TestSSHParseED25519(t *testing.T) {
	pk, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshpk, err := ssh.NewPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	npk, err := ParseED25519Key(sshpk)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pk, npk; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	ck, err := CryptoKeyFromSSHKey(sshpk)
	if err != nil {
		t.Fatal(err)
	}
	ek, ok := ck.(ed25519.PublicKey)
	if !ok {
		t.Fatalf("wrong type %T", ck)
	}
	if got, want := ek, npk; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSSHParseECDSA(t *testing.T) {
	k, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pk := &k.PublicKey
	sshpk, err := ssh.NewPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}
	npk, err := ParseECDSAKey(sshpk)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pk, npk; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	ck, err := CryptoKeyFromSSHKey(sshpk)
	if err != nil {
		t.Fatal(err)
	}
	ek, ok := ck.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("wrong type %T", ck)
	}
	if got, want := ek, npk; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCopyKeyFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestWriting")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	from, to := filepath.Join(dir, "from"), filepath.Join(dir, "to")
	if err := ioutil.WriteFile(from, []byte{'0', '1', '\n'}, 0666); err != nil {
		t.Fatal(err)
	}

	if err := CopyKeyFile(from, to); err != nil {
		t.Fatal(err)
	}
	if err := CopyKeyFile(from, to); err == nil || !strings.Contains(err.Error(), "file exists") {
		t.Fatal("expected an error")
	}

	fi, err := os.Stat(to)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fi.Mode().Perm(), os.FileMode(0400); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestPEMFiles(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestWriting")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	public, private := filepath.Join(dir, "public.pem"), filepath.Join(dir, "private.pem")

	if err := WritePEMKeyPair(key, public, private, nil); err != nil {
		t.Fatal(err)
	}

	if err := WritePEMKeyPair(key, public, private, nil); err == nil || !strings.Contains(err.Error(), "file exists") {
		t.Fatal("expected an error")
	}

	for _, filename := range []string{public, private} {
		fi, err := os.Stat(filename)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := fi.Mode().Perm(), os.FileMode(0400); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
