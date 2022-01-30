// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	gocontext "context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/sectestdata"
)

var (
	agentSockName string
	sshKeyDir     string
	sshTestKeys   []string
)

func TestMain(m *testing.M) {
	var err error
	var cleanup func()
	sshKeyDir, agentSockName, cleanup, err = sectestdata.StartPreConfiguredSSHAgent()
	if err != nil {
		flag.Parse()
		cleanup()
		os.RemoveAll(sshKeyDir)
		fmt.Fprintf(os.Stderr, "failed to start/configure agent: %v\n", err)
		os.Exit(1)
	}
	sshTestKeys = sectestdata.SSHPrivateKeys()
	sshkeys.DefaultSockNameFunc = func() string {
		return agentSockName
	}
	code := m.Run()
	cleanup()
	os.RemoveAll(sshKeyDir)
	os.Exit(code)
}

func setReadonly(t *testing.T, dir string) {
	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		mode := os.FileMode(0400)
		if fi.IsDir() {
			mode = 0500
		}
		err = os.Chmod(path, mode)
		nfi, _ := os.Stat(path)
		if nfi.Mode().Perm() != mode {
			t.Fatalf("failed to set permissions for %v", path)
		}
		return err
	})
	if err != nil {
		t.Fatalf("failed to set creds to readonly: %v", err)
	}
}

func TestLoadPersistentPEMPrincipal(t *testing.T) {
	// If the directory does not exist want os.IsNotExists.
	if _, err := LoadPersistentPrincipal("/cantexist/fake/path/", nil); !os.IsNotExist(err) {
		t.Errorf("invalid path should return does not exist error, instead got %v", err)
	}
	// If the key file exists and is unencrypted we should succeed.
	dir := generatePEMPrincipal(nil)
	defer os.RemoveAll(dir)
	if _, err := LoadPersistentPrincipal(dir, nil); err != nil {
		t.Errorf("unencrypted LoadPersistentPrincipal should have succeeded: %v", err)
	}

	// If the private key file exists and is encrypted we should succeed with correct passphrase.
	passphrase := []byte("passphrase")
	incorrectPassphrase := []byte("incorrectPassphrase")
	dir = generatePEMPrincipal(passphrase)
	defer os.RemoveAll(dir)
	if _, err := LoadPersistentPrincipal(dir, passphrase); err != nil {
		t.Errorf("encrypted LoadPersistentPrincipal should have succeeded: %v", err)
	}

	// and fail with an incorrect passphrase.
	if _, err := LoadPersistentPrincipal(dir, incorrectPassphrase); err == nil {
		t.Errorf("encrypted LoadPersistentPrincipal with incorrect passphrase should fail")
	}
	// and return ErrPassphraseRequired if the passphrase is nil.
	if _, err := LoadPersistentPrincipal(dir, nil); !errors.Is(err, ErrPassphraseRequired) {
		t.Errorf("encrypted LoadPersistentPrincipal with nil passphrase should return ErrPassphraseRequired: %v", err)
	}
}

func TestReadonlyAccess(t *testing.T) {
	dir := generatePEMPrincipal(nil)
	defer os.RemoveAll(dir)
	p, err := LoadPersistentPrincipal(dir, nil)
	if err != nil {
		t.Errorf("unencrypted LoadPersistentPrincipal should have succeeded: %v", err)
	}

	// Test read-only access, should fail for LoadPersistentPrincipal.
	setReadonly(t, dir)
	_, err = LoadPersistentPrincipal(dir, nil)
	if err == nil || !strings.Contains(err.Error(), "dir.lock: permission denied") {
		t.Fatalf("missing or incorrect error: %v", err)
	}
	// Test read-only access, should not fail for LoadPersistentPrincipalDaemon
	// in read-only mode.
	rp, err := LoadPersistentPrincipalDaemon(gocontext.TODO(), dir, nil, true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rp.PublicKey().String(), p.PublicKey().String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Test read-only access, making sure that dir.lock is removed first also.
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "dir.lock")); err != nil {
		t.Error(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}

	// Read-only access without a dir.lock file should succeed for a read-only
	// filesystem since there's no need for a read-lock in that case,
	// but will otherwise fail since write-access is required to create a read-only
	// file lock.
	_, err = LoadPersistentPrincipalDaemon(gocontext.TODO(), dir, nil, true, time.Second)
	if err == nil || !strings.Contains(err.Error(), "dir.lock: permission denied") {
		t.Fatalf("missing or incorrect error: %v", err)
	}

	os.Setenv(ref.EnvCredentialsReadonlyFileSystem, "1")
	defer os.Setenv(ref.EnvCredentialsReadonlyFileSystem, "")
	envvar, ok := ref.ReadonlyCredentialsDir()
	if got, want := envvar, ref.EnvCredentialsReadonlyFileSystem; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ok, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	rp, err = LoadPersistentPrincipalDaemon(gocontext.TODO(), dir, nil, true, time.Second)
	if err != nil {
		t.Fatalf("encrypted LoadPersistentPrincipal from readonly directory should have succeeded: %v", err)
	}
	if got, want := rp.PublicKey().String(), p.PublicKey().String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestLoadPersistentSSHPrincipal(t *testing.T) {
	dir, err := os.MkdirTemp("", "TestLoadPersistentPrincipal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for _, keyName := range sshTestKeys {
		keyName += ".pub"
		// use an ssh key and agent for signing.
		useSSHPublicKeyAsPrincipal(sshKeyDir, dir, keyName)
		p, err := LoadPersistentPrincipal(dir, nil)
		if err != nil {
			t.Errorf("unencrypted LoadPersistentPrincipal should have succeeded: %v", err)
			continue
		}

		message := []byte("hello")
		sig, err := p.Sign(message)
		if err != nil {
			t.Fatal(err)
		}
		if !sig.Verify(p.PublicKey(), message) {
			t.Errorf("%s failed: p.PublicKey=%v", message, p.PublicKey())
		}

		// make sure that multiple keys lead to a failure.
		if err := os.WriteFile(filepath.Join(dir, privateKeyFile), []byte{'\n'}, 0666); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPersistentPrincipal(dir, nil); err == nil {
			t.Error("unencrypted LoadPersistentPrincipal should have failed complaining about multiple key files")
		}

		// make sure that no keys lead to a failure.
		err = os.Remove(filepath.Join(dir, privateKeyFile))
		if err != nil {
			t.Fatal(err)
		}
		err = os.Remove(filepath.Join(dir, keyName))
		if err != nil {
			t.Fatal(err)
		}
		_, err = LoadPersistentPrincipal(dir, nil)
		if err == nil {
			t.Errorf("unencrypted LoadPersistentPrincipal should have failed")
		}
	}
}

func TestCreatePrincipalSSH(t *testing.T) {
	dir, err := os.MkdirTemp("", "TestLoadPersistentPrincipal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ctx := gocontext.TODO()

	// non-existent ssh public key file
	_, err = NewSSHAgentHostedKey("does-not-exist.pub")
	if err == nil || !strings.Contains(err.Error(), "no such file") {
		t.Errorf("CreatePersistentPrincipalUsingKey should have failed with no such file error")
	}

	// malformed ssh public key file
	invalid := filepath.Join(dir, "invalid.pub")
	os.WriteFile(invalid, []byte{'1', '\n'}, 0600)

	_, err = NewSSHAgentHostedKey(invalid)
	if err == nil || !strings.Contains(err.Error(), "no key found") {
		t.Errorf("CreatePersistentPrincipalUsingKey should have failed: %v", err)
	}

	// ssh key that doesn't exist in the agent.
	ek, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	missingKey, err := ssh.NewPublicKey(ek)
	if err != nil {
		t.Fatal(err)
	}
	missing := "private-key-not-in-agent.pub"
	if err := os.WriteFile(missing, ssh.MarshalAuthorizedKey(missingKey), 0600); err != nil {
		t.Fatal(err)
	}

	sshKey, err := NewSSHAgentHostedKey(missing)
	if err != nil {
		t.Fatal(err)
	}
	_, err = CreatePersistentPrincipalUsingKey(ctx, sshKey, dir, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Log(err)
		t.Errorf("CreatePersistentPrincipalUsingKey should have failed with a key not found error")
	}
}

func funcForKey(keyType KeyType) func(dir string, pass []byte) (security.Principal, error) {
	return func(dir string, pass []byte) (security.Principal, error) {
		key, err := NewPrivateKey(keyType)
		if err != nil {
			return nil, err
		}
		return CreatePersistentPrincipalUsingKey(gocontext.TODO(), key, dir, pass)
	}
}

func funcForSSHKey(keyFile string) func(dir string, pass []byte) (security.Principal, error) {
	return func(dir string, pass []byte) (security.Principal, error) {
		key, err := NewSSHAgentHostedKey(filepath.Join(sshKeyDir, keyFile))
		if err != nil {
			return nil, err
		}
		return CreatePersistentPrincipalUsingKey(gocontext.TODO(), key, dir, pass)
	}
}

func funcForSSLKey(key crypto.PrivateKey) func(dir string, pass []byte) (security.Principal, error) {
	return func(dir string, pass []byte) (security.Principal, error) {
		return CreatePersistentPrincipalUsingKey(gocontext.TODO(), key, dir, pass)
	}
}

func TestCreatePersistentPrincipal(t *testing.T) {
	sslKeys, _, _ := sectestdata.VanadiumSSLData()

	tests := []struct {
		fn                  func(dir string, pass []byte) (security.Principal, error)
		Message, Passphrase []byte
	}{
		{funcForKey(ECDSA256), []byte("unencrypted"), nil},
		{funcForKey(ECDSA384), []byte("encrypted"), []byte("passphrase")},
		{funcForKey(ECDSA521), []byte("encrypted"), []byte("passphrase")},
		{funcForKey(ED25519), []byte("unencrypted"), nil},
		{funcForKey(ED25519), []byte("encrypted"), []byte("passphrase")},
		{funcForSSHKey("ssh-ecdsa-256.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-ed25519.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-rsa-2048.pub"), []byte("unencrypted"), nil},

		{funcForSSLKey(sslKeys["rsa-2048"]), []byte("unencrypted"), nil},
		{funcForSSLKey(sslKeys["rsa-4096"]), []byte("unencrypted"), nil},
		{funcForSSLKey(sslKeys["ed25519"]), []byte("unencrypted"), nil},
		{funcForSSLKey(sslKeys["ecdsa-256"]), []byte("unencrypted"), nil},
	}
	for _, test := range tests {
		testCreatePersistentPrincipal(t, test.fn, test.Message, test.Passphrase)
	}
}

func testCreatePersistentPrincipal(t *testing.T, fn func(dir string, pass []byte) (security.Principal, error), message, passphrase []byte) {
	// Persistence of the BlessingRoots and BlessingStore objects is
	// tested in other files. Here just test the persistence of the key.
	dir, err := os.MkdirTemp("", "TestCreatePersistentPrincipal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	p, err := fn(dir, passphrase)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fn(dir, passphrase)
	if err == nil {
		t.Errorf("CreatePersistentPrincipal succeeded unexpectedly, it should fail since the credentials keys should already exist")
	}

	sig, err := p.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	p2, err := LoadPersistentPrincipal(dir, passphrase)
	if err != nil {
		t.Fatalf("%s failed: %v", message, err)
	}

	if !sig.Verify(p2.PublicKey(), message) {
		t.Errorf("%s failed: p.PublicKey=%v, p2.PublicKey=%v", message, p.PublicKey(), p2.PublicKey())
	}
}

func useSSHPublicKeyAsPrincipal(from, to, name string) {
	err := internal.CopyKeyFile(
		filepath.Join(from, name),
		filepath.Join(to, name))
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(to, directoryLockfileName), nil, 0666)
	if err != nil {
		panic(err)
	}
}

func generatePEMPrincipal(passphrase []byte) (dir string) {
	dir, err := os.MkdirTemp("", "TestLoadPersistentPrincipal")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(dir, directoryLockfileName), nil, 0666)
	if err != nil {
		panic(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	if err := internal.WritePEMKeyPair(
		key,
		path.Join(dir, privateKeyFile),
		path.Join(dir, publicKeyFile),
		passphrase,
	); err != nil {
		panic(err)
	}
	return dir
}

func createAliceAndBob(ctx gocontext.Context, t *testing.T, creator func(dir string, pass []byte) (security.Principal, error)) (principals, daemons map[string]security.Principal) {
	principals, daemons = map[string]security.Principal{}, map[string]security.Principal{}
	for _, p := range []string{"alice", "bob"} {
		dir := t.TempDir()
		if _, err := creator(dir, nil); err != nil {
			t.Fatal(err)
		}
		principal, err := LoadPersistentPrincipalDaemon(ctx, dir, nil, true, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		daemons[p] = principal
		principal, err = LoadPersistentPrincipal(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		principals[p] = principal
	}
	return
}

func waitForDefaultChanges(oab, obb security.Blessings, ap, bp security.Principal) {
	a, b := false, false
	_, aCh := ap.BlessingStore().Default()
	_, bCh := bp.BlessingStore().Default()
	for {
		// just in case we missed the update
		nab, _ := ap.BlessingStore().Default()
		nbb, _ := bp.BlessingStore().Default()
		if !nab.Equivalent(oab) && !nbb.Equivalent(obb) {
			return
		}
		select {
		case <-aCh:
			a = true
		case <-bCh:
			b = true
		}
		if a && b {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func waitForRootChanges(ap, bp security.Principal) {
	// There is currently no better way to wait for the blessing roots
	// to change than polling.
	a, b := false, false
	for {
		if len(ap.Roots().Dump()) > 0 {
			a = true
		}
		if len(bp.Roots().Dump()) > 0 {
			b = true
		}
		if a && b {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestDaemonMode(t *testing.T) {
	ctx, cancel := gocontext.WithCancel(gocontext.Background())
	defer cancel()
	// Create two principls that don't trust each other.
	principals, daemons := createAliceAndBob(ctx, t, funcForKey(ECDSA256))
	testDaemonMode(ctx, t, principals, daemons)
	principals, daemons = createAliceAndBob(ctx, t, funcForSSHKey("ssh-ed25519.pub"))
	testDaemonMode(ctx, t, principals, daemons)
}

func testDaemonMode(ctx gocontext.Context, t *testing.T, principals, daemons map[string]security.Principal) {
	alice, bob := principals["alice"], principals["bob"]
	aliced, bobd := daemons["alice"], daemons["bob"]

	origAliceDBlessings, _ := aliced.BlessingStore().Default()
	origBobDBlessings, _ := aliced.BlessingStore().Default()

	for _, p := range []string{"alice", "bob"} {
		self, err := principals[p].BlessSelf(p)
		if err != nil {
			t.Fatal(err)
		}
		SetDefaultBlessings(principals[p], self)
		// Default blessings should not have been reloaded by the daemons yet,
		// but there is a very slight possibility that they have so let's just
		// log that fact for now.
		dp := daemons[p]
		if got, want := len(dp.Roots().Dump()), 0; got != want {
			t.Logf("got %v, want %v", got, want)
		}
		def, _ := dp.BlessingStore().Default()
		if got, want := def.IsZero(), true; got != want {
			t.Logf("got %v, want %v", got, want)
		}
	}

	// Don't send a SIGHUP to ourselves here since it seems to crash vscode!

	// Wait for default blessings to change.
	waitForDefaultChanges(origAliceDBlessings, origBobDBlessings, aliced, bobd)
	waitForRootChanges(aliced, bobd)

	for _, p := range []string{"alice", "bob"} {
		dp := daemons[p]
		if got, want := len(dp.Roots().Dump()), 1; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		def, _ := dp.BlessingStore().Default()
		if got, want := def.IsZero(), false; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	if got, want := bobd.BlessingStore().DebugString(), bob.BlessingStore().DebugString(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	blessings, _ := alice.BlessingStore().Default()
	for1h, _ := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	forBob, err := alice.Bless(bob.PublicKey(), blessings, "friend:bob", for1h)
	if err != nil {
		t.Fatal(err)
	}
	bob.BlessingStore().Set(forBob, "alice")
	if got, want := bobd.BlessingStore().DebugString(), bob.BlessingStore().DebugString(); got == want {
		t.Errorf("got %v should not equal want %v", got, want)
	}

	for i := 0; i < 6; i++ { // 3 seconds at most.
		time.Sleep(time.Millisecond * 500)
		if got, want := bobd.BlessingStore().DebugString(), bob.BlessingStore().DebugString(); got == want {
			break
		}
	}
	if got, want := bobd.BlessingStore().DebugString(), bob.BlessingStore().DebugString(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDaemonPublicKeyOnly(t *testing.T) {
	ctx := context.Background()
	passphrase := []byte("with-passphrase")
	ctx = sshkeys.WithAgentPassphrase(ctx, passphrase)
	testDaemonPublicKeyOnly(t, funcForKey(ECDSA256), passphrase)
	client := sshkeys.NewClient()
	if err := client.Lock(ctx); err != nil {
		t.Fatal(err)
	}
	defer client.Unlock(ctx)
	// passphrase is zeroed out when it's used above.
	passphrase = []byte("with-passphrase")
	testDaemonPublicKeyOnly(t, funcForSSHKey("ssh-ecdsa-256.pub"), passphrase)
}

func testDaemonPublicKeyOnly(t *testing.T, creator func(dir string, pass []byte) (security.Principal, error), passphrase []byte) {
	dir, err := os.MkdirTemp("", "TestCreatePersistentPrincipal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	p, err := creator(dir, passphrase)
	if err != nil {
		t.Fatal(err)
	}

	message := []byte("to-be-signed")
	sig, err := p.Sign(message)
	if err != nil {
		t.Fatal(err)
	}

	// Create a principal with only the public key from the original
	// principal above.
	ctx := gocontext.Background()
	pk, err := LoadPersistentPrincipalDaemon(ctx, dir, nil, true, time.Duration(0))
	if err != nil {
		t.Fatalf("%s failed: %v", message, err)
	}
	if !sig.Verify(pk.PublicKey(), message) {
		t.Errorf("%s failed: p.PublicKey=%v, p2.PublicKey=%v", message, p.PublicKey(), pk.PublicKey())
	}
}
