// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package revocation

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type database interface {
	InsertCaveat(thirdPartyCaveatID string, revocationCaveatID []byte) error
	Revoke(thirdPartyCaveatID string) error
	IsRevoked(revocationCaveatID []byte) (bool, error)
	RevocationTime(thirdPartyCaveatID string) (*time.Time, error)
}

// Table with 3 columns:
// (1) ThirdPartyCaveatID= string thirdPartyCaveatID.
// (2) RevocationCaveatID= hex encoded revcationCaveatID.
// (3) RevocationTime= time (if any) that the Caveat was revoked.
type sqlDatabase struct {
	insertCaveatStmt, revokeStmt, isRevokedStmt, revocationTimeStmt *sql.Stmt
}

func (s *sqlDatabase) InsertCaveat(thirdPartyCaveatID string, revocationCaveatID []byte) error {
	_, err := s.insertCaveatStmt.Exec(thirdPartyCaveatID, hex.EncodeToString(revocationCaveatID))
	return err
}

func (s *sqlDatabase) Revoke(thirdPartyCaveatID string) error {
	_, err := s.revokeStmt.Exec(time.Now(), thirdPartyCaveatID)
	return err
}

func (s *sqlDatabase) IsRevoked(revocationCaveatID []byte) (bool, error) {
	rows, err := s.isRevokedStmt.Query(hex.EncodeToString(revocationCaveatID))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), err
}

func (s *sqlDatabase) RevocationTime(thirdPartyCaveatID string) (*time.Time, error) {
	rows, err := s.revocationTimeStmt.Query(thirdPartyCaveatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var timestamp time.Time
		if err := rows.Scan(&timestamp); err != nil {
			return nil, err
		}
		return &timestamp, nil
	}
	return nil, fmt.Errorf("the caveat (%v) was not revoked", thirdPartyCaveatID)
}

func newSQLDatabase(db *sql.DB, table string) (database, error) {
	createStmt, err := db.Prepare(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ( ThirdPartyCaveatID NVARCHAR(255), RevocationCaveatID NVARCHAR(255), RevocationTime DATETIME, PRIMARY KEY (ThirdPartyCaveatID), KEY (RevocationCaveatID) );", table))
	if err != nil {
		return nil, err
	}
	if _, err = createStmt.Exec(); err != nil {
		return nil, err
	}
	insertCaveatStmt, err := db.Prepare(fmt.Sprintf("INSERT INTO %s (ThirdPartyCaveatID, RevocationCaveatID, RevocationTime) VALUES (?, ?, NULL)", table))
	if err != nil {
		return nil, err
	}
	revokeStmt, err := db.Prepare(fmt.Sprintf("UPDATE %s SET RevocationTime=? WHERE ThirdPartyCaveatID=?", table))
	if err != nil {
		return nil, err
	}
	isRevokedStmt, err := db.Prepare(fmt.Sprintf("SELECT 1 FROM %s WHERE RevocationCaveatID=? AND RevocationTime IS NOT NULL", table))
	if err != nil {
		return nil, err
	}
	revocationTimeStmt, err := db.Prepare(fmt.Sprintf("SELECT RevocationTime FROM %s WHERE ThirdPartyCaveatID=?", table))
	return &sqlDatabase{insertCaveatStmt, revokeStmt, isRevokedStmt, revocationTimeStmt}, err
}
