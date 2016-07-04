// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package leveldb

import (
	"testing"

	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/benchmark"
)

func testConfig(db store.Store) *benchmark.Config {
	return &benchmark.Config{
		Rand:     benchmark.NewRandomGenerator(23917, 0.5),
		St:       db,
		KeyLen:   20,
		ValueLen: 100,
	}
}

func runBenchmark(b *testing.B, f func(*testing.B, *benchmark.Config)) {
	db, dbPath := newDB()
	defer destroyDB(db, dbPath)
	f(b, testConfig(db))
}

// BenchmarkWriteSequential writes b.N values in sequential key order.
func BenchmarkWriteSequential(b *testing.B) {
	runBenchmark(b, benchmark.WriteSequential)
}

// BenchmarkWriteRandom writes b.N values in random key order.
func BenchmarkWriteRandom(b *testing.B) {
	runBenchmark(b, benchmark.WriteRandom)
}

// BenchmarkOverwrite overwrites b.N values in random key order.
func BenchmarkOverwrite(b *testing.B) {
	runBenchmark(b, benchmark.Overwrite)
}

// BenchmarkReadSequential reads b.N values in sequential key order.
func BenchmarkReadSequential(b *testing.B) {
	runBenchmark(b, benchmark.ReadSequential)
}

// BenchmarkReadRandom reads b.N values in random key order.
func BenchmarkReadRandom(b *testing.B) {
	runBenchmark(b, benchmark.ReadRandom)
}
