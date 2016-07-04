// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"v.io/v23/naming"
	"v.io/v23/verror"
)

func TestConcurrentMountUnmount(t *testing.T) {
	rootCtx, _, _, shutdown := initTest()
	defer shutdown()

	stop, mtAddr, bt, _ := newMT(t, "", rootCtx, nil)
	defer stop()

	var wg sync.WaitGroup
	busyMountUnmount := func(name string) {
		defer wg.Done()
		for i := 0; i < 250; i++ {
			n := fmt.Sprintf("%s/%d", name, rand.Intn(10))
			doMount(t, rootCtx, mtAddr, n, "/example.com:12345", true)
			if _, err := resolve(rootCtx, naming.JoinAddressName(mtAddr, n)); err != nil {
				t.Errorf("resolve(%q) failed: %v", n, err)
			}
			doUnmount(t, rootCtx, mtAddr, n, "", true)
		}
	}

	for i := 1; i <= 5; i++ {
		wg.Add(3)
		go busyMountUnmount(fmt.Sprintf("a/%d", i))
		go busyMountUnmount(fmt.Sprintf("a/b/%d", i))
		go busyMountUnmount(fmt.Sprintf("a/c/d/e/%d", i))
	}
	wg.Wait()

	checkMatch(t, []string{}, doGlob(t, rootCtx, mtAddr, "", "*"))

	count, err := bt.CountRows(rootCtx)
	if err != nil {
		t.Errorf("bt.CountRows failed: %v", err)
	}
	if expected := 1; count != expected {
		t.Errorf("Unexpected number of rows. Got %d, expected %d", count, expected)
		bt.DumpTable(rootCtx)
	}
}

func TestConcurrentMountDelete(t *testing.T) {
	rootCtx, _, _, shutdown := initTest()
	defer shutdown()

	stop, mtAddr, bt, _ := newMT(t, "", rootCtx, nil)
	defer stop()

	var wg sync.WaitGroup
	busyMount := func(name string) {
		defer wg.Done()
		for i := 0; i < 250; i++ {
			doMount(t, rootCtx, mtAddr, name, "/example.com:12345", true)
		}
	}
	busyDelete := func(name string) {
		defer wg.Done()
		for i := 0; i < 250; i++ {
			doDeleteSubtree(t, rootCtx, mtAddr, name, true)
		}
	}

	for i := 1; i <= 5; i++ {
		wg.Add(4)
		go busyMount(fmt.Sprintf("a/%d", i))
		go busyDelete(fmt.Sprintf("a/%d", i))
		go busyMount(fmt.Sprintf("b/%d/c/d/e/f/g/h/i", i))
		go busyDelete(fmt.Sprintf("b/%d", i))
	}
	wg.Wait()

	doDeleteSubtree(t, rootCtx, mtAddr, "a", true)
	doDeleteSubtree(t, rootCtx, mtAddr, "b", true)

	count, err := bt.CountRows(rootCtx)
	if err != nil {
		t.Errorf("bt.CountRows failed: %v", err)
	}
	if expected := 1; count != expected {
		t.Errorf("Unexpected number of rows. Got %d, expected %d", count, expected)
		bt.DumpTable(rootCtx)
	}
}

func TestConcurrentExpiry(t *testing.T) {
	rootCtx, _, _, shutdown := initTest()
	defer shutdown()

	stop, mtAddr, bt, clock := newMT(t, "", rootCtx, nil)
	defer stop()

	const N = 100
	name := func(i int) string { return fmt.Sprintf("a/b/c/d/e/f/g/h/i/j/k/l/%05d/%05d", i, i) }
	for i := 0; i < N; i++ {
		doMount(t, rootCtx, mtAddr, name(i), "/example.com:12345", true)
	}
	clock.AdvanceTime(time.Duration(ttlSecs+4) * time.Second)

	var wg sync.WaitGroup
	concurrentResolve := func(name string) {
		defer wg.Done()
		if _, err := resolve(rootCtx, naming.JoinAddressName(mtAddr, name)); verror.ErrorID(err) != naming.ErrNoSuchName.ID {
			t.Errorf("resolve(%q) returned unexpected error: %v", name, err)
		}
	}

	for i := 0; i < N; i++ {
		wg.Add(2)
		go concurrentResolve(name(i))
		go concurrentResolve(name(i))
	}
	wg.Wait()

	count, err := bt.CountRows(rootCtx)
	if err != nil {
		t.Errorf("bt.CountRows failed: %v", err)
	}
	if expected := 1; count != expected {
		t.Errorf("Unexpected number of rows. Got %d, expected %d", count, expected)
		bt.DumpTable(rootCtx)
	}
}
