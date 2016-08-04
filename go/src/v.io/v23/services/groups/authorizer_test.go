// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Tests copied from v23/security/access.
// TODO(hpucha): Nuke these when this package is merged into v23/security/acess.

package groups

import (
	"testing"

	"v.io/v23/security"
	"v.io/v23/security/access"
)

func TestInclude(t *testing.T) {
	acl := access.AccessList{
		In:    []security.BlessingPattern{"alice:$", "alice:friend", "bob:family"},
		NotIn: []string{"alice:friend:carol", "bob:family:mallory"},
	}
	type V []string // shorthand
	tests := []struct {
		Blessings []string
		Want      bool
	}{
		{nil, false}, // No blessings presented, cannot access
		{V{}, false},
		{V{"alice"}, true},
		{V{"bob"}, false},
		{V{"carol"}, false},
		{V{"alice:colleague"}, false},
		{V{"alice", "carol:friend"}, true}, // Presenting one blessing that grants access is sufficient
		{V{"alice:friend:bob"}, true},
		{V{"alice:friend:carol"}, false},        // alice:friend:carol is blacklisted
		{V{"alice:friend:carol:family"}, false}, // alice:friend:carol is blacklisted, thus her delegates must be too.
		{V{"alice:friend:bob", "alice:friend:carol"}, true},
		{V{"bob:family:eve", "bob:family:mallory"}, true},
		{V{"bob:family:mallory", "alice:friend:carol"}, false},
	}
	for _, test := range tests {
		if got, want := includes(nil, acl, convertToSet(test.Blessings...)), test.Want; got != want {
			t.Errorf("Includes(%v): Got %v, want %v", test.Blessings, got, want)
		}
	}
}

func TestOpenAccessList(t *testing.T) {
	acl := access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}}
	if !includes(nil, acl, nil) {
		t.Errorf("OpenAccessList should allow principals that present no blessings")
	}
	if !includes(nil, acl, convertToSet("frank")) {
		t.Errorf("OpenAccessList should allow principals that present any blessings")
	}
}
