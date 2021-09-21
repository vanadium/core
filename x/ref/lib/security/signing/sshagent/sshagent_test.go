// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshagent_test

import (
	"context"
	"crypto"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/signing/sshagent"
	"v.io/x/ref/test/testutil/testsshagent"
)

var (
	agentSockName string
	sshKeyDir     string
	sshTestKeys   = []string{
		"ssh-ecdsa-256",
		"ssh-ecdsa-384",
		"ssh-ecdsa-521",
		"ssh-ed25519",
		"ssh-rsa-2048",
		"ssh-rsa-3072",
	}
)

func TestMain(m *testing.M) {
	sshKeyDir = "testdata"
	cleanup, addr, err := testsshagent.StartPreconfiguredAgent(sshKeyDir, sshTestKeys...)
	if err != nil {
		flag.Parse()
		cleanup()
		fmt.Fprintf(os.Stderr, "failed to start/configure agent: %v\n", err)
		os.Exit(1)
	}
	agentSockName = addr
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func loadSSHPrivateKey(filename string) (crypto.PrivateKey, error) {
	pemBlockBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	pemBlock, _ := pem.Decode(pemBlockBytes)
	switch pemBlock.Type {
	case "OPENSSH PRIVATE KEY":
		return ssh.ParseRawPrivateKey(pemBlockBytes)
	}
	panic("x")
}

func testAgentSigningVanadiumVerification(ctx context.Context, t *testing.T, passphrase []byte) {
	service := sshagent.NewSigningService()
	sshagent.SetAgentSockName(service, agentSockName)
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, keyName := range sshTestKeys {
		data := make([]byte, 4096)
		_, err := randSource.Read(data)
		if err != nil {
			t.Fatalf("rand: %v", err)
		}

		// Compute signature using vanadium code.
		vpkey, err := loadSSHPrivateKey(filepath.Join("testdata", keyName))
		if err != nil {
			t.Fatalf("file %v %v", filepath.Join("testdata", keyName), err)
		}
		vsigner, err := security.NewInMemorySigner(vpkey)
		if err != nil {
			t.Fatalf("file %v %v", filepath.Join("testdata", keyName), err)
		}
		vsig, err := vsigner.Sign([]byte("testing"), data)
		if err != nil {
			t.Fatalf("file %v %v", filepath.Join("testdata", keyName), err)
		}

		pubKey := filepath.Join(sshKeyDir, keyName+".pub")
		signer, err := service.Signer(ctx, pubKey, passphrase)
		if err != nil {
			t.Fatalf("service.Signer: %v", err)
		}
		sig, err := signer.Sign([]byte("testing"), data)
		if err != nil {
			t.Fatalf("signer.Sign: %v", err)
		}
		// Verify using Vanadium code.
		publicKey := signer.PublicKey()
		if !sig.Verify(publicKey, data) {
			t.Errorf("failed to verify ssh generated signature for %v", keyName)
		}

		if !vsig.Verify(publicKey, data) {
			t.Errorf("failed to verify vanadium generated signature from ssh private for %v", keyName)
		}

		data[1]++
		if sig.Verify(publicKey, data) {
			t.Errorf("failed to detect changed message for %v", keyName)
		}

	}
	defer service.Close(ctx)
}

func TestAgentSigningVanadiumVerification(t *testing.T) {
	ctx := context.Background()
	testAgentSigningVanadiumVerification(ctx, t, nil)
}

func TestAgentSigningVanadiumVerificationPassphrase(t *testing.T) {
	ctx := context.Background()
	passphrase := []byte("something")
	service := sshagent.NewSigningService()
	agent := service.(*sshagent.Client)
	agent.SetAgentSockName(agentSockName)
	if err := agent.Lock(passphrase); err != nil {
		t.Fatalf("agent.Lock: %v", err)
	}

	edkey := filepath.Join(sshKeyDir, "ssh-ed25519.pub")
	_, err := service.Signer(ctx, edkey, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("service.Signer: should have failed with a key not found error: %v", err)
	}
	testAgentSigningVanadiumVerification(ctx, t, passphrase)

	// make sure agent is still locked.
	_, err = service.Signer(ctx, edkey, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("service.Signer: should have failed with a key not found error: %v", err)
	}
}
