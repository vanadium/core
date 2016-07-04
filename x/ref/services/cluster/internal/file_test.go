// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"v.io/v23/security"
	impl "v.io/x/ref/services/cluster/internal"
	"v.io/x/ref/test/testutil"
)

func TestFileStorage(t *testing.T) {
	workdir, err := ioutil.TempDir("", "TestFileStorage")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	fs := impl.NewFileStorage(workdir)

	p := testutil.NewPrincipal("p")

	secrets := make(map[string]security.Blessings)

	const N = 100
	for i := 0; i < N; i++ {
		name := fmt.Sprintf("blessing%d", i)
		secret := name
		b, _ := p.BlessingStore().Default()
		b, _ = p.Bless(p.PublicKey(), b, name, security.UnconstrainedUse())
		secrets[secret] = b
		if err := fs.Put(secret, b); err != nil {
			t.Errorf("unexpected Put(%v) failure: %v", secret, err)
		}
		bb, err := fs.Get(secret)
		if err != nil {
			t.Errorf("unexpected Get(%v) failure: %v", secret, err)
		}
		if !b.Equivalent(bb) {
			t.Errorf("blessing mismatch for %s. Got %v, expected %v", name, bb, b)
		}
	}

	for _, i := range rand.Perm(N) {
		secret := fmt.Sprintf("blessing%d", i)
		b := secrets[secret]
		bb, err := fs.Get(secret)
		if err != nil {
			t.Errorf("unexpected Get(%v) failure: %v", secret, err)
		}
		if !b.Equivalent(bb) {
			t.Errorf("blessing mismatch for %s. Got %v, expected %v", secret, bb, b)
		}
		if err := fs.Delete(secret); err != nil {
			t.Errorf("unexpected Delete(%v) failure: %v", secret, err)
		}
	}
}
