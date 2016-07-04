// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"testing"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// Tests schema checking logic within Service.DatabaseForId() method.
// This test as following steps:
// 1) Call DatabaseForId() for a non existent db.
// 2) Create the database, and verify if Schema got stored properly.
// 3) Call DatabaseForId() on the same db to create a new handle with new
//    schema metadata, call EnforceSchema() and check if the new metadata was
//    stored appropriately.
func TestSchemaCheck(t *testing.T) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	defer cleanup()
	s := syncbase.NewService(sName)
	schema := tu.DefaultSchema(0)

	db1 := s.DatabaseForId(wire.Id{"a", "db1"}, schema)

	if err := db1.EnforceSchema(ctx); err != nil {
		t.Fatalf("db1.EnforceSchema() failed: %v", err)
	}

	// Create db1, this step also stores the schema provided above
	if err := db1.Create(ctx, nil); err != nil {
		t.Fatalf("db1.Create() failed: %v", err)
	}
	// verify if schema was stored as part of create
	if _, err := getSchemaMetadata(ctx, db1.FullName()); err != nil {
		t.Fatalf("Failed to lookup schema after create: %v", err)
	}

	// try to make a new database object for the same database but this time
	// with a new schema version
	schema.Metadata.Version = 1
	rule := wire.CrRule{wire.Id{"u", "c1"}, "foo", "", wire.ResolverTypeLastWins}
	policy := wire.CrPolicy{
		Rules: []wire.CrRule{rule},
	}
	schema.Metadata.Policy = policy
	otherdb1 := s.DatabaseForId(wire.Id{"a", "db1"}, schema)
	if err := otherdb1.EnforceSchema(ctx); err != nil {
		t.Fatalf("otherdb1.EnforceSchema() failed: %v", err)
	}

	// check if the contents of SchemaMetadata are correctly stored in the db.
	metadata, err := getSchemaMetadata(ctx, otherdb1.FullName())
	if err != nil {
		t.Fatalf("GetSchemaMetadata failed: %v", err)
	}
	if metadata.Version != 1 {
		t.Fatalf("Unexpected version number: %d", metadata.Version)
	}
	if len(metadata.Policy.Rules) != 1 {
		t.Fatalf("Unexpected number of rules: %d", len(metadata.Policy.Rules))
	}
	if metadata.Policy.Rules[0] != rule {
		t.Fatalf("Unexpected number of rules: %d", len(metadata.Policy.Rules))
	}
}

func getSchemaMetadata(ctx *context.T, dbName string) (wire.SchemaMetadata, error) {
	return wire.DatabaseClient(dbName).GetSchemaMetadata(ctx)
}
