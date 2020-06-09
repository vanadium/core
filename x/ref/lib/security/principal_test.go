// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	gocontext "context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/internal"
)

func TestLoadPersistentPrincipal(t *testing.T) {
	// If the directory does not exist want os.IsNotExists.
	if _, err := LoadPersistentPrincipal("/tmp/fake/path/", nil); !os.IsNotExist(err) {
		t.Errorf("invalid path should return does not exist error, instead got %v", err)
	}
	// If the key file exists and is unencrypted we should succeed.
	dir := generatePEMFile(nil)
	if _, err := LoadPersistentPrincipal(dir, nil); err != nil {
		t.Errorf("unencrypted LoadPersistentPrincipal should have succeeded: %v", err)
	}
	os.RemoveAll(dir)

	// If the private key file exists and is encrypted we should succeed with correct passphrase.
	passphrase := []byte("passphrase")
	incorrectPassphrase := []byte("incorrectPassphrase")
	dir = generatePEMFile(passphrase)
	if _, err := LoadPersistentPrincipal(dir, passphrase); err != nil {
		t.Errorf("encrypted LoadPersistentPrincipal should have succeeded: %v", err)
	}

	// and fail with an incorrect passphrase.
	if _, err := LoadPersistentPrincipal(dir, incorrectPassphrase); err == nil {
		t.Errorf("encrypted LoadPersistentPrincipal with incorrect passphrase should fail")
	}
	// and return ErrPassphraseRequired if the passphrase is nil.
	if _, err := LoadPersistentPrincipal(dir, nil); verror.ErrorID(err) != ErrPassphraseRequired.ID {
		t.Errorf("encrypted LoadPersistentPrincipal with nil passphrase should return ErrPassphraseRequired: %v", err)
	}
	os.RemoveAll(dir)
}

func TestCreatePersistentPrincipal(t *testing.T) {
	tests := []struct {
		Message, Passphrase []byte
	}{
		{[]byte("unencrypted"), nil},
		{[]byte("encrypted"), []byte("passphrase")},
	}
	for _, test := range tests {
		testCreatePersistentPrincipal(t, test.Message, test.Passphrase)
	}
}

func testCreatePersistentPrincipal(t *testing.T, message, passphrase []byte) {
	// Persistence of the BlessingRoots and BlessingStore objects is
	// tested in other files. Here just test the persistence of the key.
	dir, err := ioutil.TempDir("", "TestCreatePersistentPrincipal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	p, err := CreatePersistentPrincipal(dir, passphrase)
	if err != nil {
		t.Fatal(err)
	}

	_, err = CreatePersistentPrincipal(dir, passphrase)
	if err == nil {
		t.Error("CreatePersistentPrincipal passed unexpectedly")
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

func generatePEMFile(passphrase []byte) (dir string) {
	dir, err := ioutil.TempDir("", "TestLoadPersistentPrincipal")
	if err != nil {
		panic(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	f, err := os.Create(path.Join(dir, privateKeyFile))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err = internal.SavePEMKey(f, key, passphrase); err != nil {
		panic(err)
	}
	return dir
}

func TestDaemonMode(t *testing.T) {
	ctx, cancel := gocontext.WithCancel(gocontext.Background())
	defer cancel()
	// Create two principls that don't trust each other.
	dirs := map[string]string{}
	principals := map[string]security.Principal{}
	daemons := map[string]security.Principal{}
	for _, p := range []string{"alice", "bob"} {
		dir, err := ioutil.TempDir("", "alice")
		if err != nil {
			t.Fatal(err)
		}
		dirs[p] = dir
		if _, err := CreatePersistentPrincipal(dir, nil); err != nil {
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

	alice, bob := principals["alice"], principals["bob"]
	aliced, bobd := daemons["alice"], daemons["bob"]

	for _, p := range []string{"alice", "bob"} {
		self, err := principals[p].BlessSelf(p)
		if err != nil {
			t.Fatal(err)
		}
		SetDefaultBlessings(principals[p], self)
		// Default blessings will not have been reloaded by the daemons yet.
		dp := daemons[p]
		if got, want := len(dp.Roots().Dump()), 0; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		def, _ := dp.BlessingStore().Default()
		if got, want := def.IsZero(), true; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	// Don't send a SIGHIP to ourselves here since it seems to crash vscode!

	// Wait for default blessings to change.
	_, aliceCh := aliced.BlessingStore().Default()
	_, bobCh := bobd.BlessingStore().Default()

	a, b := false, false
	for {
		select {
		case <-aliceCh:
			a = true
		case <-bobCh:
			b = true
		}
		time.Sleep(time.Millisecond)
		if a && b {
			break
		}
	}
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
