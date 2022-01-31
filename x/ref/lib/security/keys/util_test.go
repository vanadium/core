// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/youmark/pkcs8"
	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/lib/security/keys/x509keys"
)

var keyRegistrar = keys.NewRegistrar()

func init() {
	keys.MustRegister(keyRegistrar)
	sshkeys.MustRegister(keyRegistrar)
	x509keys.MustRegister(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
}

func sha256Signature(bytes []byte) string {
	hash := sha256.Sum256(bytes)
	return base64.RawStdEncoding.EncodeToString(hash[:])
}

func testParsing(ctx context.Context, t *testing.T, kt keys.CryptoAlgo, plain, encrypted, public []byte) (publicKey crypto.PublicKey, plainPrivateKey, decryptedPrivateKey crypto.PrivateKey) {
	if bytes.Equal(plain, encrypted) {
		t.Errorf("plaintext and encrypted are the same")
		return
	}
	var err error
	plainPrivateKey, err = keyRegistrar.ParsePrivateKey(ctx, plain, nil)
	if err != nil {
		t.Fatalf("%v: parse private key: %v", kt, err)
	}

	buf, err := keyRegistrar.MarshalPrivateKey(plainPrivateKey, nil)
	if err != nil {
		t.Fatalf("%v: failed to marshal key: %v", kt, err)
	}
	sum1 := sha256Signature(buf)

	decryptedPrivateKey, err = keyRegistrar.ParsePrivateKey(ctx, encrypted, []byte("password"))
	if err != nil {
		t.Fatalf("%v: parse private key %v", kt, err)
	}

	buf, err = keyRegistrar.MarshalPrivateKey(plainPrivateKey, nil)
	if err != nil {
		t.Fatalf("%v: failed to marshal key: %v", kt, err)
	}
	sum2 := sha256Signature(buf)

	if got, want := sum1, sum2; got != want {
		t.Fatalf("%v: got %v, want %v", kt, got, want)
	}

	publicKey, err = keyRegistrar.ParsePublicKey(public)
	if err != nil {
		t.Fatalf("%v: parse public key: %v", kt, err)
	}

	if err := verifySigning(kt, publicKey, decryptedPrivateKey); err != nil {
		t.Fatalf("%v: signing failed: %v", kt, err)
	}
	return
}

func verifySigning(kt keys.CryptoAlgo, public crypto.PublicKey, private crypto.PrivateKey) error {
	api, err := keyRegistrar.APIForKey(private)
	if err != nil {
		return fmt.Errorf("%v: api error: %v", kt, err)
	}
	signer, err := api.Signer(context.Background(), private)
	if err != nil {
		return fmt.Errorf("%v: signer error: %v", kt, err)
	}

	sig, err := signer.Sign([]byte("test"), []byte("a message"))
	if err != nil {
		return fmt.Errorf("%v: sign error: %v", kt, err)
	}

	api, err = keyRegistrar.APIForKey(public)
	if err != nil {
		return fmt.Errorf("%v: api error: %v", kt, err)
	}
	vpk, err := api.PublicKey(public)
	if err != nil {
		return fmt.Errorf("%v: public key error: %v", kt, err)
	}

	if !sig.Verify(vpk, []byte("a message")) {
		return fmt.Errorf("%v: verification failed", kt)
	}
	if sig.Verify(vpk, []byte("xxa message")) {
		return fmt.Errorf("%v: verification should have failed", kt)
	}
	return nil
}

func testPasswordProtection(ctx context.Context, t *testing.T, kt keys.CryptoAlgo, encrypted []byte) {
	_, err := keyRegistrar.ParsePrivateKey(ctx, encrypted, nil)
	if !errors.Is(err, &keys.ErrPassphraseRequired{}) {
		t.Fatalf("%v: wrong or missing error: %v", kt, err)
	}
	_, err = keyRegistrar.ParsePrivateKey(ctx, encrypted, []byte("wrong"))
	if !errors.Is(err, &keys.ErrBadPassphrase{}) {
		t.Fatalf("%v: wrong or missing error: %v", kt, err)
	}

	// Try all supported parsing to ensure that it fails both
	// for the complete buffer and the pem decoded version.

	block, _ := pem.Decode(encrypted)
	if block == nil {
		t.Fatalf("expected a block")
	}

	for _, shouldBeEncrypted := range [][]byte{block.Bytes, encrypted} {
		_, _, err = pkcs8.ParsePrivateKey(shouldBeEncrypted, nil)
		if err == nil {
			t.Fatalf("%v: encryption failed for pkcs8.ParsePrivateKey", kt)
		}
		t.Logf("%v: error for pkcs8.ParsePrivateKey: %v", kt, err)

		_, err = x509.ParsePKCS8PrivateKey(shouldBeEncrypted)
		if err == nil {
			t.Fatalf("%v: encryption failed for x509.ParsePKCS8PrivateKey", kt)
		}
		t.Logf("%v: error forx509.ParsePKCS8PrivateKey: %v", kt, err)

		_, err = x509.ParseECPrivateKey(shouldBeEncrypted)
		if err == nil {
			t.Fatalf("%v: encryption failed for x509.ParseECPrivateKey", kt)
		}
		t.Logf("%v: error forx509.ParseECPrivateKey: %v", kt, err)

		_, err = ssh.ParseRawPrivateKey(shouldBeEncrypted)
		if err == nil {
			t.Fatalf("%v: encryption failed for  ssh.ParseRawPrivateKey", kt)
		}
		t.Logf("%v: error for ssh.ParseRawPrivateKey: %v", kt, err)
	}

}

func testPasswordEncryption(ctx context.Context, t *testing.T, kt keys.CryptoAlgo, plain, encrypted, public []byte) {
	assert := func(err error, format string, args ...interface{}) {
		if err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf(fmt.Sprintf("line: %v: ", line)+format, args...)
		}
	}

	key, err := keyRegistrar.ParsePrivateKey(ctx, plain, nil)
	assert(err, "%v: parse private key error: %v", kt, err)

	newlyEncrypted, err := keyRegistrar.MarshalPrivateKey(key, []byte("a-new-passhrase"))
	assert(err, "%v: marshal error: %v", kt, err)
	testPasswordProtection(ctx, t, kt, newlyEncrypted)

	decryptedKey, err := keyRegistrar.ParsePrivateKey(ctx, newlyEncrypted, []byte("a-new-passhrase"))
	assert(err, "%v: parse error: %v", kt, err)

	publicKey, err := keyRegistrar.ParsePublicKey(public)
	assert(err, "%v: parse private key error: %v", kt, err)

	if err := verifySigning(kt, publicKey, decryptedKey); err != nil {
		t.Fatalf("%v: signing failed: %v", kt, err)
	}
}
