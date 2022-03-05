// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/x509"
	"reflect"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func testPublicKeyMarshaling(t *testing.T, k1 security.PublicKey) {
	bytes, err := k1.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	k2, err := security.UnmarshalPublicKey(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k1, k2) {
		t.Errorf("UnmarshalBinary did not reproduce the key. Before [%v], After [%v]", k1, k2)
	}
	if !security.CryptoPublicKeyEqual(k1, k2) {
		t.Errorf("UnmarshalBinary did not reproduce the key. Before [%v], After [%v]", k1, k2)
	}
}

func TestPublicKeyMarshaling(t *testing.T) {
	for _, kt := range testCryptoAlgos {
		key := sectestdata.V23PrivateKey(kt, sectestdata.V23KeySetA)
		k1, err := security.NewPublicKey(key)
		if err != nil {
			t.Fatal(err)
		}
		testPublicKeyMarshaling(t, k1)
	}
	_, sslCerts, _ := sectestdata.VanadiumSSLData()
	for _, cert := range sslCerts {
		k1, err := security.NewPublicKey(cert)
		if err != nil {
			t.Fatal(err)
		}
		testPublicKeyMarshaling(t, k1)
	}

}

func twoPublicKeys(t *testing.T, kt keys.CryptoAlgo) (a, b security.PublicKey) {
	ka := sectestdata.V23PrivateKey(kt, sectestdata.V23KeySetA)
	kb := sectestdata.V23PrivateKey(kt, sectestdata.V23KeySetB)

	pa, err := security.NewPublicKey(ka)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := security.NewPublicKey(kb)
	if err != nil {
		t.Fatal(err)
	}
	if security.CryptoPublicKeyEqual(pa, pb) {
		t.Errorf("identical keys [%v], After [%v]", pa, pb)
	}
	return pa, pb
}

func TestPublicKeyEquality(t *testing.T) {
	for i, ikt := range sectestdata.SupportedKeyAlgos {
		for j, jkt := range sectestdata.SupportedKeyAlgos {

			compareAndTest := func(a, b interface{}) {
				same := security.CryptoPublicKeyEqual(a, b)
				if i == j && !same {
					t.Errorf("keys should have been identical: [%v] [%v]", a, b)
				}
				if i != j && same {
					t.Errorf("keys should have been different: [%v] [%v]", a, b)
				}
			}

			ki := sectestdata.V23Signer(ikt, sectestdata.V23KeySetA).PublicKey()
			kj := sectestdata.V23Signer(jkt, sectestdata.V23KeySetA).PublicKey()
			compareAndTest(ki, kj)

			cki := security.ExposeCryptoPublicKey(ki)
			ckj := security.ExposeCryptoPublicKey(kj)
			compareAndTest(cki, ckj)

			compareAndTest(cki, &x509.Certificate{PublicKey: ckj})
			compareAndTest(x509.Certificate{PublicKey: cki}, ckj)

			// Should always fail.
			kj = sectestdata.V23Signer(jkt, sectestdata.V23KeySetB).PublicKey()
			if security.CryptoPublicKeyEqual(ki, kj) {
				t.Errorf("keys should have been different: [%v] [%v]", ki, kj)
			}

			ckj = security.ExposeCryptoPublicKey(kj)
			if security.CryptoPublicKeyEqual(cki, ckj) {
				t.Errorf("keys should have been different: [%v] [%v]", ki, kj)
			}
		}
	}
}

func TestPublicKeyStrings(t *testing.T) {

	k1, k2 := twoPublicKeys(t, keys.ECDSA256)
	k3, k4 := twoPublicKeys(t, keys.RSA2048)
	k5, k6 := twoPublicKeys(t, keys.ED25519)

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

	// Exercise the String method.
	if k3.String() == k4.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k3.String() == k5.String() {
		t.Errorf("Two keys produced the same string representation")
	}
	if k4.String() == k6.String() {
		t.Errorf("Two keys produced the same string representation")
	}
}

func BenchmarkUnmarshalECDSAPublicKey(b *testing.B) {
	benchmarkUnmarshal(b, keys.ECDSA256)
}

func BenchmarkUnmarshalED25519PublicKey(b *testing.B) {
	benchmarkUnmarshal(b, keys.ED25519)
}

func BenchmarkUnmarshalRSA2048PublicKey(b *testing.B) {
	benchmarkUnmarshal(b, keys.RSA2048)
}

func BenchmarkUnmarshalRSA4096PublicKey(b *testing.B) {
	benchmarkUnmarshal(b, keys.RSA4096)
}

func benchmarkUnmarshal(b *testing.B, kt keys.CryptoAlgo) {
	key := sectestdata.V23Signer(keys.RSA4096, sectestdata.V23KeySetA).PublicKey()
	der, err := key.MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := security.UnmarshalPublicKey(der); err != nil {
			b.Fatal(err)
		}
	}
}
