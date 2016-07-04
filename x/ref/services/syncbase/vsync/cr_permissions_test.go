// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"sort"
	"testing"
	"v.io/v23/security"
	"v.io/v23/security/access"
)

func TestResolvePermissions(t *testing.T) {
	ancestor := access.Permissions{}
	left := access.Permissions{}
	right := access.Permissions{}
	expected := access.Permissions{}

	ancestor["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	ancestor["W"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}

	left["A"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}}

	right["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "don"}, NotIn: []string{"bob:bad"}}
	right["W"] = access.AccessList{In: []security.BlessingPattern{"eric"}, NotIn: []string{"bob:bad"}}
	right["Z"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}

	expected["A"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	expected["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol", "don"}, NotIn: []string{"bob:bad"}}
	expected["W"] = access.AccessList{In: []security.BlessingPattern{"eric"}, NotIn: nil}
	expected["Z"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}

	result := resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)
}

func TestResolvePermissionsRemovals(t *testing.T) {
	ancestor := access.Permissions{}
	left := access.Permissions{}
	right := access.Permissions{}
	expected := access.Permissions{}

	ancestor["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol", "don"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}}
	right["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "don"}, NotIn: []string{"bob:bad"}}
	expected["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	result := resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)

	ancestor["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}}
	right["R"] = access.AccessList{In: []security.BlessingPattern{"bob"}, NotIn: []string{"bob:bad"}}
	expected["R"] = access.AccessList{In: []security.BlessingPattern{"bob", "carol"}, NotIn: []string{"bob:bad"}}
	result = resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)
}

func TestResolvePermissionsDroppedTags(t *testing.T) {
	ancestor := access.Permissions{}
	left := access.Permissions{}
	right := access.Permissions{}
	expected := access.Permissions{}

	ancestor["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol", "don"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	right["R"] = access.AccessList{In: []security.BlessingPattern{"carol", "don"}, NotIn: []string{"bob:bad"}}
	expected["R"] = access.AccessList{In: nil, NotIn: []string{"bob:bad"}}
	result := resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)

	ancestor["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol", "don"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: nil}
	right["R"] = access.AccessList{In: []security.BlessingPattern{"carol", "don"}, NotIn: nil}
	expected = access.Permissions{}
	result = resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)
}

func TestResolvePermissionsNoAncestor(t *testing.T) {
	var ancestor access.Permissions
	left := access.Permissions{}
	right := access.Permissions{}
	expected := access.Permissions{}

	left["A"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	left["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}}
	left["W"] = access.AccessList{In: []security.BlessingPattern{"bob"}, NotIn: []string{"bob:bad"}}

	right["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "don"}, NotIn: []string{"bob:bad"}}
	right["W"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "eric"}, NotIn: []string{"bob:bad"}}
	right["Z"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}

	expected["A"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}
	expected["R"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol", "don"}, NotIn: []string{"bob:bad"}}
	expected["W"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob", "eric"}, NotIn: []string{"bob:bad"}}
	expected["Z"] = access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}}

	result := resolvePermissions(left, right, ancestor)
	assertPermsEqual(t, expected, result)
}

func TestResolveSlice(t *testing.T) {
	ancestor := []string{"a", "b"}
	left := []string{"b"}
	right := []string{"a", "b", "e"}
	expected := []string{"b", "e"}
	result := resolveSlice(left, right, ancestor)
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(t, expected, result)

	ancestor = []string{"a", "b", "c"}
	left = []string{"a", "b", "d"}
	right = []string{"b", "c", "e"}
	expected = []string{"b", "d", "e"}
	result = resolveSlice(left, right, ancestor)
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(t, expected, result)

	// Empty left.
	ancestor = []string{"a", "b"}
	left = nil
	right = []string{"b", "c", "e"}
	expected = []string{"c", "e"}
	result = resolveSlice(left, right, ancestor)
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(t, expected, result)

	// Empty right.
	ancestor = []string{"a", "b"}
	left = []string{"b", "c", "e"}
	right = nil
	expected = []string{"c", "e"}
	result = resolveSlice(left, right, ancestor)
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(t, expected, result)

	// Empty ancestor.
	ancestor = nil
	left = []string{"b", "c", "e"}
	right = []string{"a", "b"}
	expected = []string{"a", "b", "c", "e"}
	result = resolveSlice(left, right, ancestor)
	sort.Strings(expected)
	sort.Strings(result)
	assert.Equal(t, expected, result)
}

func assertPermsEqual(t *testing.T, expected, actual access.Permissions) {
	assert.True(t, reflect.DeepEqual(expected, actual))
}
