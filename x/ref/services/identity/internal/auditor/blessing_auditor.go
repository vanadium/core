// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package auditor

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/lib/security/audit"
)

// BlessingLogReader provides the Read method to read audit logs.
// Read returns a channel of BlessingEntrys whose extension matches the provided email.
type BlessingLogReader interface {
	Read(ctx *context.T, email string) <-chan BlessingEntry
}

// BlessingEntry contains important logged information about a blessed principal.
type BlessingEntry struct {
	Email              string
	Caveats            []security.Caveat
	Timestamp          time.Time // Time when the blesings were created.
	RevocationCaveatID string
	Blessings          security.Blessings
	DecodeError        error
}

// NewSQLBlessingAuditor returns an auditor for wrapping a principal with, and a BlessingLogReader
// for reading the audits made by that auditor. The config is used to construct the connection
// to the SQL database that the auditor and BlessingLogReader use.
func NewSQLBlessingAuditor(ctx *context.T, sqlDB *sql.DB) (audit.Auditor, BlessingLogReader, error) {
	db, err := newSQLDatabase(ctx, sqlDB, "BlessingAudit")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create sql db: %v", err)
	}
	auditor, reader := &blessingAuditor{db}, &blessingLogReader{db}
	return auditor, reader, nil
}

type blessingAuditor struct {
	db database
}

func (a *blessingAuditor) Audit(ctx *context.T, entry audit.Entry) error {
	if entry.Method != "Bless" {
		return nil
	}
	dbentry, err := newDatabaseEntry(entry)
	if err != nil {
		return err
	}
	return a.db.Insert(ctx, dbentry)
}

type blessingLogReader struct {
	db database
}

func (r *blessingLogReader) Read(ctx *context.T, email string) <-chan BlessingEntry {
	c := make(chan BlessingEntry)
	go r.sendAuditEvents(ctx, c, email)
	return c
}

func (r *blessingLogReader) sendAuditEvents(ctx *context.T, dst chan<- BlessingEntry, email string) {
	defer close(dst)
	dbch := r.db.Query(ctx, email)
	for dbentry := range dbch {
		dst <- newBlessingEntry(dbentry)
	}
}

func newDatabaseEntry(entry audit.Entry) (databaseEntry, error) {
	d := databaseEntry{timestamp: entry.Timestamp}
	extension, ok := entry.Arguments[2].(string)
	if !ok {
		return d, fmt.Errorf("failed to extract extension")
	}
	// Find the first email component
	for _, n := range strings.Split(extension, security.ChainSeparator) {
		// HACK ALERT: An email is the first entry to end up with
		// a single "@" in it
		if strings.Count(n, "@") == 1 {
			d.email = n
			break
		}
	}
	if len(d.email) == 0 {
		return d, fmt.Errorf("failed to extract email address from extension %q", extension)
	}
	var caveats []security.Caveat
	for _, arg := range entry.Arguments[3:] {
		cav, ok := arg.(security.Caveat)
		if !ok {
			return d, fmt.Errorf("failed to extract Caveat")
		}
		caveats = append(caveats, cav)
	}
	var blessings security.Blessings
	if blessings, ok = entry.Results[0].(security.Blessings); !ok {
		return d, fmt.Errorf("failed to extract result blessing")
	}
	var err error
	if d.blessings, err = vom.Encode(blessings); err != nil {
		return d, err
	}
	if d.caveats, err = vom.Encode(caveats); err != nil {
		return d, err
	}
	return d, nil
}

func newBlessingEntry(dbentry databaseEntry) BlessingEntry {
	if dbentry.decodeErr != nil {
		return BlessingEntry{DecodeError: dbentry.decodeErr}
	}
	b := BlessingEntry{
		Email:     dbentry.email,
		Timestamp: dbentry.timestamp,
	}
	if err := vom.Decode(dbentry.blessings, &b.Blessings); err != nil {
		return BlessingEntry{DecodeError: fmt.Errorf("failed to decode blessings: %s", err)}
	}
	if err := vom.Decode(dbentry.caveats, &b.Caveats); err != nil {
		return BlessingEntry{DecodeError: fmt.Errorf("failed to decode caveats: %s", err)}
	}
	b.RevocationCaveatID = revocationCaveatID(b.Caveats)
	return b
}

func revocationCaveatID(caveats []security.Caveat) string {
	for _, cav := range caveats {
		if tp := cav.ThirdPartyDetails(); tp != nil {
			return tp.ID()
		}
	}
	return ""
}
