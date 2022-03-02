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
	"reflect"
	"regexp"
	"runtime"
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

func TestCreatePrincipalKeyOpts(t *testing.T) {
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

func TestCreatePrincipalPublicKeyOnly(t *testing.T) {
	ctx := context.Background()
	publicKeyBytes := sectestdata.V23PublicKeyBytes(keys.ECDSA384, sectestdata.V23KeySetA)
	p, err := CreatePrincipalOpts(ctx,
		WithPublicKeyBytes(publicKeyBytes),
		WithPublicKeyOnly(true))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Sign([]byte("error"))
	if err == nil || !strings.Contains(err.Error(), "signing not supported") {
		t.Fatalf("missing or incorrect error: %q", err)
	}

	dir, storeOpt := newStoreOpt(t)
	p, err = CreatePrincipalOpts(ctx,
		WithPublicKeyBytes(publicKeyBytes),
		WithPublicKeyOnly(true),
		storeOpt)
	if err != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}

	p, err = LoadPrincipalOpts(ctx,
		FromPublicKeyOnly(true),
		FromWritable(FilesystemStoreWriter(dir)))
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Sign([]byte("error"))
	if err == nil || !strings.Contains(err.Error(), "signing not supported") {
		t.Fatalf("missing or incorrect error: %q", err)
	}
}

func TestCreatePrincipalStoreOpts(t *testing.T) {
	ctx := context.Background()
	var err error
	assert := func() {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Fatalf("line %v: err %v", line, err)
		}
	}

	// Create a in-memory principal with custom in-memory blessing and root
	// stores, and verify that they are set correctly.
	privateKey := sectestdata.V23PrivateKey(keys.ED25519, sectestdata.V23KeySetA)
	signer := sectestdata.V23Signer(keys.ED25519, sectestdata.V23KeySetA)
	blessingStore, err := NewBlessingStoreOpts(ctx, signer.PublicKey())
	assert()
	blessingRoots, err := NewBlessingRootsOpts(ctx)
	assert()
	p, err := CreatePrincipalOpts(ctx,
		WithSigner(signer),
		WithBlessingRoots(blessingRoots),
		WithBlessingStore(blessingStore))
	assert()

	if got, want := p.BlessingStore(), blessingStore; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := p.Roots(), blessingRoots; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Create a persistent principal and blessing and root stores.
	dir, storeOpt := newStoreOpt(t)
	blessingStore, err = NewBlessingStoreOpts(ctx,
		signer.PublicKey(),
		BlessingStoreWriteable(FilesystemStoreWriter(dir), signer))
	assert()
	blessingRoots, err = NewBlessingRootsOpts(ctx,
		BlessingRootsWriteable(FilesystemStoreWriter(dir), signer))
	assert()

	p, err = CreatePrincipalOpts(ctx,
		WithPrivateKey(privateKey, nil),
		WithBlessingRoots(blessingRoots),
		WithBlessingStore(blessingStore),
		storeOpt)
	assert()
	blessing, err := p.BlessSelf("test")
	assert()
	if err := SetDefaultBlessings(p, blessing); err != nil {
		t.Fatal(err)
	}

	// Verify that the blessing and root changes made above were persisted.
	lp, err := LoadPrincipalOpts(ctx, FromWritable(FilesystemStoreWriter(dir)))
	assert()

	lblessing, _ := lp.BlessingStore().Default()
	if got, want := lblessing.String(), blessing.String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := p.Roots().Dump(), lp.Roots().Dump(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Make another change and verify that it too is persisted.
	_, err = lp.BlessingStore().Set(blessing, "some-pattern")
	assert()

	lpa, err := LoadPrincipalOpts(ctx, FromWritable(FilesystemStoreWriter(dir)))
	assert()

	if got, want := lp.BlessingStore().DebugString(), lpa.BlessingStore().DebugString(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := lpa.BlessingStore().DebugString(), "some-pattern"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
}

func setPeers(ctx context.Context, blessing security.Blessings, dir string, customStores bool, peers ...string) error {
	var blessingStoreFactory CreateBlessingStore
	var blessingRootsFactory CreateBlessingRoots
	if customStores {
		blessingStoreFactory = func(ctx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingStore, error) {
			return NewBlessingStoreOpts(ctx, publicKey,
				BlessingStoreWriteable(FilesystemStoreWriter(dir), signer),
			)
		}

		blessingRootsFactory = func(tx context.Context, publicKey security.PublicKey, signer security.Signer) (security.BlessingRoots, error) {
			return NewBlessingRootsOpts(ctx,
				BlessingRootsWriteable(FilesystemStoreWriter(dir), signer),
			)
		}
	}
	for i, peer := range peers {
		peer += "-peer"
		publicKey := sectestdata.V23Signer(keys.ECDSA256, sectestdata.V23KeySetID(i+1)).PublicKey()
		der, err := publicKey.MarshalBinary()
		if err != nil {
			return err
		}
		p, err := LoadPrincipalOpts(ctx,
			FromBlessingStore(blessingStoreFactory),
			FromBlessingRoots(blessingRootsFactory),
			FromWritable(FilesystemStoreWriter(dir)))
		if err != nil {
			return err
		}
		_, err = p.BlessingStore().Set(blessing, security.BlessingPattern(peer))
		if err != nil {
			return err
		}
		err = p.Roots().Add(der, security.BlessingPattern(peer))
		if err != nil {
			return err
		}
	}
	return nil
}

func hasPeers(p security.Principal, peers ...string) error {
	for _, peer := range peers {
		peer += "-peer"
		blessings := p.BlessingStore().DebugString()
		roots := p.Roots().DebugString()
		bre := regexp.MustCompile(peer + `[ ]+test\n`)
		if !bre.MatchString(blessings) {
			return fmt.Errorf("failed to find %v in\n%v", bre, blessings)
		}
		rre := regexp.MustCompile(`\[` + peer + `\]\n`)
		if !rre.MatchString(roots) {
			return fmt.Errorf("failed to find %v in\n%v", rre, roots)
		}
	}
	return nil
}

func TestPrincipalMultiPersistence(t *testing.T) {
	var err error
	assert := func() {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Fatalf("line %v: err %v", line, err)
		}
	}

	ctx := context.Background()
	privateKey := sectestdata.V23PrivateKey(keys.ED25519, sectestdata.V23KeySetA)

	dir, storeOpt := newStoreOpt(t)
	p, err := CreatePrincipalOpts(ctx,
		WithPrivateKey(privateKey, nil),
		storeOpt)
	assert()
	blessing, err := p.BlessSelf("test")
	assert()
	err = SetDefaultBlessings(p, blessing)
	assert()

	err = setPeers(ctx, blessing, dir, false, "a", "b", "c", "d")
	assert()
	p, err = LoadPrincipalOpts(ctx, FromReadonly(FilesystemStoreReader(dir)))
	assert()
	err = hasPeers(p, "a", "b", "c", "d")
	assert()

	dir, storeOpt = newStoreOpt(t)
	_, err = CreatePrincipalOpts(ctx,
		WithPrivateKey(privateKey, nil),
		storeOpt)
	assert()

	err = setPeers(ctx, blessing, dir, true, "e", "f", "g", "h")
	assert()
	p, err = LoadPrincipalOpts(ctx, FromReadonly(FilesystemStoreReader(dir)))
	assert()
	err = hasPeers(p, "e", "f", "g", "h")
	assert()
}
