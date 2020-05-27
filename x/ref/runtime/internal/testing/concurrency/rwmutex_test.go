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

	"v.io/x/lib/vlog"
	"v.io/x/ref/runtime/internal/testing/concurrency"
	"v.io/x/ref/runtime/internal/testing/concurrency/sync"
)

var rw sync.RWMutex

// createRWMutexSets returns sets of thread identifiers that match the
// logic of rwMutexThreadClosure.
func createRWMutexSets(n int) (map[int]bool, map[int]bool, map[int]bool, map[int]bool, map[int]bool) {
	locks := make(map[int]bool)
	rlocks := make(map[int]bool)
	runlocks := make(map[int]bool)
	llocks := make(map[int]bool)
	lunlocks := make(map[int]bool)
	for i := 1; i <= n; i++ {
		switch i % 3 {
		case 0:
			locks[i] = true
		case 1:
			rlocks[i] = true
			runlocks[i] = true
		case 2:
			llocks[i] = true
			lunlocks[i] = true
		}
	}
	return locks, rlocks, runlocks, llocks, lunlocks
}

// generateRWMutexOutputs generates all legal outputs of sequencing calls to
// rw.Lock(), rw.Unlock(), rw.RLock(), rw.RUnlock(),
// rw.RLocker().Lock(), and rw.RLocker().Unlock(). The inputs identify
// the threads that wish to invoke these functions.
func generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks map[int]bool) []string { //nolint:gocyclo
	if length(locks) == 0 && length(rlocks) == 0 && length(runlocks) == 0 && length(llocks) == 0 && length(lunlocks) == 0 {
		// Base case.
		return []string{""}
	}
	result := make([]string, 0)
	if length(rlocks) == length(runlocks) && length(llocks) == length(lunlocks) {
		// rw.Lock() + rw.Unlock() can happen next if the previous calls
		// to rw.RLock(), rw.RUnlock, rw.RLocker().Lock(), and
		// rw.RLocker.Unlock() are balanced.
		for lock, ok := range locks {
			if ok {
				locks[lock] = false
				for _, s := range generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks) {
					result = append(result, fmt.Sprintf("%d:Lock()%d:Unlock()%s", lock, lock, s))
				}
				locks[lock] = true
			}
		}
	}
	for rlock, ok := range rlocks {
		if ok {
			// rw.RLock() can happen next any time.
			rlocks[rlock] = false
			for _, s := range generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks) {
				result = append(result, fmt.Sprintf("%d:RLock()%s", rlock, s))
			}
			rlocks[rlock] = true
		}
	}
	for runlock, ok := range runlocks {
		if ok {
			if ok := rlocks[runlock]; !ok {
				// rw.RUnlock() can happen next as long as the same thread
				// already invoked rw.RLock().
				runlocks[runlock] = false
				for _, s := range generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks) {
					result = append(result, fmt.Sprintf("%d:RUnlock()%s", runlock, s))
				}
				runlocks[runlock] = true
			}
		}
	}
	for llock, ok := range llocks {
		if ok {
			// rw.RLocker().Lock() can happen next any time.
			llocks[llock] = false
			for _, s := range generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks) {
				result = append(result, fmt.Sprintf("%d:RLocker().Lock()%s", llock, s))
			}
			llocks[llock] = true
		}
	}
	for lunlock, ok := range lunlocks {
		if ok {
			if ok := llocks[lunlock]; !ok {
				// rw.RLocker().Unlock() can happen next as long as the same thread
				// already invoked rw.RLocker().Lock().
				lunlocks[lunlock] = false
				for _, s := range generateRWMutexOutputs(locks, rlocks, runlocks, llocks, lunlocks) {
					result = append(result, fmt.Sprintf("%d:RLocker().Unlock()%s", lunlock, s))
				}
				lunlocks[lunlock] = true
			}
		}
	}
	return result
}

// rwMutexThreadClosure folds the input arguments inside of the
// function body as the testing framework only supports functions with
// no arguments.
func rwMutexThreadClosure(t *testing.T, n, max int, out *os.File) func() {
	return func() {
		defer concurrency.Exit()
		if n < max {
			child := rwMutexThreadClosure(t, n+1, max, out)
			concurrency.Start(child)
		}
		switch n % 3 {
		case 0:
			rw.Lock()
			fmt.Fprintf(out, "%d:Lock()", n)
		case 1:
			rw.RLock()
			fmt.Fprintf(out, "%d:RLock()", n)
		case 2:
			rw.RLocker().Lock()
			fmt.Fprintf(out, "%d:RLocker().Lock()", n)
		}
		switch n % 3 {
		case 0:
			rw.Unlock()
			fmt.Fprintf(out, "%d:Unlock()", n)
		case 1:
			rw.RUnlock()
			fmt.Fprintf(out, "%d:RUnlock()", n)
		case 2:
			rw.RLocker().Unlock()
			fmt.Fprintf(out, "%d:RLocker().Unlock()", n)
		}
	}
}

// TestRWMutex runs rwMutexThreadClosure() without systematically
// testing concurrency.
func TestRWMutex(t *testing.T) {
	for n := 2; n < 5; n++ {
		thread := rwMutexThreadClosure(t, 1, n, nil)
		thread()
	}
}

// TestRWMutexExplore runs rwMutexThreadClosure() using the framework
// for systematic testing of concurrency, checking that the
// exploration explores the correct number of interleavings.
func TestRWMutexExplore(t *testing.T) {
	for n := 2; n < 5; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := rwMutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		niterations, err := tester.Explore()
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		outputs := processOutput(t, out)
		expectedOutputs := generateRWMutexOutputs(createRWMutexSets(n))
		checkExpectedOutputs(t, outputs, expectedOutputs)
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		vlog.VI(1).Infof("Explored %v iterations.", niterations)
	}
}

// TestRWMutexExploreN runs rwMutexThreadClosure() using the framework
// for systematic testing of concurrency, checking that the
// exploration explores at most the given number of interleavings.
func TestRWMutexExploreN(t *testing.T) {
	stopAfter := 100
	for n := 2; n < 5; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := rwMutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		niterations, err := tester.ExploreN(stopAfter)
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		outputs := processOutput(t, out)
		expectedOutputs := generateRWMutexOutputs(createRWMutexSets(n))
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		if niterations < stopAfter {
			checkExpectedOutputs(t, outputs, expectedOutputs)
		}
		if niterations > stopAfter {
			t.Fatalf("Unexpected number of iterations: expected at most %v, got %v", stopAfter, niterations)
		}
		vlog.VI(1).Infof("Explored %v iterations.", niterations)
	}
}

// TestRWMutexExploreFor runs rwMutexThreadClosure() using the
// framework for systematic testing of concurrency, checking that the
// exploration respects the given "soft" deadline.
func TestRWMutexExploreFor(t *testing.T) {
	deadline := 10 * time.Millisecond
	for n := 2; n < 5; n++ {
		out, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatalf("TempFile() failed: %v", err)
		}
		defer os.Remove(out.Name())
		defer out.Close()
		body := rwMutexThreadClosure(t, 1, n, out)
		tester := concurrency.Init(setup, body, cleanupClosure(out))
		defer concurrency.Finish()
		start := time.Now()
		niterations, err := tester.ExploreFor(deadline)
		if err != nil {
			t.Fatalf("Unexpected error encountered: %v", err)
		}
		end := time.Now()
		outputs := processOutput(t, out)
		expectedOutputs := generateRWMutexOutputs(createRWMutexSets(n))
		checkUnexpectedOutputs(t, outputs, expectedOutputs)
		if start.Add(deadline).After(end) {
			checkExpectedOutputs(t, outputs, expectedOutputs)
		}
		vlog.VI(1).Infof("Explored %v iterations.", niterations)
	}
}
