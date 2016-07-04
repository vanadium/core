// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	sbUtil "v.io/v23/syncbase/util"
	"v.io/v23/vdl"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/longevity_tests/util"
	"v.io/x/ref/services/syncbase/testutil"
)

func TestDumpStream(t *testing.T) {
	ctx, serverName, cleanup := testutil.SetupOrDie(nil)
	defer cleanup()

	// All database and collection blessings must be rooted in our context
	// blessing, otherwise we can't write to them.
	myBlessing := security.DefaultBlessingNames(v23.GetPrincipal(ctx))[0]

	// Create some permissions for the databases and collections.  The
	// permissions must contain our own blessing, otherwise we will be
	// prevented from writing to the database/collection.
	myPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing)
	meAndAlicePermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing, myBlessing+security.ChainSeparator+"alice")
	meAndAlicePermsCx := sbUtil.FilterTags(meAndAlicePermsDb, wire.AllCollectionTags...)
	meAndBobPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing, myBlessing+security.ChainSeparator+"bob")
	meAndBobPermsCx := sbUtil.FilterTags(meAndBobPermsDb, wire.AllCollectionTags...)

	dbs := util.Databases{
		"db1": util.Database{
			Permissions: myPermsDb,
			Collections: util.Collections{
				"col1": util.Collection{
					Permissions: meAndAlicePermsCx,
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
				"col2": util.Collection{
					Permissions: meAndBobPermsCx,
					Rows:        util.Rows{},
				},
			},
		},
		"db2": util.Database{
			Permissions: meAndBobPermsDb,
			Collections: util.Collections{},
		},
		"db3": util.Database{
			Permissions: meAndAlicePermsDb,
			Collections: util.Collections{
				"col3": util.Collection{
					Permissions: meAndBobPermsCx,
					Rows:        util.Rows{},
				},
				"col4": util.Collection{
					Permissions: meAndAlicePermsCx,
					Rows: util.Rows{
						"key4": "val4",
						"key5": "val5",
						"key6": "val6",
					},
				},
			},
		},
	}

	// Seed service with databases defined above.
	service := syncbase.NewService(serverName)
	if err := util.SeedService(ctx, service, dbs); err != nil {
		t.Fatal(err)
	}

	// Get expected blessings for database and collection.
	dbBlessing, colBlessing, err := sbUtil.AppAndUserPatternFromBlessings(myBlessing)
	if err != nil {
		t.Fatal(err)
	}

	// Get expected Ids for database and collection.
	db1Id := wire.Id{Name: "db1", Blessing: string(dbBlessing)}
	db2Id := wire.Id{Name: "db2", Blessing: string(dbBlessing)}
	db3Id := wire.Id{Name: "db3", Blessing: string(dbBlessing)}
	col1Id := wire.Id{Name: "col1", Blessing: string(colBlessing)}
	col2Id := wire.Id{Name: "col2", Blessing: string(colBlessing)}
	col3Id := wire.Id{Name: "col3", Blessing: string(colBlessing)}
	col4Id := wire.Id{Name: "col4", Blessing: string(colBlessing)}

	// Get expected permissions.
	db1Perms := dbs["db1"].Permissions
	db2Perms := dbs["db2"].Permissions
	db3Perms := dbs["db3"].Permissions
	col1Perms := dbs["db1"].Collections["col1"].Permissions
	col2Perms := dbs["db1"].Collections["col2"].Permissions
	col3Perms := dbs["db3"].Collections["col3"].Permissions
	col4Perms := dbs["db3"].Collections["col4"].Permissions

	// Expected rows from dump stream.
	wantRows := []util.Row{
		util.Row{DatabaseId: db1Id, Permissions: db1Perms},
		util.Row{DatabaseId: db1Id, CollectionId: col1Id, Permissions: col1Perms},
		util.Row{DatabaseId: db1Id, CollectionId: col1Id, Key: "key1", Value: vdl.StringValue(nil, "val1")},
		util.Row{DatabaseId: db1Id, CollectionId: col1Id, Key: "key2", Value: vdl.StringValue(nil, "val2")},
		util.Row{DatabaseId: db1Id, CollectionId: col1Id, Key: "key3", Value: vdl.StringValue(nil, "val3")},
		util.Row{DatabaseId: db1Id, CollectionId: col2Id, Permissions: col2Perms},

		util.Row{DatabaseId: db2Id, Permissions: db2Perms},

		util.Row{DatabaseId: db3Id, Permissions: db3Perms},
		util.Row{DatabaseId: db3Id, CollectionId: col3Id, Permissions: col3Perms},
		util.Row{DatabaseId: db3Id, CollectionId: col4Id, Permissions: col4Perms},
		util.Row{DatabaseId: db3Id, CollectionId: col4Id, Key: "key4", Value: vdl.StringValue(nil, "val4")},
		util.Row{DatabaseId: db3Id, CollectionId: col4Id, Key: "key5", Value: vdl.StringValue(nil, "val5")},
		util.Row{DatabaseId: db3Id, CollectionId: col4Id, Key: "key6", Value: vdl.StringValue(nil, "val6")},
	}

	dumpStreamMatches(t, ctx, service, wantRows)
}

func dumpStreamMatches(t *testing.T, ctx *context.T, service syncbase.Service, wantRows []util.Row) {
	stream, err := util.NewDumpStream(ctx, service)
	if err != nil {
		t.Fatalf("NewDumpStream failed: %v", err)
	}

	gotRows := []util.Row{}
	for stream.Advance() {
		if stream.Err() != nil {
			t.Fatalf("stream.Err() returned error: %v", err)
		}

		gotRows = append(gotRows, *stream.Row())
	}

	if !reflect.DeepEqual(wantRows, gotRows) {
		t.Errorf("wanted rows %v but got %v", wantRows, gotRows)
	}
}
