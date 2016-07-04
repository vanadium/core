// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
)

func NewService(fullName string) Service {
	return &service{
		c:        wire.ServiceClient(fullName),
		fullName: fullName,
	}
}

type service struct {
	c        wire.ServiceClientMethods
	fullName string
}

var _ Service = (*service)(nil)

// FullName implements Service.FullName.
func (s *service) FullName() string {
	return s.fullName
}

// Database implements Service.Database.
func (s *service) Database(ctx *context.T, name string, schema *Schema) Database {
	app, _, err := util.AppAndUserPatternFromBlessings(security.DefaultBlessingNames(v23.GetPrincipal(ctx))...)
	if err != nil {
		ctx.Error(verror.New(wire.ErrInferAppBlessingFailed, ctx, "Database", name, err))
		// A handle with a no-match Id blessing is returned, so all RPCs will fail.
		// TODO(ivanpi): Return the more specific error from RPCs instead of logging
		// it here.
	}
	return newDatabase(s.fullName, wire.Id{Blessing: string(app), Name: name}, schema)
}

// DatabaseForId implements Service.DatabaseForId.
func (s *service) DatabaseForId(id wire.Id, schema *Schema) Database {
	return newDatabase(s.fullName, id, schema)
}

// ListDatabases implements Service.ListDatabases.
func (s *service) ListDatabases(ctx *context.T) ([]wire.Id, error) {
	return util.ListChildIds(ctx, s.fullName)
}

// SetPermissions implements Service.SetPermissions.
func (s *service) SetPermissions(ctx *context.T, perms access.Permissions, version string) error {
	return s.c.SetPermissions(ctx, perms, version)
}

// GetPermissions implements Service.GetPermissions.
func (s *service) GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error) {
	return s.c.GetPermissions(ctx)
}
