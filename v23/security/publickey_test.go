// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"reflect"
	"testing"
)

func mkPublicKey() PublicKey {
	eckey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return NewECDSAPublicKey(&eckey.PublicKey)
}

func TestPublicKeyMarshaling(t *testing.T) {
	k1 := mkPublicKey()
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

func TestPublicKeyString(t *testing.T) {
	var (
		k1 = mkPublicKey()
		k2 = mkPublicKey()
	)
	// Exercise the String method.
	if k1.String() == k2.String() {
		t.Errorf("Two keys produced the same string representation")
	}
}

func BenchmarkUnmarshalPublicKey(b *testing.B) {
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
