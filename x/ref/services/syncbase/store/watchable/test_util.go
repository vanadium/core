// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(razvanm): Switch to package_test. This is a little involved because
// createStore is used by the other _test.go files in this directory and
// TestLogEntryTimestamps is poking inside the watchable store to read the
// sequence number.
package watchable

import (
	"fmt"
	"io/ioutil"
	"math"
	"time"

	"v.io/v23/vom"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/util"
	"v.io/x/ref/services/syncbase/vclock"
)

// This file provides utility methods for tests related to watchable store.

////////////////////////////////////////////////////////////
// Functions for store creation/cleanup

// createStore returns a store along with a function to destroy the store once
// it is no longer needed.
func createStore() (store.Store, func()) {
	var st store.Store
	path := getPath()
	st, _ = util.OpenStore(store.EngineForTest, path, util.OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	return st, func() { util.DestroyStore(store.EngineForTest, path) }
}

func getPath() string {
	path, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		panic(fmt.Sprintf("can't create temp dir: %v", err))
	}
	return path
}

////////////////////////////////////////////////////////////
// Functions related to watchable store

// logEntryReader provides a stream-like interface to scan over the log entries
// of a single batch, starting for a given sequence number.  It opens a stream
// that scans the log from the sequence number given.  It stops after reading
// the last entry in that batch (indicated by a false Continued flag).
type logEntryReader struct {
	stream store.Stream // scan stream on the store Database
	done   bool         // true after reading the last batch entry
	key    string       // key of most recent log entry read
	entry  LogEntry     // most recent log entry read
}

func newLogEntryReader(st store.Store, seq uint64) *logEntryReader {
	stream := st.Scan([]byte(logEntryKey(seq)), []byte(logEntryKey(math.MaxUint64)))
	return &logEntryReader{stream: stream}
}

func (ler *logEntryReader) Advance() bool {
	if ler.done {
		return false
	}

	if ler.stream.Advance() {
		ler.key = string(ler.stream.Key(nil))
		if err := vom.Decode(ler.stream.Value(nil), &ler.entry); err != nil {
			panic(fmt.Errorf("Failed to decode LogEntry for key: %q", ler.key))
		}
		if ler.entry.Continued == false {
			ler.done = true
		}
		return true
	}

	ler.key = ""
	ler.entry = LogEntry{}
	return false
}

func (ler *logEntryReader) GetEntry() (string, LogEntry) {
	return ler.key, ler.entry
}

////////////////////////////////////////////////////////////
// VClock related utility code

type mockSystemClock struct {
	Ncalls int64         // number of times Now() has been called
	Time   time.Time     // initial time for Now()
	Inc    time.Duration // amount by which to increment 'time' on each call to Now()
}

var _ vclock.SystemClock = (*mockSystemClock)(nil)

func (sc *mockSystemClock) Now() time.Time {
	now := sc.Time.Add(time.Duration(sc.Ncalls * int64(sc.Inc)))
	sc.Ncalls++
	return now
}

func (sc *mockSystemClock) ElapsedTime() (time.Duration, error) {
	return time.Duration(sc.Ncalls * int64(sc.Inc)), nil
}
