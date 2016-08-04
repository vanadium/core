// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// concurrency_test is a simple test of the framework for systematic
// testing of concurrency.
package concurrency_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"v.io/x/ref/internal/logger"
	"v.io/x/ref/runtime/internal/testing/concurrency"
	"v.io/x/ref/runtime/internal/testing/concurrency/sync"
)

var m sync.Mutex

// createMutexSets returns sets of thread identifiers that match the
// logic of mutexThreadClosure.
func createMutexSet(n int) map[int]bool {
	locks := make(map[int]bool)
	for i := 1; i <= n; i++ {
		locks[i] = true
	}
	return locks
}

// generateMutexOutputs generates all legal outputs of sequencing
// calls to rw.Lock(), rw.Unlock(). The input identifies the threads
// that wish to invoke these functions.
func generateMutexOutputs(locks map[int]bool) []string {
	if length(locks) == 0 {
		return []string{""}
	}
	result := make([]string, 0)
	for lock, ok := range locks {
		if ok {
			locks[lock] = false
			for _, s := range generateMutexOutputs(locks) {
				result = append(result, fmt.Sprintf("%d:Lock()%d:Unlock()%s", lock, lock, s))
			}
			locks[lock] = true
		}
	}
	return result
}

// mutexThreadClosure folds the input arguments inside of the function body
// as the testing framework only supports functions with no arguments.
func mutexThreadClosure(t *testing.T, n, max int, out *os.File) func() {
	return func() {
		defer concurrency.Exit()
		if n < max {
			child := mutexThreadClosure(t, n+1, max, out)
			concurrency.Start(child)
		}
		m.Lock()
		fmt.Fprintf(out, "%d:Lock()", n)
		m.Unlock()
		fmt.Fprintf(out, "%d:Unlock()", n)
	}
}

// TestMutex runs mutexThreadCLosure() without systematically testing
// concurrency.
func TestMutex(t *testing.T) {
	for n := 2; n < 6; n++ {
		thread := mutexThreadClosure(t, 1, n, nil)
		thread()
	}
}

// TestMutexExplore runs mutexThreadClosure() using the framework for systematic
// testing of concurrency, checking that the exploration explores the
// correct number of interleavings.
func TestMutexExplore(t *testing.T) {
	for n := 2; n < 6; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := mutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		niterations, err := tester.Explore()
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		outputs := processOutput(t, out)
		expectedOutputs := generateMutexOutputs(createMutexSet(n))
		checkExpectedOutputs(t, outputs, expectedOutputs)
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		logger.Global().VI(1).Infof("Explored %v iterations.", niterations)
	}
}

// TestMutexExploreN runs mutexThreadClosure() using the framework for
// systematic testing of concurrency, checking that the exploration
// explores at most the given number of interleavings.
func TestMutexExploreN(t *testing.T) {
	for n := 2; n < 6; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := mutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		stopAfter := 100
		niterations, err := tester.ExploreN(stopAfter)
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		outputs := processOutput(t, out)
		expectedOutputs := generateMutexOutputs(createMutexSet(n))
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		if niterations < stopAfter {
			checkExpectedOutputs(t, outputs, expectedOutputs)
		}
		if niterations > stopAfter {
			t.Fatalf("Unexpected number of iterations: expected at most %v, got %v", stopAfter, niterations)
		}
		logger.Global().VI(1).Infof("Explored %v iterations.", niterations)
	}
}

// TestMutexExploreFor runs mutexThreadClosure() using the framework
// for systematic testing of concurrency, checking that the
// exploration respects the given "soft" deadline.
func TestMutexExploreFor(t *testing.T) {
	for n := 2; n < 6; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := mutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		start := time.Now()
		deadline := 10 * time.Millisecond
		niterations, err := tester.ExploreFor(deadline)
		end := time.Now()
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		outputs := processOutput(t, out)
		expectedOutputs := generateMutexOutputs(createMutexSet(n))
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		if start.Add(deadline).After(end) {
			checkExpectedOutputs(t, outputs, expectedOutputs)
		}
		logger.Global().VI(1).Infof("Explored %v iterations.", niterations)
	}
}
