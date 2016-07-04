// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package model_test

import (
	"testing"

	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

func TestGenerateDatabaseSet(t *testing.T) {
	dbs := model.GenerateDatabaseSet(5)
	if want, got := 5, len(dbs); want != got {
		t.Errorf("wanted %v databases but got %v", want, got)
	}

	// Check that all databases have unique names.
	names := map[string]struct{}{}
	for _, db := range dbs {
		names[db.Name] = struct{}{}
	}
	if want, got := 5, len(names); want != got {
		t.Errorf("wanted %v unique database names but got %v", want, got)
	}
}

func TestDatabaseRandomSubset(t *testing.T) {
	dbs := model.GenerateDatabaseSet(5)

	// Create a subset with exactly 2 databases.
	subset1 := dbs.RandomSubset(2, 2)
	if want, got := 2, len(subset1); want != got {
		t.Errorf("wanted a subset of size %v but got %v", want, got)
	}

	// Create a subset with between 2 and 3 databases.
	subset2 := dbs.RandomSubset(2, 3)
	if got := len(subset2); got < 2 || got > 3 {
		t.Errorf("wanted a subset of size between 2 and 3 but got %v", got)
	}

	// Create a subset with exactly 1 database.
	subset3 := dbs.RandomSubset(1, 1)
	if want, got := 1, len(subset3); want != got {
		t.Errorf("wanted a subset of size %v but got %v", want, got)
	}

	// Create a subset with exactly 5 databases.
	subset4 := dbs.RandomSubset(5, 5)
	if want, got := 5, len(subset4); want != got {
		t.Errorf("wanted a subset of size %v but got %v", want, got)
	}
}

func TestGenerateDeviceSet(t *testing.T) {
	dbs := model.GenerateDatabaseSet(5)
	specs := []model.DeviceSpec{model.LaptopSpec, model.PhoneSpec, model.CloudSpec}
	devices := model.GenerateDeviceSet(5, dbs, specs)
	if want, got := 5, len(devices); want != got {
		t.Errorf("wanted %v devices but got %v", want, got)
	}

	// Check that all devices have unique names.
	names := map[string]struct{}{}
	for _, device := range devices {
		names[device.Name] = struct{}{}
	}
	if want, got := 5, len(names); want != got {
		t.Errorf("wanted %v unique device names but got %v", want, got)
	}

	// Check that all devices have non-empty database set, and no more
	// databases than device.Spec.MaxDatabases.
	for _, device := range devices {
		if len(device.Databases) == 0 {
			t.Errorf("device %v has empty database set", device)
		}
		if len(device.Databases) > device.Spec.MaxDatabases {
			t.Errorf("device %v has %v databases, which is more than allowed by device spec %v", device, len(device.Databases), device.Spec)
		}
	}
}

func TestGenerateTopology(t *testing.T) {
	dbs := model.GenerateDatabaseSet(5)
	specs := []model.DeviceSpec{model.LaptopSpec, model.PhoneSpec, model.CloudSpec}
	devices := model.GenerateDeviceSet(5, dbs, specs)

	// With affinity = 0, devices should only be connected to themselves.
	top1 := model.GenerateTopology(devices, 0)
	if want, got := 5, len(top1); want != got {
		t.Errorf("wanted topology to contain %v devices but got %v", want, got)
	}
	for device, connected := range top1 {
		if want, got := len(connected), 1; want != got {
			t.Errorf("wanted device to be connected to %v others, but got %v", want, got)
		}
		if device != connected[0] {
			t.Errorf("wanted device to be connected to itself it wasn't")
		}
	}

	// With affinity = 1, devices should be connected to all others.
	top2 := model.GenerateTopology(devices, 1)
	if want, got := 5, len(top2); want != got {
		t.Errorf("wanted topology to contain %v devices but got %v", want, got)
	}
	for _, connected := range top2 {
		if want, got := 5, len(connected); want != got {
			t.Errorf("wanted device to be connected to %v others, but got %v", want, got)
		}
	}
}

func TestGenerateUser(t *testing.T) {
	// Generate a user with exactly 5 databases and exactly 3 devices.
	dbs := model.GenerateDatabaseSet(5)
	user := model.GenerateUser(dbs, model.UserOpts{
		MaxDatabases: 5,
		MaxDevices:   3,
		MinDevices:   3,
	})
	if want, got := 5, len(user.Databases()); want < got {
		t.Errorf("wanted user to have at most %v databases but got %v", want, got)
	}
	if want, got := 3, len(user.Devices); got != want {
		t.Errorf("wanted user to have %v devices but got %v", want, got)
	}
}

func TestGenerateUniverse(t *testing.T) {
	opts := model.UniverseOpts{
		DeviceAffinity:      0.5,
		MaxDatabases:        10,
		NumUsers:            5,
		MaxDatabasesPerUser: 5,
		MaxDevicesPerUser:   5,
		MinDevicesPerUser:   2,
	}
	universe := model.GenerateUniverse(opts)
	if want, got := opts.MaxDatabases, len(universe.Databases()); want < got {
		t.Errorf("wanted universe to have at most %v databases but got %v", want, got)
	}
	if want, got := opts.NumUsers, len(universe.Users); want != got {
		t.Errorf("wanted universe to have %v users but got %v", want, got)
	}
	for _, user := range universe.Users {
		if got := len(user.Databases()); got > opts.MaxDatabasesPerUser {
			t.Errorf("wanted user to have at most %v databases but got %v", opts.MaxDatabasesPerUser, got)
		}
		if got := len(user.Devices); got < opts.MinDevicesPerUser || got > opts.MaxDevicesPerUser {
			t.Errorf("wanted user to have between %v and %v devices but got %v", opts.MinDevicesPerUser, opts.MaxDevicesPerUser, got)
		}
	}
}
