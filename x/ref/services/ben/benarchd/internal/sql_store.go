// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/x/ref/services/ben"
)

const driverSqlite3 = "sqlite3"

// NewSQLStore returns a Store implementation that uses the provided database
// for persistent storage.
func NewSQLStore(driver string, db *sql.DB) (Store, error) {
	store := &sqlStore{driver: driver, db: db}
	if err := store.initDB(); err != nil {
		return nil, err
	}
	if err := store.initStmts(); err != nil {
		return nil, err
	}
	return store, nil
}

type sqlStore struct {
	driver string
	db     *sql.DB

	insertOS, selectOS               *sql.Stmt
	insertCPU, selectCPU             *sql.Stmt
	insertScenario, selectScenario   *sql.Stmt
	insertSourceCode                 *sql.Stmt
	insertUpload                     *sql.Stmt
	insertBenchmark, selectBenchmark *sql.Stmt
	updateBenchmark                  *sql.Stmt
	insertRun                        *sql.Stmt
	selectRunsByBenchmark            *sql.Stmt
	selectSourceCode                 *sql.Stmt
	searchBenchmarks                 *sql.Stmt
	describeBenchmark                *sql.Stmt
}

// tweakSQL returns a SQL string appropriate for the database implementation
// given a SQL string appropriate for MySQL.
func (s *sqlStore) tweakSQL(mysql string) string {
	if s.driver == driverSqlite3 {
		stmt := strings.ReplaceAll(mysql, "AUTO_INCREMENT", "AUTOINCREMENT")
		stmt = strings.ReplaceAll(stmt, "INSERT IGNORE", "INSERT OR IGNORE")
		stmt = strings.ReplaceAll(stmt, "CONCAT('%',?,'%')", "'%'||?||'%'")
		stmt = strings.ReplaceAll(stmt, "CONCAT('%',LOWER(?),'%')", "'%'||LOWER(?)||'%'")
		return stmt
	}
	return mysql
}

func (s *sqlStore) Save(ctx *context.T, scenario ben.Scenario, code ben.SourceCode, uploader string, uploadTime time.Time, runs []ben.Run) error {
	if len(runs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	// If tx.Commit is called, then this tx.Rollback is a no-op
	defer tx.Rollback() //nolint:errcheck
	cpu, err := s.insertAndGetID(tx, s.insertCPU, s.selectCPU, scenario.Cpu.Architecture, scenario.Cpu.Description, scenario.Cpu.ClockSpeedMhz)
	if err != nil {
		return tagerr("cpu", err)
	}
	os, err := s.insertAndGetID(tx, s.insertOS, s.selectOS, scenario.Os.Name, scenario.Os.Version)
	if err != nil {
		return tagerr("os", err)
	}
	scn, err := s.insertAndGetID(tx, s.insertScenario, s.selectScenario, cpu, os, uploader, scenario.Label)
	if err != nil {
		return tagerr("scenario", err)
	}
	codeID := code.ID()
	if _, err := tx.Stmt(s.insertSourceCode).Exec(codeID, string(code)); err != nil {
		return tagerr("sourcecode", err)
	}
	result, err := tx.Stmt(s.insertUpload).Exec(uploadTime, codeID)
	if err != nil {
		return tagerr("upload", err)
	}
	upload, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, run := range runs {
		bm, err := s.insertAndGetID(tx, s.insertBenchmark, s.selectBenchmark, scn, run.Name)
		if err != nil {
			return tagerr("benchmark", err)
		}
		// Cache this "latest" result values
		if _, err := tx.Stmt(s.updateBenchmark).Exec(run.NanoSecsPerOp, run.MegaBytesPerSec, uploadTime, bm); err != nil {
			return tagerr("update_benchmark", err)
		}
		if _, err := tx.Stmt(s.insertRun).Exec(bm, upload, run.Iterations, run.NanoSecsPerOp, run.AllocsPerOp, run.AllocedBytesPerOp, run.MegaBytesPerSec, run.Parallelism); err != nil {
			return tagerr("run", err)
		}
	}
	return tx.Commit()
}

func (s *sqlStore) DescribeSource(id string) (ben.SourceCode, error) {
	var str string
	err := s.selectSourceCode.QueryRow(id).Scan(&str)
	return ben.SourceCode(str), err
}

type nullItr struct{ err error }

func (*nullItr) Advance() bool { return false }
func (*nullItr) Close()        {}
func (i *nullItr) Err() error  { return i.err }

type nullBmItr struct{ nullItr }

func (i *nullBmItr) Value() Benchmark  { return Benchmark{} }
func (i *nullBmItr) Runs() RunIterator { return &nullRunsItr{} }

type nullRunsItr struct{ nullItr }

func (i *nullRunsItr) Value() (ben.Run, string, time.Time) { return ben.Run{}, "", time.Time{} }

type sqlItr struct {
	rows    *sql.Rows
	scanErr error
}

func (i *sqlItr) Advance() bool { return i.rows.Next() }
func (i *sqlItr) Close()        { i.rows.Close() }
func (i *sqlItr) Err() error {
	if i.scanErr != nil {
		return i.scanErr
	}
	return i.rows.Err()
}

type sqlBmItr struct {
	sqlItr
	id         int64 // BenchmarkID of the last scanned row.
	selectRuns *sql.Stmt
}

func (i *sqlBmItr) Value() Benchmark {
	var ret Benchmark
	i.scanErr = i.rows.Scan(
		&i.id,
		&ret.Name,
		&ret.NanoSecsPerOp,
		&ret.MegaBytesPerSec,
		&ret.LastUpdate,
		&ret.Scenario.Os.Name,
		&ret.Scenario.Os.Version,
		&ret.Scenario.Cpu.Architecture,
		&ret.Scenario.Cpu.Description,
		&ret.Scenario.Cpu.ClockSpeedMhz,
		&ret.Uploader,
		&ret.Scenario.Label)
	ret.ID = fmt.Sprintf("%x", i.id)
	return ret
}
func (i *sqlBmItr) Runs() RunIterator {
	rows, err := i.selectRuns.Query(i.id)
	if err != nil {
		return &nullRunsItr{nullItr{err}}
	}
	return &sqlRunItr{sqlItr: sqlItr{rows: rows}}
}

type sqlRunItr struct {
	sqlItr
}

func (i *sqlRunItr) Value() (ben.Run, string, time.Time) {
	var (
		r ben.Run
		s string
		t time.Time
	)
	i.scanErr = i.rows.Scan(
		&r.Name,
		&r.Iterations,
		&r.NanoSecsPerOp,
		&r.AllocsPerOp,
		&r.AllocedBytesPerOp,
		&r.MegaBytesPerSec,
		&r.Parallelism,
		&t,
		&s,
	)
	return r, s, t
}

func (s *sqlStore) Benchmarks(query *Query) BenchmarkIterator {
	cpuMHz, err := strconv.Atoi(query.CPU)
	if err != nil {
		cpuMHz = -1
	}
	rows, err := s.searchBenchmarks.Query(query.Name, query.OS, query.OS, query.CPU, query.CPU, cpuMHz, query.Uploader, query.Label)
	if err != nil {
		return &nullBmItr{nullItr: nullItr{err}}
	}
	return &sqlBmItr{sqlItr: sqlItr{rows: rows}, selectRuns: s.selectRunsByBenchmark}
}

func (s *sqlStore) Runs(id string) (Benchmark, RunIterator) {
	key, err := strconv.ParseInt(id, 16, 64)
	if err != nil {
		return Benchmark{}, &nullRunsItr{nullItr{err}}
	}
	row := s.describeBenchmark.QueryRow(key)
	var bm Benchmark
	if err := row.Scan(
		&key,
		&bm.Name,
		&bm.Scenario.Os.Name,
		&bm.Scenario.Os.Version,
		&bm.Scenario.Cpu.Architecture,
		&bm.Scenario.Cpu.Description,
		&bm.Scenario.Cpu.ClockSpeedMhz,
		&bm.Uploader,
		&bm.Scenario.Label); err != nil {
		return bm, &nullRunsItr{nullItr{err}}
	}
	bm.ID = id
	rows, err := s.selectRunsByBenchmark.Query(key)
	if err != nil {
		return bm, &nullRunsItr{nullItr{err}}
	}
	return bm, &sqlRunItr{sqlItr: sqlItr{rows: rows}}
}

func (s *sqlStore) insertAndGetID(tx *sql.Tx, insrt, slct *sql.Stmt, args ...interface{}) (int64, error) {
	// First try selecting since the common case is expected to be that.
	var id int64
	if err := tx.Stmt(slct).QueryRow(args...).Scan(&id); err == nil {
		return id, nil
	} else if err != sql.ErrNoRows {
		return 0, err
	}
	if result, err := tx.Stmt(insrt).Exec(args...); err != nil {
		return 0, fmt.Errorf("INSERT failed. Arguments: %v. Error: %v", args, err)
	} else if id, err := result.LastInsertId(); err == nil {
		return id, nil
	}
	err := tx.Stmt(slct).QueryRow(args...).Scan(&id)
	return id, err
}

func (s *sqlStore) initStmts() error {
	stmts := []struct {
		stmt **sql.Stmt
		sql  string
	}{
		{
			&s.insertOS,
			"INSERT IGNORE INTO OS (Name, Version) VALUES (LOWER(?), LOWER(?))",
		},
		{
			&s.selectOS,
			"SELECT ID FROM OS WHERE Name = LOWER(?) AND Version = LOWER(?)",
		},
		{
			&s.insertCPU,
			"INSERT IGNORE INTO CPU (Architecture, Description, ClockSpeedMHz) VALUES (LOWER(?), LOWER(?), ?)",
		},
		{
			&s.selectCPU,
			"SELECT ID FROM CPU WHERE Architecture = LOWER(?) AND Description = LOWER(?) AND ClockSpeedMHz = ?",
		},
		{
			&s.insertScenario,
			"INSERT IGNORE INTO Scenario (CPU, OS, Uploader, Label) VALUES (?, ?, LOWER(?), LOWER(?))",
		},
		{
			&s.selectScenario,
			"SELECT ID FROM Scenario WHERE CPU = ? AND OS = ? AND Uploader = LOWER(?) AND Label = LOWER(?)",
		},
		{
			&s.insertSourceCode,
			"INSERT IGNORE INTO SourceCode (ID, Description) VALUES (?, ?)",
		},
		{
			&s.selectSourceCode,
			"SELECT Description FROM SourceCode WHERE ID=?",
		},
		{
			&s.insertUpload,
			"INSERT INTO Upload (Timestamp, SourceCode) VALUES (?, ?)",
		},
		{
			&s.insertBenchmark,
			"INSERT IGNORE INTO Benchmark (Scenario, Name) VALUES (?, ?)",
		},
		{
			&s.selectBenchmark,
			"SELECT ID From Benchmark WHERE Scenario = ? AND Name = ?",
		},
		{
			&s.updateBenchmark,
			"UPDATE Benchmark SET NanoSecsPerOp = ?, MegaBytesPerSec = ?, LastUpdate = ? WHERE ID = ?",
		},
		{
			&s.insertRun,
			`
INSERT INTO Run (Benchmark, Upload, Iterations, NanoSecsPerOp, AllocsPerOp, AllocedBytesPerOp, MegaBytesPerSec, Parallelism)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`,
		},
		{
			&s.selectRunsByBenchmark,
			`
SELECT Benchmark.Name, Run.Iterations, Run.NanoSecsPerOp, Run.AllocsPerOp, Run.AllocedBytesPerOp, Run.MegaBytesPerSec, Run.Parallelism, Upload.Timestamp, Upload.SourceCode
FROM Run
INNER JOIN Benchmark ON (Run.Benchmark = Benchmark.ID)
INNER JOIN Upload ON (Run.Upload = Upload.ID)
WHERE Benchmark.ID = ?
ORDER BY Upload.Timestamp DESC
`,
		},
		{
			&s.searchBenchmarks,
			// Full text search using LIKE CONCAT('%',?,'%')
			// This can't be good!
			`
SELECT
	Benchmark.ID,
	Benchmark.Name,
	Benchmark.NanoSecsPerOp,
	Benchmark.MegaBytesPerSec,
	Benchmark.LastUpdate,
	OS.Name,
	OS.Version,
	CPU.Architecture,
	CPU.Description,
	CPU.ClockSpeedMHz,
	Scenario.Uploader, 
	Scenario.Label
FROM Benchmark
INNER JOIN Scenario ON (Benchmark.Scenario = Scenario.ID)
INNER JOIN OS ON (Scenario.OS = OS.ID)
INNER JOIN CPU ON (Scenario.CPU = CPU.ID)
WHERE Benchmark.Name LIKE CONCAT('%',?,'%')
AND (OS.Name LIKE CONCAT('%',LOWER(?),'%') OR OS.Version LIKE CONCAT('%',LOWER(?),'%'))
AND (CPU.Architecture LIKE CONCAT('%',LOWER(?),'%') OR CPU.Description LIKE CONCAT('%',LOWER(?),'%') OR CPU.ClockSpeedMHz = ?)
AND Scenario.Uploader LIKE CONCAT('%',LOWER(?),'%')
AND Scenario.Label LIKE CONCAT('%',LOWER(?),'%')
ORDER BY Benchmark.LastUpdate DESC
`,
		},
		{
			&s.describeBenchmark,
			`
SELECT
	Benchmark.ID,
	Benchmark.Name,
	OS.Name,
	OS.Version,
	CPU.Architecture,
	CPU.Description,
	CPU.ClockSpeedMHz,
	Scenario.Uploader,
	Scenario.Label
FROM Benchmark
INNER JOIN Scenario ON (Benchmark.Scenario = Scenario.ID)
INNER JOIN OS ON (Scenario.OS = OS.ID)
INNER JOIN CPU ON (Scenario.CPU = CPU.ID)
WHERE Benchmark.ID = ?
`,
		},
	}
	for _, stmt := range stmts {
		var err error
		sql := s.tweakSQL(stmt.sql)
		if *stmt.stmt, err = s.db.Prepare(sql); err != nil {
			return fmt.Errorf("failed to prepare [%s]: %v", strings.Join(strings.Fields(sql), " "), err)
		}
	}
	return nil
}

func (s *sqlStore) initDB() error {
	if s.driver == driverSqlite3 {
		// https://www.sqlite.org/foreignkeys.html#fk_enable
		if _, err := s.db.Exec("PRAGMA foreign_keys=on"); err != nil {
			return err
		}
	}
	// One might have considered using a single table for all Scenario
	// related information with a UNIQUE constraint on all the fields
	// (e.g., UNIQUE(CPUArchitecture, CPUDescription, CPUMHz, OSName, OSVersion, ...))
	//
	// However, in MySQL (and Google Cloud SQL) there are limits on the
	// size of an index key (and the UNIQUE constraint implies an index
	// over all the included fields). For example, in Google Cloud SQL,
	// the limit is 3072 bytes, which is easily met by 6 VARCHAR(255)
	// columns (since VARCHAR(x) consumes 3x bytes in the index key).
	//
	// Instead, normalize the bejesus about of the fields and create
	// multiple tables (at the cost of additional SELECT statements
	// to find out IDs of previously inserted values).
	tables := []struct{ Name, Schema string }{
		{
			"CPU",
			`
ID            INTEGER PRIMARY KEY AUTO_INCREMENT,
Architecture  VARCHAR(255),
Description   VARCHAR(255),
ClockSpeedMHz INTEGER,

UNIQUE(Architecture, Description, ClockSpeedMHz)
`,
		},
		{
			"OS", `
ID      INTEGER PRIMARY KEY AUTO_INCREMENT,
Name    VARCHAR(255),
Version VARCHAR(255),

UNIQUE(Name, Version)
`,
		},
		{
			"Scenario", `
ID       INTEGER PRIMARY KEY AUTO_INCREMENT,
CPU      INTEGER,
OS       INTEGER,
Uploader VARCHAR(255),
Label    VARCHAR(255),

UNIQUE(CPU, OS, Uploader, Label),

FOREIGN KEY(CPU) REFERENCES CPU(ID),
FOREIGN KEY(OS)  REFERENCES OS(ID)
`,
		},
		{
			"SourceCode", `
ID          CHAR(64) PRIMARY KEY,
Description TEXT
`,
		},
		{
			"Upload", `
ID               INTEGER PRIMARY KEY AUTO_INCREMENT,
Timestamp        DATETIME,
SourceCode       CHAR(64),

FOREIGN KEY(SourceCode) REFERENCES SourceCode(ID)
`,
		},
		{
			"Benchmark", `
ID              INTEGER PRIMARY KEY AUTO_INCREMENT,
Scenario        INTEGER,
Name            VARCHAR(255),
NanoSecsPerOp   DOUBLE,
MegaBytesPerSec DOUBLE,
LastUpdate      DATETIME,

UNIQUE(Scenario, Name),

FOREIGN KEY(Scenario) REFERENCES Scenario(ID)
`,
		},
		{
			"Run", `
Benchmark         INTEGER,
Upload            INTEGER,
Iterations        INTEGER,
NanoSecsPerOp     DOUBLE,
AllocsPerOp       INTEGER,
AllocedBytesPerOp INTEGER,
MegaBytesPerSec   DOUBLE,
Parallelism       INTEGER,

FOREIGN KEY(Benchmark) REFERENCES Benchmark(ID),
FOREIGN KEY(Upload) REFERENCES Upload(ID)
`,
		},
	}
	for _, t := range tables {
		stmt := s.tweakSQL(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", t.Name, t.Schema))
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to create %v table (SQL: [%s]): %v", t.Name, strings.Join(strings.Fields(stmt), " "), err)
		}
	}
	return nil
}

func tagerr(tag string, err error) error {
	return fmt.Errorf("[%s]: %v", tag, err)
}
