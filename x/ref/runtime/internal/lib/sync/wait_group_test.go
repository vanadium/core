// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"fmt"
	"testing"

	"v.io/x/ref/test/testutil"
)

// TestRandom tests Wait after a random sequence of TryAdd's and Done's that
// leaves the counter at 0.
func TestRandom(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	var w WaitGroup
	N := 100

	count := 0
	for n := 0; n < N; n++ {
		if count == 0 || testutil.RandomIntn(2) == 0 {
			if !w.TryAdd() {
				t.Fatal("TryAdd failed")
			}
			count++
			continue
		}
		w.Done()
		count--
	}
	for d := 0; d < count; d++ {
		w.Done()
	}

	w.Wait()
}

func TestConcurrentWait(t *testing.T) {
	for r := 0; r < 100; r++ {
		var w WaitGroup

		done := make(chan struct{}, 1)
		errCh := make(chan error, 1)

		if !w.TryAdd() {
			t.Fatal("TryAdd failed")
		}

		go func() {
			w.Wait()
			// w.Wait() should not return before w.Done() sets the counter
			// to 0.
			// This test is not deterministic as we cannot infer the order
			// in which Wait() and the last Done() return. Hopefully, bugs
			// will revealed by repeating the test.
			select {
			case <-done:
			default:
				errCh <- fmt.Errorf("wait returned before Done")
				return
			}
			errCh <- nil
		}()

		for w.TryAdd() {
			w.Done()
		}
		close(done)
		w.Done()
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

func TestTryAddFailsAfterWait(t *testing.T) {
	var w WaitGroup

	if !w.TryAdd() {
		t.Fatal("TryAdd failed")
	}

	go w.Wait()

	// At some point, another goroutine will be in w.Wait() and w.TryAdd()
	// should fail. If this doesn't happen, the test will never terminate.
	for w.TryAdd() {
		w.Done()
	}
	w.Done()
}

func TestIdempotentWait(t *testing.T) {
	var w WaitGroup

	done := make(chan struct{}, 1)

	if !w.TryAdd() {
		t.Fatal("TryAdd failed")
	}

	errCh := make(chan error, 2)
	// w.Wait() should be idempotent.
	for i := 0; i < 2; i++ {
		go func() {
			w.Wait()
			select {
			case <-done:
				errCh <- nil
			default:
				errCh <- fmt.Errorf("wait returned before Done")
			}
		}()
	}

	for w.TryAdd() {
		w.Done()
	}
	close(done)
	w.Done()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

func TestDoneFailsBeforeAdd(t *testing.T) {
	var w WaitGroup
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("done succeeded before Add")
		}
	}()
	w.Done()
}
