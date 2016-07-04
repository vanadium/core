// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/syncbaselib"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23test"
)

// NOTE(sadovsky): These tests take a very long time to run - nearly 4 minutes
// on my Macbook Pro! Various instances of time.Sleep() below likely contribute
// to the problem.

// TestV23VSyncCompEval is a comprehensive sniff test for core sync
// functionality. It tests the exchange of data and syncgroup deltas between two
// Syncbase instances and their clients. The 1st client creates a syncgroup and
// puts some database entries in it. The 2nd client joins that syncgroup and
// reads the database entries. Syncbases are restarted. The 1st client then
// updates syncgroup metadata and the 2nd client obtains the metadata. The 2nd
// client then updates a subset of existing keys, adds more keys, and updates
// syncgroup metadata and the 1st client verifies that it can read these
// updates. This verifies the end-to-end bi-directional synchronization of data
// and syncgroup metadata along the path:
// client0--Syncbase0--Syncbase1--client1. After this phase is done, both
// Syncbase instances are shutdown and restarted, and new data is synced once
// again.
func TestV23VSyncCompEval(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 2 Syncbases.
	sbs := setupSyncbases(t, sh, 2, false)

	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, "c", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 0, 10))

	// This is a decoy syncgroup that no other Syncbase joins, but is on the
	// same database as the first syncgroup. Populating it after the first
	// syncgroup causes the database generations to go up, but the joiners
	// on the first syncgroup do not get any data belonging to this
	// syncgroup. This triggers the handling of filtered log records in the
	// restartability code.
	sgId1 := wire.Id{Name: "SG2", Blessing: testCx.Blessing}

	// Verify data syncing (client0 updates).
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId1, "c1", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c1", "foo", 0, 10))

	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbName, sgId))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 10))

	// Shutdown and restart Syncbase instances.
	sbs[0].cleanup(os.Interrupt)
	sbs[1].cleanup(os.Interrupt)

	acl := func(clientId string) string {
		return fmt.Sprintf(`{"Resolve":{"In":["root:%s"]},"Read":{"In":["root:%s"]},"Write":{"In":["root:%s"]},"Admin":{"In":["root:%s"]}}`, clientId, clientId, clientId, clientId)
	}

	sbs[0].cleanup = sh.StartSyncbase(sbs[0].sbCreds, syncbaselib.Opts{Name: sbs[0].sbName, RootDir: sbs[0].rootDir}, acl(sbs[0].clientId))
	sbs[1].cleanup = sh.StartSyncbase(sbs[1].sbCreds, syncbaselib.Opts{Name: sbs[1].sbName, RootDir: sbs[1].rootDir}, acl(sbs[1].clientId))

	// Verify syncgroup syncing (client0 updates).
	ok(t, setSyncgroupSpec(sbs[0].clientCtx, sbs[0].sbName, sgId, "v2", "c", "", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c3", nil))
	ok(t, verifySyncgroupSpec(sbs[1].clientCtx, sbs[1].sbName, sgId, "v2", "c", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c3"))

	// Verify data and syncgroup syncing (client1 updates).
	ok(t, updateData(sbs[1].clientCtx, sbs[1].sbName, 5, 10, ""))
	ok(t, populateData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", 10, 20))
	ok(t, setSyncgroupSpec(sbs[1].clientCtx, sbs[1].sbName, sgId, "v3", "c", "", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c4", nil))

	// Verify that the last updates are synced right away so that we are
	// assured that all the updates are synced.
	ok(t, verifySyncgroupData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", "", 10, 10))
	ok(t, verifySyncgroupData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", "testkey"+sbs[1].sbName, 5, 5))
	ok(t, verifySyncgroupData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", "", 0, 5))
	ok(t, verifySyncgroupSpec(sbs[0].clientCtx, sbs[0].sbName, sgId, "v3", "c", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c4"))

	// Shutdown and restart Syncbase instances.
	sbs[0].cleanup(os.Interrupt)
	sbs[1].cleanup(os.Interrupt)

	sbs[0].cleanup = sh.StartSyncbase(sbs[0].sbCreds, syncbaselib.Opts{Name: sbs[0].sbName, RootDir: sbs[0].rootDir}, acl(sbs[0].clientId))
	sbs[1].cleanup = sh.StartSyncbase(sbs[1].sbCreds, syncbaselib.Opts{Name: sbs[1].sbName, RootDir: sbs[1].rootDir}, acl(sbs[1].clientId))

	ok(t, verifySyncgroupSpec(sbs[0].clientCtx, sbs[0].sbName, sgId, "v3", "c", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c4"))
	ok(t, verifySyncgroupSpec(sbs[1].clientCtx, sbs[1].sbName, sgId, "v3", "c", "root:o:app:client:c0;root:o:app:client:c1;root:o:app:client:c4"))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, testCx.Name, "foo", 20, 30))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 10, 20))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "testkey"+sbs[1].sbName, 5, 5))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, testCx.Name, "foo", "", 0, 5))
}

// TestV23VSyncWithPutDelWatch tests the sending of deltas between two Syncbase
// instances and their clients. The 1st client creates a syncgroup and puts some
// database entries in it. The 2nd client joins that syncgroup and reads and
// watches the database entries. The 1st client then deletes a portion of this
// data, and adds new entries. The 2nd client verifies that these changes are
// correctly synced. This verifies the end-to-end synchronization of data along
// the path: client0--Syncbase0--Syncbase1--client1 with a workload of puts and
// deletes.
func TestV23VSyncWithPutDelWatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 2 Syncbases.
	sbs := setupSyncbases(t, sh, 2, false)

	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, "c1,c2", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c1", "foo", 0, 10))

	beforeSyncMarker, err := getResumeMarker(sbs[1].clientCtx, sbs[1].sbName)
	ok(t, err)
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbName, sgId))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c1", "foo", "", 0, 10))
	ok(t, verifySyncgroupDataWithWatch(sbs[1].clientCtx, sbs[1].sbName, "c1", "foo", "", 10, false, beforeSyncMarker))

	beforeDeleteMarker, err := getResumeMarker(sbs[1].clientCtx, sbs[1].sbName)
	ok(t, err)
	ok(t, deleteData(sbs[0].clientCtx, sbs[0].sbName, "c1", "foo", 0, 5))
	ok(t, verifySyncgroupDeletedData(sbs[0].clientCtx, sbs[0].sbName, "c1", "foo", "", 5, 5))
	ok(t, verifySyncgroupDeletedData(sbs[1].clientCtx, sbs[1].sbName, "c1", "foo", "", 5, 5))
	ok(t, verifySyncgroupDataWithWatch(sbs[1].clientCtx, sbs[1].sbName, "c1", "foo", "", 5, true, beforeDeleteMarker))

	beforeSyncMarker, err = getResumeMarker(sbs[1].clientCtx, sbs[1].sbName)
	ok(t, err)
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c2", "foo", 0, 10))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c2", "foo", "", 0, 10))
	ok(t, verifySyncgroupDataWithWatch(sbs[1].clientCtx, sbs[1].sbName, "c2", "foo", "", 10, false, beforeSyncMarker))
}

// TestV23VSyncWithAcls tests the exchange of deltas including acls between two
// Syncbase instances and their clients. The 1st client creates a syncgroup on a
// collection and puts some database entries in it. The 2nd client joins that
// syncgroup and reads the database entries. The 2nd client then changes the
// collection acl to allow access to only itself. The 1st client should be
// unable to access the keys. The 2nd client then modifies the collection acl to
// restore access to both clients. The 1st client should regain access.
func TestV23VSyncWithAcls(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 2 Syncbases.
	sbs := setupSyncbases(t, sh, 2, false)

	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, "c", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c", "foo", 0, 10))
	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbName, sgId))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c", "foo", "", 0, 10))

	ok(t, setCollectionPermissions(sbs[1].clientCtx, sbs[1].sbName, "root:o:app:client:c1"))
	ok(t, verifyLostAccess(sbs[0].clientCtx, sbs[0].sbName, "c", "foo", 0, 10))

	ok(t, setCollectionPermissions(sbs[1].clientCtx, sbs[1].sbName, "root:o:app:client:c0;root:o:app:client:c1"))
	ok(t, verifySyncgroupData(sbs[0].clientCtx, sbs[0].sbName, "c", "foo", "", 0, 10))
}

// TestV23VSyncWithPeerSyncgroups tests the exchange of deltas between three
// Syncbase instances and their clients consisting of peer syncgroups. The 1st
// client creates a syncgroup: SG1 at collection "c" and puts some database
// entries. The 2nd client joins the syncgroup SG1 and verifies that it reads
// the corresponding database entries. Client 2 then creates SG2 at the same
// collection. The 3rd client joins the syncgroup SG2 and verifies that it can
// read all the keys created by client 1. Client 2 also verifies that it can
// read all the keys created by client 1.
func TestV23VSyncWithPeerSyncgroups(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 3 Syncbases.
	sbs := setupSyncbases(t, sh, 3, false)

	sb1Name := sbs[0].sbName
	sg1Id := wire.Id{Name: "SG1", Blessing: testCx.Blessing}
	sb2Name := sbs[1].sbName
	sg2Id := wire.Id{Name: "SG2", Blessing: testCx.Blessing}

	// Pre-populate the data before creating the syncgroup.
	ok(t, createCollection(sbs[0].clientCtx, sbs[0].sbName, "c"))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c", "f", 0, 10))
	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sg1Id, "c", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c", "foo", 0, 10))

	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sb1Name, sg1Id))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c", "foo", "", 0, 10))
	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c", "f", "", 0, 10))

	// We are setting the collection ACL again at syncbase2 but it is not
	// required.
	ok(t, createSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sg2Id, "c", "", nil, clBlessings(sbs)))

	ok(t, joinSyncgroup(sbs[2].clientCtx, sbs[2].sbName, sb2Name, sg2Id))
	ok(t, verifySyncgroupData(sbs[2].clientCtx, sbs[2].sbName, "c", "foo", "", 0, 10))
	ok(t, verifySyncgroupData(sbs[2].clientCtx, sbs[2].sbName, "c", "f", "", 0, 10))
}

// TestV23VSyncSyncgroups tests the syncing of syncgroup metadata. The 1st
// client creates the syncgroup SG1, and clients 2 and 3 join this
// syncgroup. All three clients must learn of the remaining two. Note that
// client 2 relies on syncgroup metadata syncing to learn of client 3 .
func TestV23VSyncSyncgroups(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 3 Syncbases.
	sbs := setupSyncbases(t, sh, 3, false)

	sbName := sbs[0].sbName
	sgId := wire.Id{Name: "SG1", Blessing: testCx.Blessing}

	ok(t, createSyncgroup(sbs[0].clientCtx, sbs[0].sbName, sgId, "c", "", nil, clBlessings(sbs)))
	ok(t, populateData(sbs[0].clientCtx, sbs[0].sbName, "c", "foo", 0, 10))
	ok(t, verifySyncgroupMembers(sbs[0].clientCtx, sbs[0].sbName, sgId, 1))

	ok(t, joinSyncgroup(sbs[1].clientCtx, sbs[1].sbName, sbName, sgId))
	ok(t, verifySyncgroupMembers(sbs[0].clientCtx, sbs[0].sbName, sgId, 2))
	ok(t, verifySyncgroupMembers(sbs[1].clientCtx, sbs[1].sbName, sgId, 2))

	ok(t, joinSyncgroup(sbs[2].clientCtx, sbs[2].sbName, sbName, sgId))
	ok(t, verifySyncgroupMembers(sbs[0].clientCtx, sbs[0].sbName, sgId, 3))
	ok(t, verifySyncgroupMembers(sbs[1].clientCtx, sbs[1].sbName, sgId, 3))
	ok(t, verifySyncgroupMembers(sbs[2].clientCtx, sbs[2].sbName, sgId, 3))

	ok(t, verifySyncgroupData(sbs[1].clientCtx, sbs[1].sbName, "c", "foo", "", 0, 10))
	ok(t, verifySyncgroupData(sbs[2].clientCtx, sbs[2].sbName, "c", "foo", "", 0, 10))
}

// TestV23VSyncMultiApp tests the sending of deltas between two Syncbase
// instances and their clients across multiple databases and collections. The
// 1st client puts entries in multiple collections across multiple databases
// then creates multiple syncgroups (one per database) over that data. The 2nd
// client joins these syncgroups and reads all the data.
func TestV23VSyncMultiApp(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Setup 2 Syncbases.
	sbs := setupSyncbases(t, sh, 2, false)

	na, nd, nc := 2, 2, 2 // number of apps, dbs, collections

	ok(t, setupAppMulti(sbs[0].clientCtx, sbs[0].sbName, na, nd))
	ok(t, setupAppMulti(sbs[1].clientCtx, sbs[1].sbName, na, nd))

	ok(t, populateAndCreateSyncgroupMulti(sbs[0].clientCtx, sbs[0].sbName, na, nd, nc, "foo,bar", clBlessings(sbs)))
	ok(t, joinSyncgroupMulti(sbs[1].clientCtx, sbs[1].sbName, sbs[0].sbName, na, nd))
	ok(t, verifySyncgroupDataMulti(sbs[1].clientCtx, sbs[1].sbName, na, nd, nc, "foo,bar"))
}

////////////////////////////////////////////////////////////
// Helpers for adding or updating data

func setSyncgroupSpec(ctx *context.T, syncbaseName string, sgId wire.Id, sgDesc, sgColls, mtName, blessingPatterns string, perms access.Permissions) error {
	if mtName == "" {
		roots := v23.GetNamespace(ctx).Roots()
		if len(roots) == 0 {
			return errors.New("no namespace roots")
		}
		mtName = roots[0]
	}

	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)

	if perms == nil {
		perms = tu.DefaultPerms(wire.AllSyncgroupTags, strings.Split(blessingPatterns, ";")...)
	}

	spec := wire.SyncgroupSpec{
		Description: sgDesc,
		Perms:       perms,
		Collections: parseSgCollections(sgColls),
		MountTables: []string{mtName},
	}

	sg := d.SyncgroupForId(sgId)
	if err := sg.SetSpec(ctx, spec, ""); err != nil {
		return fmt.Errorf("SetSpec SG %q failed: %v\n", sgId, err)
	}
	return nil
}

func deleteData(ctx *context.T, syncbaseName string, collectionName, keyPrefix string, start, end int) error {
	if end <= start {
		return fmt.Errorf("end (%d) <= start (%d)", end, start)
	}

	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	c := d.CollectionForId(collectionId)

	for i := start; i < end; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := c.Row(key)
		if err := r.Delete(ctx); err != nil {
			return fmt.Errorf("r.Delete() failed: %v\n", err)
		}
	}
	return nil
}

func setCollectionPermissions(ctx *context.T, syncbaseName string, blessingPatterns string) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	c := d.CollectionForId(testCx)

	perms := tu.DefaultPerms(wire.AllCollectionTags, strings.Split(blessingPatterns, ";")...)

	if err := c.SetPermissions(ctx, perms); err != nil {
		return fmt.Errorf("c.SetPermissions() failed: %v\n", err)
	}
	return nil
}

func setupAppMulti(ctx *context.T, syncbaseName string, numApps, numDbs int) error {
	svc := syncbase.NewService(syncbaseName)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)
		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := svc.DatabaseForId(wire.Id{appName, dbName}, nil)
			d.Create(ctx, nil)
		}
	}
	return nil
}

func populateAndCreateSyncgroupMulti(ctx *context.T, syncbaseName string, numApps, numDbs, numCxs int, prefixStr, blessings string) error {
	roots := v23.GetNamespace(ctx).Roots()
	if len(roots) == 0 {
		return errors.New("no namespace roots")
	}
	mtName := roots[0]

	sgperms := tu.DefaultPerms(wire.AllSyncgroupTags, strings.Split(blessings, ";")...)
	clperms := tu.DefaultPerms(wire.AllCollectionTags, strings.Split(blessings, ";")...)

	svc := syncbase.NewService(syncbaseName)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)

		for j := 0; j < numDbs; j++ {

			dbName := fmt.Sprintf("d%d", j)
			d := svc.DatabaseForId(wire.Id{appName, dbName}, nil)

			// For each collection, pre-populate entries on each prefix.
			// Also determine the syncgroup collections.
			var sgColls []wire.Id
			for k := 0; k < numCxs; k++ {
				cName := fmt.Sprintf("c%d", k)
				cId := wire.Id{testCx.Blessing, cName}
				c := d.CollectionForId(cId)
				if err := c.Create(ctx, clperms); err != nil {
					return fmt.Errorf("{%q, %v} c.Create failed %v", syncbaseName, cId, err)
				}

				sgColls = append(sgColls, cId)

				prefixes := strings.Split(prefixStr, ",")
				for _, pfx := range prefixes {
					for n := 0; n < 10; n++ {
						key := fmt.Sprintf("%s%d", pfx, n)
						r := c.Row(key)
						if err := r.Put(ctx, "testkey"+key); err != nil {
							return fmt.Errorf("r.Put() failed: %v\n", err)
						}
					}
				}
			}

			// Create one syncgroup per database across all collections.
			sgName := fmt.Sprintf("%s_%s", appName, dbName)
			sgId := wire.Id{Name: sgName, Blessing: testCx.Blessing}
			spec := wire.SyncgroupSpec{
				Description: fmt.Sprintf("test sg %s/%s", appName, dbName),
				Perms:       sgperms,
				Collections: sgColls,
				MountTables: []string{mtName},
			}

			sg := d.SyncgroupForId(sgId)
			info := wire.SyncgroupMemberInfo{SyncPriority: 8}
			if err := sg.Create(ctx, spec, info); err != nil {
				return fmt.Errorf("Create SG %q failed: %v\n", sgName, err)
			}
		}
	}
	return nil
}

func joinSyncgroupMulti(ctx *context.T, sbNameLocal, sbNameRemote string, numApps, numDbs int) error {
	svc := syncbase.NewService(sbNameLocal)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)

		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := svc.DatabaseForId(wire.Id{appName, dbName}, nil)

			sgName := fmt.Sprintf("%s_%s", appName, dbName)
			sg := d.SyncgroupForId(wire.Id{Name: sgName, Blessing: testCx.Blessing})
			info := wire.SyncgroupMemberInfo{SyncPriority: 10}
			if _, err := sg.Join(ctx, sbNameRemote, nil, info); err != nil {
				return fmt.Errorf("Join SG Multi %q failed: %v\n", sgName, err)
			}
		}
	}
	return nil
}

func getResumeMarker(ctx *context.T, syncbaseName string) (watch.ResumeMarker, error) {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	return d.GetResumeMarker(ctx)
}

// ByRow implements sort.Interface for []syncbase.WatchChange based on the Row field.
type ByRow []syncbase.WatchChange

func (c ByRow) Len() int           { return len(c) }
func (c ByRow) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c ByRow) Less(i, j int) bool { return c[i].Row < c[j].Row }

////////////////////////////////////////////////////////////
// Helpers for verifying data

func verifySyncgroupSpec(ctx *context.T, syncbaseName string, sgId wire.Id, wantDesc, wantColls, wantBlessingPatterns string) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	sg := d.SyncgroupForId(sgId)

	wantCollections := parseSgCollections(wantColls)
	wantPerms := tu.DefaultPerms(wire.AllSyncgroupTags, strings.Split(wantBlessingPatterns, ";")...)

	var spec wire.SyncgroupSpec
	var err error
	for i := 0; i < 20; i++ {
		spec, _, err = sg.GetSpec(ctx)
		if err != nil {
			return fmt.Errorf("GetSpec SG %q failed: %v\n", sgId, err)
		}
		if spec.Description == wantDesc {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if spec.Description != wantDesc || !reflect.DeepEqual(spec.Collections, wantCollections) || !reflect.DeepEqual(spec.Perms, wantPerms) {
		return fmt.Errorf("GetSpec SG %q failed: description got %v, want %v, collections got %v, want %v, perms got %v, want %v\n",
			sgId, spec.Description, wantDesc, spec.Collections, wantCollections, spec.Perms, wantPerms)
	}
	return nil
}

func verifySyncgroupDeletedData(ctx *context.T, syncbaseName, collectionName, keyPrefix, valuePrefix string, start, count int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	c := d.CollectionForId(collectionId)

	// Wait for a bit for deletions to propagate.
	lastKey := fmt.Sprintf("%s%d", keyPrefix, start-1)
	for i := 0; i < 20; i++ {
		r := c.Row(lastKey)
		var value string
		if err := r.Get(ctx, &value); verror.ErrorID(err) == verror.ErrNoExist.ID {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if valuePrefix == "" {
		valuePrefix = "testkey"
	}

	// Verify using a scan operation.
	stream := c.Scan(ctx, syncbase.Prefix(keyPrefix))
	cGot := 0
	for i := start; stream.Advance(); i++ {
		want := fmt.Sprintf("%s%d", keyPrefix, i)
		got := stream.Key()
		if got != want {
			return fmt.Errorf("unexpected key in scan: got %q, want %q\n", got, want)
		}
		want = valuePrefix + want
		if err := stream.Value(&got); err != nil {
			return fmt.Errorf("cannot fetch value in scan: %v\n", err)
		}
		if got != want {
			return fmt.Errorf("unexpected value in scan: got %q, want %q\n", got, want)
		}
		cGot++
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("scan stream error: %v\n", err)
	}

	if cGot != count {
		return fmt.Errorf("scan stream count error: %v %v\n", cGot, count)
	}
	return nil
}

func verifySyncgroupDataWithWatch(ctx *context.T, syncbaseName, collectionName, keyPrefix, valuePrefix string, count int, expectDelete bool, beforeSyncMarker watch.ResumeMarker) error {
	if count == 0 {
		return fmt.Errorf("count cannot be 0: got %d", count)
	}
	if valuePrefix == "" {
		valuePrefix = "testkey"
	}

	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream := d.Watch(ctxWithTimeout, beforeSyncMarker, []wire.CollectionRowPattern{
		util.RowPrefixPattern(collectionId, keyPrefix),
	})

	var changes []syncbase.WatchChange
	for len(changes) < count && stream.Advance() {
		if stream.Change().EntityType == syncbase.EntityRow {
			changes = append(changes, stream.Change())
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("watch stream error: %v\n", err)
	}

	sort.Sort(ByRow(changes))

	if got, want := len(changes), count; got != want {
		return fmt.Errorf("unexpected number of changes: got %d, want %d", got, want)
	}

	for i, change := range changes {
		if got, want := change.Collection, collectionId; got != want {
			return fmt.Errorf("unexpected watch collection: got %v, want %v", got, want)
		}
		if got, want := change.Row, fmt.Sprintf("%s%d", keyPrefix, i); got != want {
			return fmt.Errorf("unexpected watch row: got %q, want %q", got, want)
		}
		if got, want := change.FromSync, true; got != want {
			return fmt.Errorf("unexpected FromSync value: got %t, want %t", got, want)
		}
		if expectDelete {
			if got, want := change.ChangeType, syncbase.DeleteChange; got != want {
				return fmt.Errorf("unexpected watch change type: got %q, want %q", got, want)
			}
			return nil
		}
		var result string
		if got, want := change.ChangeType, syncbase.PutChange; got != want {
			return fmt.Errorf("unexpected watch change type: got %q, want %q", got, want)
		}
		if err := change.Value(&result); err != nil {
			return fmt.Errorf("couldn't decode watch value: %v", err)
		}
		if got, want := result, fmt.Sprintf("%s%s%d", valuePrefix, keyPrefix, i); got != want {
			return fmt.Errorf("unexpected watch value: got %q, want %q", got, want)
		}
	}
	return nil
}

func verifyLostAccess(ctx *context.T, syncbaseName, collectionName, keyPrefix string, start, count int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	c := d.CollectionForId(collectionId)

	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)
	r := c.Row(lastKey)
	for i := 0; i < 20; i++ {
		var value string
		if err := r.Get(ctx, &value); verror.ErrorID(err) == verror.ErrNoAccess.ID {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Verify that all keys have lost access.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		r := c.Row(key)
		var got string
		if err := r.Get(ctx, &got); verror.ErrorID(err) != verror.ErrNoAccess.ID {
			return fmt.Errorf("r.Get() didn't fail: %v\n", err)
		}
	}
	return nil
}

func verifySyncgroupDataMulti(ctx *context.T, syncbaseName string, numApps, numDbs, numCxs int, prefixStr string) error {
	svc := syncbase.NewService(syncbaseName)

	for i := 0; i < numApps; i++ {
		appName := fmt.Sprintf("a%d", i)

		for j := 0; j < numDbs; j++ {
			dbName := fmt.Sprintf("d%d", j)
			d := svc.DatabaseForId(wire.Id{appName, dbName}, nil)

			for k := 0; k < numCxs; k++ {
				cName := fmt.Sprintf("c%d", k)
				c := d.CollectionForId(wire.Id{testCx.Blessing, cName})

				prefixes := strings.Split(prefixStr, ",")
				for _, pfx := range prefixes {
					for n := 0; n < 10; n++ {
						key := fmt.Sprintf("%s%d", pfx, n)
						r := c.Row(key)
						var got string
						var err error
						// Wait for some time to sync.
						for t := 0; t < 20; t++ {
							if err = r.Get(ctx, &got); err == nil {
								break
							}
							time.Sleep(500 * time.Millisecond)
						}
						if err != nil {
							return fmt.Errorf("r.Get() failed: %v\n", err)
						}
						want := "testkey" + key
						if got != want {
							return fmt.Errorf("unexpected value: got %q, want %q\n",
								got, want)
						}
					}
				}
			}
		}
	}

	return nil
}
