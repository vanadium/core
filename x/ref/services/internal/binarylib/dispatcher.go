// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

import (
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/repository"
	"v.io/x/ref/services/internal/pathperms"
)

const (
	VersionFile = "VERSION"
	Version     = "1.0"
)

// dispatcher holds the state of the binary repository dispatcher.
type dispatcher struct {
	state      *state
	permsStore *pathperms.PathStore
}

// NewDispatcher is the dispatcher factory.
func NewDispatcher(ctx *context.T, state *state) (rpc.Dispatcher, error) {
	return &dispatcher{
		state:      state,
		permsStore: pathperms.NewPathStore(ctx),
	}, nil
}

// DISPATCHER INTERFACE IMPLEMENTATION

func permsPath(rootDir, suffix string) string {
	var dir string
	if suffix == "" {
		// Directory is in namespace overlapped with Vanadium namespace
		// so hide it.
		dir = filepath.Join(rootDir, "__acls")
	} else {
		dir = filepath.Join(rootDir, suffix, "acls")
	}
	return dir
}

func newAuthorizer(rootDir, suffix string, permsStore *pathperms.PathStore) (security.Authorizer, error) {
	return pathperms.NewHierarchicalAuthorizer(
		permsPath(rootDir, ""),
		permsPath(rootDir, suffix),
		permsStore)
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	auth, err := newAuthorizer(d.state.rootDir, suffix, d.permsStore)
	if err != nil {
		return nil, nil, err
	}
	return repository.BinaryServer(newBinaryService(d.state, suffix, d.permsStore)), auth, nil
}
