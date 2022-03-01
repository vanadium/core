// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"crypto"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/sshkeys"
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

func sshData(kt keys.CryptoAlgo) principalOptValues {
	return principalOptValues{
		signer:          sectestdata.SSHKeySigner(kt, sectestdata.SSHKeyPrivate),
		privateKey:      sectestdata.SSHPrivateKey(kt, sectestdata.SSHKeyPrivate),
		publicKeyBytes:  sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic),
		privateKeyBytes: sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate),
	}
}

func sshAgentData(kt keys.CryptoAlgo) principalOptValues {
	pk := sectestdata.SSHPublicKey(kt)
	return principalOptValues{
		signer:          sectestdata.SSHKeySigner(kt, sectestdata.SSHKeyPrivate),
		privateKey:      sshkeys.NewHostedKey(pk.(ssh.PublicKey), kt.String(), nil),
		publicKeyBytes:  sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic),
		privateKeyBytes: sectestdata.SSHPrivateKeyBytes(kt, sectestdata.SSHKeyPrivate),
	}
}

func isSSHFile(dir string) ([]byte, bool) {
	data, err := os.ReadFile(filepath.Join(dir, publicKeyFile))
	if err != nil {
		return nil, false
	}
	return data, bytes.Contains(data, []byte("ssh-"))
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

func TestCreatePrincipalOptsErrors(t *testing.T) {
	ctx := context.Background()
	msg := []byte("hi there")

	for i, tc := range []principalOptValues{
		v23Data(keys.ECDSA256),
		sslData(keys.ED25519),
	} {
		p, err := CreatePrincipalOpts(ctx, WithSigner(tc.signer))
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
		_, err = CreatePrincipalOpts(ctx, WithSigner(tc.signer), storeOpt)
		if err == nil || !strings.Contains(err.Error(), "cannot create a new persistent principal without a private key") {
			t.Fatalf("missing or incorrect error: %q", err)
		}
		_, err = CreatePrincipalOpts(ctx, WithPublicKeyBytes(tc.publicKeyBytes), storeOpt)
		if err == nil || !strings.Contains(err.Error(), "cannot create a new persistent principal without a private key") {
			t.Fatalf("missing or incorrect error: %q", err)
		}

		_, err = CreatePrincipalOpts(ctx, WithPublicKeyBytes(tc.publicKeyBytes), storeOpt, WithPublicKeyOnly(true))
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
	}
}

func TestCreatePrincipalBytesOpts(t *testing.T) {
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
		{sshData(keys.RSA2048), []string{
			"PUBLIC KEY",
			"PUBLIC KEY",
			"",
		}, []string{
			"PRIVATE KEY",
			"OPENSSH PRIVATE KEY",
			"OPENSSH PRIVATE KEY",
		}},
		{sshAgentData(keys.ED25519), []string{
			"PUBLIC KEY",
			"PUBLIC KEY",
			"",
		}, []string{
			"VANADIUM INDIRECT PRIVATE KEY",
			"OPENSSH PRIVATE KEY",
			"OPENSSH PRIVATE KEY",
		}},
	} {

		for j, opt := range []CreatePrincipalOption{
			WithPrivateKey(tc.vals.privateKey, nil),
			WithPrivateKeyBytes(ctx, nil, tc.vals.privateKeyBytes, nil),
			WithPrivateKeyBytes(ctx, tc.vals.publicKeyBytes, tc.vals.privateKeyBytes, nil),
		} {
			// Create and use in memory principal.
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

			// Create persistent principal.
			dir, storeOpt := newStoreOpt(t)
			pp, err := CreatePrincipalOpts(ctx, opt, storeOpt)
			if err != nil {
				t.Fatalf("%v: %v: %v", i, j, err)
			}

			// Load persistent principal.
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

			if err := checkPEM(filepath.Join(dir, privateKeyFile), tc.privatePEM[j]); err != nil {
				t.Errorf("%v: %v: %v", i, j, err)
			}

			// Verify formats of the key files created for the persistent principal.
			if len(tc.publicPEM[j]) == 0 {
				if data, ok := isSSHFile(dir); !ok {
					t.Fatalf("%v: %v: %s doesn't look like an ssh public key", i, j, data)
				}
			} else {
				if err := checkPEM(filepath.Join(dir, publicKeyFile), tc.publicPEM[j]); err != nil {
					t.Errorf("%v: %v: %v", i, j, err)
				}
			}

		}

	}
}

/*

func TestCreatePrincipalX509Opts(t *testing.T) {
	ctx := context.Background()

	keyType := keys.ECDSA521
	keys, certs, opts := sectestdata.VanadiumSSLData()

	dir, storeOpt := newStoreOpt(t)
	p, err := CreatePrincipalOpts(ctx,
		WithPrivateKey(keys[keyType.String()], nil),
		WithX509Certificate(certs[keyType.String()]),
		WithX509VerifyOptions(opts),
		storeOpt)
	if err != nil {
		t.Fatal(err)
	}
	validHost := "ecdsa-521.vanadium.io"
	invalidHost := "invalid.host.com"

	if _, err := p.BlessSelf(validHost); err != nil {
		t.Fatal(err)
	}
	invalidHostErr := fmt.Sprintf(", not %v", invalidHost)
	if err == nil || !strings.Contains(err.Error(), invalidHostErr) {
		t.Errorf("unexpected or missing error: %q does not contain %q", err, invalidHostErr)
	}

	rd := FilesystemStoreReader(dir)
	bs, err := NewBlessingRootsOpts(ctx, rd)
	// Make sure the the loaded principal has the correct x509 certificate
	// info.
	lp, err := LoadPrincipalOpts(ctx,
		FromReadonly(rd))

	if err != nil {
		t.Fatal(err)
	}

	if _, err := lp.BlessSelf(validHost); err != nil {
		t.Fatal(err)
	}
	_, err = lp.BlessSelf(invalidHost)
	if err == nil || !strings.Contains(err.Error(), invalidHostErr) {
		t.Errorf("unexpected or missing error: %q does not contain %q", err, invalidHostErr)
	}

}
*/
