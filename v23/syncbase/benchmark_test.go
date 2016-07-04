// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This benchmark measures time spent on local (non-synced) operations.
//
// All tests use 9-byte keys.
// "Tiny" benchmarks use testStruct values with empty []byte.
// "Huge" benchmarks use testStruct values with 100K-byte []byte.
// Scan and Exec benchmarks iterate once through all b.N records.
//
// To run: jiri go test v.io/v23/syncbase -test.bench=. -test.run=Benchmark -alsologtostderr=false -stderrthreshold=3
//
// Results (2016-01-15, MacBook Pro, 2.8 GHz Intel Core i7):
// BenchmarkTinyPut-8      	    1000	   1260983 ns/op
// BenchmarkTinyGet-8      	    2000	    772075 ns/op
// BenchmarkTinyDelete-8   	    2000	   1497883 ns/op
// BenchmarkTinyScan-8     	      50	  25266715 ns/op
// BenchmarkTinyExec-8     	     300	   6076609 ns/op
// BenchmarkTinyWatchPuts-8	      10	 179642079 ns/op
// BenchmarkHugePut-8      	    1000	   2916679 ns/op
// BenchmarkHugeGet-8      	    1000	   1982792 ns/op
// BenchmarkHugeDelete-8   	    1000	   1606726 ns/op
// BenchmarkHugeScan-8     	      10	 133327996 ns/op
// BenchmarkHugeExec-8     	      20	 104448117 ns/op
// BenchmarkHugeWatchPuts-8	       3	 384569733 ns/op
package syncbase_test

import (
	"fmt"
	"math/rand"
	"testing"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	_ "v.io/x/ref/runtime/factories/roaming"
	tu "v.io/x/ref/services/syncbase/testutil"
)

// prepare creates hierarchy "{a,d}/{u,c}" and returns some handles along with a
// cleanup function.
func prepare(b *testing.B) (*context.T, syncbase.Database, syncbase.Collection, func()) {
	ctx, sName, cleanup := tu.SetupOrDie(nil)
	d := syncbase.NewService(sName).DatabaseForId(wire.Id{"a", "d"}, nil)
	if err := d.Create(ctx, nil); err != nil {
		b.Fatalf("can't create database: %v", err)
	}
	c := d.CollectionForId(wire.Id{"u", "c"})
	if err := c.Create(ctx, nil); err != nil {
		b.Fatalf("can't create collection: %v", err)
	}
	return ctx, d, c, func() {
		b.StopTimer()
		cleanup()
		b.StartTimer()
	}
}

type blobStruct struct {
	String  string
	BlobRef wire.BlobRef
}

type testStruct struct {
	A string
	B int
	C *blobStruct
	D []byte
}

func makeKey(i int) string {
	return fmt.Sprintf("%09d", i)
}

func makeTestStruct() interface{} {
	return testStruct{A: "hello, world!", B: 42}
}

func makeTestStruct100K() interface{} {
	var byteSlice []byte
	r := rand.New(rand.NewSource(23917))
	for i := 0; i < 100*1000; i++ {
		byteSlice = append(byteSlice, byte(r.Intn(256)))
	}
	return testStruct{A: "hello, world!", B: 42, D: byteSlice}
}

func writeRows(b *testing.B, ctx *context.T, c syncbase.Collection, value interface{}) {
	writeRowsCustom(b, ctx, c, value, b.N)
}

func writeRowsCustom(b *testing.B, ctx *context.T, c syncbase.Collection, value interface{}, n int) {
	for i := 0; i < n; i++ {
		if err := c.Put(ctx, makeKey(i), value); err != nil {
			b.Fatal(err)
		}
	}
}

func runPutBenchmark(b *testing.B, value interface{}) {
	ctx, _, c, cleanup := prepare(b)
	defer cleanup()
	b.ResetTimer()
	writeRows(b, ctx, c, value)
}

func runGetBenchmark(b *testing.B, value interface{}) {
	ctx, _, c, cleanup := prepare(b)
	defer cleanup()
	writeRows(b, ctx, c, value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var got testStruct
		if err := c.Get(ctx, makeKey(i), &got); err != nil {
			b.Fatalf("c.Get failed: %v", err)
		}
	}
}

func runDeleteBenchmark(b *testing.B, value interface{}) {
	ctx, _, c, cleanup := prepare(b)
	defer cleanup()
	writeRows(b, ctx, c, value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := c.Delete(ctx, makeKey(i)); err != nil {
			b.Fatalf("c.Delete failed: %v", err)
		}
	}
}

const numRows = 100

// Measures how long it takes to process 'numRows' rows using scan.
func runScanBenchmark(b *testing.B, value interface{}) {
	ctx, _, c, cleanup := prepare(b)
	defer cleanup()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// TODO(sadovsky): Write rows just once, and clear any read caches on every
		// iteration.
		writeRowsCustom(b, ctx, c, value, numRows)
		b.StartTimer()
		s := c.Scan(ctx, syncbase.Prefix(""))
		var got testStruct
		for s.Advance() {
			s.Value(&got)
		}
		if s.Err() != nil {
			b.Fatalf("stream error: %s", s.Err())
		}
	}
}

// Measures how long it takes to process 'numRows' rows using exec.
func runExecBenchmark(b *testing.B, value interface{}) {
	ctx, d, c, cleanup := prepare(b)
	defer cleanup()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// TODO(sadovsky): Write rows just once, and clear any read caches on every
		// iteration.
		writeRowsCustom(b, ctx, c, value, numRows)
		b.StartTimer()
		_, s, err := d.Exec(ctx, "select v from c")
		if err != nil {
			b.Fatalf("exec error: %s", s.Err())
		}
		for s.Advance() {
			s.ResultCount()
		}
		if s.Err() != nil {
			b.Fatalf("stream error: %s", s.Err())
		}
	}
}

// Measures how long it takes to put and get notified about 'numRows' rows.
func runWatchPutsBenchmark(b *testing.B, value interface{}) {
	b.Skip("Hangs on occasion, for unknown reasons - v.io/i/1134")
	ctx, d, c, cleanup := prepare(b)
	defer cleanup()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := d.Watch(ctx, watch.ResumeMarker("now"), []wire.CollectionRowPattern{
			util.RowPrefixPattern(wire.Id{"u", "c"}, ""),
		})
		if w.Err() != nil {
			b.Fatalf("watch error: %v", w.Err())
		}
		done := make(chan struct{})
		go func() {
			seen := 0
			for seen < numRows && w.Advance() {
				seen++
				w.Change()
			}
			if w.Err() != nil {
				b.Fatalf("stream error: %v", w.Err())
			}
			close(done)
		}()
		writeRowsCustom(b, ctx, c, value, numRows)
		<-done
		w.Cancel()
	}
}

// Measures how long it takes to put and get notified about a single value.
func runWatchOnePutBenchmark(b *testing.B, value interface{}) {
	b.Skip("Hangs on occasion, for unknown reasons - v.io/i/1134")
	ctx, d, c, cleanup := prepare(b)
	defer cleanup()
	w := d.Watch(ctx, watch.ResumeMarker("now"), []wire.CollectionRowPattern{
		util.RowPrefixPattern(wire.Id{"u", "c"}, ""),
	})
	if w.Err() != nil {
		b.Fatalf("watch error: %v", w.Err())
	}
	row := make(chan struct{})
	go func() {
		seen := 0
		for seen < b.N && w.Advance() {
			seen++
			w.Change()
			row <- struct{}{}
		}
		if w.Err() != nil {
			b.Fatalf("stream error: %v", w.Err())
		}
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writeRowsCustom(b, ctx, c, value, 1)
		<-row
	}
	w.Cancel()
}

func BenchmarkTinyPut(b *testing.B) {
	runPutBenchmark(b, makeTestStruct())
}

func BenchmarkTinyGet(b *testing.B) {
	runGetBenchmark(b, makeTestStruct())
}

func BenchmarkTinyDelete(b *testing.B) {
	runDeleteBenchmark(b, makeTestStruct())
}

func BenchmarkTinyScan(b *testing.B) {
	runScanBenchmark(b, makeTestStruct())
}

func BenchmarkTinyExec(b *testing.B) {
	runExecBenchmark(b, makeTestStruct())
}

func BenchmarkTinyWatchPuts(b *testing.B) {
	runWatchPutsBenchmark(b, makeTestStruct())
}

func BenchmarkTinyWatchOnePut(b *testing.B) {
	runWatchOnePutBenchmark(b, makeTestStruct())
}

func BenchmarkHugePut(b *testing.B) {
	runPutBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeGet(b *testing.B) {
	runGetBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeDelete(b *testing.B) {
	runDeleteBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeScan(b *testing.B) {
	runScanBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeExec(b *testing.B) {
	runExecBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeWatchPuts(b *testing.B) {
	runWatchPutsBenchmark(b, makeTestStruct100K())
}

func BenchmarkHugeWatchOnePut(b *testing.B) {
	runWatchOnePutBenchmark(b, makeTestStruct100K())
}
