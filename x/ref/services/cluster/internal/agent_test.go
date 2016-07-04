// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23/security"
	impl "v.io/x/ref/services/cluster/internal"
	"v.io/x/ref/test/testutil"
)

func TestClusterAgent(t *testing.T) {
	workdir, err := ioutil.TempDir("", "TestClusterAgent")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)

	agentP := testutil.NewPrincipal("agent")
	serviceP := testutil.NewPrincipal("service")
	instanceP := testutil.NewPrincipal("instance")
	serviceB, _ := serviceP.BlessingStore().Default()

	agent := impl.NewAgent(agentP, impl.NewFileStorage(workdir))

	blessings1, err := serviceP.Bless(agentP.PublicKey(), serviceB, "foo", security.UnconstrainedUse())
	if err != nil {
		t.Fatalf("serviceP.Bless failed: %v", err)
	}

	secret, err := agent.NewSecret(blessings1)
	if err != nil {
		t.Fatalf("agent.NewSecret failed: %v", err)
	}

	blessings2, err := agent.Bless(secret, instanceP.PublicKey(), "bar")
	if err != nil {
		t.Fatalf("agent.Bless failed: %v", err)
	}
	if blessings2.PublicKey() != instanceP.PublicKey() {
		t.Errorf("unexpected PublicKey mismatch. Got %v, expected %v", blessings2.PublicKey(), instanceP.PublicKey())
	}
	if expected := "service:foo:bar"; blessings2.String() != expected {
		t.Errorf("unexpected blessings. Got %q, expected %q", blessings2, expected)
	}

	if err := agent.ForgetSecret(secret); err != nil {
		t.Errorf("agent.ForgetSecret failed: %v", err)
	}
}
