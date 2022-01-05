// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
)

func TestX509(t *testing.T) {
	bundle := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	pk, err := seclib.ParsePEMPrivateKeyFile(bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	certs, err := seclib.ParseX509CertificateFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(certs), 3; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	signer, err := seclib.NewInMemorySigner(pk)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seclib.NewPrincipalFromSigner(signer)
	if err != nil {
		t.Fatal(err)
	}
	blessings, err := p.BlessSelfX509(certs[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(p, blessings); err != nil {
		t.Fatal(err)
	}

	/*	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		t.Fatal(err)
	}*/
	fmt.Printf("PK: %T\n", pk)
	fmt.Printf("#certs %v\n", len(certs))
	fmt.Printf("store %v\n", p.BlessingStore().DebugString())
	fmt.Printf("blessings: %v\n", blessings.String())
	names := security.BlessingNames(p, blessings)
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	t.Fail()

}
