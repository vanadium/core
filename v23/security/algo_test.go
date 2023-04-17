// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func TestRSAPanic(t *testing.T) {
	// Make sure that using a key with < 2048 bits causes a panic.
	key, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		e := recover()
		t.Log(e)
	}()
	security.NewRSAPublicKey(&key.PublicKey)
	t.Fatal("failed to panic")
}

func testSigningAndMarshaling(t *testing.T, signers ...security.Signer) {
	for i, signer := range signers {
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("%v: sign operation failed: %v", i, err)
			continue
		}
		pk := signer.PublicKey()
		if !sig.Verify(pk, message) {
			t.Errorf("%v: failed to verify signature", i)
			continue
		}

		buf, err := pk.MarshalBinary()
		if err != nil {
			t.Errorf("%v: marshall operation failed: %v", i, err)
			continue
		}

		npk, err := security.UnmarshalPublicKey(buf)
		if err != nil {
			t.Errorf("%v: marshall operation failed: %v", i, err)
			continue
		}

		if got, want := pk.String(), npk.String(); got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
			continue
		}

		if len(npk.String()) == 0 {
			t.Errorf("%v: zero len public key", i)
			continue
		}

		if !sig.Verify(npk, message) {
			t.Errorf("%v: failed to verify signature with unmarshalled key", i)
			t.FailNow()
			continue
		}
	}
}

func TestSigningAndMasrshaling(t *testing.T) {
	testSigningAndMarshaling(t,
		sectestdata.V23Signer(keys.ECDSA256, sectestdata.V23KeySetA),
		sectestdata.V23Signer(keys.ECDSA384, sectestdata.V23KeySetA),
		sectestdata.V23Signer(keys.ECDSA521, sectestdata.V23KeySetA),
		sectestdata.V23Signer(keys.ED25519, sectestdata.V23KeySetA),
		sectestdata.V23Signer(keys.RSA2048, sectestdata.V23KeySetA),
		sectestdata.V23Signer(keys.RSA4096, sectestdata.V23KeySetA),
	)
}
