// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keyfile_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing/keyfile"
)

func writeKeyFiles(t *testing.T, key interface{}, dir, privateKey, publicKey string) {
	err := internal.WritePEMKeyPair(
		key,
		path.Join(dir, privateKey),
		path.Join(dir, publicKey),
		nil,
	)
	if err != nil {
		t.Fatalf("failt to write %v, %v in dir %v: %v", privateKey, publicKey, dir, err)
	}
}

func createPEMKeys(t *testing.T, dir string) {
	_, key1, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to genered ed25519 key: %v", err)
	}
	writeKeyFiles(t, key1, dir, "privatekey.pem", "publickey.pem")
	key2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to genered ecdsa key: %v", err)
	}
	writeKeyFiles(t, key2, dir, "ecdsa.pem", "ecdsa-public.pem")
}

func TestKeyFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "test-key-files")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	createPEMKeys(t, tmpDir)
	for _, keyFilename := range []string{"privatekey.pem", "ecdsa.pem"} {
		svc := keyfile.NewSigningService()
		signer, err := svc.Signer(ctx, filepath.Join(tmpDir, keyFilename), nil)
		if err != nil {
			t.Fatalf("failed to get signer for %v: %v", keyFilename, err)
		}
		sig, err := signer.Sign([]byte("testing"), []byte("hello"))
		if err != nil {
			t.Fatalf("failed to sign message for %v: %v", keyFilename, err)
		}
		if !sig.Verify(signer.PublicKey(), []byte("hello")) {
			t.Errorf("failed to verify signature for %v", keyFilename)
		}
	}
}
