// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func TestX509(t *testing.T) {
	privKey, pubCerts, opts := sectestdata.LetsencryptData()
	signer, err := seclib.NewInMemorySigner(privKey)
	if err != nil {
		t.Fatal(err)
	}

	p, err := security.CreatePrincipal(signer, seclib.NewBlessingStore(signer.PublicKey()),
		seclib.NewBlessingRootsWuthX509Options(opts))
	if err != nil {
		t.Fatal(err)
	}
	blessings, err := p.BlessSelfX509(pubCerts[0])
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("PK: %T\n", privKey)
	fmt.Printf("#certs %v\n", len(pubCerts))
	fmt.Printf("store %v\n", p.BlessingStore().DebugString())
	fmt.Printf("blessings: %v\n", blessings.String())
	names := security.BlessingNames(p, blessings)
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	t.Fail()

}
