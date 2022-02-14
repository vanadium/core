// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/sectestdata"
)

var (
	sshKeyDir        string
	sshTestKeys      []string
	sshAgentSockName string
)

var keyRegistrar = keys.NewRegistrar()

func init() {
	sshkeys.MustRegister(keyRegistrar)
	keys.MustRegister(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
}

func TestMain(m *testing.M) {
	var err error
	var cleanup func()
	sshKeyDir, sshAgentSockName, cleanup, err = sectestdata.StartPreConfiguredSSHAgent()
	if err != nil {
		flag.Parse()
		cleanup()
		fmt.Fprintf(os.Stderr, "failed to start/configure agent: %v\n", err)
		os.Exit(1)
	}
	sshTestKeys = sectestdata.SSHPrivateKeys()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestContext(t *testing.T) {
	ctx := context.Background()
	orig := sshkeys.DefaultSockNameFunc
	defer func() {
		sshkeys.DefaultSockNameFunc = orig
	}()

	if got, want := sshkeys.AgentSocketName(ctx), os.Getenv("SSH_AUTH_SOCK"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	sshkeys.DefaultSockNameFunc = func() string { return "my-value" }

	if got, want := sshkeys.AgentSocketName(ctx), "my-value"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func testAgent(ctx context.Context, t *testing.T, kt keys.CryptoAlgo, passphrase []byte) {
	publicKeyBytes, privateKeyBytes, err := sshkeys.ImportAgentHostedKeyBytes(
		sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic))
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}
	key, err := keyRegistrar.ParsePublicKey(publicKeyBytes)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	hk, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, passphrase)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	if got, want := hk.(*sshkeys.HostedKey).Comment(), kt.String(); got != want {
		t.Errorf("%v: got %v, want %v", kt, got, want)
	}

	api, err := keyRegistrar.APIForKey(hk)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}
	signer, err := api.Signer(ctx, hk)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	msg := []byte("my-message-" + kt.String())

	sig, err := signer.Sign([]byte("testing"), msg)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	api, err = keyRegistrar.APIForKey(key)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}
	vpk, err := api.PublicKey(key)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	if !sig.Verify(vpk, msg) {
		t.Fatalf("%v: failed to verify signature", kt)
	}
	if sig.Verify(vpk, append(msg, 'a')) {
		t.Fatalf("%v: signature incorrectly verified", kt)
	}
}

func TestAgent(t *testing.T) {
	ctx := context.Background()
	ctx = sshkeys.WithAgentSocketName(ctx, sshAgentSockName)
	for _, kt := range sectestdata.SupportedKeyAlgos {
		testAgent(ctx, t, kt, nil)
	}
}

func getSigner(ctx context.Context, t *testing.T, kt keys.CryptoAlgo) error {
	publicKeyBytes := sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)
	_, privateKeyBytes, err := sshkeys.ImportAgentHostedKeyBytes(publicKeyBytes)
	hk, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, nil)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	api, err := keyRegistrar.APIForKey(hk)
	if err != nil {
		return fmt.Errorf("%v: %v", kt, err)
	}
	_, err = api.Signer(ctx, hk)
	return err
}

func TestAgentLocked(t *testing.T) {
	ctx := context.Background()
	ctx = sshkeys.WithAgentSocketName(ctx, sshAgentSockName)

	client := sshkeys.NewClient()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		passphrase := []byte("locked")
		if err := client.Lock(ctx, passphrase); err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
		testAgent(ctx, t, kt, passphrase)

		err := getSigner(ctx, t, kt)
		if err == nil || !strings.Contains(err.Error(), "key not found in ssh agent") {
			t.Fatalf("%v: missing or unexpected error: %v", kt, err)
		}

		if err := client.Unlock(ctx, passphrase); err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
	}
}
