// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/lib/ibe"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/bcrypter"
	"v.io/x/ref/services/iberoot"
	iroot "v.io/x/ref/services/iberoot/iberootd/internal"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"

	_ "v.io/x/ref/runtime/factories/generic"
)

func TestSeekPrivateKeys(t *testing.T) {
	// Create a context for starting an IBE
	// root service.
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Create a new IDP.
	idpName := "idp"
	idp := testutil.NewPrincipal(idpName)

	// Make the root service context recognize the IDP.
	idpB, _ := idp.BlessingStore().Default()
	if err := security.AddToRoots(v23.GetPrincipal(ctx), idpB); err != nil {
		t.Fatal(err)
	}

	// Setup the root service.
	_, dir := initIBEMaster(t, ctx)
	defer os.RemoveAll(dir)
	addr := newRootServer(t, ctx, dir, idpName)

	// Obtain the IDP's params.
	client := iberoot.RootClient(addr)
	wireParams, err := client.Params(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var params bcrypter.Params
	if err := params.FromWire(wireParams); err != nil {
		t.Fatal(err)
	}

	ptxt := []byte("plaintext")
	// Helper function to encrypt 'ptxt' for specific patterns
	// using the obtained params.
	encrypt := func(ctx *context.T, forPattern security.BlessingPattern) *bcrypter.Ciphertext {
		crypter := bcrypter.NewCrypter()
		if err := crypter.AddParams(ctx, params); err != nil {
			t.Fatal(err)
		}
		ctxt, err := crypter.Encrypt(ctx, forPattern, ptxt)
		if err != nil {
			t.Fatal(err)
		}
		return ctxt
	}

	// Helper function to check whether 'ctxt' can be decrypted to
	// 'ptxt' using one of the provided 'keys'.
	decrypt := func(ctx *context.T, ctxt *bcrypter.Ciphertext, keys []bcrypter.WirePrivateKey) error {
		crypter := bcrypter.NewCrypter()
		// Add all keys to crypter.
		for _, wireKey := range keys {
			var key bcrypter.PrivateKey
			if err := key.FromWire(wireKey); err != nil {
				return err
			}
			if err := crypter.AddKey(ctx, &key); err != nil {
				return err
			}
		}
		decrypted, err := crypter.Decrypt(ctx, ctxt)
		if err != nil {
			return fmt.Errorf("decryption failed: %v", err)
		}
		if !bytes.Equal(decrypted, ptxt) {
			return fmt.Errorf("decryption returned %v, want %v", decrypted, ptxt)
		}
		return nil
	}

	// Test private keys obtained using IDP blessings
	testcases := []struct {
		userExtensions  []string
		validPatterns   []security.BlessingPattern
		invalidPatterns []security.BlessingPattern
	}{
		{
			[]string{"user:batman"},
			[]security.BlessingPattern{"idp", "idp:user", "idp:user:batman", "idp:user:batman:$"},
			[]security.BlessingPattern{"idp:user:batman:phone", "idp:user:brucewayne", "idp:user:brucewayne:phone"},
		},
		{
			[]string{"user:batman", "user:brucewayne"},
			[]security.BlessingPattern{"idp", "idp:user", "idp:user:batman", "idp:user:brucewayne", "idp:user:batman:$", "idp:user:brucewayne:$"},
			[]security.BlessingPattern{"idp:user:carol", "idp:user:carol:phone"},
		},
	}
	for _, test := range testcases {
		user := derive(t, idp, test.userExtensions...)
		// Make the user principal recognize the root service.
		rootB, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
		if err := security.AddToRoots(user, rootB); err != nil {
			t.Fatal(err)
		}

		userCtx, err := v23.WithPrincipal(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		keys, err := client.SeekPrivateKeys(userCtx)
		if err != nil {
			t.Fatalf("SeekPrivateKeys failed: %v", err)
		}
		for _, p := range test.validPatterns {
			if err := decrypt(userCtx, encrypt(userCtx, p), keys); err != nil {
				t.Fatalf("Failed to decrypt ciphertext for pattern %v: %v", p, err)
			}
		}
		for _, p := range test.invalidPatterns {
			if err := decrypt(userCtx, encrypt(userCtx, p), keys); err == nil || !strings.Contains(err.Error(), "decryption failed") {
				t.Fatalf("Decrypting ciphertext for pattern %v returned %v, want error matching 'decryption failed'", p, err)
			}
		}
	}

	// Test that SeekPrivateKeys fails when the client presents
	// a blessing not derived from IDP.
	user := testutil.NewPrincipal("selfsigned")
	rootB, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	if err := security.AddToRoots(user, rootB); err != nil {
		t.Fatal(err)
	}
	userCtx, err := v23.WithPrincipal(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SeekPrivateKeys(userCtx); err == nil || verror.ErrorID(err) != iroot.ErrUnrecognizedRemoteBlessings.ID {
		t.Fatalf("Got error %v when seeking private keys for a self-signed blessing, wanted error with ID %v", err, "user", iroot.ErrUnrecognizedRemoteBlessings.ID)
	}
}

func TestParams(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()

	// Setup an IBE root service.
	master, dir := initIBEMaster(t, ctx)
	defer os.RemoveAll(dir)
	idpName := "idp"
	addr := newRootServer(t, ctx, dir, idpName)

	// Obtain Params from the IBE root service.
	gotWireParams, err := iberoot.RootClient(addr).Params(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Compute expected params from the IBE Master object used by the
	// root service.
	var wantWireParams bcrypter.WireParams
	p := bcrypter.NewRoot(idpName, master).Params()
	if err := p.ToWire(&wantWireParams); err != nil {
		t.Fatal(err)
	}

	// Test got == want.
	if !reflect.DeepEqual(gotWireParams, wantWireParams) {
		t.Fatalf("Got params %v, want %v", gotWireParams, wantWireParams)
	}
}

func derive(t *testing.T, idp security.Principal, extensions ...string) security.Principal {
	p := testutil.NewPrincipal()
	blessings := make([]security.Blessings, len(extensions))
	var err error
	for i, e := range extensions {
		b, _ := idp.BlessingStore().Default()
		if blessings[i], err = idp.Bless(p.PublicKey(), b, e, security.UnconstrainedUse()); err != nil {
			t.Fatal(err)
		}
	}
	bUnion, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		t.Fatal(err)
	}
	if err := vsecurity.SetDefaultBlessings(p, bUnion); err != nil {
		t.Fatal(err)
	}
	return p
}

func newRootServer(t *testing.T, ctx *context.T, dir string, idpName string) string {
	root, err := iroot.NewRootServer(dir, idpName)
	if err != nil {
		t.Fatalf("Cannot create iberoot.Root object: %v", err)
	}

	ctx, server, err := v23.WithNewServer(ctx, "", root, security.AllowEveryone())
	if err != nil {
		t.Fatalf("WithNewServer failed: %v", err)
	}
	return server.Status().Endpoints[0].Name()
}

func initIBEMaster(t *testing.T, ctx *context.T) (ibe.Master, string) {
	dir, err := ioutil.TempDir("", "ibe_master_key")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	master, err := ibe.SetupBB2()
	if err != nil {
		t.Fatalf("ibe.SetupBB2() failed: %v", err)
	}
	if err := iroot.SaveMaster(master, dir); err != nil {
		t.Fatalf("SaveMaster failed: %v", err)
	}
	return master, dir
}
