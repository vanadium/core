// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys_test

import (
	"bytes"
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

	if got, want := sshkeys.AgentPassphrase(ctx), []byte(nil); !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	ctx = sshkeys.WithAgentPassphrase(ctx, []byte("oh-my"))
	if got, want := sshkeys.AgentPassphrase(ctx), []byte("oh-my"); !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := sshkeys.AgentPassphrase(ctx), []byte{0x0, 0x0, 0x0, 0x0, 0x0}; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	ctx = sshkeys.WithAgentPassphrase(context.Background(), nil)
	if got := sshkeys.AgentPassphrase(ctx); got != nil {
		t.Errorf("got %v, want nil", got)
	}

	ctx = sshkeys.WithAgentPassphrase(context.Background(), []byte{})
	if got := sshkeys.AgentPassphrase(ctx); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func testAgent(ctx context.Context, t *testing.T, kt keys.CryptoAlgo) {
	publicKeyBytes := sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)

	publicKeyBytes, privateKeyBytes, err := sshkeys.MarshalForImport(ctx, publicKeyBytes, sshkeys.ImportUsingAgent(true))
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	key, err := keyRegistrar.ParsePublicKey(publicKeyBytes)
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

	hk, err := keyRegistrar.ParsePrivateKey(ctx, privateKeyBytes, nil)
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
		testAgent(ctx, t, kt)
	}
}

func getSigner(ctx context.Context, t *testing.T, kt keys.CryptoAlgo) error {
	publicKeyBytes := sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)

	_, privateKeyBytes, err := sshkeys.MarshalForImport(ctx, publicKeyBytes, sshkeys.ImportUsingAgent(true))
	if err != nil {
		t.Fatalf("%v: %v", kt, err)
	}

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
	withoutPassphrase := ctx

	client := sshkeys.NewClient()
	for _, kt := range sectestdata.SupportedKeyAlgos {
		withPassphrase := sshkeys.WithAgentPassphrase(ctx, []byte("locked"))
		if err := client.Lock(withPassphrase); err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
		withPassphrase = sshkeys.WithAgentPassphrase(ctx, []byte("locked"))
		testAgent(withPassphrase, t, kt)

		err := getSigner(withoutPassphrase, t, kt)
		if err == nil || !strings.Contains(err.Error(), "key not found in ssh agent") {
			t.Fatalf("%v: missing or unexpected error: %v", kt, err)
		}
		withPassphrase = sshkeys.WithAgentPassphrase(ctx, []byte("locked"))
		if err := client.Unlock(withPassphrase); err != nil {
			t.Fatalf("%v: %v", kt, err)
		}
	}
}
