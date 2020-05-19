// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

func TestFormatLogline(t *testing.T) {
	line, want := testutil.FormatLogLine(2, "test"), "testing.go:.*"
	if ok, err := regexp.MatchString(want, line); !ok || err != nil {
		t.Errorf("got %v, want %v", line, want)
	}
}

func panicHelper(ch chan string) {
	defer func() {
		if r := recover(); r != nil {
			ch <- r.(string)
		}
	}()
	testutil.RandomInt()
}

func TestPanic(t *testing.T) {
	testutil.Rand = nil
	ch := make(chan string)
	go panicHelper(ch)
	str := <-ch
	if got, want := str, "It looks like the singleton random number generator has not been initialized, please call InitRandGenerator."; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRandSeed(t *testing.T) {
	var seed int64
	rnd := testutil.NewRandGenerator(func(format string, args ...interface{}) {
		seed = args[0].(int64)
	})
	firstNumber := rnd.RandomInt()

	// Instantiate another random number generator with the same seed and
	// verify that it produces the same random number.
	os.Setenv("V23_RNG_SEED", fmt.Sprintf("%d", seed))
	rnd = testutil.NewRandGenerator(t.Logf)
	secondNumber := rnd.RandomInt()

	t.Logf("Seed: %d First: %d Second: %d", seed, firstNumber, secondNumber)
	if firstNumber != secondNumber {
		t.Errorf("Unexpected random number: got %d, expected %d", secondNumber, firstNumber)
	}
}

func TestFileTreeEqual(t *testing.T) {
	tests := []struct {
		A, B, Err, Debug         string
		FileA, DirA, FileB, DirB *regexp.Regexp
	}{
		{"./testdata/NOEXIST", "./testdata/A", "no such file", "", nil, nil, nil, nil},
		{"./testdata/A", "./testdata/NOEXIST", "no such file", "", nil, nil, nil, nil},

		{"./testdata/A", "./testdata/A", "", "", nil, nil, nil, nil},
		{"./testdata/A/subdir", "./testdata/A/subdir", "", "", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/SameSubdir/subdir", "", "", nil, nil, nil, nil},
		{"./testdata/SameSubdir/subdir", "./testdata/A/subdir", "", "", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/DiffSubdirFileName/subdir", "", "relative path doesn't match", nil, nil, nil, nil},
		{"./testdata/DiffSubdirFileName/subdir", "./testdata/A/subdir", "", "relative path doesn't match", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/DiffSubdirFileBytes/subdir", "", "bytes don't match", nil, nil, nil, nil},
		{"./testdata/DiffSubdirFileBytes/subdir", "./testdata/A/subdir", "", "bytes don't match", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/ExtraFile/subdir", "", "node count mismatch", nil, nil, nil, nil},
		{"./testdata/ExtraFile/subdir", "./testdata/A/subdir", "", "node count mismatch", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/ExtraFile/subdir", "", "", nil, nil, regexp.MustCompile(`file3`), nil},
		{"./testdata/ExtraFile/subdir", "./testdata/A/subdir", "", "", regexp.MustCompile(`file3`), nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/ExtraSubdir/subdir", "", "node count mismatch", nil, nil, nil, nil},
		{"./testdata/ExtraSubdir/subdir", "./testdata/A/subdir", "", "node count mismatch", nil, nil, nil, nil},

		{"./testdata/A/subdir", "./testdata/ExtraSubdir/subdir", "", "", nil, nil, regexp.MustCompile(`file3`), regexp.MustCompile(`subdir$`)},
		{"./testdata/ExtraSubdir/subdir", "./testdata/A/subdir", "", "", regexp.MustCompile(`file3`), regexp.MustCompile(`subdir$`), nil, nil},
	}
	for _, test := range tests {
		name := fmt.Sprintf("(%v,%v)", test.A, test.B)
		var debug bytes.Buffer
		opts := testutil.FileTreeOpts{
			Debug: &debug,
			FileA: test.FileA, DirA: test.DirA,
			FileB: test.FileB, DirB: test.DirB,
		}
		equal, err := testutil.FileTreeEqual(test.A, test.B, opts)
		if got, want := err == nil, test.Err == ""; got != want {
			t.Errorf("%v got success %v, want %v", name, got, want)
		}
		if got, want := fmt.Sprint(err), test.Err; err != nil && !strings.Contains(got, want) {
			t.Errorf("%v got error str %v, want substr %v", name, got, want)
		}
		if got, want := equal, test.Err == "" && test.Debug == ""; got != want {
			t.Errorf("%v got %v, want %v", name, got, want)
		}
		if got, want := debug.String(), test.Debug; !strings.Contains(got, want) || got != "" && want == "" {
			t.Errorf("%v got debug %v, want substr %v", name, got, want)
		}
	}
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
