// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"strings"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/groups"
	"v.io/x/ref/services/groups/internal/store"
)

type manager struct {
	st               store.Store
	createAuthorizer security.Authorizer
}

// NewManager returns an rpc.Dispatcher implementation for a namespace of groups.
//
// The authorization policy for the creation of new groups will be controlled
// by the provided Authorizer.
func NewManager(st store.Store, auth security.Authorizer) rpc.Dispatcher {
	return &manager{st: st, createAuthorizer: auth}
}

func (m *manager) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	suffix = strings.TrimPrefix(suffix, "/")
	// TODO(sadovsky): Check that suffix is a valid group name.
	// A permissive authorizer (AllowEveryone) is used here since access
	// control happens in the implementation of individual RPC methods. See
	// the implementation of the group operations on the 'group' type.
	return groups.GroupServer(&group{name: suffix, m: m}), security.AllowEveryone(), nil
}
