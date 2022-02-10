// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	gocontext "context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"v.io/v23/security"
	"v.io/x/ref"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/sshkeys"
	"v.io/x/ref/test/sectestdata"
)

var (
	sshKeyDir           string
	sshTestKeys         []string
	legacyPrincipalDir  string
	currentPrincipalDir string
)

func TestMain(m *testing.M) {
	principalDir, err := os.MkdirTemp("", "principals")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(principalDir)
	legacyPrincipalDir = filepath.Join(principalDir, "legacy")
	currentPrincipalDir = filepath.Join(principalDir, "current")
	for _, dir := range []string{legacyPrincipalDir, currentPrincipalDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			panic(err)
		}
	}

	sectestdata.V23CopyLegacyPrincipals(legacyPrincipalDir)
	if err := createPersistentPrincipals(currentPrincipalDir); err != nil {
		panic(err)
	}

	var cleanup func()
	var agentSockName string

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

func initAndLockPrincipalDir(dir string) (func(), error) {
	if err := mkDir(dir); err != nil {
		return nil, err
	}
	flock := lockedfile.MutexAt(filepath.Join(dir, directoryLockfileName))
	unlock, err := flock.Lock()
	if err != nil {
		return nil, fmt.Errorf("failed to lock %v: %v", flock, err)
	}
	return unlock, nil
}

func writeKeyPairUsingPrivateKey(dir string, private crypto.PrivateKey, passphrase []byte) error {
	pubBytes, privBytes, err := marshalKeyPair(private, passphrase)
	if err != nil {
		return err
	}
	if err := writeKeyFile(filepath.Join(dir, publicKeyFile), pubBytes); err != nil {
		return err
	}
	return writeKeyFile(filepath.Join(dir, privateKeyFile), privBytes)
}

func writeKeyPairUsingBytes(dir string, pubBytes, privBytes []byte) error {
	if err := writeKeyFile(filepath.Join(dir, publicKeyFile), pubBytes); err != nil {
		return err
	}
	if len(privBytes) == 0 {
		return nil
	}
	return writeKeyFile(filepath.Join(dir, privateKeyFile), privBytes)
}

func createPersistentPrincipals(dir string) error {
	for _, kt := range sectestdata.SupportedKeyAlgos {
		for _, pp := range [][]byte{nil, sectestdata.Password()} {
			basename := kt.String()
			if pp != nil {
				basename = "encrypted-" + basename
			}
			key, err := keys.NewPrivateKeyForAlgo(kt)
			if err != nil {
				return err
			}
			dirname := filepath.Join(dir, basename)
			unlock, err := initAndLockPrincipalDir(dirname)
			if err != nil {
				return err
			}
			unlock()
			if err := writeKeyPairUsingPrivateKey(dirname, key, pp); err != nil {
				return err
			}
			readonly := filepath.Join(dir, "readonly-"+basename)
			unlock, err = initAndLockPrincipalDir(readonly)
			if err != nil {
				return err
			}
			unlock()
			if err := writeKeyPairUsingPrivateKey(readonly, key, pp); err != nil {
				return err
			}
			if err := setReadonly(readonly); err != nil {
				return err
			}
			readonly = filepath.Join(dir, "readonly-nolock-"+basename)
			unlock, err = initAndLockPrincipalDir(readonly)
			if err != nil {
				return err
			}
			unlock()
			if err := writeKeyPairUsingPrivateKey(readonly, key, pp); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(readonly, "dir.lock")); err != nil {
				return err
			}
			if err := setReadonly(readonly); err != nil {
				return err
			}
		}
	}
	return nil
}

func setReadonly(dir string) error {
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
			return fmt.Errorf("failed to set permissions for %v", path)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to set creds to readonly: %v", err)
	}
	return nil
}

func TestLoadPersistentPEMPrincipal(t *testing.T) {
	// If the directory does not exist want os.IsNotExists.
	if _, err := LoadPersistentPrincipal("/cantexist/fake/path/", nil); !os.IsNotExist(err) {
		t.Errorf("invalid path should return does not exist error, instead got %v", err)
	}

	for _, kt := range sectestdata.SupportedKeyAlgos {
		for _, tc := range []struct {
			plaintextDir string
			encryptedDir string
		}{
			{
				filepath.Join(legacyPrincipalDir, sectestdata.V23PrincipalDir(kt, false)),
				filepath.Join(legacyPrincipalDir, sectestdata.V23PrincipalDir(kt, true)),
			},
			{
				filepath.Join(currentPrincipalDir, kt.String()),
				filepath.Join(currentPrincipalDir, "encrypted-"+kt.String()),
			},
		} {
			if _, err := LoadPersistentPrincipal(tc.plaintextDir, nil); err != nil {
				t.Errorf("unencrypted LoadPersistentPrincipal should have succeeded: %v", err)
			}

			// If the private key file exists and is encrypted we should succeed with correct passphrase.
			if _, err := LoadPersistentPrincipal(tc.encryptedDir, sectestdata.Password()); err != nil {
				t.Errorf("encrypted LoadPersistentPrincipal should have succeeded: %v", err)
			}

			// and fail with an incorrect passphrase.
			if _, err := LoadPersistentPrincipal(tc.encryptedDir, []byte("nonsense")); err == nil {
				t.Errorf("encrypted LoadPersistentPrincipal with incorrect passphrase should fail")
			}
			// and return ErrPassphraseRequired if the passphrase is nil.
			if _, err := LoadPersistentPrincipal(tc.encryptedDir, nil); !errors.Is(err, ErrPassphraseRequired) {
				t.Errorf("encrypted LoadPersistentPrincipal with nil passphrase should return ErrPassphraseRequired: %v", err)
			}
		}
	}
}

func TestReadonlyAccess(t *testing.T) {
	kt := keys.ECDSA256
	dir := filepath.Join(currentPrincipalDir, kt.String())
	p, err := LoadPersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatalf("failed to load principal from %v: %v", dir, err)
	}

	dir = filepath.Join(currentPrincipalDir, "readonly-"+kt.String())

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
	dir = filepath.Join(currentPrincipalDir, "readonly-nolock-"+kt.String())
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
	for _, keyName := range sshTestKeys {
		dir := t.TempDir()
		keyName += ".pub"
		// use an ssh key and agent for signing.
		if err := useSSHPublicKeyAsPrincipal(sshKeyDir, dir, keyName); err != nil {
			t.Errorf("useSSHPublicKeyAsPrincipal: %v", err)
			continue
		}
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
	}
}

func TestMissingSSHPrivateKey(t *testing.T) {
	ctx := gocontext.TODO()

	// ssh key that doesn't exist in the agent.
	ek, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	missingKey, err := ssh.NewPublicKey(ek)
	if err != nil {
		t.Fatal(err)
	}
	sshKey := sshkeys.NewHostedKey(missingKey, "some comment")
	_, err = CreatePersistentPrincipalUsingKey(ctx, sshKey, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Log(err)
		t.Errorf("CreatePersistentPrincipalUsingKey should have failed with a key not found error")
	}
}

func funcForKey(keyType keys.CryptoAlgo) func(dir string, pass []byte) (security.Principal, error) {
	return func(dir string, pass []byte) (security.Principal, error) {
		key, err := keys.NewPrivateKeyForAlgo(keyType)
		if err != nil {
			return nil, err
		}
		return CreatePersistentPrincipalUsingKey(gocontext.TODO(), key, dir, pass)
	}
}

func funcForSSHKey(keyFile string) func(dir string, pass []byte) (security.Principal, error) {
	return func(dir string, pass []byte) (security.Principal, error) {
		ctx := gocontext.TODO()
		if len(pass) > 0 {
			ctx = sshkeys.WithAgentPassphrase(ctx, pass)
		}
		key, err := sshkeys.NewHostedKeyFile(filepath.Join(sshKeyDir, keyFile))
		if err != nil {
			return nil, err
		}
		return CreatePersistentPrincipalUsingKey(ctx, key, dir, pass)
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
		{funcForKey(keys.ECDSA256), []byte("unencrypted"), nil},
		{funcForKey(keys.ECDSA384), []byte("encrypted"), []byte("passphrase")},
		{funcForKey(keys.ECDSA521), []byte("encrypted"), []byte("passphrase")},
		{funcForKey(keys.ED25519), []byte("unencrypted"), nil},
		{funcForKey(keys.ED25519), []byte("encrypted"), []byte("passphrase")},
		{funcForKey(keys.RSA2048), []byte("encrypted"), nil},
		{funcForKey(keys.RSA4096), []byte("encrypted"), []byte("passphrase")},

		{funcForSSHKey("ssh-ecdsa-256.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-ecdsa-521.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-ed25519.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-rsa-2048.pub"), []byte("unencrypted"), nil},
		{funcForSSHKey("ssh-rsa-4096.pub"), []byte("unencrypted"), nil},

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
	dir := t.TempDir()

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

func useSSHPublicKeyAsPrincipal(from, to, name string) error {
	pubBytes, err := os.ReadFile(filepath.Join(from, name))
	if err != nil {
		return err
	}
	pubBytes, privBytes, err := sshkeys.MarshalForImport(
		context.Background(),
		pubBytes,
		sshkeys.ImportUsingAgent(true))

	if err != nil {
		return err
	}
	err = writeKeyPairUsingBytes(to, pubBytes, privBytes)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(to, directoryLockfileName), nil, 0666)
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
	principals, daemons := createAliceAndBob(ctx, t, funcForKey(keys.ECDSA256))
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
	testDaemonPublicKeyOnly(t, funcForKey(keys.ECDSA256), passphrase)
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
	dir := t.TempDir()

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
