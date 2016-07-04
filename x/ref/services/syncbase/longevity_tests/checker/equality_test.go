// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package checker_test

import (
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	sbutil "v.io/v23/syncbase/util"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/syncbase/longevity_tests/checker"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/longevity_tests/util"
	"v.io/x/ref/services/syncbase/server"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/testutil"
	tsecurity "v.io/x/ref/test/testutil"
)

func newServer(t *testing.T, ctx *context.T) (string, func()) {
	rootDir, err := ioutil.TempDir("", "syncbase")
	if err != nil {
		t.Fatalf("ioutil.TempDir() failed: %v", err)
	}
	perms := testutil.DefaultPerms(access.AllTypicalTags(), "...")
	serverCtx, cancel := context.WithCancel(ctx)
	service, err := server.NewService(serverCtx, server.ServiceOptions{
		Perms:           perms,
		RootDir:         rootDir,
		Engine:          store.EngineForTest,
		DevMode:         true,
		SkipPublishInNh: true,
	})
	if err != nil {
		t.Fatalf("server.NewService() failed: %v", err)
	}
	serverCtx, s, err := v23.WithNewDispatchingServer(serverCtx, "", server.NewDispatcher(service))
	if err != nil {
		t.Fatalf("v23.WithNewDispatchingServer() failed: %v", err)
	}
	name := s.Status().Endpoints[0].Name()
	return name, func() {
		cancel()
		<-s.Closed()
		service.Close()
		os.RemoveAll(rootDir)
	}
}

// setupServers runs n syncbase servers and returns a slice of services, a
// model universe containing the servers, and a cancel func.
func setupServers(t *testing.T, n int) (*context.T, []syncbase.Service, model.Universe, func()) {
	ctx, cancel := v23.Init()

	principal := tsecurity.NewPrincipal("root:u:equality")
	ctx, _ = v23.WithPrincipal(ctx, principal)

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})

	services := []syncbase.Service{}
	cancelFuncs := []func(){}

	universe := model.Universe{
		Users: model.UserSet{},
	}

	for i := 0; i < n; i++ {
		serverName, cancel := newServer(t, ctx)
		services = append(services, syncbase.NewService(serverName))
		cancelFuncs = append(cancelFuncs, cancel)

		universe.Users = append(universe.Users,
			&model.User{
				Devices: model.DeviceSet{
					&model.Device{Name: serverName},
				},
			},
		)
	}

	return ctx, services, universe, func() {
		for _, cancelFunc := range cancelFuncs {
			cancelFunc()
		}
		cancel()
	}
}

// TestEqualityEqualServices checks that Equality returns no errors if the
// syncbases have identical databases, collections, and rows.
func TestEqualityEqualServices(t *testing.T) {
	// Start three syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 3)
	defer cancel()

	myBlessing := security.DefaultBlessingNames(v23.GetPrincipal(ctx))[0]
	myPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing)
	myPermsCx := sbutil.FilterTags(myPermsDb, wire.AllCollectionTags...)
	otherPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing, myBlessing+security.ChainSeparator+"alice")
	otherPermsCx := sbutil.FilterTags(otherPermsDb, wire.AllCollectionTags...)

	// Seed services with same data.
	dbs := util.Databases{
		"db1": util.Database{
			Permissions: myPermsDb,
			Collections: util.Collections{
				"col1": util.Collection{
					Permissions: otherPermsCx,
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
				"col2": util.Collection{
					Permissions: myPermsCx,
					Rows:        util.Rows{},
				},
			},
		},
		"db2": util.Database{
			Permissions: otherPermsDb,
			Collections: util.Collections{},
		},
		"db3": util.Database{
			Permissions: otherPermsDb,
			Collections: util.Collections{
				"col3": util.Collection{
					Permissions: myPermsCx,
					Rows:        util.Rows{},
				},
				"col4": util.Collection{
					Permissions: otherPermsCx,
					Rows: util.Rows{
						"key4": "val4",
						"key5": "val5",
						"key6": "val6",
					},
				},
			},
		},
	}
	for _, s := range services {
		if err := util.SeedService(ctx, s, dbs); err != nil {
			t.Fatal(err)
		}
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err != nil {
		t.Errorf("expected eq.Run() not to error but got: %v", err)
	}
}

// TestEqualityDifferentRowValues checks that Equality returns an error if the
// syncbases have the same databases and collections, but different row values.
func TestEqualityDifferentRowValues(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	// Seed services with different rows.
	dbs1 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "VAL-TWO", // Different.
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "DIFFERENT-VAL", // Different.
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error, but it did not")
	}
}

// TestEqualityExtraRows checks that Equality returns an error if the syncbases
// have the same databases and collections, but one has extra rows.
func TestEqualityExtraRows(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	// Seed services with different number of rows.
	dbs1 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2", // Missing in above db.
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error but it did not")
	}
}

// TestEqualityDifferentCollections checks that Equality returns an error if
// the syncbases have the same databases and rows but different collection
// names.
func TestEqualityDifferentCollections(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	// Seed services with databases with different collection name.
	dbs1 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col_DIFFERENT": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error but it did not")
	}
}

// TestEqualityDifferentDatabases checks that Equality returns an error if the
// syncbases have the same collections and rows but different database names.
func TestEqualityDifferentDatabases(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	// Seed services with databases with different database name.
	dbs1 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db_DIFFERENT": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error but it did not")
	}
}

// TestEqualityDifferentDatabasePermissions checks that Equality returns an
// error if databases have different Permissions.
func TestEqualityDifferentDatabasePermissions(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	myBlessing := security.DefaultBlessingNames(v23.GetPrincipal(ctx))[0]
	myPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing)
	otherPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, myBlessing, myBlessing+security.ChainSeparator+"alice")

	// Seed services with databases with different permissions.
	dbs1 := util.Databases{
		"db1": util.Database{
			Permissions: myPermsDb, // Different.
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db1": util.Database{
			Permissions: otherPermsDb, // Different.
			Collections: util.Collections{
				"col1": util.Collection{
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error but it did not")
	}
}

// TestEqualityDifferentCollectionPermissions checks that Equality returns an
// error if collections have different permissions.
func TestEqualityDifferentCollectionPermissions(t *testing.T) {
	// Start two syncbase servers.
	ctx, services, universe, cancel := setupServers(t, 2)
	defer cancel()

	myBlessing := security.DefaultBlessingNames(v23.GetPrincipal(ctx))[0]
	myPermsCx := testutil.DefaultPerms(wire.AllCollectionTags, myBlessing)
	otherPermsCx := testutil.DefaultPerms(wire.AllCollectionTags, myBlessing, myBlessing+security.ChainSeparator+"alice")

	// Seed services with databases with different collection name.
	dbs1 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Permissions: myPermsCx, // Different.
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	dbs2 := util.Databases{
		"db1": util.Database{
			Collections: util.Collections{
				"col1": util.Collection{
					Permissions: otherPermsCx, // Different.
					Rows: util.Rows{
						"key1": "val1",
						"key2": "val2",
						"key3": "val3",
					},
				},
			},
		},
	}
	if err := util.SeedService(ctx, services[0], dbs1); err != nil {
		t.Fatal(err)
	}
	if err := util.SeedService(ctx, services[1], dbs2); err != nil {
		t.Fatal(err)
	}

	eq := checker.Equality{}
	if err := eq.Run(ctx, universe); err == nil {
		t.Errorf("expected eq.Run() to error but it did not")
	}
}
