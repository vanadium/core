// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// These benchmarks are useful for seeing how many allocations it takes for
// various context operations.

package context_test

import (
	"testing"
	"time"

	"v.io/v23/context"
)

func BenchmarkWithCancel(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	rootCtx, rootFinish := context.RootContext()
	b.StartTimer()
	for i := 0; i != b.N; i++ {
		_, finish := context.WithCancel(rootCtx)
		finish()
	}
	rootFinish()
}

func BenchmarkWithTimeout(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	rootCtx, rootFinish := context.RootContext()
	b.StartTimer()
	for i := 0; i != b.N; i++ {
		_, finish := context.WithTimeout(rootCtx, 100*time.Second)
		finish()
	}
	rootFinish()
}

func BenchmarkWithValue(b *testing.B) {
	b.StopTimer()
	b.ReportAllocs()
	rootCtx, rootFinish := context.RootContext()
	type myKeyType string
	key := myKeyType("a key")
	value := "value"
	b.StartTimer()
	for i := 0; i != b.N; i++ {
		context.WithValue(rootCtx, key, value)
	}
	rootFinish()
}
