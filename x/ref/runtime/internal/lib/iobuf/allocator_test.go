// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

import (
	"fmt"
	"testing"
)

func testAllocatorAlloc(t *testing.T, bytes, reserve uint) {
	pool := NewPool(iobufSize)
	defer pool.Close()
	alloc := NewAllocator(pool, reserve)
	defer alloc.Release()

	const count = 1000
	var slices [count]*Slice
	for i := 0; i != count; i++ {
		slices[i] = alloc.Alloc(bytes)
		copy(slices[i].Contents, fmt.Sprintf("slice[%d]", i))
	}
	for i := 0; i != count; i++ {
		expected := fmt.Sprintf("slice[%d]", i)
		expectEq(t, expected, string(slices[i].Contents[0:len(expected)]))
		if slices[i].ExpandFront(reserve + 1) {
			t.Errorf("slice[%d] should not have a reserved byte %d", i, reserve+1)
		}
		if !slices[i].ExpandFront(reserve) {
			t.Errorf("slice[%d] should have a reserved byte %d", i, reserve)
		}
		slices[i].Release()
	}
}

func TestAllocatorAllocSmallWitReserve_0(t *testing.T)   { testAllocatorAlloc(t, 50, 0) }
func TestAllocatorAllocSmallWitReserve_10(t *testing.T)  { testAllocatorAlloc(t, 50, 10) }
func TestAllocatorAllocSmallWitReserve_100(t *testing.T) { testAllocatorAlloc(t, 50, 100) }

func TestAllocatorAllocLargeWitReserve_0(t *testing.T)   { testAllocatorAlloc(t, 1000, 0) }
func TestAllocatorAllocLargeWitReserve_10(t *testing.T)  { testAllocatorAlloc(t, 1000, 10) }
func TestAllocatorAllocLargeWitReserve_100(t *testing.T) { testAllocatorAlloc(t, 1000, 100) }

func testAllocatorCopy(t *testing.T, reserve uint) {
	pool := NewPool(iobufSize)
	defer pool.Close()
	alloc := NewAllocator(pool, reserve)
	defer alloc.Release()

	const count = 1000
	var slices [count]*Slice
	for i := 0; i != count; i++ {
		slices[i] = alloc.Copy([]byte(fmt.Sprintf("slice[%d]", i)))
	}
	for i := 0; i != count; i++ {
		expectEq(t, fmt.Sprintf("slice[%d]", i), string(slices[i].Contents))
		if slices[i].ExpandFront(reserve + 1) {
			t.Errorf("slice[%d] should not have a reserved byte %d", i, reserve+1)
		}
		if !slices[i].ExpandFront(reserve) {
			t.Errorf("slice[%d] should have a reserved byte %d", i, reserve)
		}
		slices[i].Release()
	}
}

func TestAllocatorCopyWitReserve_0(t *testing.T)   { testAllocatorCopy(t, 0) }
func TestAllocatorCopyWitReserve_10(t *testing.T)  { testAllocatorCopy(t, 10) }
func TestAllocatorCopyWitReserve_100(t *testing.T) { testAllocatorCopy(t, 100) }

// Check that the Allocator is unusable after it is closed.
func TestAllocatorClose(t *testing.T) {
	pool := NewPool(iobufSize)
	defer pool.Close()
	alloc := NewAllocator(NewPool(iobufSize), 0)

	slice := alloc.Alloc(10)
	if slice == nil {
		t.Fatalf("slice should not be nil")
	}
	slice.Release()

	alloc.Release()
	slice = alloc.Alloc(10)
	if slice != nil {
		t.Errorf("slice should be nil")
	}
}
