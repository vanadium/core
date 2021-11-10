// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"
	"time"

	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/internal"
)

func customCertPool(t *testing.T, cafile string) *x509.CertPool {
	rf, err := os.Open(cafile)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	rootCert, err := internal.LoadCertificate(rf)
	if err != nil {
		t.Fatal(err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(rootCert)
	return certPool
}

func TestSSLKeys(t *testing.T) {
	certPool := customCertPool(t, filepath.Join("testdata", "root-ca.pem"))
	purpose, message := []byte("testing"), []byte("a message")
	for _, tc := range []struct {
		prefix string
		pk     string
	}{
		{"ec256.vanadium.io", "7e:1f:4d:6d:99:3b:6c:51:16:90:83:cf:07:a9:a3:fc"},
		{"ed25519.vanadium.io", "c4:33:17:69:02:42:8c:19:5d:69:77:02:71:c5:1d:7e"},
		{"rsa2048.vanadium.io", "53:fb:b2:07:10:fd:9c:89:16:f5:76:4b:e8:5c:17:30"},
		{"rsa4096.vanadium.io", "10:d6:7b:2f:7d:a2:6b:96:c1:27:50:05:ce:d6:d5:26"},
	} {

		privKeyFile := tc.prefix + ".key"
		cpriv, err := seclib.ParsePEMPrivateKeyFile(filepath.Join("testdata", privKeyFile),
			nil)
		if err != nil {
			t.Errorf("failed to load %v: %v", privKeyFile, err)
		}
		signer, err := seclib.NewInMemorySigner(cpriv)
		if err != nil {
			t.Errorf("failed to create signer for %v: %v", privKeyFile, err)
		}
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("failed to sign using %v: %v", privKeyFile, err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("failed to verify signature using %v: %v", privKeyFile, err)
		}

		opts := x509.VerifyOptions{
			Roots: certPool,
		}
		crtFile := tc.prefix + ".crt"
		cert, err := seclib.ParseOpenSSLCertificateFile(filepath.Join("testdata", crtFile), opts)
		if err != nil {
			t.Errorf("failed to load %v: %v", crtFile, err)
			continue
		}

		if got, want := cert.PublicKey.String(), tc.pk; got != want {
			t.Errorf("%v: got %v, want %v", crtFile, got, want)
		}
	}
}

func TestLetsEncryptKeys(t *testing.T) {
	filename := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	rf, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()

	purpose, message := []byte("testing"), []byte("another message")

	cpriv, err := seclib.ParsePEMPrivateKeyFile(filename, nil)
	if err != nil {
		t.Errorf("failed to load %v: %v", filename, err)
	}
	signer, err := seclib.NewInMemorySigner(cpriv)
	if err != nil {
		t.Errorf("failed to create signer for %v: %v", filename, err)
	}
	sig, err := signer.Sign(purpose, message)
	if err != nil {
		t.Errorf("failed to sign using %v: %v", filename, err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("failed to verify signature using %v: %v", filename, err)
	}

	pastTime, _ := time.Parse("2006-Jan-02", "2021-Nov-02")
	opts := x509.VerifyOptions{
		Roots:       customCertPool(t, filepath.Join("testdata", "letsencrypt-stg-int-e1.pem")),
		CurrentTime: pastTime,
	}
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
		filepath.Join("testdata", "letsencrypt-stg-int-e1.pem"), opts)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}
	// openssl x509 -in testdata/letsencrypt-stg-int-e1.pem --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := cert.PublicKey.String(), "8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}

}
