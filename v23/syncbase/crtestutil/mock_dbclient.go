// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crtestutil

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

type MockWireDatabaseClient struct {
	wire.DatabaseClientMethods
	crs *CrStreamImpl
}

func MockDbClient(c wire.DatabaseClientMethods, crs *CrStreamImpl) *MockWireDatabaseClient {
	return &MockWireDatabaseClient{c, crs}
}

func (wclient *MockWireDatabaseClient) GetSchemaMetadata(ctx *context.T, opts ...rpc.CallOpt) (sm wire.SchemaMetadata, err error) {
	err = verror.NewErrNoExist(ctx)
	return
}

func (wclient *MockWireDatabaseClient) SetSchemaMetadata(ctx *context.T, i0 wire.SchemaMetadata, opts ...rpc.CallOpt) error {
	return nil
}

func (wclient *MockWireDatabaseClient) StartConflictResolver(ctx *context.T, opts ...rpc.CallOpt) (wire.ConflictManagerStartConflictResolverClientCall, error) {
	return wclient.crs, nil
}
