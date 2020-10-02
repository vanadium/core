// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

import (
	"runtime"
	"sync"
	"testing"
)

const (
	iobufSize = 1 << 16
)

func expectEq(t *testing.T, x, y interface{}) {
	if x != y {
		_, file, line, _ := runtime.Caller(1)
		t.Errorf("%s(%d): expected %v, actual %v", file, line, x, y)
	}
}

// Test basic reference counting.
func TestFreelist(t *testing.T) {
	pool := NewPool(iobufSize)
	defer pool.Close()

	iobuf := pool.alloc(0)
	expectEq(t, iobufSize, len(iobuf.Contents))
	expectEq(t, uint64(1), pool.allocated)
	expectEq(t, 0, len(pool.freelist))
	iobuf.release()

	expectEq(t, 1, len(pool.freelist))
	iobuf = pool.alloc(0)
	expectEq(t, 0, len(pool.freelist))
	pool.alloc(0).release()
	iobuf.release()
	expectEq(t, 2, len(pool.freelist))
}

// Test slice reference counting.
func TestRefcount(t *testing.T) {
	pool := NewPool(iobufSize)
	defer pool.Close()

	iobuf := pool.alloc(0)
	slice1 := iobuf.slice(0, 0, 10)
	slice2 := iobuf.slice(10, 10, 20)
	iobuf.release()
	expectEq(t, 0, len(pool.freelist))
	slice1.Release()
	expectEq(t, 0, len(pool.freelist))
	slice2.Release()
	expectEq(t, 1, len(pool.freelist))
}

// Check that the Pool is unusable after it is closed.
func TestPoolClose(t *testing.T) {
	pool := NewPool(iobufSize)

	iobuf := pool.alloc(1024)
	if iobuf == nil {
		t.Fatalf("iobuf should not be nil")
	}
	iobuf.release()
	pool.Close()
	iobuf = pool.alloc(1024)
	if iobuf != nil {
		t.Errorf("iobuf should be nil")
	}
}

func TestIOBUFConcurrency(t *testing.T) {
	pool := NewPool(iobufSize)
	defer pool.Close()

	const threadCount = 100

	var pending sync.WaitGroup
	pending.Add(threadCount)
	for i := 0; i != threadCount; i++ {
		go func() {
			iobufThrasher(t, pool)
			pending.Done()
		}()
	}
	pending.Wait()
}

func iobufThrasher(t *testing.T, pool *Pool) {
	const (
		iobufCount = 100
		sliceCount = 100
	)
	message := "Hello world"
	for i := 0; i != iobufCount; i++ {
		iobuf := pool.alloc(0)
		var slices []*Slice
		var base uint
		for j := 0; j != sliceCount; j++ {
			if base+uint(len(message)) > uint(len(iobuf.Contents)) {
				iobuf.release()
				iobuf = pool.alloc(0)
			}
			slice := iobuf.slice(base, base, base+uint(len(message)))
			base += uint(len(message))
			copy(slice.Contents, message)
			slices = append(slices, slice)
		}
		iobuf.release()
		for _, slice := range slices {
			content := string(slice.Contents)
			if content != message {
				t.Errorf("Expected %q, got %q", message, content)
			}
			slice.Release()
		}
	}
}
