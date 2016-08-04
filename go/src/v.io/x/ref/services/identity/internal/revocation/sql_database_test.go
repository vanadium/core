// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package revocation

import (
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create new mock database stub: %v", err)
	}
	columns := []string{"ThirdPartyCaveatID", "RevocationCaveatID", "RevocationTime"}
	mock.ExpectPrepare("CREATE TABLE IF NOT EXISTS tableName (.+)").
		ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 1))
	insertCaveatStmt := mock.ExpectPrepare("INSERT INTO tableName (.+) VALUES (.+)")
	revokeStmt := mock.ExpectPrepare("UPDATE tableName SET RevocationTime=.+")
	isRevokedStmt := mock.ExpectPrepare("SELECT 1 FROM tableName")
	revocationTimeStmt := mock.ExpectPrepare("SELECT RevocationTime FROM tableName")
	d, err := newSQLDatabase(db, "tableName")
	if err != nil {
		t.Fatalf("failed to create SQLDatabase: %v", err)
	}

	tpCavID, revCavID := "tpCavID", []byte("revCavID")
	tpCavID2, revCavID2 := "tpCavID2", []byte("revCavID2")
	encRevCavID := hex.EncodeToString(revCavID)
	encRevCavID2 := hex.EncodeToString(revCavID2)
	insertCaveatStmt.ExpectExec().
		WithArgs(tpCavID, encRevCavID).
		WillReturnResult(sqlmock.NewResult(0, 1)) // no insert id, 1 affected row
	if err := d.InsertCaveat(tpCavID, revCavID); err != nil {
		t.Errorf("failed to InsertCaveat into SQLDatabase: %v", err)
	}

	insertCaveatStmt.ExpectExec().
		WithArgs(tpCavID2, encRevCavID2).
		WillReturnResult(sqlmock.NewResult(0, 1)) // no insert id, 1 affected row
	if err := d.InsertCaveat(tpCavID2, revCavID2); err != nil {
		t.Errorf("second InsertCaveat into SQLDatabase failed: %v", err)
	}

	// Test Revocation
	revokeStmt.ExpectExec().
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := d.Revoke(tpCavID); err != nil {
		t.Errorf("failed to Revoke Caveat: %v", err)
	}

	// Test IsRevoked returns true.
	isRevokedStmt.ExpectQuery().
		WithArgs(encRevCavID).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(1, 1, 1))
	if revoked, err := d.IsRevoked(revCavID); err != nil || !revoked {
		t.Errorf("expected revCavID to be revoked: err: (%v)", err)
	}

	// Test IsRevoked returns false.
	isRevokedStmt.ExpectQuery().
		WithArgs(encRevCavID2).
		WillReturnRows(sqlmock.NewRows(columns))
	if revoked, err := d.IsRevoked(revCavID2); err != nil || revoked {
		t.Errorf("expected revCavID to not be revoked: err: (%v)", err)
	}

	// Test RevocationTime.
	revocationTime := time.Now()
	revocationTimeStmt.ExpectQuery().
		WithArgs(tpCavID).
		WillReturnRows(sqlmock.NewRows([]string{"RevocationTime"}).AddRow(revocationTime))
	if got, err := d.RevocationTime(tpCavID); err != nil || !reflect.DeepEqual(*got, revocationTime) {
		t.Errorf("got %v, expected %v: err : %v", got, revocationTime, err)
	}
}
