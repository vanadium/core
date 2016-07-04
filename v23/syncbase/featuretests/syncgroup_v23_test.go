// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains feature tests that test the life cycle of a syncgroup
// under various application, topology and network scenarios.

package featuretests_test

import (
	"fmt"
	"testing"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/test/v23test"
)

// TestV23SyncgroupRendezvousOnline tests that Syncbases can join a syncgroup
// when: all Syncbases are online and a creator creates the syncgroup and shares
// the syncgroup name with all the joiners.
func TestV23SyncgroupRendezvousOnline(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	N := 5
	// Setup N Syncbases.
	sbs := setupSyncbases(t, sh, N, false)

	// Syncbase s0 is the creator.
	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs)))

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sbName, sgId, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, keypfx, 0, 5))

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgId, N))
	}

	fmt.Println("TestV23SyncgroupRendezvousOnline=====Phase 1 Done")
}

// TestV23SyncgroupRendezvousOnlineCloud tests that Syncbases can join a
// syncgroup when: all Syncbases are online and a creator creates the syncgroup
// and nominates a cloud syncbase for the other joiners to join at.
func TestV23SyncgroupRendezvousOnlineCloud(t *testing.T) {
	// TODO(hpucha): There is a potential bug that is currently preventing
	// this test from succeeding.
	t.Skip()

	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	N := 5
	// Setup N+1 Syncbases (1 for the cloud instance).
	sbs := setupSyncbases(t, sh, N+1, false)

	// Syncbase s0 is the creator, and sN is the cloud.
	sbName := sbs[N].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "", nil, clBlessings(sbs)))

	// Remaining N-1 syncbases run the specified workload concurrently.
	for i := 1; i < N; i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sbName, sgId, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, keypfx, 0, 5))

	// Verify steady state sequentially.
	for i := 0; i < N; i++ {
		ok(t, verifySync(sbs[i].clientCtx, sbs[i].sbName, N, "foo"))
		// ok(t, verifySyncgroupMembers(sbs[i].clientCtx, sbs[i].sbName, sgId, N+1))
	}

	fmt.Println("TestV23SyncgroupRendezvousOnlineCloud=====Phase 1 Done")
}

// TestV23SyncgroupNeighborhoodOnly tests that Syncbases can join a syncgroup
// when: all Syncbases do not have general connectivity but can reach each other
// over neighborhood, and a creator creates the syncgroup and shares the
// syncgroup name with all the joiners. Restricted connectivity is simulated by
// picking a syncgroup name that is not reachable and a syncgroup mount table
// that doesn't exist.
func TestV23SyncgroupNeighborhoodOnly(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	N := 5

	// Setup N Syncbases.
	sbs := setupSyncbases(t, sh, N, false)

	// Syncbase s0 is the creator, but the syncgroup refers to non-existent
	// Syncbase "s6".
	sbName := "s6"
	sgId := wire.Id{Name: "SG1", Blessing: "root"}

	// For now, we set the permissions to have a single admin. Once we have
	// syncgroup conflict resolution in place, we should be able to have
	// multiple admins on the neighborhood.
	//
	// TODO(hpucha): Change it to multi-admin scenario.
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, testCx.Name, "/mttable", nil, clBlessings(sbs)))

	// Remaining syncbases run the specified workload concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, runSyncWorkload(sbs[i].clientCtx, sbs[i].sbName, sbName, sgId, "foo"))
		}(i)
	}

	// Populate data on creator as well.
	keypfx := "foo==" + sbs[0].sbName + "=="
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, keypfx, 0, 5))

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgId, N))
	}

	fmt.Println("TestV23SyncgroupNeighborhoodOnly=====Phase 1 Done")
}

// TestV23SyncgroupPreknownStaggered tests that Syncbases can join a syncgroup
// when: all Syncbases come online in a staggered fashion. Each Syncbase always
// tries to join a syncgroup with a predetermined name, and if join fails,
// creates the syncgroup.
func TestV23SyncgroupPreknownStaggered(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	N := 5
	// Setup N Syncbases.
	sbs := setupSyncbases(t, sh, N, false)

	// Syncbase s0 is the first to join or create. Run s0 separately to
	// stagger the process.
	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}
	ok(t, joinOrCreateSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sbName, sgId, testCx.Name, "", clBlessings(sbs)))

	// Remaining syncbases join the syncgroup concurrently.
	for i := 1; i < len(sbs); i++ {
		go func(i int) {
			ok(t, joinOrCreateSyncgroup(sbs[i].clientCtx, sbs[i].sbName, sbName, sgId, testCx.Name, "", clBlessings(sbs)))
		}(i)
	}

	// Populate data concurrently.
	for _, sb := range sbs {
		go func(sb *testSyncbase) {
			ok(t, populateDataWithRetry(sb.clientCtx, sb.sbName, "foo"))
		}(sb)
	}

	// Verify steady state sequentially.
	for _, sb := range sbs {
		ok(t, verifySync(sb.clientCtx, sb.sbName, N, "foo"))
		ok(t, verifySyncgroupMembers(sb.clientCtx, sb.sbName, sgId, N))
	}

	fmt.Println("TestV23SyncgroupPreknownStaggered=====Phase 1 Done")
}

////////////////////////////////////////
// Helpers.

func runSyncWorkload(ctx *context.T, syncbaseName, sbName string, sgId wire.Id, prefix string) error {
	var err error
	for i := 0; i < 8; i++ {
		time.Sleep(500 * time.Millisecond)
		if err = joinSyncgroup(ctx, syncbaseName, sbName, sgId); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}

	// Populate some data without colliding with data from other Syncbases.
	return populateDataWithRetry(ctx, syncbaseName, prefix)
}

func populateDataWithRetry(ctx *context.T, syncbaseName, prefix string) error {
	var err error
	keypfx := prefix + "==" + syncbaseName + "=="
	for i := 0; i < 8; i++ {
		// Wait for the presence of the collection and its ACL to be
		// synced.
		time.Sleep(500 * time.Millisecond)
		if err = populateData(ctx, syncbaseName, testCx.Name, keypfx, 0, 5); err == nil {
			break
		}
	}
	return err
}

func verifySync(ctx *context.T, syncbaseName string, numSyncbases int, prefix string) error {
	for i := numSyncbases - 1; i >= 0; i-- {
		keypfx := fmt.Sprintf("%s==s%d==", prefix, i)
		if err := verifySyncgroupData(ctx, syncbaseName, testCx.Name, keypfx, "", 0, 5); err != nil {
			return err
		}
	}
	return nil
}

func joinOrCreateSyncgroup(ctx *context.T, sbNameLocal, sbNameRemote string, sgId wire.Id, sgColls, mtName, clbps string) error {
	if err := joinSyncgroup(ctx, sbNameLocal, sbNameRemote, sgId); err == nil {
		return nil
	}
	return createSyncgroup(ctx, sbNameLocal, sgId, sgColls, mtName, nil, clbps)
}
