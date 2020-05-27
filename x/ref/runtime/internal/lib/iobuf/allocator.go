// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iobuf

import "v.io/x/ref/internal/logger"

// Allocator is an allocator for Slices that tries to allocate contiguously.
// That is, sequential allocations will tend to be contiguous, which means
// that Coalesce() will usually be able to perform coalescing (without
// copying the data).
//
//    calloc := iobuf.Allocator(...)
//    slice1 := calloc.Alloc(10)
//    slice2 := calloc.Alloc(20)
//    slices := iobuf.Coalesce([]*iobuf.Slice{slice1, slice2})
//    // slices should contain 1 element with length 30.
type Allocator struct {
	pool    *Pool
	iobuf   *buf
	index   uint
	reserve uint
}

// NewAllocator returns a new Slice allocator.
//
// <reserve> is the number of spare bytes to reserve at the beginning of
// each allocated Slice. This can be used to reserve space for a header,
// for example.
//
// NOTE: It's a bit weird to set the number of reserve bytes in the NewAllocator
// call; it seems more natural in the Alloc call. But it's convenient to set it
// in NewAllocator, because in our use-cases, the code that calls Alloc doesn't
// know the number of reserve bytes.
func NewAllocator(pool *Pool, reserve uint) *Allocator {
	return &Allocator{pool: pool, reserve: reserve}
}

// Release releases the allocator.
func (a *Allocator) Release() {
	if a.iobuf != nil {
		a.iobuf.release()
		a.iobuf = nil
	}
	a.pool = nil
}

// Alloc allocates a new Slice.
func (a *Allocator) Alloc(bytes uint) *Slice {
	n := bytes + a.reserve
	if a.iobuf == nil {
		if a.pool == nil {
			logger.Global().Info("iobuf.Allocator has already been closed")
			return nil
		}
		a.iobuf = a.pool.alloc(n)
	} else if uint(len(a.iobuf.Contents)) < a.index+n {
		a.iobuf.release()
		a.iobuf = a.pool.alloc(n)
		a.index = 0
	}
	free := a.index
	base := free + a.reserve
	a.index += n
	return a.iobuf.slice(free, base, a.index)
}

// Copy allocates a Slice and copies the buf into it.
func (a *Allocator) Copy(buf []byte) *Slice {
	slice := a.Alloc(uint(len(buf)))
	copy(slice.Contents, buf)
	return slice
}
