// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lib

import (
	"fmt"
	"strings"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/services/groups/internal/server"
	"v.io/x/ref/services/groups/internal/store/mem"
)

// Authorizer implementing the authorization policy for Create operations.
//
// A user is allowed to create any group that begins with the user id.
//
// TODO(ashankar): This is experimental use of the "conventions" API and of a
// creation policy. This policy was thought of in a 5 minute period. Think
// about this more!
type createAuthorizer struct{}

func (createAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	userids := conventions.GetClientUserIds(ctx, call)
	for _, uid := range userids {
		if strings.HasPrefix(call.Suffix(), uid+"/") {
			return nil
		}
	}
	// Revert to the default authorization policy.
	if err := security.DefaultAuthorizer().Authorize(ctx, call); err == nil {
		return nil
	}
	return fmt.Errorf("creator user ids %v are not authorized to create group %v, group name must begin with one of the user ids", userids, call.Suffix())
}

// NewGroupsDispatcher creates a new dispatcher for the groups service.
//
// rootDir is the directory for persisting groups.
//
// engine is the storage engine for groups.  Currently, only "memstore"
// is supported.
func NewGroupsDispatcher(rootDir, engine string) (rpc.Dispatcher, error) {
	switch engine {
	case "memstore":
		return server.NewManager(mem.New(), createAuthorizer{}), nil
	default:
		return nil, fmt.Errorf("unknown storage engine %v", engine)
	}
}
