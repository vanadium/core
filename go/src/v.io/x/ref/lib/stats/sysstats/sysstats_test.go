// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysstats_test

import (
	"os"
	"runtime"
	"testing"

	"v.io/x/ref/lib/stats"
	_ "v.io/x/ref/lib/stats/sysstats"
)

func TestMemStats(t *testing.T) {
	alloc, err := stats.GetStatsObject("system/memstats/Alloc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v := alloc.Value(); v == uint64(0) {
		t.Errorf("unexpected Alloc value. Got %v, want != 0", v)
	}
}

func TestVars(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("Hostname: unexpected error: %v", err)
	}
	for i, c := range []struct {
		name     string
		expected interface{}
	}{
		{"system/pid", int64(os.Getpid())},
		{"system/hostname", hostname},
		{"system/GOMAXPROCS", int64(runtime.GOMAXPROCS(0))},
	} {
		obj, err := stats.GetStatsObject(c.name)
		if err != nil {
			t.Fatalf("Case #%d: unexpected error: %v", i, err)
		}
		if got, want := obj.Value(), c.expected; got != want {
			t.Errorf("Case #%d: unexpected result. Got %v, want %v", i, got, want)
		}
	}
	oldGOMAXPROCS := runtime.GOMAXPROCS(0)
	// set new value of GOMAXPROCS to the old value +/- 1 (doesn't matter as
	// long as it's different).
	newGOMAXPROCS := oldGOMAXPROCS - 1
	if newGOMAXPROCS < 1 {
		newGOMAXPROCS = oldGOMAXPROCS + 1
	}
	runtime.GOMAXPROCS(newGOMAXPROCS)
	defer runtime.GOMAXPROCS(oldGOMAXPROCS)
	obj, err := stats.GetStatsObject("system/GOMAXPROCS")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := obj.Value(), int64(newGOMAXPROCS); got != want {
		t.Errorf("unexpected result. Got %v, want %v", got, want)
	}
}
