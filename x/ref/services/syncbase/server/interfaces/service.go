// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package interfaces

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/store"
)

// Service is an internal interface to the service layer.
type Service interface {
	// St returns the storage engine instance for this service.
	St() store.Store

	// Sync returns the sync instance for this service.
	Sync() SyncServerMethods

	// Database returns a handle for the specified database.
	Database(ctx *context.T, call rpc.ServerCall, dbId wire.Id) (Database, error)

	// DatabaseIds returns ids for all databases.
	DatabaseIds(ctx *context.T, call rpc.ServerCall) ([]wire.Id, error)
}
