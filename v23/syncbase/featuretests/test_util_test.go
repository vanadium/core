// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package featuretests_test

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/x/ref/services/syncbase/syncbaselib"
	tu "v.io/x/ref/services/syncbase/testutil"
	"v.io/x/ref/test/v23test"
)

const (
	testSbName = "syncbase" // Name that syncbase mounts itself at.
)

var (
	appExtension    = "o:app"
	clientExtension = appExtension + ":client"
	testDb          = wire.Id{Blessing: "root:" + appExtension, Name: "d"}
	testCx          = wire.Id{Blessing: "root:" + clientExtension, Name: "c"}
)

////////////////////////////////////////////////////////////
// Helpers for setting up Syncbases, dbs, and collections

func setupHierarchy(ctx *context.T, syncbaseName string, perms access.Permissions) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	return d.Create(ctx, perms)
}

func createCollection(ctx *context.T, syncbaseName, collectionName string) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	return d.CollectionForId(collectionId).Create(ctx, nil)
}

type testSyncbase struct {
	sbName    string
	sbCreds   *v23test.Credentials
	rootDir   string
	clientId  string
	clientCtx *context.T
	cleanup   func(sig os.Signal)
}

// Spawns "num" Syncbase instances and returns handles to them.
func setupSyncbases(t testing.TB, sh *v23test.Shell, num int, devMode bool) []*testSyncbase {
	sbs := make([]*testSyncbase, num)

	for i, _ := range sbs {
		sbName, clientId := fmt.Sprintf("s%d", i), fmt.Sprintf("%s:c%d", clientExtension, i)
		clientCtx := sh.ForkContext(clientId)
		sbCreds := sh.ForkCredentialsFromPrincipal(v23.GetPrincipal(clientCtx), sbName)
		if sh.Err != nil {
			tu.Fatal(t, sh.Err)
		}
		sbs[i] = &testSyncbase{
			sbName:    sbName,
			sbCreds:   sbCreds,
			rootDir:   sh.MakeTempDir(),
			clientId:  clientId,
			clientCtx: clientCtx,
		}
		// Give XRWA permissions to this Syncbase's client.
		clientBlessing := fmt.Sprintf("root:%s", clientId)
		acl := fmt.Sprintf(`{"Resolve":{"In":["%s"]},"Read":{"In":["%s"]},"Write":{"In":["%s"]},"Admin":{"In":["%s"]}}`, clientBlessing, clientBlessing, clientBlessing, clientBlessing)
		sbs[i].cleanup = sh.StartSyncbase(sbs[i].sbCreds, syncbaselib.Opts{Name: sbs[i].sbName, RootDir: sbs[i].rootDir, DevMode: devMode}, acl)

		// Call setupHierarchy on each Syncbase.
		dbPerms := tu.DefaultPerms(wire.AllDatabaseTags, clientBlessing)
		ok(t, setupHierarchy(sbs[i].clientCtx, sbs[i].sbName, dbPerms))
	}
	return sbs
}

// Returns a ";"-separated list of Syncbase clients blessing names.
func clBlessings(sbs []*testSyncbase) string {
	names := make([]string, len(sbs))
	for i, sb := range sbs {
		names[i] = "root:" + sb.clientId
	}
	return strings.Join(names, ";")
}

////////////////////////////////////////////////////////////
// Helpers for adding or updating data

func populateData(ctx *context.T, syncbaseName, collectionName, keyPrefix string, start, end int) error {
	if end <= start {
		return fmt.Errorf("end (%d) <= start (%d)", end, start)
	}

	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	c := d.CollectionForId(collectionId)

	for i := start; i < end; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		if err := c.Put(ctx, key, "testkey"+key); err != nil {
			return fmt.Errorf("c.Put() failed: %v", err)
		}
	}
	return nil
}

// Shared by updateData and updateBatchData.
// TODO(sadovsky): This is eerily similar to populateData. We should strive to
// avoid such redundancies by implementing common helpers and avoiding spurious
// differences.
func updateDataImpl(ctx *context.T, d syncbase.DatabaseHandle, syncbaseName string, start, end int, valuePrefix string) error {
	if end <= start {
		return fmt.Errorf("end (%d) <= start (%d)", end, start)
	}
	if valuePrefix == "" {
		valuePrefix = "testkey"
	}

	c := d.CollectionForId(testCx)
	for i := start; i < end; i++ {
		key := fmt.Sprintf("foo%d", i)
		if err := c.Put(ctx, key, valuePrefix+syncbaseName+key); err != nil {
			return fmt.Errorf("c.Put() failed: %v", err)
		}
	}
	return nil
}

func updateData(ctx *context.T, syncbaseName string, start, end int, valuePrefix string) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	return updateDataImpl(ctx, d, syncbaseName, start, end, valuePrefix)
}

// Signal key tells the module to send an end signal, using the signalKey
// provided as the key, once the module finishes its execution. This is useful
// if the module is run as a goroutine and the parent needs to wait for it to
// end.
func updateDataInBatch(ctx *context.T, syncbaseName string, start, end int, valuePrefix, signalKey string) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	batch, err := d.BeginBatch(ctx, wire.BatchOptions{})
	if err != nil {
		return fmt.Errorf("BeginBatch failed: %v", err)
	}
	if err = updateDataImpl(ctx, batch, syncbaseName, start, end, valuePrefix); err != nil {
		return err
	}
	if err = batch.Commit(ctx); err != nil {
		return fmt.Errorf("Commit failed: %v", err)
	}
	if signalKey != "" {
		return sendSignal(ctx, d, signalKey)
	}
	return nil
}

// TODO(ivanpi): Remove sendSignal now that all functions using it are in the
// same process.
func sendSignal(ctx *context.T, d syncbase.Database, signalKey string) error {
	c := d.CollectionForId(testCx)
	r := c.Row(signalKey)

	if err := r.Put(ctx, true); err != nil {
		return fmt.Errorf("r.Put() failed: %v", err)
	}
	return nil
}

////////////////////////////////////////////////////////////
// Helpers for verifying data

func verifySyncgroupData(ctx *context.T, syncbaseName, collectionName, keyPrefix, valuePrefix string, start, count int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	collectionId := wire.Id{Blessing: testCx.Blessing, Name: collectionName}
	c := d.CollectionForId(collectionId)

	// Wait a bit (up to 10 seconds) for the last key to appear.
	lastKey := fmt.Sprintf("%s%d", keyPrefix, start+count-1)
	for i := 0; i < 20; i++ {
		var value string
		if err := c.Get(ctx, lastKey, &value); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if valuePrefix == "" {
		valuePrefix = "testkey"
	}

	// Verify that all keys and values made it over correctly.
	for i := start; i < start+count; i++ {
		key := fmt.Sprintf("%s%d", keyPrefix, i)
		var got string
		if err := c.Get(ctx, key, &got); err != nil {
			return fmt.Errorf("c.Get() failed: %v", err)
		}
		want := valuePrefix + key
		if got != want {
			return fmt.Errorf("unexpected value: got %q, want %q", got, want)
		}
	}
	return nil
}

////////////////////////////////////////////////////////////
// Helpers for managing syncgroups

// blessingPatterns is a ";"-separated list of blessing patterns.
func createSyncgroup(ctx *context.T, syncbaseName string, sgId wire.Id, sgColls, mtName string, perms access.Permissions, clBlessings string) error {
	if mtName == "" {
		roots := v23.GetNamespace(ctx).Roots()
		if len(roots) == 0 {
			return errors.New("no namespace roots")
		}
		mtName = roots[0]
	}

	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)

	if perms == nil {
		perms = tu.DefaultPerms(wire.AllSyncgroupTags, strings.Split(clBlessings, ";")...)
	}
	collectionPerms := tu.DefaultPerms(wire.AllCollectionTags, strings.Split(clBlessings, ";")...)

	spec := wire.SyncgroupSpec{
		Description: "test syncgroup sg",
		Perms:       perms,
		Collections: parseSgCollections(sgColls),
		MountTables: []string{mtName},
	}

	// Change the collection ACLs to enable syncing.
	for _, cId := range spec.Collections {
		// TODO(ivanpi,hpucha): Switch to blessings of the form "idp:o:root:c<n>"
		// (different users instead of delegates of one user) and get the collection
		// id from context.
		c := d.CollectionForId(cId)
		// Ignore the error since sometimes a collection might already exist.
		c.Create(ctx, nil)
		if err := c.SetPermissions(ctx, collectionPerms); err != nil {
			return fmt.Errorf("{%q, %v} c.SetPermissions failed: %v", syncbaseName, cId, err)
		}
	}

	sg := d.SyncgroupForId(sgId)

	info := wire.SyncgroupMemberInfo{SyncPriority: 8}
	if err := sg.Create(ctx, spec, info); err != nil {
		return fmt.Errorf("{%q, %v} sg.Create() failed: %v", syncbaseName, sgId, err)
	}
	return nil
}

func joinSyncgroup(ctx *context.T, sbNameLocal, sbNameRemote string, sgId wire.Id) error {
	d := syncbase.NewService(sbNameLocal).DatabaseForId(testDb, nil)
	sg := d.SyncgroupForId(sgId)
	info := wire.SyncgroupMemberInfo{SyncPriority: 10}
	if _, err := sg.Join(ctx, sbNameRemote, nil, info); err != nil {
		return fmt.Errorf("{%q, %q} sg.Join() failed: %v", sbNameRemote, sgId, err)
	}
	return nil
}

func verifySyncgroupMembers(ctx *context.T, syncbaseName string, sgId wire.Id, wantMembers int) error {
	d := syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil)
	sg := d.SyncgroupForId(sgId)

	var gotMembers int
	for i := 0; i < 8; i++ {
		members, err := sg.GetMembers(ctx)
		if err != nil {
			return fmt.Errorf("{%q, %q} sg.GetMembers() failed: %v", syncbaseName, sgId, err)
		}
		gotMembers = len(members)
		if gotMembers == wantMembers {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if gotMembers != wantMembers {
		return fmt.Errorf("{%q, %q} verifySyncgroupMembers failed: got %d members, want %d", syncbaseName, sgId, gotMembers, wantMembers)
	}
	return nil
}

func pauseSync(ctx *context.T, syncbaseName string) error {
	return syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil).PauseSync(ctx)
}

func resumeSync(ctx *context.T, syncbaseName string) error {
	return syncbase.NewService(syncbaseName).DatabaseForId(testDb, nil).ResumeSync(ctx)
}

////////////////////////////////////////////////////////////
// Syncbase-specific testing helpers

// parseSgCollections converts, for example, "a,c" to
// [Collection: {clientBlessing, "a"}, Collection: {clientBlessing, "c"}].
func parseSgCollections(csv string) []wire.Id {
	strs := strings.Split(csv, ",")
	res := make([]wire.Id, len(strs))
	for i, v := range strs {
		res[i] = wire.Id{testCx.Blessing, v}
	}
	return res
}

////////////////////////////////////////////////////////////
// Helpers to interact with the Syncbase service directly.

func sc(name string) wire.ServiceClientStub {
	return wire.ServiceClient(name)
}

////////////////////////////////////////////////////////////
// Generic testing helpers

func ok(t testing.TB, err error) {
	if err != nil {
		tu.Fatal(t, err)
	}
}

func nok(t testing.TB, err error) {
	if err == nil {
		tu.Fatal(t, "nil err")
	}
}

func eq(t testing.TB, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		tu.Fatalf(t, "got %v, want %v", got, want)
	}
}

func neq(t testing.TB, got, notWant interface{}) {
	if reflect.DeepEqual(got, notWant) {
		tu.Fatalf(t, "got %v", got)
	}
}
