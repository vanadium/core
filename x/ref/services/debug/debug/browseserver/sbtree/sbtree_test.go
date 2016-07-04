// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sbtree_test

import (
	"errors"
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/debug/debug/browseserver/sbtree"
	"v.io/x/ref/services/syncbase/fake"
	tu "v.io/x/ref/services/syncbase/testutil"
)

func TestWithEmptyService(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	service := syncbase.NewService(serverName)
	dbIds, err := service.ListDatabases(ctx)
	if err != nil {
		t.Fatal(err)
	}

	got := sbtree.AssembleSyncbaseTree(ctx, serverName, service, dbIds)

	if got.Service.FullName() != serverName {
		t.Errorf("got %q, want %q", got.Service.FullName(), serverName)
	}
	if len(got.Dbs) != 0 {
		t.Errorf("want no databases, got %v", got.Dbs)
	}
}

func TestWithMultipleEmptyDbs(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service = syncbase.NewService(serverName)
		dbNames = []string{"db_a", "db_b", "db_c"}
	)
	for _, dbName := range dbNames {
		tu.CreateDatabase(t, ctx, service, dbName)
	}
	dbIds, err := service.ListDatabases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbIds) != 3 {
		t.Fatalf("Got %d, want 3", len(dbIds))
	}

	got := sbtree.AssembleSyncbaseTree(ctx, serverName, service, dbIds)

	if len(got.Dbs) != 3 {
		t.Fatalf("want 3 databases, got %v", got.Dbs)
	}
	for i, db := range got.Dbs {
		if db.Database.Id().Name != dbNames[i] {
			t.Errorf("got %q, want %q", db.Database.Id().Name, dbNames[i])
		}
		if len(db.Collections) != 0 {
			t.Errorf("want no collections, got %v", db.Collections)
		}
		if len(db.Syncgroups) != 0 {
			t.Errorf("want no syncgroups, got %v", db.Syncgroups)
		}
		if len(db.Errs) != 0 {
			t.Errorf("want no errors, got %v", db.Errs)
		}
	}
}

func TestWithMultipleCollections(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service   = syncbase.NewService(serverName)
		collNames = []string{"coll_a", "coll_b", "coll_c"}
		database  = tu.CreateDatabase(t, ctx, service, "the_db")
	)
	for _, collName := range collNames {
		tu.CreateCollection(t, ctx, database, collName)
	}
	dbIds, err := service.ListDatabases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbIds) != 1 {
		t.Fatalf("Got %d, want 1", len(dbIds))
	}

	got := sbtree.AssembleSyncbaseTree(ctx, serverName, service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	for i, coll := range got.Dbs[0].Collections {
		if coll.Id().Name != collNames[i] {
			t.Errorf("got %q, want %q", coll.Id().Name, collNames[i])
		}
		exists, err := coll.Exists(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("want collection to exist, but it does not")
		}
	}
}

func TestWithMultipleSyncgroups(t *testing.T) {
	ctx, serverName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	var (
		service        = syncbase.NewService(serverName)
		sgNames        = []string{"syncgroup_a", "syncgroup_b", "syncgroup_c"}
		sgDescriptions = []string{"AAA", "BBB", "CCC"}
		database       = tu.CreateDatabase(t, ctx, service, "the_db")
		coll           = tu.CreateCollection(t, ctx, database, "the_collection")
	)
	for i, sgName := range sgNames {
		tu.CreateSyncgroup(t, ctx, database, coll, sgName, sgDescriptions[i])
	}

	dbIds, err := service.ListDatabases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dbIds) != 1 {
		t.Fatalf("Got %d, want 1", len(dbIds))
	}

	got := sbtree.AssembleSyncbaseTree(ctx, serverName, service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	for i, sg := range got.Dbs[0].Syncgroups {
		if sg.Syncgroup.Id().Name != sgNames[i] {
			t.Errorf("got %q, want %q", sg.Syncgroup.Id().Name, sgNames[i])
		}
		if sg.Spec.Description != sgDescriptions[i] {
			t.Errorf("got %q, want %q", sg.Spec.Description, sgDescriptions[i])
		}
		if sg.Spec.Collections[0].Name != "the_collection" {
			t.Errorf(`got %q, want "the_collection"`,
				sg.Spec.Collections[0].Name)
		}
		if len(sg.Errs) != 0 {
			t.Errorf("want no errors, got %v", sg.Errs)
		}
	}
}

func TestWithNoErrs(t *testing.T) {
	service := fake.Service(nil, nil)
	dbIds := []wire.Id{wire.Id{}}

	got := sbtree.AssembleSyncbaseTree(nil, "", service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	if len(got.Dbs[0].Errs) != 0 {
		t.Fatalf("want 0 error, got %v", got.Dbs[0].Errs)
	}
}

func TestWithListCollectionsErr(t *testing.T) {
	service := fake.Service(errors.New("<<injected list-collections error>>"), nil)
	dbIds := []wire.Id{wire.Id{}}

	got := sbtree.AssembleSyncbaseTree(nil, "", service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	if len(got.Dbs[0].Errs) != 1 {
		t.Fatalf("want 1 error, got %v", got.Dbs[0].Errs)
	}
	want := "Problem listing collections: <<injected list-collections error>>"
	if got.Dbs[0].Errs[0].Error() != want {
		t.Fatalf("got %v, want %q", got.Dbs[0].Errs, want)
	}
}

func TestWithNoSyncgroupErrs(t *testing.T) {
	service := fake.Service(nil, nil)
	dbIds := []wire.Id{wire.Id{}}

	got := sbtree.AssembleSyncbaseTree(nil, "", service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	if len(got.Dbs[0].Syncgroups) != 1 {
		t.Fatalf("want 1 syncgroups, got %v", got.Dbs[0].Syncgroups)
	}

	sg := got.Dbs[0].Syncgroups[0]
	if len(sg.Errs) != 0 {
		t.Fatalf("want 0 error, got %v", sg.Errs)
	}
}

func TestWithSyncgroupErr(t *testing.T) {
	service := fake.Service(nil, errors.New("<<injected syncgroup spec error>>"))
	dbIds := []wire.Id{wire.Id{}}

	got := sbtree.AssembleSyncbaseTree(nil, "", service, dbIds)

	if len(got.Dbs) != 1 {
		t.Fatalf("want 1 database, got %v", got.Dbs)
	}
	if len(got.Dbs[0].Syncgroups) != 1 {
		t.Fatalf("want 1 syncgroups, got %v", got.Dbs[0].Syncgroups)
	}

	sg := got.Dbs[0].Syncgroups[0]
	if len(sg.Errs) != 1 {
		t.Fatalf("want 1 error, got %v", sg.Errs)
	}
	want := "Problem getting spec of syncgroup: <<injected syncgroup spec error>>"
	if sg.Errs[0].Error() != want {
		t.Fatalf("got %v, want %q", sg.Errs, want)
	}
}
