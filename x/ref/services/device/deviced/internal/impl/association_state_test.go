// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl_test

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"v.io/v23/services/device"

	"v.io/x/ref/services/device/deviced/internal/impl"
	"v.io/x/ref/services/device/deviced/internal/impl/utiltest"
)

// TestAssociationPersistence verifies correct operation of association
// persistence code.
func TestAssociationPersistence(t *testing.T) {
	td, err := ioutil.TempDir("", "device_test")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	defer os.RemoveAll(td)
	nbsa1, err := impl.NewBlessingSystemAssociationStore(td)
	if err != nil {
		t.Fatalf("NewBlessingSystemAssociationStore failed: %v", err)
	}

	// Insert an association.
	err = nbsa1.AssociateSystemAccountForBlessings([]string{"alice", "bob"}, "alice_account")
	if err != nil {
		t.Fatalf("AssociateSystemAccount failed: %v", err)
	}

	got1, err := nbsa1.AllBlessingSystemAssociations()
	if err != nil {
		t.Fatalf("AllBlessingSystemAssociations failed: %v", err)
	}

	utiltest.CompareAssociations(t, got1, []device.Association{
		{
			"alice",
			"alice_account",
		},
		{
			"bob",
			"alice_account",
		},
	})

	nbsa2, err := impl.NewBlessingSystemAssociationStore(td)
	if err != nil {
		t.Fatalf("NewBlessingSystemAssociationStore failed: %v", err)
	}

	got2, err := nbsa2.AllBlessingSystemAssociations()
	if err != nil {
		t.Fatalf("AllBlessingSystemAssociations failed: %v", err)
	}
	utiltest.CompareAssociations(t, got1, got2)

	sysacc, have := nbsa2.SystemAccountForBlessings([]string{"bob"})
	if expected := true; have != expected {
		t.Fatalf("SystemAccountForBlessings failed. got %v, expected %v", have, expected)
	}
	if expected := "alice_account"; sysacc != expected {
		t.Fatalf("SystemAccountForBlessings failed. got %v, expected %v", sysacc, expected)
	}

	sysacc, have = nbsa2.SystemAccountForBlessings([]string{"doug"})
	if expected := false; have != expected {
		t.Fatalf("SystemAccountForBlessings failed. got %v, expected %v", have, expected)
	}
	if expected := ""; sysacc != expected {
		t.Fatalf("SystemAccountForBlessings failed. got %v, expected %v", sysacc, expected)
	}

	// Remove "bob".
	err = nbsa1.DisassociateSystemAccountForBlessings([]string{"bob"})
	if err != nil {
		t.Fatalf("DisassociateSystemAccountForBlessings failed: %v", err)
	}

	// Verify that "bob" has been removed.
	got1, err = nbsa1.AllBlessingSystemAssociations()
	if err != nil {
		t.Fatalf("AllBlessingSystemAssociations failed: %v", err)
	}
	utiltest.CompareAssociations(t, got1, []device.Association{
		{
			"alice",
			"alice_account",
		},
	})

	err = nbsa1.AssociateSystemAccountForBlessings([]string{"alice", "bob"}, "alice_other_account")
	if err != nil {
		t.Fatalf("AssociateSystemAccount failed: %v", err)
	}
	// Verify that "bob" and "alice" have new values.
	got1, err = nbsa1.AllBlessingSystemAssociations()
	if err != nil {
		t.Fatalf("AllBlessingSystemAssociations failed: %v", err)
	}
	utiltest.CompareAssociations(t, got1, []device.Association{
		{
			"alice",
			"alice_other_account",
		},
		{
			"bob",
			"alice_other_account",
		},
	})

	// Make future serialization attempts fail.
	if err := os.RemoveAll(td); err != nil {
		t.Fatalf("os.RemoveAll: couldn't delete %s: %v", td, err)
	}
	err = nbsa1.AssociateSystemAccountForBlessings([]string{"doug"}, "alice_account")
	if err == nil {
		t.Fatalf("AssociateSystemAccount should have failed but didn't")
	}
}

func TestAssociationPersistenceDetectsBadStartingConditions(t *testing.T) {
	dir := "/i-am-hoping-that-there-is-no-such-directory"
	nbsa1, err := impl.NewBlessingSystemAssociationStore(dir)
	if nbsa1 != nil || err == nil {
		t.Fatalf("bad root directory %s ought to have caused an error", dir)
	}

	// Create a NewBlessingSystemAssociationStore directory as a side-effect.
	dir, err = ioutil.TempDir("", "bad-starting-conditions")
	if err != nil {
		t.Fatalf("TempDir failed: %v", err)
	}
	nbsa1, err = impl.NewBlessingSystemAssociationStore(dir)
	defer os.RemoveAll(dir)
	if err != nil {
		t.Fatalf("NewBlessingSystemAssociationStore failed: %v", err)
	}

	tpath := path.Join(dir, "device-manager", "device-data", "associated.accounts")
	f, err := os.Create(tpath)
	if err != nil {
		t.Fatalf("could not open backing file for setup: %v", err)
	}

	if _, err := io.WriteString(f, "bad-json\""); err != nil {
		t.Fatalf("could not write to test file  %s: %v", tpath, err)
	}
	f.Close()

	nbsa1, err = impl.NewBlessingSystemAssociationStore(dir)
	if nbsa1 != nil || err == nil {
		t.Fatalf("invalid JSON ought to have caused an error")
	}

	// This test will fail if executed as root or if your system is configured oddly.
	unreadableFile := "/dev/autofs"
	nbsa1, err = impl.NewBlessingSystemAssociationStore(unreadableFile)
	if nbsa1 != nil || err == nil {
		t.Fatalf("unreadable file %s ought to have caused an error", unreadableFile)
	}
}
