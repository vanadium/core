// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fake provides a fake version of syncbase API for testing
// with error conditions.
package fake

import (
	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
)

// Service returns a fake implementation, returning the given errors (which may
// be nil).
func Service(listCollectionsErr, specErr error) syncbase.Service {
	return &service{listCollectionsErr: listCollectionsErr, specErr: specErr}
}

// service implements the syncbase.Service interface.
type service struct {
	common
	listCollectionsErr error
	specErr            error
}

func (*service) FullName() string { return "the-service-name" }

func (s *service) Database(*context.T, string, *syncbase.Schema) syncbase.Database {
	return &database{
		listCollectionsErr: s.listCollectionsErr,
		specErr:            s.specErr,
	}
}

func (s *service) DatabaseForId(wire.Id, *syncbase.Schema) syncbase.Database {
	return s.Database(nil, "", nil)
}

func (*service) ListDatabases(*context.T) ([]wire.Id, error) { return []wire.Id{wire.Id{}}, nil }
