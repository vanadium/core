// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func TestX509(t *testing.T) {
	privKey, pubCerts, opts := sectestdata.LetsEncryptData()
	serverSigner, err := seclib.NewInMemorySigner(privKey)
	if err != nil {
		t.Fatal(err)
	}
	server, err := security.CreatePrincipal(serverSigner,
		seclib.NewBlessingStore(serverSigner.PublicKey()),
		seclib.NewBlessingRootsWithX509Options(opts))
	if err != nil {
		t.Fatal(err)
	}
	blessings, err := server.BlessSelfX509(pubCerts[0])
	if err != nil {
		t.Fatal(err)
	}

	clientSigner := sectest.NewED25519Signer(t)
	client, err := security.CreatePrincipal(clientSigner,
		seclib.NewBlessingStore(clientSigner.PublicKey()),
		seclib.NewBlessingRootsWithX509Options(opts))
	if err != nil {
		t.Fatal(err)
	}

	names := security.BlessingNames(server, blessings)
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := blessings.Expiry(), pubCerts[0].NotAfter; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ctx, cancel := context.RootContext()
	defer cancel()

	call := security.NewCall(&security.CallParams{
		LocalPrincipal:  client,
		RemoteBlessings: blessings,
		Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
	})
	names, rejected := security.RemoteBlessingNames(ctx, call)
	if len(rejected) != 0 {
		t.Errorf("rejected blessings: %v", rejected)
	}
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// After expiration, ie. now.
	call = security.NewCall(&security.CallParams{
		LocalPrincipal:  client,
		RemoteBlessings: blessings,
	})
	names, rejected = security.RemoteBlessingNames(ctx, call)
	if len(rejected) == 0 || !strings.Contains(rejected[0].Err.Error(), "is after expiry") {
		t.Errorf("incorrect rejected blessings: %v", rejected)
	}

	// Before coming into effect.
	call = security.NewCall(&security.CallParams{
		LocalPrincipal:  client,
		RemoteBlessings: blessings,
		Timestamp:       pubCerts[0].NotBefore.Add(-48 * time.Hour),
	})
	names, rejected = security.RemoteBlessingNames(ctx, call)
	if len(rejected) == 0 || !strings.Contains(rejected[0].Err.Error(), "is not before") {
		t.Errorf("incorrect rejected blessings: %v", rejected)
	}
}
