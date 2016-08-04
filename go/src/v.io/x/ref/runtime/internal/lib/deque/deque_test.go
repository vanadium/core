// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deque

import (
	"testing"

	"v.io/x/ref/test/testutil"
)

func TestBasic(t *testing.T) {
	var q T
	if q.Size() != 0 {
		t.Errorf("Expected size to be 0, got %d", q.Size())
	}
	item := q.PopFront()
	if item != nil {
		t.Errorf("Expected front to be nil, got %v", item)
	}
	item = q.PopBack()
	if item != nil {
		t.Errorf("Expected back to be nil, got %v", item)
	}

	q.PushFront(1)
	item = q.PopFront()
	if item != 1 {
		t.Errorf("Expected item to be 1, got %v", item)
	}
	item = q.PopFront()
	if item != nil {
		t.Errorf("Expected back to be nil, got %v", item)
	}

	q.PushBack(2)
	item = q.PopBack()
	if item != 2 {
		t.Errorf("Expected item to be 2, got %v, %#v", item, q)
	}
	item = q.PopBack()
	if item != nil {
		t.Errorf("Expected back to be nil, got %v", item)
	}
}

func TestBackToFront(t *testing.T) {
	var q T
	for i := 0; i != 100; i++ {
		q.PushBack(i)
	}
	for i := 0; i != 100; i++ {
		item := q.PopFront()
		if item != i {
			t.Errorf("Expected %d, got %v", i, item)
		}
	}
	item := q.PopFront()
	if item != nil {
		t.Errorf("Expected nil, got %v", item)
	}
}

func TestFrontToBack(t *testing.T) {
	var q T
	for i := 0; i != 100; i++ {
		q.PushFront(i)
	}
	for i := 0; i != 100; i++ {
		item := q.PopBack()
		if item != i {
			t.Errorf("Expected %d, got %v", i, item)
		}
	}
	item := q.PopBack()
	if item != nil {
		t.Errorf("Expected nil, got %v", item)
	}
}

func TestFrontToFront(t *testing.T) {
	var q T
	for i := 0; i != 100; i++ {
		q.PushFront(i)
	}
	for i := 99; i >= 0; i-- {
		item := q.PopFront()
		if item != i {
			t.Errorf("Expected %d, got %v", i, item)
		}
	}
	item := q.PopFront()
	if item != nil {
		t.Errorf("Expected nil, got %v", item)
	}
}

func TestBackToBack(t *testing.T) {
	var q T
	for i := 0; i != 100; i++ {
		q.PushBack(i)
	}
	for i := 99; i >= 0; i-- {
		item := q.PopBack()
		if item != i {
			t.Errorf("Expected %d, got %v", i, item)
		}
	}
	item := q.PopBack()
	if item != nil {
		t.Errorf("Expected nil, got %v", item)
	}
}

func TestClear(t *testing.T) {
	var q T
	for i := 0; i != 100; i++ {
		q.PushBack(i)
	}
	q.Clear()
	size := q.Size()
	if size != 0 {
		t.Errorf("Expected size 0, but got %d", size)
	}
	item := q.PopFront()
	if item != nil {
		t.Errorf("Expected front to be nil, got %v", item)
	}
	item = q.PopBack()
	if item != nil {
		t.Errorf("Expected back to be nil, got %v", item)
	}
}

func TestRandom(t *testing.T) {
	testutil.InitRandGenerator(t.Logf)
	var q T
	var contents []int
	for i := 0; i != 1000; i++ {
		switch testutil.RandomIntn(4) {
		case 0:
			i := testutil.RandomInt()
			contents = append([]int{i}, contents...)
			q.PushFront(i)
		case 1:
			i := testutil.RandomInt()
			contents = append(contents, i)
			q.PushBack(i)
		case 2:
			size := len(contents)
			if q.Size() != size {
				t.Errorf("Expected size %d, but got %d", size, q.Size())
			}
			item := q.PopFront()
			if size == 0 {
				if item != nil {
					t.Errorf("Expected nil, but got %v", item)
				}
			} else {
				if item != contents[0] {
					t.Errorf("Expected %d, got %d", contents[0], item)
				}
				contents = contents[1:]
			}
		case 3:
			size := len(contents)
			if q.Size() != size {
				t.Errorf("Expected size %d, got %d", size, q.Size())
			}
			item := q.PopBack()
			if size == 0 {
				if item != nil {
					t.Errorf("Expected nil, got %v", item)
				}
			} else {
				if item != contents[len(contents)-1] {
					t.Errorf("Expected %d, got %d", contents[len(contents)-1], item)
				}
				contents = contents[:len(contents)-1]
			}
		}
	}
}

func TestIter(t *testing.T) {
	var q T

	// Iterate, quadratic.
	for i := 0; i != 10; i++ {
		check := 0
		q.Iter(func(it interface{}) bool {
			if it != check {
				t.Errorf("Expected %d, actual %v", check, it)
			}
			check++
			return true
		})
		if check != i {
			t.Errorf("Queue is too short, expected %d, actual %d", i, check)
		}
		q.PushBack(i)
	}

	// Check short iteration.
	iterations := 0
	q.Iter(func(it interface{}) bool {
		iterations++
		return it.(int) < 5
	})
	if iterations != 6 {
		t.Errorf("Iteration did not stop correctly,expected 6 iterations, got %d", iterations)
	}
}

func BenchmarkPushFront(b *testing.B) {
	var q T
	for i := 0; i < b.N; i++ {
		q.PushFront(i)
	}
}

func BenchmarkPushBack(b *testing.B) {
	var q T
	for i := 0; i < b.N; i++ {
		q.PushBack(i)
	}
}

func BenchmarkPopFront(b *testing.B) {
	var q T
	for i := 0; i < b.N; i++ {
		q.PushBack(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PopFront()
	}
}

func BenchmarkPopBack(b *testing.B) {
	var q T
	for i := 0; i < b.N; i++ {
		q.PushFront(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.PopBack()
	}
}
