// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb

import (
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"testing"

	"v.io/x/ref/lib/stats"
	"v.io/x/ref/services/syncbase/store/transactions"
)

// In this file we tests using private function in the batchstore wrapper around
// LevelDB.  For higher-level tests via the syncbase public API see the
// stats_black_box_test.go file.

func TestWithNoData(t *testing.T) {
	bs, cleanup := newBatchStore()
	defer cleanup()

	// New db with no data added.

	byteCount := bs.filesystemBytes()
	if byteCount != 0 {
		t.Errorf("Wanted zero byteCount for new database. Got %d", byteCount)
	}

	fileCount := bs.fileCount()
	if fileCount != 0 {
		t.Errorf("Got %d files from fileCount(), want 0", fileCount)
	}

	fileCounts, fileMBs, readMBs, writeMBs, err := bs.levelInfo()
	if err != nil {
		t.Errorf("Problem getting stats: %v", err)
	}
	if len(fileCounts) != 0 {
		t.Errorf("Got %d levels, want 0", len(fileCounts))
	}
	if sum(fileCounts) != 0 {
		t.Errorf("Got %d files from levelInfo(), want 0", sum(fileCounts))
	}
	if sum(fileMBs) != 0 {
		t.Errorf("Got %d MB files, want 0", sum(fileMBs))
	}
	if sum(readMBs) != 0 {
		t.Errorf("Got %d MB of reads, want 0", sum(readMBs))
	}
	if sum(writeMBs) != 0 {
		t.Errorf("Got %d MB of writes, want 0", sum(writeMBs))
	}
}

func TestWithNoData_ViaStats(t *testing.T) {
	bs, cleanup := newBatchStore()
	defer cleanup()

	// New db with no data added.

	byteCount, err := stats.GetStatsObject(bs.statsPrefix + "/filesystem_bytes")
	if err != nil {
		t.Fatalf("Problem getting stats object: %v", err)
	}
	if byteCount.Value().(int64) != 0 {
		t.Errorf("Wanted zero byteCount for new database. Got %v", byteCount.Value())
	}

	fileCount, err := stats.GetStatsObject(bs.statsPrefix + "/file_count")
	if err != nil {
		t.Fatalf("Problem getting stats object: %v", err)
	}
	if fileCount.Value().(int64) != 0 {
		t.Errorf("Got %v files from fileCount(), want 0", fileCount.Value())
	}
}

func TestWithLotsOfData(t *testing.T) {
	bs, cleanup := newBatchStore()
	defer cleanup()

	keys := write100MB(bs)

	val, err := bs.Get(keys[0], nil)
	if err != nil {
		t.Errorf("Problem reading from database: %v", err)
	}
	if len(val) != 100*1024 {
		t.Errorf(`Got %d, wanted %d`, len(val), 100*1024)
	}

	// Assume at least half the 100 MB of data made it out to level files.
	byteCount := bs.filesystemBytes()
	if byteCount < 50*1024*1024 {
		t.Errorf("Wanted more than a kB. Got %d B", byteCount)
	}

	// According to https://rawgit.com/google/leveldb/master/doc/impl.html
	// the files are of size 2 MB, so because we have writen 100 MB we
	// expect that eventually that will be 50 files.  But let's be
	// conservative and assume only half of the data has made it out to the
	// level files.
	fileCount := bs.fileCount()
	if fileCount < 25 {
		t.Errorf("Got %d files from fileCount(), want 10 or more", fileCount)
	}

	fileCountsFromStats, fileMBs, readMBs, writeMBs, err := bs.levelInfo()
	if err != nil {
		t.Errorf("Problem getting stats: %v", err)
	}

	// For 100 MB of data, we expect at least two levels of files.
	if len(fileCountsFromStats) < 2 {
		t.Errorf("Got %d levels, want 2 or more", len(fileCountsFromStats))
	}
	// We previously got this a different way.  Let's make sure the two ways
	// match.
	if sum(fileCountsFromStats) != fileCount {
		t.Errorf("Got %d files, want %d", sum(fileCountsFromStats), fileCount)
	}
	// We also previously got this a different way.  Let's make sure the two
	// ways match, at least approximately.
	totFileBytes := uint64(sum(fileMBs)) * 1024 * 1024
	if !approxEquals(totFileBytes, byteCount) {
		t.Errorf("Got %d B of files, want approximately %d B",
			totFileBytes, byteCount)
	}
	// As LevelDB does compression, we assume it has to read a significant
	// amount from the file system.
	if sum(readMBs) < 10 {
		t.Errorf("Got %d MB of reads, want 1 MB or more", sum(readMBs))
	}
	// For writing, we have the initial writing plus extra writing for
	// compaction.
	if sum(writeMBs) < 60 {
		t.Errorf("Got %d MB of writes, want 50 MB or more", sum(writeMBs))
	}
}

func TestWithLotsOfData_ViaStats(t *testing.T) {
	bs, cleanup := newBatchStore()
	defer cleanup()

	keys := write100MB(bs)

	val, err := bs.Get(keys[0], nil)
	if err != nil {
		t.Errorf("Problem reading from database: %v", err)
	}
	if len(val) != 100*1024 {
		t.Errorf(`Got %d, wanted %d`, len(val), 100*1024)
	}

	// Assume at least half the 100 MB of data made it out to level files.
	byteCount, err := stats.GetStatsObject(bs.statsPrefix + "/filesystem_bytes")
	if err != nil {
		t.Fatalf("Problem getting stats object: %v", err)
	}
	if byteCount.Value().(int64) < 50*1024*1024 {
		t.Errorf("Wanted more than 50 MB. Got %v B", byteCount.Value())
	}

	// According to https://rawgit.com/google/leveldb/master/doc/impl.html
	// the files are of size 2 MB, so because we have writen 100 MB we
	// expect that eventually that will be 50 files.  But let's be
	// conservative and assume only half of the data has made it out to the
	// level files.
	fileCount, err := stats.GetStatsObject(bs.statsPrefix + "/file_count")
	if err != nil {
		t.Fatalf("Problem getting stats object: %v", err)
	}
	if fileCount.Value().(int64) < 25 {
		t.Errorf("Got %v files from fileCount(), want 25 or more", fileCount.Value())
	}
}

// True if within 10%.
func approxEquals(a, b uint64) bool {
	diff := math.Abs(float64(a) - float64(b))
	mean := float64(a+b) / 2.0
	return diff/mean < 0.1
}

var nextTestNum = 0

// newBatchStore returns an empty leveldb store in a new temp directory.
func newBatchStore() (bs *db, cleanup func()) {
	path, err := ioutil.TempDir("", "syncbase_leveldb")
	if err != nil {
		panic(err)
	}
	bs, err = openBatchStore(path, OpenOptions{CreateIfMissing: true, ErrorIfExists: true})
	if err != nil {
		panic(err)
	}
	cleanup = func() {
		err := bs.Close()
		if err != nil {
			panic(err)
		}
		err = Destroy(path)
		if err != nil {
			panic(err)
		}
	}
	return
}

// Write 10 bytes each to 1024 keys, returning keys written.
func write10kB(bs transactions.BatchStore) (keys [][]byte) {
	tenB := make([]byte, 10)

	// Write 1024 X 10 B  (10 kB).
	for i := 0; i < 1024; i++ {
		rand.Read(tenB) // Randomize so it won't compress.
		key := []byte(fmt.Sprintf("foo-%d", i))
		err := bs.WriteBatch(transactions.WriteOp{
			T:     transactions.PutOp,
			Key:   key,
			Value: tenB,
		})
		if err != nil {
			panic(err)
		}
		keys = append(keys, key)
	}
	return
}

// Write 100 kB each to 1024 keys, returning keys written.
func write100MB(bs transactions.BatchStore) (keys [][]byte) {
	hundredKB := make([]byte, 100*1024)

	// Write 1024 X 100 KB  (100 MB).
	for i := 0; i < 1024; i++ {
		rand.Read(hundredKB) // Randomize so it won't compress.
		key := []byte(fmt.Sprintf("foo-%d", i))
		err := bs.WriteBatch(transactions.WriteOp{
			T:     transactions.PutOp,
			Key:   key,
			Value: hundredKB,
		})
		if err != nil {
			panic(err)
		}
		keys = append(keys, key)
	}
	return
}

func sum(xs []int) (result int) {
	for _, x := range xs {
		result += x
	}
	return
}
