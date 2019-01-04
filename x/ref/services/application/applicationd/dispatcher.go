// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/ref/services/internal/fs"
	"v.io/x/ref/services/internal/pathperms"
	"v.io/x/ref/services/repository"
)

// dispatcher holds the state of the application repository dispatcher.
type dispatcher struct {
	store     *fs.Memstore
	storeRoot string
}

// NewDispatcher is the dispatcher factory. storeDir is a path to a directory in which to
// serialize the applicationd state.
func NewDispatcher(storeDir string) (rpc.Dispatcher, error) {
	store, err := fs.NewMemstore(filepath.Join(storeDir, "applicationdstate.db"))
	if err != nil {
		return nil, err
	}
	return &dispatcher{store: store, storeRoot: storeDir}, nil
}

func (d *dispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	name, _, err := parse(nil, suffix)
	if err != nil {
		return nil, nil, err
	}

	auth, err := pathperms.NewHierarchicalAuthorizer(
		naming.Join("/acls", "data"),
		naming.Join("/acls", name, "data"),
		(*applicationPermsStore)(d.store))
	if err != nil {
		return nil, nil, err
	}
	return repository.ApplicationServer(NewApplicationService(d.store, d.storeRoot, suffix)), auth, nil
}

type applicationPermsStore fs.Memstore

// PermsForPath implements PermsGetter so that applicationd can use the
// hierarchicalAuthorizer.
func (store *applicationPermsStore) PermsForPath(ctx *context.T, path string) (access.Permissions, bool, error) {
	perms, _, err := getPermissions(ctx, (*fs.Memstore)(store), path)

	if verror.ErrorID(err) == verror.ErrNoExist.ID {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return perms, false, nil
}
