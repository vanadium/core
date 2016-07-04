// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package auditor

import (
	"reflect"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	_ "v.io/x/ref/runtime/factories/fake"
	"v.io/x/ref/test"
)

func TestSQLDatabaseQuery(t *testing.T) {
	ctx, cancel := test.TestContext()
	defer cancel()
	db, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create new mock database stub: %v", err)
	}
	columns := []string{"Email", "Caveat", "Timestamp", "Blessings"}
	sqlmock.ExpectExec("CREATE TABLE IF NOT EXISTS tableName (.+)").
		WillReturnResult(sqlmock.NewResult(0, 1))
	d, err := newSQLDatabase(ctx, db, "tableName")
	if err != nil {
		t.Fatalf("failed to create SQLDatabase: %v", err)
	}

	entry := databaseEntry{
		email:     "email",
		caveats:   []byte("caveats"),
		timestamp: time.Now(),
		blessings: []byte("blessings"),
	}
	sqlmock.ExpectExec("INSERT INTO tableName (.+) VALUES (.+)").
		WithArgs(entry.email, entry.caveats, entry.timestamp, entry.blessings).
		WillReturnResult(sqlmock.NewResult(0, 1)) // no insert id, 1 affected row
	if err := d.Insert(ctx, entry); err != nil {
		t.Errorf("failed to insert into SQLDatabase: %v", err)
	}

	// Test the querying.
	sqlmock.ExpectQuery("SELECT Email, Caveats, Timestamp, Blessings FROM tableName").
		WithArgs(entry.email).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(entry.email, entry.caveats, entry.timestamp, entry.blessings))
	ch := d.Query(ctx, entry.email)
	if res := <-ch; !reflect.DeepEqual(res, entry) {
		t.Errorf("got %#v, expected %#v", res, entry)
	}

	var extra bool
	for _ = range ch {
		// Drain the channel to prevent the producer goroutines from being leaked.
		extra = true
	}
	if extra {
		t.Errorf("Got more entries that expected")
	}
}
