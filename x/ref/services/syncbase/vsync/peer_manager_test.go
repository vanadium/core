// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"v.io/v23/discovery"
	wire "v.io/v23/services/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

func TestPeerManager(t *testing.T) {
	// Set a large value to prevent the syncer and the peer manager from
	// running. Since this test adds fake syncgroup members, if they run,
	// they will attempt to use this fake and partial member data.
	peerSyncInterval = 1 * time.Hour
	peerManagementInterval = 1 * time.Hour
	pingFanout = 5

	svc := createService(t)
	defer destroyService(t, svc)
	s := svc.sync

	// No peers exist.
	if _, err := s.pm.pickPeer(nil); err == nil {
		t.Fatalf("pickPeer didn't fail")
	}

	// Add one joiner.
	now, err := s.vclock.Now()
	if err != nil {
		t.Fatalf("unable to get time: %v\n", err)
	}
	nullInfo := wire.SyncgroupMemberInfo{}
	sg := &interfaces.Syncgroup{
		Id:          wire.Id{Name: "sg", Blessing: "blessing"},
		DbId:        mockDbId,
		Creator:     "mockCreator",
		SpecVersion: "etag-0",
		Spec: wire.SyncgroupSpec{
			Collections: []wire.Id{makeCxId("foo"), makeCxId("bar")},
			Perms:       mockSgPerms,
			MountTables: []string{"1/2/3/4", "5/6/7/8"},
		},
		Joiners: map[string]interfaces.SyncgroupMemberState{
			"a": interfaces.SyncgroupMemberState{WhenUpdated: now.Unix(), MemberInfo: nullInfo},
		},
	}

	tx := createDatabase(t, svc).St().NewWatchableTransaction()
	if err = s.addSyncgroup(nil, tx, NoVersion, true, "", nil, s.id, 1, 1, sg); err != nil {
		t.Fatalf("cannot add syncgroup %v, err %v", sg.Id, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("cannot commit adding syncgroup %v, err %v", sg.Id, err)
	}

	// Add a few peers to simulate neighborhood.
	s.updateDiscoveryInfo("a", &discovery.Advertisement{
		Attributes: discovery.Attributes{wire.DiscoveryAttrPeer: "a"},
		Addresses:  []string{"aa", "aaa"},
	})
	s.updateDiscoveryInfo("b", &discovery.Advertisement{
		Attributes: discovery.Attributes{wire.DiscoveryAttrPeer: "b"},
		Addresses:  []string{"bb", "bbb"},
	})

	s.allMembers = nil // force a refresh of the members.

	// connInfo when peer is reachable via mount table.
	peersViaMt := []*connInfo{&connInfo{relName: "a", mtTbls: sg.Spec.MountTables}}

	// connInfo when peer is reachable via neighborhood.
	peersViaNh := []*connInfo{&connInfo{relName: "a", addrs: []string{"aa", "aaa"}}}

	pm := s.pm.(*peerManagerImpl)

	// Test peer selection when there are no failures. It should always
	// return peers whose connInfo indicates reachability via mount table.
	for i := 0; i < 2; i++ {
		checkPickPeers(t, pm, peersViaMt, true)
	}

	pm.curCount = 2

	// Test peer selection when there are failures. It should always
	// return peers whose connInfo indicates reachability via neighborhood.
	for i := 0; i < 2; i++ {
		checkPickPeers(t, pm, peersViaNh, false)
	}

	// curCount is zero, try mount table again.
	checkPickPeers(t, pm, peersViaMt, true)

	// Simulate successful mount table reachability. This doesn't impact the
	// peer selection because managePeers is blocked from running in this
	// test and managePeersInternal() has not yet been called.
	for i := 0; i < 2; i++ {
		curTime := time.Now()
		if err := s.pm.updatePeerFromSyncer(nil, *peersViaMt[0], curTime, false); err != nil {
			t.Fatalf("updatePeerFromSyncer failed, err %v", err)
		}

		want := &peerSyncInfo{attemptTs: curTime}
		got := pm.peerTbl[peersViaMt[0].relName]

		checkEqualPeerSyncInfo(t, got, want)
		checkPickPeers(t, pm, peersViaMt, true)
	}

	// Simulate that peer is not reachable via mount table. This doesn't
	// impact the peer selection because managePeers is blocked from running
	// in this test and managePeersInternal() has not yet been called.
	for i := 1; i <= 10; i++ {
		curTime := time.Now()
		if err := s.pm.updatePeerFromSyncer(nil, *peersViaMt[0], curTime, true); err != nil {
			t.Fatalf("updatePeerFromSyncer failed, err %v", err)
		}

		want := &peerSyncInfo{numFailuresMountTable: uint64(i), attemptTs: curTime}
		got := pm.peerTbl[peersViaMt[0].relName]

		checkEqualPeerSyncInfo(t, got, want)
		checkPickPeers(t, pm, peersViaMt, true)
	}

	var failures uint64 = 1
	for i := 0; i < 7; i++ {
		checkPickPeers(t, pm, peersViaMt, true)

		// This causes ping failures since this is a fake setup and
		// selection flips to neighborhood mode.
		pm.managePeersInternal(s.ctx)

		for j := roundsToBackoff(failures); j > 0; j-- {
			if pm.numFailuresMountTable != failures || pm.curCount != j {
				t.Fatalf("managePeersInternal failed, got %v %v, want %v %v", pm.numFailuresMountTable, pm.curCount, failures, j)
			}
			checkPickPeers(t, pm, peersViaNh, false)
		}

		if pm.numFailuresMountTable != failures || pm.curCount != 0 {
			t.Fatalf("managePeersInternal failed, got %v %v, want %v 0", pm.numFailuresMountTable, pm.curCount, failures)
		}

		failures++
	}
}

func checkEqualPeerSyncInfo(t *testing.T, got, want *peerSyncInfo) {
	if got.numFailuresNeighborhood != want.numFailuresNeighborhood ||
		got.numFailuresMountTable != want.numFailuresMountTable ||
		got.attemptTs != want.attemptTs {
		t.Fatalf("checkEqualPeerSyncInfo failed, got %v, want %v", got, want)
	}
}

func checkPickPeers(t *testing.T, pm *peerManagerImpl, want []*connInfo, wantViaMtTbl bool) {
	got, gotViaMtTbl := pm.pickPeersToPingRandom(nil)

	if len(got) != len(want) {
		t.Fatalf("pickPeersToPingRandom failed, got %v, want %v", got, want)
	}

	for _, p := range got {
		sort.Strings(p.mtTbls)
	}

	if gotViaMtTbl != wantViaMtTbl || !reflect.DeepEqual(got, want) {
		t.Fatalf("pickPeersToPingRandom failed, got %v gotViaMtTbl %v, want %v wantViaMtTbl %v", got, gotViaMtTbl, want, wantViaMtTbl)
	}
}
