// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"reflect"
	"testing"
)

const (
	ec256KeyPEM = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIAYkd29NwBnAkhzARDwx+KhOiJLMDSAxf8E67Xug3SpIoAoGCCqGSM49
AwEHoUQDQgAEIHvCWOkEjvGQYwdTzASIDuYSByJ9qYVZ1Hw2ecbIqu2AS5Ms5EN/
NEcea36rZAOVnVyaU5GgPpYfgoLeCOnIRQ==
-----END EC PRIVATE KEY-----`

	ed25519KeyPEM = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIBxrp6nd9uI9qMPqn9EiZEMu/8AMSArRN3gJscvJnJuE
-----END PRIVATE KEY-----
`
	rsa2048PEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA2CjRtb1yRuGjeHTLe37/rjDZhHGaAAg8DcV85Hr5oJDgrez8
+RCEJy6eoYqv+Z1yD0Tg41fI3hoYzfes1xUV71ejHEcaeTzWrqJSFve9QM0Y0Mii
3qj4K1b6FA3jyNMCyh8GjXPI3QGINMFaQbwUSKzP+0LtaajwxvQ2R3krMZyiU0Cg
e/zkLHNz5CVxVVWdS3GXaOLAKs/3whVsX+lUwu1dIvPY+p44y1+GQydL379379GN
bXj2TQ2TbzH3uw7NxmTLlLWz+aimN4xs8Dk6ptSL9+baUa6IRGXXE68MWmlwpFz+
Fh0ANiMS31LfI65S2p4ehVjfDSIFy5hMp1YrmwIDAQABAoIBAQCEuDNiwiobSUlk
mVmivuxf2JCFmHa01FmDHyG666K/qpS5VYxRpIlvwVkW2J+JxNkWdPUbwXeMnzth
o1PVT5YDOazlnOatT+SEnxeGEKB73DIDZ11RFzAg9CtiCtE0KhNJZNlSGqhWwi0O
LzWqrL9LjAe7P3Gj8V282o9FPSl/MIssr+UPsUrFyC9JQESbUGM//KIZDm6SLQIf
ysrLEJHgzELOyDpMj/1K/Sthtga4WLMQFajmN1BElASHfBVmForVfP+Pq7yu8IDz
BK/VlzuRDfV0p4Y4PF3fBF6MdL0N76PI1mVnDJkl4hi7s2ZuB9zAomDNw4V23Num
brMC4lZBAoGBAPCSNc8Nofkh6JyANAJ0AFnKHabIb0quaXU1a3Es7ldICr3MdJHC
p5ZqDFfyCBvEt+Nb97pSKyB4tkDoxbVzIKx5yR1de91saKaQp8Y6Z9nP0b6TKkKU
/zB/bQJNtpmngYwMIIZVOyzWbZEDNjT5ZIhY4jnTpVn5TvBu7TWL3thhAoGBAOYF
zvw2MhV85jsa/JYrCwdHvWvQ3mTBo/8k1j6FySaVHWWfJP0FJFvbkUwI0/YGTlK0
938+ZTmn9UOt66owslPwwt0bDZt4VKGbD+uGrNWCstQyadsbDG5Gk62l8UMZvJAv
6UOiGlNCbFjLYq1zm7ja/4f8goFxOqwjpMN87FV7AoGBAIdrFkUOPH3722+1LxGu
cMAaaOSIcTVNxmlG/8r+as/Q6tL6MygVtbaSzY333R6cdpLSIznLpSErMhSJszk3
rE3KZC5WgDIdIy+XwAlyuSC/viaTurcuHhQTtq4URtRpmR/Xd7uGYMAVmCmH8EyT
kka6GeZJQAvMreXj2z1IFdIhAoGAK5lSQE/tclE4Ol/FHJmP/5NDfhoa7TIe6Y+L
5FHrbJq69bYShrrgx2B4y9aohmtvkRGoD8A9443IWUmv75RHWM27wbkM/TjzoaYI
gHYIcHfTeZEMq3EXDBfoifN4JWXNbe8G4cDqCHoAm1wBmirdi05HPPlJq0tQ+avP
4RXend0CgYAtPrwmvJ+JZ5TKJsj0y176KwFR7TmsG6VZGd6RHcCP/yjGZowktSOt
GHep8kwXNgB4u+uZsKlHmNOjWpwwruMt+LmKD7IlbvCKlnQfIw6hmTYhp3tzCR8r
LUv800pKz/E490ugsQkHPd49EzwylxQEFliG7t4FwlMt8erdP3k/7A==
-----END RSA PRIVATE KEY-----`

	rsa4096PEM = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`

	rsa2048Sig = `QTsG6Hm7QckWjGvhA/HiKOhb5FGnH530ZJOnWAAkHXhS97TDC+aSVEhGS9lLKPALkf4fMODEo9PYufJQjOeVhBf8rgASIQJUb0qxLuigC5Gyvg+GiNfQhAOYvrQJjJeM0wGIFvr8JmMlbkj4+QdrtdQqzAtUq3jVMeaOvslIGHgoAqxoCcxCuKDjey21fYjmy9G7AWLsZVb4MCfCSwDGdhP3agtIuLYB2snkLKIiFAh4FJ0PdT1GPRWDkiXxxc3JWTdPHwu/AXS0Ibjk3RXQaVcRjkZwjPIr6dAN0pGp8MYkgMYjXHvccGPWDmZNWamwz/vCR1D6IUbYWFnI/WHgrg`

	rsa4096Sig = `s6A+HuzZyxSTwaEgK5jC1LVXWRxItBSSkZuEU2GCOK/M4fLfU+6KoPWgbwj6ubnAIL5/H7DwePin5imC8oN/UDBw7ba3g/cpscBwB6Yy1gAwFQfNEBZ+VVon3yBUaHflisbp7Eefy7BiTwzmrs/6zpfYKdMhdNE5DNNVHEER5jASOuXjrqjBAILP3qz4+L/dC/RElpoS4oMLPqb3xM0OiM5vc94da4PsoQF+Q60v6nI3mfxCO636P60H3+z8gSVZ7QfbAEbvUPoWWdtuRfKdRZwOvaCnK3BfzdWzGyEN0S0fTkwHaJWfkSoPqXIQ7thsxW7mz1MdIXWsN0RjO3KWFzBgboJmU9PV8SgOliVz5NIfhyKgWnlpd4hj3XkwKI2kHZlxpHEwoGuqtN09Q466l24KxaBjXbqnD2D6sI0xys6eeX0DCiPbvhSOFavTygZFq7SwBacXh3dLjtYl3fvWw6CO3u2lJfOJ8GIZAggbhlKo/OKhOk4Ropnrm35CHhatunCYBN6ghnSRq2gjwoCrdil7IMtVVnTjHTm5G187F33YbsfYb7gzSnR5F0vlRVevcIGFHEj0KUcRr8FCER6O1JNbnqRZXHnJ9VRVN7Kff9TS2QNawXTuzNo84fwDwnltwed8zu34G8L/CNQWyNJb+M8g+Nt72yQaJfyN2ooYrOc`

	ed25519Sig = `YAjuVb7EY711MTXSwpCQUcfY2w3V0SH9Odj8yBLnIFQR4sy9ZM7dnup2T0cB4xiTJWatp9VtjJEtdWMMGnLPCw`
)

func loadPEM(pemKey string) (interface{}, error) {
	pemBlock, _ := pem.Decode([]byte(pemKey))
	if pemBlock == nil {
		return nil, fmt.Errorf("no PEM key block read")
	}

	switch pemBlock.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("RSA: %v", err)
		}
		return key, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("ECDSA: %v", err)
		}
		return key, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
		if err != nil {
			return nil, fmt.Errorf("ED: %v", err)
		}
		return key, nil
	}
	return nil, fmt.Errorf("PEM key block has an unrecognized type: %v", pemBlock.Type)
}

func loadSigner(t *testing.T, pemKey string) Signer {
	key, err := loadPEM(pemKey)
	if err != nil {
		t.Fatalf("loadSigner: %v", err)
	}
	var signer Signer
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		signer, err = NewInMemoryECDSASigner(k)
	case *rsa.PrivateKey:
		signer, err = NewInMemoryRSASigner(k)
	case ed25519.PrivateKey:
		signer, err = NewInMemoryED25519Signer(k)
	}
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	return signer
}

func loadPublicKey(t *testing.T, pemKey string) PublicKey {
	return loadSigner(t, pemKey).PublicKey()
}

func TestDigestStability(t *testing.T) {
	purpose := []byte("testing")
	message := []byte("a stable message to hash")
	for i, tc := range []struct {
		k     PublicKey
		value string // base64 encoded string
	}{
		{loadPublicKey(t, rsa2048PEM), "jDB5awjhiD1kTG2Yl5BX7kjuv1qWONq0QftVn/2yWMbLRsnuZSdRSsrcoHHU10+gbqR2BDtP7AlWoE8sK9XKIQ"},
		{loadPublicKey(t, rsa4096PEM), "lQnH/QP0At/XpTQHKOtkD0aiUBQTP4MzBgYxG1dxp/m/2BXia53IA6njkm3tWG1K3PDP2l7n6FwFlZCpoLTc9w"},
		{loadPublicKey(t, ed25519KeyPEM), "nhka/MfPMCwUQCupuiUbggFc+W5ERBfqbgoPlP75y4aYg25XEO9m+V7QENmIgOJLE3/H9Xy4GDZC34WI+YB9pA"},
		{loadPublicKey(t, ec256KeyPEM), "LAOHjgJ51OK3/kFNNaCC4jJgHkYKbjEoe4rccjq6B9Q"},
	} {
		hashed := messageDigest(tc.k.hashAlgo(), purpose, message, tc.k)
		if got, want := len(hashed), tc.k.hashAlgo().Size(); got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		encoded := base64.RawStdEncoding.EncodeToString(hashed)
		if got, want := encoded, tc.value; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		// Make sure the concatenation is the correct size for the sums
		// for each of the three fields.
		keybytes, err := tc.k.MarshalBinary()
		if err != nil {
			t.Errorf("%v: %v", i, err)
			continue
		}
		fields := messageDigestFields(tc.k.hashAlgo(), keybytes, purpose, message)
		if got, want := len(fields), tc.k.hashAlgo().Size()*3; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := cryptoSum(tc.k.hashAlgo(), fields), hashed; !bytes.Equal(got, want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
}

func rsaSignature(h Hash, sig string) Signature {
	dec, err := base64.RawStdEncoding.DecodeString(sig)
	if err != nil {
		panic(err)
	}
	return Signature{
		Purpose: []byte("testing"),
		Hash:    h,
		Rsa:     dec,
	}
}

func ed25519Signature(sig string) Signature {
	dec, err := base64.RawStdEncoding.DecodeString(sig)
	if err != nil {
		panic(err)
	}
	return Signature{
		Purpose: []byte("testing"),
		Hash:    SHA512Hash,
		Ed25519: dec,
	}
}

func TestSignerStability(t *testing.T) {
	purpose := []byte("testing")
	message := []byte("a stable message to hash")
	for i, tc := range []struct {
		k   Signer
		sig Signature
	}{
		// NOTE that the following signing algorithms are deterministic,
		//      but that ecdsa is not and hence cannot be tested here.
		//      These tests are intended to catch accidental changes to
		//      the contents signed or the signing algorithm.
		{loadSigner(t, rsa2048PEM), rsaSignature(SHA512Hash, rsa2048Sig)},
		{loadSigner(t, rsa4096PEM), rsaSignature(SHA512Hash, rsa4096Sig)},
		{loadSigner(t, ed25519KeyPEM), ed25519Signature(ed25519Sig)},
	} {
		sig, err := tc.k.Sign(purpose, message)
		if err != nil {
			t.Errorf("%v: sign failed: %v", i, err)
			continue
		}
		if got, want := sig, tc.sig; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", i, got, want)
			break
		}
		if !sig.Verify(tc.k.PublicKey(), message) {
			t.Errorf("%v: verify failed", i)
		}
	}
}
