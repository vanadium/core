// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/services/build"
	"v.io/x/ref/services/profile"
	"v.io/x/ref/services/repository"
	"v.io/x/ref/test"
)

var (
	// spec is an example profile specification used throughout the test.
	spec = profile.Specification{
		Arch:        build.ArchitectureAmd64,
		Description: "Example profile to test the profile repository implementation.",
		Format:      build.FormatElf,
		Libraries:   map[profile.Library]struct{}{profile.Library{Name: "foo", MajorVersion: "1", MinorVersion: "0"}: struct{}{}},
		Label:       "example",
		Os:          build.OperatingSystemLinux,
	}
)

// TestInterface tests that the implementation correctly implements
// the Profile interface.
func TestInterface(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	dir, prefix := "", ""
	store, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%q, %q) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(store)
	dispatcher, err := NewDispatcher(store, nil)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stub := repository.ProfileClient(naming.JoinAddressName(endpoint, "linux/base"))
	// Put
	if err := stub.Put(ctx, spec); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	// Label
	label, err := stub.Label(ctx)
	if err != nil {
		t.Fatalf("Label() failed: %v", err)
	}
	if label != spec.Label {
		t.Fatalf("Unexpected output: expected %v, got %v", spec.Label, label)
	}

	// Description
	description, err := stub.Description(ctx)
	if err != nil {
		t.Fatalf("Description() failed: %v", err)
	}
	if description != spec.Description {
		t.Fatalf("Unexpected output: expected %v, got %v", spec.Description, description)
	}

	// Specification
	specification, err := stub.Specification(ctx)
	if err != nil {
		t.Fatalf("Specification() failed: %v", err)
	}
	if !reflect.DeepEqual(spec, specification) {
		t.Fatalf("Unexpected output: expected %v, got %v", spec, specification)
	}

	// Remove
	if err := stub.Remove(ctx); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
}

func TestPreserveAcrossRestarts(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	dir, prefix := "", ""
	store, err := ioutil.TempDir(dir, prefix)
	if err != nil {
		t.Fatalf("TempDir(%q, %q) failed: %v", dir, prefix, err)
	}
	defer os.RemoveAll(store)
	dispatcher, err := NewDispatcher(store, nil)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}
	sctx, cancel := context.WithCancel(ctx)
	_, server, err := v23.WithNewDispatchingServer(sctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	endpoint := server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stub := repository.ProfileClient(naming.JoinAddressName(endpoint, "linux/base"))

	if err := stub.Put(ctx, spec); err != nil {
		t.Fatalf("Put() failed: %v", err)
	}

	label, err := stub.Label(ctx)
	if err != nil {
		t.Fatalf("Label() failed: %v", err)
	}
	if label != spec.Label {
		t.Fatalf("Unexpected output: expected %v, got %v", spec.Label, label)
	}

	// Stop the first server.
	cancel()
	<-server.Closed()

	// Setup and start a second server.
	dispatcher, err = NewDispatcher(store, nil)
	if err != nil {
		t.Fatalf("NewDispatcher() failed: %v", err)
	}
	_, server, err = v23.WithNewDispatchingServer(ctx, "", dispatcher)
	if err != nil {
		t.Fatalf("NewServer() failed: %v", err)
	}
	endpoint = server.Status().Endpoints[0].String()

	// Create client stubs for talking to the server.
	stub = repository.ProfileClient(naming.JoinAddressName(endpoint, "linux/base"))

	// Label
	label, err = stub.Label(ctx)
	if err != nil {
		t.Fatalf("Label() failed: %v", err)
	}
	if label != spec.Label {
		t.Fatalf("Unexpected output: expected %v, got %v", spec.Label, label)
	}
}
