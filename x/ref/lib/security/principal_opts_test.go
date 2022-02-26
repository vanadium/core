// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

type principalOptValues struct {
	signer          security.Signer
	privateKey      crypto.PrivateKey
	publicKeyBytes  []byte
	privateKeyBytes []byte
}

func v23Data(kt keys.CryptoAlgo) principalOptValues {
	return principalOptValues{
		signer:          sectestdata.V23Signer(kt, sectestdata.V23KeySetA),
		privateKey:      sectestdata.V23PrivateKey(kt, sectestdata.V23KeySetA),
		publicKeyBytes:  sectestdata.V23PublicKeyBytes(kt, sectestdata.V23KeySetA),
		privateKeyBytes: sectestdata.V23PrivateKeyBytes(kt, sectestdata.V23KeySetA),
	}
}

func sslData(kt keys.CryptoAlgo) principalOptValues {
	return principalOptValues{
		signer:          sectestdata.X509Signer(kt),
		privateKey:      sectestdata.X509PrivateKey(kt),
		publicKeyBytes:  sectestdata.X509PublicKeyBytes(kt),
		privateKeyBytes: sectestdata.X509PrivateKeyBytes(kt, sectestdata.X509Private),
	}
}

func checkPEM(keyfile, blockType string) error {
	data, err := os.ReadFile(keyfile)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block in: %s", data)
	}
	if got, want := block.Type, blockType; got != want {
		return fmt.Errorf("got %v, want %v", got, want)
	}
	return nil
}

func newStoreOpt(t *testing.T) (string, CreatePrincipalOption) {
	dir := t.TempDir()
	store, err := CreateFilesystemStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, WithStore(store)
}

func TestCreatePrincipalOpts(t *testing.T) {
	ctx := context.Background()
	msg := []byte("hi there")
	for i, tc := range []struct {
		vals       principalOptValues
		publicPEM  []string
		privatePEM []string
	}{
		{v23Data(keys.ECDSA256), []string{
			"PUBLIC KEY",
			"PUBLIC KEY",
			"PUBLIC KEY",
		}, []string{
			"PRIVATE KEY",
			"EC PRIVATE KEY",
			"EC PRIVATE KEY",
		}},
		{sslData(keys.ED25519), []string{
			"PUBLIC KEY",
			"PUBLIC KEY",
			"CERTIFICATE",
		}, []string{
			"PRIVATE KEY",
			"PRIVATE KEY",
			"PRIVATE KEY",
		}},
	} {

		p, err := CreatePrincipalOpts(ctx, WithSigner(tc.vals.signer))
		if err != nil {
			t.Fatalf("%v: %v", i, err)
		}
		sig, err := p.Sign(msg)
		if err != nil {
			t.Fatalf("%v: %v", i, err)
		}
		if !sig.Verify(p.PublicKey(), msg) {
			t.Fatalf("%v: verify failed", i)
		}

		dir, storeOpt := newStoreOpt(t)
		_, err = CreatePrincipalOpts(ctx, WithSigner(tc.vals.signer), storeOpt)
		if err == nil || !strings.Contains(err.Error(), "cannot create a new persistent principal without a private key") {
			t.Fatalf("missing or incorrect error: %q", err)
		}
		_, err = CreatePrincipalOpts(ctx, WithPublicKeyBytes(tc.vals.publicKeyBytes), storeOpt)
		if err == nil || !strings.Contains(err.Error(), "cannot create a new persistent principal without a private key") {
			t.Fatalf("missing or incorrect error: %q", err)
		}

		_, err = CreatePrincipalOpts(ctx, WithPublicKeyBytes(tc.vals.publicKeyBytes), storeOpt, WithPublicKeyOnly(true))
		if err != nil {
			t.Fatalf("%v: %v", i, err)
		}

		_, err = LoadPrincipalOpts(ctx, FromReadonly(FilesystemStoreReader(dir)))
		if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
			t.Fatalf("missing or incorrect error: %q", err)
		}

		_, err = LoadPrincipalOpts(ctx, FromReadonly(FilesystemStoreReader(dir)), FromPublicKeyOnly(true))
		if err != nil {
			t.Fatalf("%v: %v", i, err)
		}

		for j, opt := range []CreatePrincipalOption{
			WithPrivateKey(tc.vals.privateKey, nil),
			WithPrivateKeyBytes(ctx, nil, tc.vals.privateKeyBytes, nil),
			WithPrivateKeyBytes(ctx, tc.vals.publicKeyBytes, tc.vals.privateKeyBytes, nil),
		} {
			p, err := CreatePrincipalOpts(ctx, opt)
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}
			sig, err := p.Sign(msg)
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}
			if !sig.Verify(p.PublicKey(), msg) {
				t.Fatalf("%v: %v: verify failed", i, j)
			}
			dir, storeOpt := newStoreOpt(t)
			pp, err := CreatePrincipalOpts(ctx, opt, storeOpt)
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}
			lp, err := LoadPrincipalOpts(ctx, FromReadonly(FilesystemStoreReader(dir)))
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}
			if got, want := lp.PublicKey().String(), pp.PublicKey().String(); got != want {
				t.Fatalf("%v: %v: got %v, want %v", i, j, got, want)
			}
			sig, err = lp.Sign(msg)
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}
			if !sig.Verify(p.PublicKey(), msg) {
				t.Fatalf("%v: %v: verify failed", i, j)
			}
			if err := checkPEM(filepath.Join(dir, publicKeyFile), tc.publicPEM[j]); err != nil {
				t.Errorf("%v: %v: %v", i, j, err)
			}
			if err := checkPEM(filepath.Join(dir, privateKeyFile), tc.privatePEM[j]); err != nil {
				t.Errorf("%v: %v: %v", i, j, err)
			}
		}

	}
}
