// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

func TestResolveSyncgroup(t *testing.T) {
	ancestor := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "1",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"a", "b", "c"},
			IsPrivate:   false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishPending,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"adminc": mkSm(100, false, 123, wire.BlobDevTypeNormal),
		},
	}

	local := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "2",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"a", "b", "d"},
			IsPrivate:   true,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusRunning,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"admind": mkSm(100, false, 124, wire.BlobDevTypeNormal),
		},
	}

	remote := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "3",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg2",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"bob"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"b", "c", "e"},
			IsPrivate:   false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishRejected,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"admine": mkSm(100, false, 125, wire.BlobDevTypeNormal),
		},
	}

	expected := interfaces.Syncgroup{
		Id: wire.Id{"name1", "blessing2"},
		Spec: wire.SyncgroupSpec{
			Description:         "mysg2",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"bob", "carol"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"b", "d", "e"},
			IsPrivate:   false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusRunning,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"admind": mkSm(100, false, 124, wire.BlobDevTypeNormal),
			"admine": mkSm(100, false, 125, wire.BlobDevTypeNormal),
		},
	}

	result, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor)
	if err != nil {
		t.Fatalf("merge failed, %v", err)
	}
	assertSyncgroupsEqual(t, expected, *result)

	expected = interfaces.Syncgroup{
		Id: wire.Id{"name1", "blessing2"},
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"bob", "carol"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"b", "d", "e"},
			IsPrivate:   true,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusRunning,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"admind": mkSm(100, false, 124, wire.BlobDevTypeNormal),
			"admine": mkSm(100, false, 125, wire.BlobDevTypeNormal),
		},
	}
	result, err = resolveSyncgroup(nil, true, &local, &remote, &ancestor)
	if err != nil {
		t.Fatalf("merge failed, %v", err)
	}
	assertSyncgroupsEqual(t, expected, *result)

	// try again with no ancestor
	expected = interfaces.Syncgroup{
		Id: wire.Id{"name1", "blessing2"},
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms: access.Permissions{
				"R": access.AccessList{In: []security.BlessingPattern{"alice", "bob"}, NotIn: []string{"bob:bad"}},
				"W": access.AccessList{In: []security.BlessingPattern{"alice", "bob", "carol"}, NotIn: []string{"bob:bad"}},
			},
			Collections: []wire.Id{wire.Id{"a", "b"}},
			MountTables: []string{"a", "b", "c", "d", "e"},
			IsPrivate:   true,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusRunning,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"admind": mkSm(100, false, 124, wire.BlobDevTypeNormal),
			"admine": mkSm(100, false, 125, wire.BlobDevTypeNormal),
		},
	}

	result, err = resolveSyncgroup(nil, true, &local, &remote, nil)
	if err != nil {
		t.Fatalf("merge failed, %v", err)
	}
	assertSyncgroupsEqual(t, expected, *result)
}

func TestResolveSyncgroupImmutablePrefix(t *testing.T) {
	ancestor := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "1",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms:               access.Permissions{},
			Collections:         []wire.Id{wire.Id{"a", "b"}, wire.Id{"b", "c"}},
			MountTables:         []string{},
			IsPrivate:           false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishPending,
		Joiners: map[string]interfaces.SyncgroupMemberState{},
	}

	local := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "1",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms:               access.Permissions{},
			Collections:         []wire.Id{wire.Id{"r", "b"}, wire.Id{"b", "c"}},
			MountTables:         []string{},
			IsPrivate:           false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishPending,
		Joiners: map[string]interfaces.SyncgroupMemberState{},
	}

	remote := interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "1",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms:               access.Permissions{},
			Collections:         []wire.Id{wire.Id{"a", "b"}, wire.Id{"b", "c"}},
			MountTables:         []string{},
			IsPrivate:           false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishPending,
		Joiners: map[string]interfaces.SyncgroupMemberState{},
	}

	if result, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor); err == nil {
		if sameCollections(ancestor.Spec.Collections, result.Spec.Collections) {
			t.Fatalf("merge should have produced an incorrect collection merge, is now %+v", result.Spec.Collections)
		}
	}

	// fix the Prefixes, merge call should succeed
	local.Spec.Collections = ancestor.Spec.Collections
	if _, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor); err != nil {
		t.Fatalf("merge failed, %+v", err)
	}

	// test the local id
	local.Id = wire.Id{"something", "else"} // change the local id to something incorrect
	if result, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor); err == nil {
		if sameCollections(ancestor.Spec.Collections, result.Spec.Collections) {
			t.Fatalf("merge should have produced an incorrect collection merge, is now %+v", result.Spec.Collections)
		}
	}
	local.Id = ancestor.Id // change it back
	if _, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor); err != nil {
		t.Fatalf("merge failed, %+v", err)
	}
}

func TestResolveSyncgroupJoinersFavorDelete(t *testing.T) {
	ancestor := mkTestSg()
	ancestor.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
	}

	local := mkTestSg()
	local.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(110, false, 122, wire.BlobDevTypeLeaf),
	}

	remote := mkTestSg()
	remote.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(105, true, 122, wire.BlobDevTypeLeaf),
	}

	expected := mkTestSg()
	expected.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(110, false, 122, wire.BlobDevTypeLeaf),
	}

	result, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor)
	if err != nil {
		t.Fatalf("merge failed, %v", err)
	}
	assertSyncgroupsEqual(t, expected, *result)
}

func TestResolveSyncgroupJoinersFavorJoin(t *testing.T) {
	ancestor := mkTestSg()
	ancestor.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
	}

	local := mkTestSg()
	local.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(110, false, 122, wire.BlobDevTypeLeaf),
	}

	remote := mkTestSg()
	remote.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(115, true, 122, wire.BlobDevTypeLeaf),
	}

	expected := mkTestSg()
	expected.Joiners = map[string]interfaces.SyncgroupMemberState{
		"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
		"adminb": mkSm(115, true, 122, wire.BlobDevTypeLeaf),
	}

	result, err := resolveSyncgroup(nil, false, &local, &remote, &ancestor)
	if err != nil {
		t.Fatalf("merge failed, %v", err)
	}
	assertSyncgroupsEqual(t, expected, *result)
}

func assertSyncgroupsEqual(t *testing.T, expected, actual interfaces.Syncgroup) {
	assert.Equal(t, expected.Id, actual.Id)
	assert.Equal(t, expected.Creator, actual.Creator)
	assert.Equal(t, expected.DbId, actual.DbId)
	assert.Equal(t, expected.Status, actual.Status)
	assert.Equal(t, expected.Joiners, actual.Joiners)
	assert.Equal(t, expected.Spec.Description, actual.Spec.Description)
	assert.Equal(t, expected.Spec.PublishSyncbaseName, actual.Spec.PublishSyncbaseName)
	assertPermsEqual(t, expected.Spec.Perms, actual.Spec.Perms)
	assert.Equal(t, expected.Spec.Collections, actual.Spec.Collections)
	sort.Strings(expected.Spec.MountTables)
	sort.Strings(actual.Spec.MountTables)
	assert.Equal(t, expected.Spec.MountTables, actual.Spec.MountTables)
	assert.Equal(t, expected.Spec.IsPrivate, actual.Spec.IsPrivate)
}

func mkSm(when int64, hasLeft bool, priority, blobType int32) interfaces.SyncgroupMemberState {
	return interfaces.SyncgroupMemberState{WhenUpdated: when, HasLeft: hasLeft, MemberInfo: wire.SyncgroupMemberInfo{byte(priority), byte(blobType)}}
}

func mkTestSg() interfaces.Syncgroup {
	return interfaces.Syncgroup{
		Id:          wire.Id{"name1", "blessing2"},
		SpecVersion: "1",
		Spec: wire.SyncgroupSpec{
			Description:         "mysg",
			PublishSyncbaseName: "mysb",
			Perms:               access.Permissions{},
			Collections:         []wire.Id{wire.Id{}},
			MountTables:         []string{},
			IsPrivate:           false,
		},
		Creator: "mysb",
		DbId:    wire.Id{"name3", "blessing4"},
		Status:  interfaces.SyncgroupStatusPublishPending,
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"admina": mkSm(100, false, 121, wire.BlobDevTypeServer),
			"adminb": mkSm(100, false, 122, wire.BlobDevTypeLeaf),
			"adminc": mkSm(100, false, 123, wire.BlobDevTypeNormal),
		},
	}
}
