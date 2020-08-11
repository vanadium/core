// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshagent_test

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/lib/security/signing/sshagent"
	"v.io/x/ref/test/testutil/testsshagent"
)

var agentSockName string
var sshKeyDir string

func TestMain(m *testing.M) {
	cleanup, addr, keydir, err := testsshagent.StartPreconfiguredAgent()
	if err != nil {
		flag.Parse()
		cleanup()
		fmt.Fprintf(os.Stderr, "failed to start/configure agent: %v\n", err)
		os.Exit(1)
	}
	agentSockName = addr
	sshKeyDir = keydir
	code := m.Run()
	cleanup()
	os.RemoveAll(keydir)
	os.Exit(code)
}

func testAgentSigningVanadiumVerification(ctx context.Context, t *testing.T, passphrase []byte) {
	service := sshagent.NewSigningService()
	sshagent.SetAgentSockName(service, agentSockName)
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, filename := range []string{
		"ecdsa-256",
		"ecdsa-384",
		"ecdsa-521",
		"ed25519",
	} {
		data := make([]byte, 4096)
		_, err := randSource.Read(data)
		if err != nil {
			t.Fatalf("rand: %v", err)
		}
		pubKey := filepath.Join(sshKeyDir, filename+".pub")
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
			t.Errorf("failed to verify signature for %v", filename)
		}
		data[1]++
		if sig.Verify(publicKey, data) {
			t.Errorf("failed to detect changed message for %v", filename)
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

	edkey := filepath.Join(sshKeyDir, "ed25519.pub")
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
