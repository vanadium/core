// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bcrypter

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security"

	"v.io/x/lib/ibe"
)

func newRoot(name string) *Root {
	master, err := ibe.SetupBB1()
	if err != nil {
		panic(err)
	}
	return NewRoot(name, master)
}

func newPlaintext() []byte {
	return []byte("AThirtyTwoBytePieceOfTextThisIs!")
}

func TestExtract(t *testing.T) {
	ctx, shutdown := context.RootContext()
	defer shutdown()

	googleYoutube := newRoot("google:youtube")

	// googleYoutube shoud not be able to extract a key for the blessing "google".
	if _, err := googleYoutube.Extract(ctx, "google"); err == nil {
		t.Fatal("extraction for google unexpectedly succeeded")
	}

	// googleYoutube should be able to extract keys for the following blessings.
	blessings := []string{"google:youtube", "google:youtube:alice", "google:youtube:bob", "google:youtube:alice:phone"}
	for _, b := range blessings {
		key, err := googleYoutube.Extract(ctx, b)
		if err != nil {
			t.Fatal(err)
		}
		if got := key.Blessing(); got != b {
			t.Fatalf("extracted key is for blessing %v, want key for blessing %v", got, b)
		}
		if got, want := key.Params(), googleYoutube.Params(); !reflect.DeepEqual(got, want) {
			t.Fatalf("extract key is for params %v, want key for params %v", got, want)
		}
	}
	// Every key should be unique.
	key1, _ := googleYoutube.Extract(ctx, "google:youtube:alice")
	key2, _ := googleYoutube.Extract(ctx, "google:youtube:alice")
	if reflect.DeepEqual(key1, key2) {
		t.Fatal("Two Extract operations yielded the same PrivateKey")
	}
}

func TestEncrypt(t *testing.T) {
	ctx, shutdown := context.RootContext()
	defer shutdown()

	var (
		googleYoutube = newRoot("google:youtube")
		google        = newRoot("google")

		encrypter = NewCrypter()
		ptxt      = newPlaintext()
	)

	// empty encrypter should not be able to encrypt for any pattern.
	if _, err := encrypter.Encrypt(ctx, "google:youtube:alice", ptxt); !errors.Is(err, ErrNoParams) {
		t.Fatalf("Got error %v, wanted error with ID %v", err, ErrNoParams.ID)
	}

	// add googleYoutube's params to the encrypter.
	if err := encrypter.AddParams(ctx, googleYoutube.Params()); err != nil {
		t.Fatal(err)
	}
	// encrypting for "google:youtube:alice" should now succeed.
	if _, err := encrypter.Encrypt(ctx, "google:youtube:alice", ptxt); err != nil {
		t.Fatal(err)
	}

	// encrypting for pattern "google" should still fail as the encrypter
	// does not have params that are authoritative on all blessings matching
	// the pattern "google" (the googleYoutube params are authoritative on
	// blessings matching "google:youtube").
	if _, err := encrypter.Encrypt(ctx, "google", ptxt); !errors.Is(err, ErrNoParams) {
		t.Fatalf("Got error %v, wanted error with ID %v", err, ErrNoParams.ID)
	}
	// add google's params to the encrypter.
	if err := encrypter.AddParams(ctx, google.Params()); err != nil {
		t.Fatal(err)
	}
	// encrypting for "google" should now succeed.
	if _, err := encrypter.Encrypt(ctx, "google", ptxt); err != nil {
		t.Fatal(err)
	}

	// Encryption should succeed for all of the following patterns
	patterns := []security.BlessingPattern{"google", "google:$", "google:alice", "google:bob", "google:bob:phone"}
	for _, p := range patterns {
		if _, err := encrypter.Encrypt(ctx, p, ptxt); err != nil {
			t.Fatal(err)
		}
	}

	// Every ciphertext should be unique.
	ctxt1, _ := encrypter.Encrypt(ctx, "google", ptxt)
	ctxt2, _ := encrypter.Encrypt(ctx, "google", ptxt)
	if reflect.DeepEqual(ctxt1, ctxt2) {
		t.Fatal("Two Encrypt operations yielded the same Ciphertext")
	}
}

func TestDecrypt(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := context.RootContext()
	defer shutdown()

	addParams := func(c *Crypter, params Params) {
		if err := c.AddParams(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
	extract := func(r *Root, b string) *PrivateKey {
		key, err := r.Extract(ctx, b)
		if err != nil {
			t.Fatal(err)
		}
		return key
	}
	var (
		// Create two roots for the name "google".
		google1 = newRoot("google")
		google2 = newRoot("google")

		encrypter = NewCrypter()
		ptxt      = newPlaintext()

		googleAlice1 = extract(google1, "google:alice:phone")
		googleAlice2 = extract(google2, "google:alice:tablet:app")
	)

	// Add roots google1 and google2 to the encrypter.
	addParams(encrypter, google1.Params())
	addParams(encrypter, google2.Params())
	// encrypt for the pattern "google:alice"
	ctxt, err := encrypter.Encrypt(ctx, "google:alice", ptxt)
	if err != nil {
		t.Fatal(err)
	}

	// A decrypter without any private key should not be able
	// to decrypt this ciphertext.
	decrypter := NewCrypter()
	if _, err := decrypter.Decrypt(ctx, ctxt); !errors.Is(err, ErrPrivateKeyNotFound) {
		t.Fatalf("Got error %v, wanted error with ID %v", err, ErrPrivateKeyNotFound.ID)
	}

	// Add key googleAlice1 to the decrypter.
	if err := decrypter.AddKey(ctx, googleAlice1); err != nil {
		t.Fatal(err)
	}
	// Decryption should now succeed.
	if got, err := decrypter.Decrypt(ctx, ctxt); err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(got, ptxt) {
		t.Fatalf("Got plaintext %v, want %v", got, ptxt)
	}

	// Decryption should have succeeded had the decrypter only contained
	// googleAlice2.
	decrypter = NewCrypter()
	if err := decrypter.AddKey(ctx, googleAlice2); err != nil {
		t.Fatal(err)
	}
	if got, err := decrypter.Decrypt(ctx, ctxt); err != nil {
		t.Fatal(err)
	} else if !bytes.Equal(got, ptxt) {
		t.Fatalf("Got plaintext %v, want %v", got, ptxt)
	}

	// Decryption should fail for ciphertexts encrypted for the following
	// patterns (At this point the decrypter only has a key for the blessing
	// "google:alice:tablet:app" from the root google2).
	patterns := []security.BlessingPattern{"google:alice:$", "google:bob", "google:alice:tablet:$", "google:bob:tablet"}
	for _, p := range patterns {
		ctxt, err := encrypter.Encrypt(ctx, p, ptxt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := decrypter.Decrypt(ctx, ctxt); !errors.Is(err, ErrPrivateKeyNotFound) {
			t.Fatalf("Got error %v, wanted error with ID %v", err, ErrPrivateKeyNotFound.ID)
		}
	}

	// Adding the private key googleAlice2 should have also added
	// google's public params. Thus encrypting for the following
	// patterns should succeed.
	patterns = []security.BlessingPattern{"google", "google:$", "google:alice", "google:bob", "google:bob:phone"}
	for _, p := range patterns {
		if _, err := decrypter.Encrypt(ctx, p, ptxt); err != nil {
			t.Fatal(err)
		}
	}
	// But encrypting for the following patterns should fail.
	patterns = []security.BlessingPattern{"youtube", "youtube:$", "youtube:alice"}
	for _, p := range patterns {
		if _, err := decrypter.Encrypt(ctx, p, ptxt); !errors.Is(err, ErrNoParams) {
			t.Fatalf("Got error %v, wanted error with ID %v", err, ErrNoParams.ID)
		}
	}
}

func TestWireCiphertext(t *testing.T) {
	ctx, shutdown := context.RootContext()
	defer shutdown()
	ptxt := newPlaintext()

	encrypt := func(params Params, pattern security.BlessingPattern) *Ciphertext {
		enc := NewCrypter()
		if err := enc.AddParams(ctx, params); err != nil {
			t.Fatal(err)
		}
		ctxt, err := enc.Encrypt(ctx, pattern, ptxt)
		if err != nil {
			t.Fatal(err)
		}
		return ctxt
	}
	decryptAndVerify := func(ctxt *Ciphertext, key *PrivateKey) error {
		dec := NewCrypter()
		if err := dec.AddKey(ctx, key); err != nil {
			return err
		}
		if got, err := dec.Decrypt(ctx, ctxt); err != nil {
			return err
		} else if !bytes.Equal(got, ptxt) {
			return fmt.Errorf("got plaintext %v, want %v", got, ptxt)
		}
		return nil
	}

	var (
		google = newRoot("google")
		params = google.Params()
		key, _ = google.Extract(ctx, "google")
		ctxt   = encrypt(params, "google:$")
	)
	// Verify that the ciphertext 'ctxt' can be decrypted using
	// private key 'key' into the desired plaintext.
	if err := decryptAndVerify(ctxt, key); err != nil {
		t.Fatal(err)
	}

	// Marshal and Unmarshal ciphertext 'ctxt' and verify that decryption
	// with private key 'key' still succeeds.
	var (
		newCtxt  Ciphertext
		wireCtxt WireCiphertext
	)
	ctxt.ToWire(&wireCtxt)
	newCtxt.FromWire(wireCtxt)
	if err := decryptAndVerify(&newCtxt, key); err != nil {
		t.Fatal(err)
	}

	// Marshal and Unmarshal the private key 'key' and verify that decryption
	// of 'ctxt' still succeeds.
	var (
		newKey  PrivateKey
		wireKey WirePrivateKey
	)
	if err := key.ToWire(&wireKey); err != nil {
		t.Fatal(err)
	} else if err = newKey.FromWire(wireKey); err != nil {
		t.Fatal(err)
	}
	if err := decryptAndVerify(ctxt, &newKey); err != nil {
		t.Fatal(err)
	}

	// Marshal and Unmarshal the root params and verify that encryption
	// still results in a ciphertext that can be decrypted with the private
	// key 'key'.
	var (
		newParams  Params
		wireParams WireParams
	)
	if err := params.ToWire(&wireParams); err != nil {
		t.Fatal(err)
	} else if err = newParams.FromWire(wireParams); err != nil {
		t.Fatal(err)
	}
	ctxt = encrypt(newParams, "google:$")
	if err := decryptAndVerify(ctxt, key); err != nil {
		t.Fatal(err)
	}
}
