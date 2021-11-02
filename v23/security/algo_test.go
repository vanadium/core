// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"math/big"
	"runtime"
	"testing"

	"v.io/v23/security"
)

func TestRSAPanic(t *testing.T) {
	// Make sure that using a key with < 2048 bits causes a panic.
	key, err := rsa.GenerateKey(rand.Reader, 1024)
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

func testSigningAlgos(t *testing.T, signers ...security.Signer) {
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

func TestSigningAlgorithms(t *testing.T) {
	var err error
	assert := func() {
		if err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: %v", line, err)
		}
	}
	rsa2048Key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert()
	rsa4096Key, err := rsa.GenerateKey(rand.Reader, 4096)
	assert()
	ec256Key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert()
	_, edKey, err := ed25519.GenerateKey(rand.Reader)
	assert()

	rsa2048S, err := security.NewInMemoryRSASigner(rsa2048Key)
	assert()
	rsa4096S, err := security.NewInMemoryRSASigner(rsa4096Key)
	assert()
	ec256S, err := security.NewInMemoryECDSASigner(ec256Key)
	assert()
	ed25519S, err := security.NewInMemoryED25519Signer(edKey)
	assert()

	testSigningAlgos(t,
		ed25519S,
		rsa4096S,
		rsa2048S,
		ec256S,
	)

	rsa2048C := security.NewRSASigner(&rsa2048Key.PublicKey,
		func(data []byte) (sig []byte, err error) {
			return nil, fmt.Errorf("bad rsa")
		})
	assert()

	ec256C := security.NewECDSASigner(&ec256Key.PublicKey,
		func(data []byte) (r, s *big.Int, err error) {
			return nil, nil, fmt.Errorf("bad ec")
		})
	assert()
	ed25519C := security.NewED25519Signer(edKey.Public().(ed25519.PublicKey), func(data []byte) (sig []byte, err error) {
		return nil, fmt.Errorf("bad ed")
	})
	assert()

	for _, signer := range []security.Signer{
		rsa2048C, ec256C, ed25519C,
	} {
		_, err := signer.Sign(purpose, message)
		if err == nil {
			t.Errorf("expected an error")
		}
	}
}
