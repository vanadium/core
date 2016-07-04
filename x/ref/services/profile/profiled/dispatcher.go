// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/services/internal/fs"
	"v.io/x/ref/services/repository"
)

// dispatcher holds the state of the profile repository dispatcher.
type dispatcher struct {
	store     *fs.Memstore
	auth      security.Authorizer
	storeRoot string
}

var _ rpc.Dispatcher = (*dispatcher)(nil)

// NewDispatcher is the dispatcher factory. storeDir is a path to a
// directory in which the profile state is persisted.
func NewDispatcher(storeDir string, authorizer security.Authorizer) (rpc.Dispatcher, error) {
	store, err := fs.NewMemstore(filepath.Join(storeDir, "profilestate.db"))
	if err != nil {
		return nil, err
	}
	return &dispatcher{store: store, storeRoot: storeDir, auth: authorizer}, nil
}

// DISPATCHER INTERFACE IMPLEMENTATION

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return repository.ProfileServer(NewProfileService(d.store, d.storeRoot, suffix)), d.auth, nil
}
