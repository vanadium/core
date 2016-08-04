// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"runtime"
	"testing"
)

func TestTrigger(t *testing.T) {
	tr := NewTrigger()
	defer tr.Stop()

	done := make(chan int)

	f0 := func() { done <- 0 }
	c0 := make(chan struct{})
	tr.Add(f0, c0)

	f1 := func() { done <- 1 }
	c1 := make(chan struct{})
	tr.Add(f1, c1)

	// No signal; Shouldn't trigger anything.
	runtime.Gosched()
	select {
	case got := <-done:
		t.Errorf("Unexpected Trigger; got %v", got)
	default:
	}

	// Send a signal to c1; Should trigger f1().
	close(c1)
	if got, want := <-done, 1; got != want {
		t.Errorf("Trigger failed; got %v, but wanted %v", got, want)
	}

	// Send a signal to c0; Should trigger f0().
	c0 <- struct{}{}
	if got, want := <-done, 0; got != want {
		t.Errorf("Trigger failed; got %v, but wanted %v", got, want)
	}

	// Make sure the callback is triggered even when it is added with a closed channel.
	close(c0)
	tr.Add(f0, c0)
	if got, want := <-done, 0; got != want {
		t.Errorf("Trigger failed; got %v, but wanted %v", got, want)
	}
}
