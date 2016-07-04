// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package control_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/verror"
	"v.io/x/lib/gosh"
	_ "v.io/x/ref/runtime/factories/roaming"
	_ "v.io/x/ref/runtime/protocols/vine"
	"v.io/x/ref/services/syncbase/longevity_tests/client"
	"v.io/x/ref/services/syncbase/longevity_tests/control"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/testutil"
)

func TestMain(m *testing.M) {
	gosh.InitMain()
	os.Exit(m.Run())
}

func newController(t *testing.T) (*control.Controller, func()) {
	rootDir, err := ioutil.TempDir("", "control-test-")
	if err != nil {
		t.Fatal(err)
	}
	ctx, shutdown := v23.Init()
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{
		Addrs: rpc.ListenAddrs{{"tcp", "127.0.0.1:0"}},
	})
	opts := control.Opts{
		DebugOutput: true,
		TB:          t,
		RootDir:     rootDir,
	}
	c, err := control.NewController(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		if err := c.TearDown(); err != nil {
			t.Fatal(err)
		}
		shutdown()
		os.RemoveAll(rootDir)
	}
	return c, cleanup
}

func mountTableIsRunning(t *testing.T, c *control.Controller) bool {
	ctxWithTimeout, cancel := context.WithTimeout(c.InternalCtx(), 1*time.Second)
	defer cancel()
	_, _, err := v23.GetNamespace(ctxWithTimeout).GetPermissions(ctxWithTimeout, "")
	return err == nil
}

func syncbaseIsRunning(t *testing.T, c *control.Controller, name string) bool {
	ctxWithTimeout, cancel := context.WithTimeout(c.InternalCtx(), 1*time.Second)
	defer cancel()
	_, _, err := syncbase.NewService(name).GetPermissions(ctxWithTimeout)
	return err == nil
}

func TestRunEmptyUniverse(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	u := &model.Universe{}

	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}
	// Check mounttable is running.
	if !mountTableIsRunning(t, c) {
		t.Errorf("expected mounttable to be running but it was not")
	}

	// Calling Run a second time should not fail.
	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}
}

func TestRunUniverseSingleDevice(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	userName := "test-user"
	deviceName := "test-device"
	u := &model.Universe{
		Users: model.UserSet{
			&model.User{
				Name: userName,
				Devices: model.DeviceSet{
					&model.Device{
						Name: deviceName,
					},
				},
			},
		},
	}

	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}
	// Check mounttable is running.
	if !mountTableIsRunning(t, c) {
		t.Errorf("expected mounttable to be running but it was not")
	}
	// Check syncbase is running.
	if !syncbaseIsRunning(t, c, deviceName) {
		t.Errorf("expected syncbase %q to be running but it was not", deviceName)
	}

	// Check that instance has correct blessing name.
	gotBlessings := c.InternalGetInstance(deviceName).InternalDefaultBlessingNames()[0]
	wantSuffix := strings.Join([]string{userName, deviceName}, security.ChainSeparator)
	if !strings.HasSuffix(gotBlessings, wantSuffix) {
		t.Errorf("wanted blessing name to have suffix %v but got %v", wantSuffix, gotBlessings)
	}

	// Calling Run a second time should not fail.
	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}
	if !syncbaseIsRunning(t, c, deviceName) {
		t.Errorf("expected syncbase %q to be running but it was not", deviceName)
	}

	// Delete the device from the universe.
	u.Users[0].Devices = model.DeviceSet{}

	// Calling Run again should error.
	if err := c.Run(u); err == nil {
		t.Fatal("expected Run to fail with shrunk universe but it did not")
	}
}

func TestRunUniverseTwoDevices(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	d1 := &model.Device{
		Name: "test-device-1",
	}
	d2 := &model.Device{
		Name: "test-device-2",
	}
	users := model.UserSet{
		&model.User{
			Name:    "user-1",
			Devices: model.DeviceSet{d1},
		},
		&model.User{
			Name:    "user-2",
			Devices: model.DeviceSet{d2},
		},
	}

	// Initially universe has devices unconnected.
	uDisconnected := &model.Universe{
		Users: users,
		Topology: model.Topology{
			d1: model.DeviceSet{d1},
			d2: model.DeviceSet{d2},
		},
	}
	if err := c.Run(uDisconnected); err != nil {
		t.Fatal(err)
	}
	// Check that the devices are not syncing.
	if syncbasesCanSync(t, c, d1.Name, d2.Name) {
		t.Fatalf("expected syncbases %v and %v not to sync but they did", d1.Name, d2.Name)
	}

	// Connect the two devices.
	uConnected := &model.Universe{
		Users: users,
		Topology: model.Topology{
			d1: model.DeviceSet{d1, d2},
			d2: model.DeviceSet{d1, d2},
		},
	}
	if err := c.Run(uConnected); err != nil {
		t.Fatal(err)
	}
	// Check that the devices are syncing.
	if !syncbasesCanSync(t, c, d1.Name, d2.Name) {
		t.Fatalf("expected syncbases %v and %v to sync but they did not", d1.Name, d2.Name)
	}

	// Revert back to unconnected devices.
	if err := c.Run(uDisconnected); err != nil {
		t.Fatal(err)
	}
	// Check that the devices are not syncing.
	if syncbasesCanSync(t, c, d1.Name, d2.Name) {
		t.Fatalf("expected syncbases %v and %v not to sync but they did", d1.Name, d2.Name)
	}
}

func TestRunUniverseSingleDeviceWithOneClient(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	var counter int32
	keyValueFunc := func(_ time.Time) (string, interface{}) {
		// Stop writing after 5 rows.
		if counter >= 5 {
			return "", nil
		}
		counter++
		return fmt.Sprintf("%d", counter), counter
	}

	// mu is locked until Writer writes 5 rows.
	mu := sync.Mutex{}
	mu.Lock()
	keysWritten := 0
	onWrite := func(_ syncbase.Collection, _ string, _ interface{}, err error) {
		keysWritten++
		if err != nil {
			t.Fatalf("error encountered during write: %v", err)
		}
		if keysWritten == 5 {
			mu.Unlock()
		}
	}

	control.RegisterClient("test-writer", func() client.Client {
		return &client.Writer{
			WriteInterval: 50 * time.Millisecond,
			KeyValueFunc:  keyValueFunc,
			OnWrite:       onWrite,
		}
	})
	defer control.InternalResetClientRegistry()

	dbModel := &model.Database{
		Name:     "test_db",
		Blessing: "root",
		Collections: []model.Collection{
			model.Collection{
				Name:     "test_col",
				Blessing: "root",
			},
		},
	}
	u := &model.Universe{
		Users: model.UserSet{
			&model.User{
				Name: "test-user",
				Devices: model.DeviceSet{
					&model.Device{
						Name:      "test-device",
						Clients:   []string{"test-writer"},
						Databases: model.DatabaseSet{dbModel},
					},
				},
			},
		},
	}

	// Start controller and wait a for Writer's KeyValueFunc to run 4 times.
	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}

	// Wait for Writer to write 5 values.
	mu.Lock()

	// Get a context with the same principal and blessings as the instance.
	ctx := c.InternalCtx()
	instCtx, err := v23.WithPrincipal(ctx, c.InternalGetInstance("test-device").InternalPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	sbService := syncbase.NewService("test-device")
	db := sbService.DatabaseForId(dbModel.Id(), nil)
	col := db.CollectionForId(dbModel.Collections[0].Id())
	stream := col.Scan(instCtx, syncbase.Range("", ""))

	// Check that at least 4 rows have been written.  Note that we don't check
	// for 5 keys because although the KeyValueFunc has run 5 times, we arn't
	// guaranteed that the last write has finished.
	for i := 0; i < 4; i++ {
		advance := stream.Advance()
		if stream.Err() != nil {
			t.Fatalf("stream.Err(): %v", stream.Err())
		}
		if !advance {
			t.Fatalf("expected scan to find at least 4 rows but only got %d", i)
		}
	}
	stream.Cancel()
}

func TestRunUniverseSingleDeviceWithTwoClients(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	control.RegisterClient("test-writer", func() client.Client {
		return &client.Writer{
			WriteInterval: 50 * time.Millisecond,
		}
	})

	// mu is locked until 5 changes have been received by the watcher.
	changesReceived := 0
	mu := sync.Mutex{}
	mu.Lock()

	control.RegisterClient("test-watcher", func() client.Client {
		return &client.Watcher{
			OnChange: func(wc syncbase.WatchChange) {
				if wc.EntityType == syncbase.EntityRow {
					changesReceived++
					if changesReceived == 5 {
						mu.Unlock()
					}
				}
			},
		}
	})
	defer control.InternalResetClientRegistry()

	// Construct model with one device and two clients (writer and watcher).
	dbModel := &model.Database{
		Name:     "test_db",
		Blessing: "root",
		Collections: []model.Collection{
			model.Collection{
				Name:     "test_col",
				Blessing: "root",
			},
		},
	}
	devModel := &model.Device{
		Name:      "test-device",
		Clients:   []string{"test-writer", "test-watcher"},
		Databases: model.DatabaseSet{dbModel},
	}
	u := &model.Universe{
		Users: model.UserSet{
			&model.User{
				Name:    "test-user",
				Devices: model.DeviceSet{devModel},
			},
		},
	}

	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher to receive 5 changes.
	mu.Lock()
}

func TestRunUniverseTwoDevicesWithClients(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	// mu is locked until 5 changes have been received by the watcher.
	changesReceived := 0
	mu := sync.Mutex{}
	mu.Lock()
	control.RegisterClient("test-watcher", func() client.Client {
		return &client.Watcher{
			OnChange: func(wc syncbase.WatchChange) {
				if wc.EntityType == syncbase.EntityRow {
					changesReceived++
					if changesReceived == 5 {
						mu.Unlock()
					}
				}
			},
		}
	})
	control.RegisterClient("test-writer", func() client.Client {
		return &client.Writer{
			WriteInterval: 50 * time.Millisecond,
		}
	})
	defer control.InternalResetClientRegistry()

	// Construct model with two devices and one client each (one writer and one
	// watcher).
	user := &model.User{
		Name: "test-user",
	}
	users := model.UserSet{user}
	permsDb := model.Permissions{
		"Admin":   users,
		"Read":    users,
		"Resolve": users,
		"Write":   users,
	}
	permsCx := permsDb.FilterTags(wire.AllCollectionTags...)
	permsSg := permsDb.FilterTags(wire.AllSyncgroupTags...)
	dbModel := &model.Database{
		Name:        "test_db",
		Blessing:    "root",
		Permissions: permsDb,
		Collections: []model.Collection{
			model.Collection{
				Name:        "test_col",
				Blessing:    "root",
				Permissions: permsCx,
			},
		},
	}
	writerDev := &model.Device{
		Name:      "writer-device",
		Clients:   []string{"test-writer"},
		Databases: model.DatabaseSet{dbModel},
	}
	watcherDev := &model.Device{
		Name:      "watcher-device",
		Clients:   []string{"test-watcher"},
		Databases: model.DatabaseSet{dbModel},
	}
	user.Devices = model.DeviceSet{writerDev, watcherDev}

	// Construct a syncgroup and add it to the database.
	sg := model.Syncgroup{
		HostDevice:     writerDev,
		Name:           "test_sg",
		Blessing:       "root",
		Collections:    dbModel.Collections,
		Permissions:    permsSg,
		CreatorDevices: model.DeviceSet{writerDev},
		JoinerDevices:  model.DeviceSet{watcherDev},
	}
	dbModel.Syncgroups = []model.Syncgroup{sg}

	u := &model.Universe{
		Users: model.UserSet{user},
		// Both devices can talk to each other.
		Topology: model.Topology{
			writerDev:  model.DeviceSet{watcherDev, writerDev},
			watcherDev: model.DeviceSet{watcherDev, writerDev},
		},
	}

	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher to receive 5 changes.
	mu.Lock()
}

func TestRunUniverseTwoUsers(t *testing.T) {
	c, cleanup := newController(t)
	defer cleanup()

	// mu is locked until 5 changes have been received by the watcher.
	changesReceived := 0
	mu := sync.Mutex{}
	mu.Lock()
	control.RegisterClient("test-watcher", func() client.Client {
		return &client.Watcher{
			OnChange: func(wc syncbase.WatchChange) {
				changesReceived++
				if changesReceived == 5 {
					mu.Unlock()
				}
			},
		}
	})
	control.RegisterClient("test-writer", func() client.Client {
		return &client.Writer{
			WriteInterval: 50 * time.Millisecond,
		}
	})
	defer control.InternalResetClientRegistry()

	// Construct model with two users, one device each.  One device has watcher
	// client and other has writer.
	userAlice := &model.User{Name: "user-alice"}
	userBob := &model.User{Name: "user-bob"}

	users := model.UserSet{userAlice, userBob}
	permsDb := model.Permissions{
		"Admin":   users,
		"Read":    users,
		"Resolve": users,
		"Write":   users,
	}

	// Alice has all permissions on collection, and gives Bob read access.
	permsAliceCx := model.Permissions{
		"Admin": model.UserSet{userAlice},
		"Read":  model.UserSet{userAlice, userBob},
		"Write": model.UserSet{userAlice},
	}
	permsAliceSg := permsAliceCx.FilterTags(wire.AllSyncgroupTags...)

	// Alice is creator of the collection.
	dbModel := &model.Database{
		Name:        "test_db",
		Blessing:    "root",
		Permissions: permsDb,
		Collections: []model.Collection{
			model.Collection{
				Name:        "test_col",
				Blessing:    "root",
				Permissions: permsAliceCx,
			},
		},
	}

	devAliceWriter := &model.Device{
		Name:      "device-alice-writer",
		Clients:   []string{"test-writer"},
		Databases: model.DatabaseSet{dbModel},
	}
	devBobWatcher := &model.Device{
		Name:      "device-bob-watcher",
		Clients:   []string{"test-watcher"},
		Databases: model.DatabaseSet{dbModel},
	}

	// Alice has the writer device.
	userAlice.Devices = model.DeviceSet{devAliceWriter}
	// Bob has the watcher device.
	userBob.Devices = model.DeviceSet{devBobWatcher}

	// Construct a syncgroup and add it to the database.
	sg := model.Syncgroup{
		HostDevice:     devAliceWriter,
		Name:           "test_sg",
		Blessing:       "root",
		Collections:    dbModel.Collections,
		Permissions:    permsAliceSg,
		CreatorDevices: model.DeviceSet{devAliceWriter},
		JoinerDevices:  model.DeviceSet{devBobWatcher},
	}
	dbModel.Syncgroups = []model.Syncgroup{sg}

	u := &model.Universe{
		Users: model.UserSet{userAlice, userBob},
		// Both devices can talk to each other.
		Topology: model.Topology{
			devAliceWriter: model.DeviceSet{devBobWatcher, devAliceWriter},
			devBobWatcher:  model.DeviceSet{devBobWatcher, devAliceWriter},
		},
	}

	if err := c.Run(u); err != nil {
		t.Fatal(err)
	}

	// Wait for watcher to receive 5 changes.
	mu.Lock()

	// Check that Bob gets ErrNoAccess when writing to the collection on his
	// own device because he does not have write permissions.
	bobCtx, err := c.InternalSetContextBlessings(c.InternalCtx(), "u"+security.ChainSeparator+userBob.Name)
	if err != nil {
		t.Fatal(err)
	}
	s := syncbase.NewService(devBobWatcher.Name)
	db := s.DatabaseForId(dbModel.Id(), nil)
	col := db.CollectionForId(dbModel.Collections[0].Id())
	if err = col.Put(bobCtx, "test-key", "should fail"); verror.ErrorID(err) != verror.ErrNoAccess.ID {
		t.Errorf("expected bob's put to collection %v to fail with ErrNoAccess, but got %v", col, err)
	}
}

var counter int

// TODO(nlacasse): Once the controller has more client-logic built-in for
// creating databases, collections, syncgroups, etc., see if this test can be
// simplified.
func syncbasesCanSync(t *testing.T, c *control.Controller, sb1Name, sb2Name string) bool {
	ctx := c.InternalCtx()
	blessing := security.DefaultBlessingNames(v23.GetPrincipal(ctx))[0]
	sb1Service, sb2Service := syncbase.NewService(sb1Name), syncbase.NewService(sb2Name)

	openPermsDb := testutil.DefaultPerms(wire.AllDatabaseTags, "...")
	openPermsCx := testutil.DefaultPerms(wire.AllCollectionTags, "...")
	openPermsSg := testutil.DefaultPerms(wire.AllSyncgroupTags, "...")

	// Create databases on both syncbase servers.
	counter++
	dbName := fmt.Sprintf("test_database_%d", counter)
	sb1Db := sb1Service.DatabaseForId(wire.Id{Blessing: blessing, Name: dbName}, nil)
	if err := sb1Db.Create(ctx, openPermsDb); err != nil {
		t.Fatal(err)
	}
	sb2Db := sb2Service.DatabaseForId(wire.Id{Blessing: blessing, Name: dbName}, nil)
	if err := sb2Db.Create(ctx, openPermsDb); err != nil {
		t.Fatal(err)
	}

	// Create collections on both syncbase servers.
	counter++
	colName := fmt.Sprintf("test_collection_%d", counter)
	sb1Col := sb1Db.CollectionForId(wire.Id{Blessing: blessing, Name: colName})
	if err := sb1Col.Create(ctx, openPermsCx); err != nil {
		t.Fatal(err)
	}
	sb2Col := sb2Db.CollectionForId(wire.Id{Blessing: blessing, Name: colName})
	if err := sb2Col.Create(ctx, openPermsCx); err != nil {
		t.Fatal(err)
	}

	// Create a syncgroup on the first syncbase.
	counter++
	sgName := fmt.Sprintf("test_sg_%d", counter)
	mounttable := v23.GetNamespace(ctx).Roots()[0]
	sbSpec := wire.SyncgroupSpec{
		Description: "test syncgroup",
		Perms:       openPermsSg,
		Collections: []wire.Id{sb1Col.Id()},
		MountTables: []string{mounttable},
	}
	sb1Sg := sb1Db.SyncgroupForId(wire.Id{Blessing: blessing, Name: sgName})
	if err := sb1Sg.Create(ctx, sbSpec, wire.SyncgroupMemberInfo{}); err != nil {
		t.Fatal(err)
	}

	// If second syncbase can join the syncgroup, they are connected.
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sb2Sg := sb2Db.SyncgroupForId(wire.Id{Blessing: blessing, Name: sgName})
	_, err := sb2Sg.Join(ctxWithTimeout, sb1Name, nil, wire.SyncgroupMemberInfo{})
	return err == nil
}
