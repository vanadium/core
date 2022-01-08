// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"v.io/x/ref/lib/security/internal"
)

func hashFromAlgo(algo x509.SignatureAlgorithm) (crypto.Hash, error) {
	switch algo {
	case x509.SHA1WithRSA:
		return crypto.SHA1, nil
	case x509.SHA256WithRSA:
		return crypto.SHA256, nil
	case x509.SHA384WithRSA:
		return crypto.SHA384, nil
	case x509.SHA512WithRSA:
		return crypto.SHA512, nil
	case x509.ECDSAWithSHA1:
		return crypto.SHA1, nil
	case x509.ECDSAWithSHA256:
		return crypto.SHA256, nil
	case x509.ECDSAWithSHA384:
		return crypto.SHA384, nil
	case x509.ECDSAWithSHA512:
		return crypto.SHA512, nil
	case x509.PureEd25519:
		return crypto.SHA512, nil
	}
	return crypto.SHA1, fmt.Errorf("unrecognised ssl algo: %v", algo)
}

func TestParseX509Certificates(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "cacert.pem"))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := internal.ReadPEMBlocks(f, regexp.MustCompile("CERTIFICATE"))
	if err != nil {
		t.Fatal(err)
	}
	// As of 10/26/21 there are 6 different signing algorithms used by
	// the mozilla include root CAs.
	for _, b := range blocks {
		if got, want := b.Type, "CERTIFICATE"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	certs, err := internal.ParseX509Certificates(blocks)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(certs), 6; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, c := range certs {
		if c.PublicKey == nil {
			t.Errorf("public key missing for cert %v", c.Issuer)
		}
		if !c.IsCA {
			t.Errorf("not a CA %v", c.Issuer)
		}
		hash, err := hashFromAlgo(c.SignatureAlgorithm)
		if err != nil {
			t.Error(err)
			continue
		}
		fn := hash.New()
		fn.Write(c.RawTBSCertificate)
		digest := fn.Sum(nil)
		switch key := c.PublicKey.(type) {
		case *ecdsa.PublicKey:
			if !ecdsa.VerifyASN1(key, digest, c.Signature) {
				t.Errorf("failed to verify signature for %v and %v", c.SignatureAlgorithm, c.Issuer)
			}
		case *rsa.PublicKey:
			err = rsa.VerifyPKCS1v15(key, hash, digest, c.Signature)
			if err != nil {
				t.Errorf("failed to verify signature for %v and %v: %v", c.SignatureAlgorithm, c.Issuer, err)
			}
		}
	}
}
