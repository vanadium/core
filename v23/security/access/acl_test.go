// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package access

import (
	"bytes"
	"reflect"
	"testing"

	"v.io/v23/security"
)

func TestInclude(t *testing.T) {
	acl := AccessList{
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
		if got, want := acl.Includes(test.Blessings...), test.Want; got != want {
			t.Errorf("Includes(%v): Got %v, want %v", test.Blessings, got, want)
		}
	}
}

func TestOpenAccessList(t *testing.T) {
	acl := AccessList{In: []security.BlessingPattern{security.AllPrincipals}}
	if !acl.Includes() {
		t.Errorf("OpenAccessList should allow principals that present no blessings")
	}
	if !acl.Includes("frank") {
		t.Errorf("OpenAccessList should allow principals that present any blessings")
	}
}

func TestPermissionsSerialization(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In:    []security.BlessingPattern{"foo", "bar"},
			NotIn: []string{"bar:baz"},
		},
		"W": AccessList{
			In:    []security.BlessingPattern{"foo", "bar:$"},
			NotIn: []string{"foo:bar", "foo:baz:boz"},
		},
	}
	txt := `
{
	"R": {
		"In":["foo","bar"],
		"NotIn":["bar:baz"]
	},
	"W": {
		"In":["foo","bar:$"],
		"NotIn":["foo:bar","foo:baz:boz"]
	}
}
`
	if got, err := ReadPermissions(bytes.NewBufferString(txt)); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, Permissions: %v, want %v", err, got, obj)
	}
	// And round-trip (don't compare with 'txt' because indentation/spacing might differ).
	var buf bytes.Buffer
	if err := WritePermissions(&buf, obj); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadPermissions(&buf); err != nil || !reflect.DeepEqual(got, obj) {
		t.Errorf("Got error %v, Permissions: %v, want %v", err, got, obj)
	}
}

func TestPermissionsAdd(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In: []security.BlessingPattern{"foo"},
		},
	}
	want := obj.Copy()
	// Adding "foo" to the Permissions again should not result in duplicate entries.
	if obj.Add("foo", "R"); !reflect.DeepEqual(obj, want) {
		t.Errorf("got %#v, want %#v", obj, want)
	}
	// Adding "bar" to the Permissions should succeed.
	want = Permissions{
		"R": AccessList{
			In: []security.BlessingPattern{"bar", "foo"},
		},
		"W": AccessList{
			In: []security.BlessingPattern{"bar"},
		},
	}
	if obj.Add("bar", "R", "W"); !reflect.DeepEqual(obj, want) {
		t.Errorf("got %#v, want %#v", obj, want)
	}
}

func TestPermissionsBlacklist(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			NotIn: []string{"foo"},
		},
	}
	want := obj.Copy()
	// Blacklisting "foo" in the Permissions again should not result in duplicate entries.
	if obj.Blacklist("foo", "R"); !reflect.DeepEqual(obj, want) {
		t.Errorf("got %#v, want %#v", obj, want)
	}
	// Blacklisting "bar" to the Permissions should succeed.
	want = Permissions{
		"R": AccessList{
			NotIn: []string{"bar", "foo"},
		},
		"W": AccessList{
			NotIn: []string{"bar"},
		},
	}
	if obj.Blacklist("bar", "R", "W"); !reflect.DeepEqual(obj, want) {
		t.Errorf("got %#v, want %#v", obj, want)
	}
}

func TestPermissionsClear(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In: []security.BlessingPattern{"foo", "bar"},
		},
		"W": AccessList{
			NotIn: []string{"foo", "bar"},
		},
		"A": AccessList{
			In:    []security.BlessingPattern{"foo", "bar"},
			NotIn: []string{"foo", "bar"},
		},
		"ThisShouldStay": AccessList{
			In: []security.BlessingPattern{"foo", "bar"},
		},
	}
	want := Permissions{
		"R": AccessList{
			In: []security.BlessingPattern{"bar"},
		},
		"W": AccessList{
			NotIn: []string{"bar"},
		},
		"A": AccessList{
			In:    []security.BlessingPattern{"bar"},
			NotIn: []string{"bar"},
		},
		"ThisShouldStay": AccessList{
			In: []security.BlessingPattern{"foo", "bar"},
		},
	}
	if obj.Clear("foo", "R", "W", "A"); !reflect.DeepEqual(obj, want) {
		t.Errorf("got %#v, want %#v", obj, want)
	}
}

func TestPermissionsCopy(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In: []security.BlessingPattern{"foo"},
		},
	}
	// obj.Copy should equal obj.
	cpy := obj.Copy()
	if !reflect.DeepEqual(cpy, obj) {
		t.Errorf("obj.Copy = %#v, want %#v", cpy, obj)
	}
	// Changes to obj.Copy should not change obj.
	if cpy.Add("bar", "R"); reflect.DeepEqual(cpy, obj) {
		t.Errorf("obj.Copy should not equal obj, %#v", obj)
	}
}

func TestPermissionsNormalize(t *testing.T) {
	obj := Permissions{
		"R": AccessList{
			In:    []security.BlessingPattern{"foo", "bar"},
			NotIn: []string{"foo", "bar"},
		},
		"W": AccessList{
			In:    []security.BlessingPattern{"foo", "foo", "bar"},
			NotIn: []string{"bar", "bar", "foo"},
		},
	}
	same := Permissions{
		"R": AccessList{
			In:    []security.BlessingPattern{"foo", "bar"},
			NotIn: []string{"foo", "bar"},
		},
		"W": AccessList{
			In:    []security.BlessingPattern{"bar", "bar", "foo"},
			NotIn: []string{"bar", "foo", "foo"},
		},
	}
	// Sanity check that these two Permissions pre-Normalize are not equal.
	if reflect.DeepEqual(obj, same) {
		t.Errorf("obj should not equal same, %#v", obj)
	}
	// After Normalize they should equal each other.
	obj, same = obj.Normalize(), same.Normalize()
	if !reflect.DeepEqual(obj, same) {
		t.Errorf("obj should equal same, got %#v, want %#v", obj, same)
	}
}
