// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"
)

func mkECDSAPublicKey() PublicKey {
	eckey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return NewECDSAPublicKey(&eckey.PublicKey)
}

func mkED25519PublicKey() PublicKey {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return NewED25519PublicKey(pub)
}

func mkRSAPublicKey() PublicKey {
	key, err := rsa.GenerateKey(rand.Reader, 3000)
	if err != nil {
		panic(err)
	}
	return NewRSAPublicKey(&key.PublicKey)
}

func TestPublicKeyMarshaling(t *testing.T) {
	for _, k1 := range []PublicKey{
		mkECDSAPublicKey(),
		mkED25519PublicKey(),
		mkRSAPublicKey(),
	} {
		bytes, err := k1.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		k2, err := UnmarshalPublicKey(bytes)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(k1, k2) {
			t.Errorf("UnmarshalBinary did not reproduce the key. Before [%v], After [%v]", k1, k2)
		}
	}
}

func TestPublicKeyStrings(t *testing.T) {
	var (
		k1 = mkECDSAPublicKey()
		k2 = mkECDSAPublicKey()
		k3 = mkRSAPublicKey()
	)
	// Exercise the String method.
	if k1.String() == k2.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k1.String() == k3.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k2.String() == k3.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	k1 = mkED25519PublicKey()
	k2 = mkED25519PublicKey()
	k3 = mkRSAPublicKey()

	// Exercise the String method.
	if k1.String() == k2.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k1.String() == k3.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k2.String() == k3.String() {
		t.Errorf("Two keys produced the same string representation")
	}
}

func BenchmarkUnmarshalECDSAPublicKey(b *testing.B) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	der, err := NewECDSAPublicKey(&key.PublicKey).MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := UnmarshalPublicKey(der); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalED25519PublicKey(b *testing.B) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		b.Fatal(err)
	}
	der, err := NewED25519PublicKey(pub).MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := UnmarshalPublicKey(der); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalRSA2048PublicKey(b *testing.B) {
	benchmarkUnmarshalRSAPublicKey(b, 2048)
}

func BenchmarkUnmarshalRSA4096PublicKey(b *testing.B) {
	benchmarkUnmarshalRSAPublicKey(b, 4096)
}

func benchmarkUnmarshalRSAPublicKey(b *testing.B, bits int) {
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		b.Fatal(err)
	}
	der, err := NewRSAPublicKey(&key.PublicKey).MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := UnmarshalPublicKey(der); err != nil {
			b.Fatal(err)
		}
	}
}
