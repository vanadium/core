// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmark

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func BenchmarkTest(b *testing.B) {
	stats := AddStats(b, 0)
	for i := 1; i <= b.N; i++ {
		time.Sleep(1 * time.Microsecond)
		stats.Add(time.Duration(i) * time.Microsecond)
	}
}

func BenchmarkTestMulti(b *testing.B) {
	stats1 := AddStatsWithName(b, "S1", 0)
	stats2 := AddStatsWithName(b, "S2", 0)
	for i := 1; i <= b.N; i++ {
		time.Sleep(1 * time.Microsecond)
		stats1.Add(time.Duration(i) * time.Microsecond)
		stats2.Add(time.Duration(i) * time.Millisecond)
	}
}

func benchmarkName(name string) string {
	procs := runtime.GOMAXPROCS(-1)
	if procs != 1 {
		return fmt.Sprintf("%s-%d", name, procs)
	}
	return name
}

func TestStatsInjection(t *testing.T) {
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)
	go func() {
		b := new(bytes.Buffer)
		io.Copy(b, r) //nolint:errcheck
		r.Close()
		outC <- b.String()
	}()

	startStatsInjector()

	fmt.Printf("%s\t", benchmarkName("BenchmarkTest"))
	result := testing.Benchmark(BenchmarkTest)
	fmt.Println(result.String())

	fmt.Printf("%s\t", benchmarkName("BenchmarkTestMulti"))
	result = testing.Benchmark(BenchmarkTestMulti)
	fmt.Println(result.String())

	stopStatsInjector()

	w.Close()
	os.Stdout = stdout
	out := <-outC

	if strings.Count(out, "Histogram") != 3 {
		t.Errorf("unexpected stats output:\n%s", out)
	}
	if matched, _ := regexp.MatchString("Histogram.*\\)\n", out); !matched {
		t.Errorf("unnamed stats not found:\n%s", out)
	}
	if matched, _ := regexp.MatchString("Histogram.*\\): S1\n", out); !matched {
		t.Errorf("stats S1 not found:\n%s", out)
	}
	if matched, _ := regexp.MatchString("Histogram.*\\): S2\n", out); !matched {
		t.Errorf("stats S2 not found:\n%s", out)
	}
}
