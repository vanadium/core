// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23/security"
	"v.io/v23/security/access"
)

func TestPersistence(t *testing.T) {
	rootCtx, _, _, shutdown := initTest()
	defer shutdown()

	td, err := ioutil.TempDir("", "upyournose")
	if err != nil {
		t.Fatalf("Failed to make temporary dir: %s", err)
	}
	defer os.RemoveAll(td)
	fmt.Printf("temp persist dir %s\n", td)
	stop, mtAddr, _ := newMT(t, "", td, "testPersistence", rootCtx)

	perms1 := access.Permissions{
		"Read":    access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Resolve": access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Create":  access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
	}
	perms2 := access.Permissions{
		"Read":    access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Resolve": access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Create":  access.AccessList{In: []security.BlessingPattern{"root"}},
	}
	perms3 := access.Permissions{
		"Read":    access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Resolve": access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}},
		"Create":  access.AccessList{In: []security.BlessingPattern{"bob"}},
	}

	// Set some permissions.  It should be persisted.
	doSetPermissions(t, rootCtx, mtAddr, "a", perms1, "", true)
	doSetPermissions(t, rootCtx, mtAddr, "a/b", perms2, "", true)
	doSetPermissions(t, rootCtx, mtAddr, "a/b/c/d", perms2, "", true)
	doSetPermissions(t, rootCtx, mtAddr, "a/b/c/d/e", perms2, "", true)
	doDeleteSubtree(t, rootCtx, mtAddr, "a/b/c", true)
	doSetPermissions(t, rootCtx, mtAddr, "a/c/d", perms3, "", true)
	stop()

	// Restart with the persisted data.
	stop, mtAddr, _ = newMT(t, "", td, "testPersistence", rootCtx)

	// Add root as Admin to each of the perms since the mounttable itself will.
	perms1["Admin"] = access.AccessList{In: []security.BlessingPattern{"root"}}
	perms2["Admin"] = access.AccessList{In: []security.BlessingPattern{"root"}}
	perms3["Admin"] = access.AccessList{In: []security.BlessingPattern{"root"}}

	// Check deletion.
	checkExists(t, rootCtx, mtAddr, "a/b", true)
	checkExists(t, rootCtx, mtAddr, "a/b/c", false)

	// Check Persistence.
	if perm, _ := doGetPermissions(t, rootCtx, mtAddr, "a", true); !reflect.DeepEqual(perm, perms1) {
		t.Fatalf("a: got %v, want %v", perm, perms1)
	}
	if perm, _ := doGetPermissions(t, rootCtx, mtAddr, "a/b", true); !reflect.DeepEqual(perm, perms2) {
		t.Fatalf("a/b: got %v, want %v", perm, perms2)
	}
	if perm, _ := doGetPermissions(t, rootCtx, mtAddr, "a/c/d", true); !reflect.DeepEqual(perm, perms3) {
		t.Fatalf("a/c/d: got %v, want %v", perm, perms3)
	}
	stop()
}
