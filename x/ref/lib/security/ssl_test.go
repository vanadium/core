// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"path/filepath"
	"testing"

	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func TestSSLKeys(t *testing.T) {
	purpose, message := []byte("testing"), []byte("a message")
	keys, certs, opts := sectestdata.VanadiumSSLData()
	for host, key := range keys {
		cert := certs[host]
		signer, err := seclib.NewInMemorySigner(key)
		if err != nil {
			t.Errorf("failed to create signer for %v: %v", host, err)
		}
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("failed to sign using %v: %v", host, err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("failed to verify signature using %v: %v", host, err)
		}
		if _, err := cert.Verify(opts); err != nil {
			t.Errorf("failed to verify cert for %v: %v", host, err)
		}
	}
}

func TestLetsEncryptKeys(t *testing.T) {
	cpriv, _, opts := sectestdata.LetsEncryptData()
	purpose, message := []byte("testing"), []byte("another message")
	signer, err := seclib.NewInMemorySigner(cpriv)
	if err != nil {
		t.Errorf("failed to create signer: %v", err)
	}
	sig, err := signer.Sign(purpose, message)
	if err != nil {
		t.Errorf("failed to sign: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("failed to verify signature: %v", err)
	}

	letsencryptDir, err := sectestdata.LetsEncryptDir()
	if err != nil {
		t.Fatal(err)
	}
	//	defer os.RemoveAll(letsencryptDir)
	filename := filepath.Join(letsencryptDir, "www.labdrive.io.letsencrypt")

	cert, err := seclib.ParseOpenSSLCertificateFile(filename, opts)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}

	// openssl x509 -in testdata/lwww.labdrive.io.letsencrypt --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := cert.PublicKey.String(), "b4:1c:fc:66:5a:60:66:ea:e1:c5:46:76:59:8c:fc:6a"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}

	// Now parse the root certificate also.
	cert, err = seclib.ParseOpenSSLCertificateFile(
		filepath.Join(letsencryptDir, "letsencrypt-stg-int-e1.pem"), opts)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}
	// openssl x509 -in testdata/letsencrypt-stg-int-e1.pem --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := cert.PublicKey.String(), "8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}
}
