// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

var (
	ecdsaKey   *ecdsa.PrivateKey
	rsaKey     *rsa.PrivateKey
	ed25519Key ed25519.PrivateKey
)

func init() {
	var err error
	ecdsaKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed ecdsa.GenerateKey: %v", err))
	}
	_, ed25519Key, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed ed25519.GenerateKey: %v", err))
	}
	rsaKey, err = rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		panic(fmt.Sprintf("Failed rsa.GenerateKey: %v", err))
	}
}

func TestLoadSavePEMKey(t *testing.T) {
	for _, tc := range []struct {
		private crypto.PrivateKey
		public  crypto.PublicKey
	}{
		{ecdsaKey, ecdsaKey.Public()},
		{ed25519Key, ed25519Key.Public()},
		{rsaKey, rsaKey.Public()},
	} {
		key := tc.private
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
		if !reflect.DeepEqual(loadedKey, tc.public) {
			t.Fatalf("Got key %v, but want %v", loadedKey, key)
		}
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

func TestSSHParse(t *testing.T) {
	for _, pk := range []crypto.PublicKey{
		ecdsaKey.Public(),
		ed25519Key.Public(),
		rsaKey.Public(),
	} {
		sshpk, err := ssh.NewPublicKey(pk)
		if err != nil {
			t.Fatal(err)
		}
		ck, err := CryptoKeyFromSSHKey(sshpk)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := ck, pk; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
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
