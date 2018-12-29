// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"io/ioutil"
	"os"
	"testing"

	"v.io/v23/services/device"
)

// TestInstallationState verifies the state transition logic for app installations.
func TestInstallationState(t *testing.T) {
	dir, err := ioutil.TempDir("", "installation")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	// Uninitialized state.
	if transitionInstallation(dir, device.InstallationStateActive, device.InstallationStateUninstalled) == nil {
		t.Fatalf("transitionInstallation should have failed")
	}
	if s, err := getInstallationState(dir); err == nil {
		t.Fatalf("getInstallationState should have failed, got state %v instead", s)
	}
	if isActive, isUninstalled := installationStateIs(dir, device.InstallationStateActive), installationStateIs(dir, device.InstallationStateUninstalled); isActive || isUninstalled {
		t.Fatalf("isActive, isUninstalled = %t, %t (expected false, false)", isActive, isUninstalled)
	}
	// Initialize.
	if err := initializeInstallation(dir, device.InstallationStateActive); err != nil {
		t.Fatalf("initializeInstallation failed: %v", err)
	}
	if !installationStateIs(dir, device.InstallationStateActive) {
		t.Fatalf("Installation state expected to be %v", device.InstallationStateActive)
	}
	if s, err := getInstallationState(dir); s != device.InstallationStateActive || err != nil {
		t.Fatalf("getInstallationState expected (%v, %v), got (%v, %v) instead", device.InstallationStateActive, nil, s, err)
	}
	if err := transitionInstallation(dir, device.InstallationStateActive, device.InstallationStateUninstalled); err != nil {
		t.Fatalf("transitionInstallation failed: %v", err)
	}
	if !installationStateIs(dir, device.InstallationStateUninstalled) {
		t.Fatalf("Installation state expected to be %v", device.InstallationStateUninstalled)
	}
	if s, err := getInstallationState(dir); s != device.InstallationStateUninstalled || err != nil {
		t.Fatalf("getInstallationState expected (%v, %v), got (%v, %v) instead", device.InstallationStateUninstalled, nil, s, err)
	}
	// Invalid transition: wrong initial state.
	if transitionInstallation(dir, device.InstallationStateActive, device.InstallationStateUninstalled) == nil {
		t.Fatalf("transitionInstallation should have failed")
	}
	if !installationStateIs(dir, device.InstallationStateUninstalled) {
		t.Fatalf("Installation state expected to be %v", device.InstallationStateUninstalled)
	}
}

// TestInstanceState verifies the state transition logic for app instances.
func TestInstanceState(t *testing.T) {
	dir, err := ioutil.TempDir("", "instance")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	// Uninitialized state.
	if transitionInstance(dir, device.InstanceStateLaunching, device.InstanceStateRunning) == nil {
		t.Fatalf("transitionInstance should have failed")
	}
	if s, err := getInstanceState(dir); err == nil {
		t.Fatalf("getInstanceState should have failed, got state %v instead", s)
	}
	// Initialize.
	if err := initializeInstance(dir, device.InstanceStateDying); err != nil {
		t.Fatalf("initializeInstance failed: %v", err)
	}
	if s, err := getInstanceState(dir); s != device.InstanceStateDying || err != nil {
		t.Fatalf("getInstanceState expected (%v, %v), got (%v, %v) instead", device.InstanceStateDying, nil, s, err)
	}
	if err := transitionInstance(dir, device.InstanceStateDying, device.InstanceStateNotRunning); err != nil {
		t.Fatalf("transitionInstance failed: %v", err)
	}
	if s, err := getInstanceState(dir); s != device.InstanceStateNotRunning || err != nil {
		t.Fatalf("getInstanceState expected (%v, %v), got (%v, %v) instead", device.InstanceStateNotRunning, nil, s, err)
	}
	// Invalid transition: wrong initial state.
	if transitionInstance(dir, device.InstanceStateDying, device.InstanceStateNotRunning) == nil {
		t.Fatalf("transitionInstance should have failed")
	}
	if err := transitionInstance(dir, device.InstanceStateNotRunning, device.InstanceStateDeleted); err != nil {
		t.Fatalf("transitionInstance failed: %v", err)
	}
	if s, err := getInstanceState(dir); s != device.InstanceStateDeleted || err != nil {
		t.Fatalf("getInstanceState expected (%v, %v), got (%v, %v) instead", device.InstanceStateDeleted, nil, s, err)
	}
}
