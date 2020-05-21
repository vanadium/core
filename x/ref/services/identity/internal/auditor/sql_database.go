// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package auditor

import (
	"database/sql"
	"fmt"
	"time"

	"v.io/v23/context"
)

type database interface {
	Insert(ctx *context.T, entry databaseEntry) error
	Query(ctx *context.T, email string) <-chan databaseEntry
}

type databaseEntry struct {
	email              string
	caveats, blessings []byte
	timestamp          time.Time
	decodeErr          error
}

// newSQLDatabase returns a SQL implementation of the database interface.
// If the table does not exist it creates it.
func newSQLDatabase(ctx *context.T, db *sql.DB, table string) (database, error) {
	createStmt, err := db.Prepare(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ( Email VARBINARY(256), Caveats BLOB, Timestamp DATETIME, Blessings BLOB, KEY (Email, Timestamp) );", table))
	if err != nil {
		return nil, err
	}
	if _, err = createStmt.Exec(); err != nil {
		return nil, err
	}
	insertStmt, err := db.Prepare(fmt.Sprintf("INSERT INTO %s (Email, Caveats, Timestamp, Blessings) VALUES (?, ?, ?, ?)", table))
	if err != nil {
		return nil, err
	}
	queryStmt, err := db.Prepare(fmt.Sprintf("SELECT Email, Caveats, Timestamp, Blessings FROM %s WHERE Email=? ORDER BY Timestamp DESC", table))
	return sqlDatabase{
		insertStmt: insertStmt,
		queryStmt:  queryStmt,
	}, err
}

// Table with 4 columns:
// (1) Email = string email of the Blessee.
// (2) Caveats = vom encoded caveats
// (3) Blessings = vom encoded resulting blessings.
// (4) Timestamp = time that the blessing happened.
type sqlDatabase struct {
	insertStmt, queryStmt *sql.Stmt
}

func (s sqlDatabase) Insert(ctx *context.T, entry databaseEntry) error {
	_, err := s.insertStmt.Exec(entry.email, entry.caveats, entry.timestamp, entry.blessings)
	return err
}

func (s sqlDatabase) Query(ctx *context.T, email string) <-chan databaseEntry {
	c := make(chan databaseEntry)
	go s.sendDatabaseEntries(ctx, email, c)
	return c
}

func (s sqlDatabase) sendDatabaseEntries(ctx *context.T, email string, dst chan<- databaseEntry) {
	defer close(dst)
	rows, err := s.queryStmt.Query(email)
	if err != nil {
		ctx.Errorf("query failed %v", err)
		dst <- databaseEntry{decodeErr: fmt.Errorf("Failed to query for all audits: %v", err)}
		return
	}
	defer rows.Close()
	for rows.Next() {
		var dbentry databaseEntry
		if err = rows.Scan(&dbentry.email, &dbentry.caveats, &dbentry.timestamp, &dbentry.blessings); err != nil {
			ctx.Errorf("scan of row failed %v", err)
			dbentry.decodeErr = fmt.Errorf("failed to read sql row, %s", err)
		}
		dst <- dbentry
	}
}
