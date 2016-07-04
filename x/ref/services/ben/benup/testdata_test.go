// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// This file doesn't contain real tests, it is used to generate
// input for tests in benup_test.go.
// The ignore build tag prevents the tests here from being compiled
// and built on their own.

package main

import (
	"encoding/base64"
	"testing"
)

func TestGood(t *testing.T) {
	// This function has intentionally been left blank.
}

func BenchmarkGood(b *testing.B) {
	// Do something so that the results are non zero
	sum := 1
	for i := 0; i < b.N; i++ {
		sum += i
	}
}

func BenchmarkThroughput(b *testing.B) {
	b.SetBytes(10)
	BenchmarkGood(b)
}

func BenchmarkAllocs(b *testing.B) {
	b.ReportAllocs()
	n := 0
	for i := 0; i < b.N; i++ {
		// Some way to force allocations!
		n += len(base64.URLEncoding.EncodeToString([]byte{byte(i)}))
	}
	if n == 0 {
		b.Fatalf("no loop iterations even though b.N=%v", b.N)
	}
}

func BenchmarkAllMetrics(b *testing.B) {
	b.SetBytes(10)
	BenchmarkAllocs(b)
}

func BenchmarkBad(b *testing.B) {
	b.Fatal("BenchmarkBad intentionally fails")
}

type s struct {
	i int
}

func tryToForceAlloc(out, in *s) {
	out.i += in.i
}
