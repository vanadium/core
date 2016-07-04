// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Compare ptrie with map[string]interface{}.
// The benchmark was executed on random 16 byte keys.
//
// Results (v23 go test v.io/x/ref/services/syncbase/store/ptrie -bench . -benchtime 0.1s -run Benchmark):
// map:
// BenchmarkMapPut-12        	 1000000	       655 ns/op
// BenchmarkMapOverwrite-12  	 1000000	       186 ns/op
// BenchmarkMapDelete-12     	 1000000	       111 ns/op
// BenchmarkMapGet-12        	 2000000	        91.5 ns/op
// BenchmarkMapScan-12       	10000000	        19.9 ns/op
//
// ptrie with copyOnWrite = false
// BenchmarkPtriePut-12      	  200000	      1742 ns/op
// BenchmarkPtrieOverwrite-12	  200000	      1596 ns/op
// BenchmarkPtrieDelete-12   	  200000	      1557 ns/op
// BenchmarkPtrieGet-12      	  300000	      1473 ns/op
// BenchmarkPtrieScan-12     	  300000	       940 ns/op
//
// ptrie with copyOnWrite = true
// BenchmarkPtriePut-12      	   50000	      4026 ns/op
// BenchmarkPtrieOverwrite-12	   50000	      4015 ns/op
// BenchmarkPtrieDelete-12   	   50000	      3367 ns/op
// BenchmarkPtrieGet-12      	  300000	      1207 ns/op
// BenchmarkPtrieScan-12     	  200000	       879 ns/op
package ptrie

import (
	"math/rand"
	"testing"
)

const (
	keyLength   = 16
	seed        = 23917
	copyOnWrite = false
)

func randomKey(r *rand.Rand) []byte {
	key := make([]byte, keyLength)
	for i := 0; i < keyLength; i++ {
		key[i] = byte(r.Intn(256))
	}
	return key
}

func generatePtrieKeys(b *testing.B) [][]byte {
	keys := make([][]byte, b.N)
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < b.N; i++ {
		keys[i] = randomKey(r)
	}
	return keys
}

func generateMapKeys(b *testing.B) []string {
	keys := make([]string, b.N)
	r := rand.New(rand.NewSource(seed))
	for i := 0; i < b.N; i++ {
		keys[i] = string(randomKey(r))
	}
	return keys
}

func fillPtrie(b *testing.B, t *T, keys [][]byte) {
	for i := 0; i < b.N; i++ {
		t.Put(keys[i], true)
	}
}

func fillMap(b *testing.B, m map[string]interface{}, keys []string) {
	for i := 0; i < b.N; i++ {
		m[keys[i]] = true
	}
}

func BenchmarkPtriePut(b *testing.B) {
	keys := generatePtrieKeys(b)
	t := New(copyOnWrite)
	b.ResetTimer()
	fillPtrie(b, t, keys)
}

func BenchmarkPtrieOverwrite(b *testing.B) {
	keys := generatePtrieKeys(b)
	t := New(copyOnWrite)
	fillPtrie(b, t, keys)
	b.ResetTimer()
	fillPtrie(b, t, keys)
}

func BenchmarkPtrieDelete(b *testing.B) {
	keys := generatePtrieKeys(b)
	t := New(copyOnWrite)
	fillPtrie(b, t, keys)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t.Delete(keys[i])
	}
}

func BenchmarkPtrieGet(b *testing.B) {
	keys := generatePtrieKeys(b)
	t := New(copyOnWrite)
	fillPtrie(b, t, keys)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = t.Get(keys[i])
	}
}

func BenchmarkPtrieScan(b *testing.B) {
	keys := generatePtrieKeys(b)
	t := New(copyOnWrite)
	fillPtrie(b, t, keys)
	b.ResetTimer()
	s := t.Scan(nil, nil)
	for i := 0; i < b.N; i++ {
		s.Advance()
	}
}

func BenchmarkMapPut(b *testing.B) {
	// Running this test takes a lot of time and benchmarking a map is not
	// really important, so we skip the test.
	b.SkipNow()
	keys := generateMapKeys(b)
	m := make(map[string]interface{})
	b.ResetTimer()
	fillMap(b, m, keys)
}

func BenchmarkMapOverwrite(b *testing.B) {
	// Running this test takes a lot of time and benchmarking a map is not
	// really important, so we skip the test.
	b.SkipNow()
	keys := generateMapKeys(b)
	m := make(map[string]interface{})
	fillMap(b, m, keys)
	b.ResetTimer()
	fillMap(b, m, keys)
}

func BenchmarkMapDelete(b *testing.B) {
	// Running this test takes a lot of time and benchmarking a map is not
	// really important, so we skip the test.
	b.SkipNow()
	keys := generateMapKeys(b)
	m := make(map[string]interface{})
	fillMap(b, m, keys)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delete(m, keys[i])
	}
}

func BenchmarkMapGet(b *testing.B) {
	// Running this test takes a lot of time and benchmarking a map is not
	// really important, so we skip the test.
	b.SkipNow()
	keys := generateMapKeys(b)
	m := make(map[string]interface{})
	fillMap(b, m, keys)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[keys[i]]
	}
}

func BenchmarkMapScan(b *testing.B) {
	// Running this test takes a lot of time and benchmarking a map is not
	// really important, so we skip the test.
	b.SkipNow()
	keys := generateMapKeys(b)
	m := make(map[string]interface{})
	fillMap(b, m, keys)
	b.ResetTimer()
	for _, _ = range m {
	}
}
