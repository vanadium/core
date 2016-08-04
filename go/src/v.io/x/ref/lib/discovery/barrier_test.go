// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"testing"
	"time"

	"v.io/x/ref/lib/discovery"
)

func TestBarrier(t *testing.T) {
	const timeout = 5 * time.Millisecond

	ch := make(chan struct{})
	done := func() { ch <- struct{}{} }

	// A new barrier; Shouldn't call done.
	br := discovery.NewBarrier(done)
	if waitDone(ch, timeout) {
		t.Error("unexpected done call")
	}

	// Make sure the barrier works with one sub-closure.
	cb := br.Add()
	cb()
	if !waitDone(ch, 0) {
		t.Error("no expected done call")
	}
	// Try to add a sub-closure, but done has been already called.
	cb = br.Add()
	cb()
	if waitDone(ch, timeout) {
		t.Error("unexpected done call")
	}

	// Make sure the barrier works with multiple sub-closures.
	br = discovery.NewBarrier(done)
	cb1 := br.Add()
	cb2 := br.Add()
	cb1()
	if waitDone(ch, timeout) {
		t.Error("unexpected done call")
	}
	cb2()
	if !waitDone(ch, 0) {
		t.Error("no expected done call")
	}
}

func waitDone(ch <-chan struct{}, timeout time.Duration) bool {
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}
	select {
	case <-ch:
		return true
	case <-timer:
		return false
	}
}
