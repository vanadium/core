// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upcqueue

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"v.io/x/ref/internal/logger"
	vsync "v.io/x/ref/runtime/internal/lib/sync"
)

const (
	elementCount = 100
	writerCount  = 10
	readerCount  = 10
)

// Test normal Put()/Get() combination.
func TestSimplePut(t *testing.T) {
	queue := New()
	done := make(chan struct{}, 1)
	go func() {
		queue.Put(1) //nolint:errcheck
		done <- struct{}{}
	}()

	<-done

	item, err := queue.Get(nil)
	if err != nil {
		t.Errorf("Get: %v", err)
	}

	if item.(int) != 1 {
		t.Errorf("Expected 1, actual=%v", item)
	}
}

// Test normal Put()/Get() combination.
func TestSimpleGet(t *testing.T) {
	queue := New()
	done := make(chan struct{}, 1)
	go func() {
		item, err := queue.Get(nil)
		if err != nil {
			t.Errorf("Get: %v", item)
		}
		if item.(int) != 1 {
			t.Errorf("Expected 1, actual=%v", item)
		}
		done <- struct{}{}
	}()

	select {
	case <-done:
		t.Errorf("Unexpected completion")
	default:
	}

	queue.Put(1) //nolint:errcheck
	<-done
}

// Test normal queue operation with a single producer and single consumer.
func TestSequential(t *testing.T) {
	queue := New()
	done := make(chan struct{}, 1)
	cancel := make(chan struct{})

	// Check that the queue elements are sequentially increasing ints.
	logger.Global().VI(1).Infof("Start consumer")
	go func() {
		for i := 0; i != elementCount; i++ {
			item, err := queue.Get(cancel)
			if err != nil {
				t.Errorf("Get: %v", err)
			}
			if item == nil {
				break
			}
			j := item.(int)
			if j != i {
				t.Errorf("Expected %d, actual %d", i, j)
			}
		}
		done <- struct{}{}
	}()

	// Generate the sequential ints.
	logger.Global().VI(1).Infof("Put values")
	for i := 0; i != elementCount; i++ {
		queue.Put(i) //nolint:errcheck
	}

	// Wait for the consumer.
	logger.Global().VI(1).Infof("Waiting for consumer")
	<-done

	// Any subsequent read should timeout.
	logger.Global().VI(1).Infof("Start consumer")
	go func() {
		_, err := queue.Get(cancel)
		if err != vsync.ErrCanceled {
			t.Errorf("Expected timeout: %v", err)
		}
		logger.Global().VI(1).Infof("Consumer done")
		done <- struct{}{}
	}()

	logger.Global().VI(1).Infof("Sleep a little")
	time.Sleep(100 * time.Millisecond)
	select {
	case <-done:
		t.Errorf("Unexpected completion")
	default:
	}

	logger.Global().VI(1).Infof("Cancel")
	close(cancel)

	logger.Global().VI(1).Infof("Wait for consumer")
	<-done
}

// Test that Get() returns an error when the queue is closed.
func TestSequentialClose(t *testing.T) {
	logger.Global().VI(1).Infof("Put")
	queue := New()
	err := queue.Put(0)
	if err != nil {
		t.Errorf("Put: %v", err)
	}
	logger.Global().VI(1).Infof("Close")
	queue.Close()

	// Check that Get() returns the element.
	logger.Global().VI(1).Infof("Get")
	item, err := queue.Get(nil)
	if err != nil {
		t.Errorf("Get: %v", err)
	}
	if item.(int) != 0 {
		t.Errorf("Unexpected value: %v", item)
	}

	// Check that Get() returns an error.
	logger.Global().VI(1).Infof("Get")
	_, err = queue.Get(nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}

	// Check that Put() returns an error.
	logger.Global().VI(1).Infof("Put")
	err = queue.Put(0)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}
}

// Test that concurrent Puts() may add values to the queue.
func TestConcurrentClose(t *testing.T) {
	queue := New()
	pending := &sync.WaitGroup{}
	pending.Add(2 * writerCount)
	for i := 0; i != writerCount; i++ {
		go func() {
			err := queue.Put(1)
			if err != nil {
				logger.Global().VI(1).Infof("Put: %v", err)
			}
			pending.Done()
		}()
	}
	time.Sleep(100 * time.Millisecond)
	queue.Close()
	for i := 0; i != writerCount; i++ {
		go func() {
			err := queue.Put(2)
			if err == nil {
				t.Errorf("Expected error")
			}
			pending.Done()
		}()
	}

	readers := 0
	for {
		item, err := queue.Get(nil)
		if err != nil {
			break
		}
		if item.(int) != 1 {
			t.Errorf("Expected 1, actual=%v", item)
		}
		readers++
	}
	logger.Global().VI(1).Infof("%d operations completed", readers)
	if readers > writerCount {
		t.Errorf("Too many readers")
	}
	pending.Wait()
}

// Test that Get() returns an error when the queue is shut down.
func TestSequentialShutdown(t *testing.T) {
	queue := New()

	logger.Global().VI(1).Infof("Put")
	err := queue.Put(0)
	if err != nil {
		t.Errorf("Put: %v", err)
	}

	logger.Global().VI(1).Infof("Shutdown")
	queue.Shutdown()

	// Check that Get() returns an error.
	logger.Global().VI(1).Infof("Get")
	_, err = queue.Get(nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}

	// Check that Put() returns an error.
	logger.Global().VI(1).Infof("Put")
	err = queue.Put(0)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}
}

// Test with concurrent producers, but a single consumer.
func TestConcurrentPutNoTimeouts(t *testing.T) {
	queue := New()
	pending := &sync.WaitGroup{}

	// Generate the sequential ints.
	for i := 0; i != writerCount; i++ {
		pending.Add(1)
		go func() {
			for j := 0; j != elementCount; j++ {
				queue.Put(j) //nolint:errcheck
			}
			pending.Done()
		}()
	}

	// Sum up the results and compare.
	sum := 0
	for i := 0; i != writerCount*elementCount; i++ {
		item, err := queue.Get(nil)
		if err != nil {
			t.Errorf("Get: %v", err)
		}
		if item == nil {
			break
		}
		sum += item.(int)
	}
	expected := writerCount * elementCount * (elementCount - 1) / 2
	if sum != expected {
		t.Errorf("Expected sum %d, received %d", expected, sum)
	}

	pending.Wait()
}

// Test with concurrent consumers and concurrent producers.
func TestConcurrentGet(t *testing.T) {
	queue := New()
	done := make(chan struct{})
	pending := &sync.WaitGroup{}
	pending.Add(readerCount + writerCount)
	cancel := make(chan struct{})

	// Sum up the results and compare.
	sum := uint32(0)
	count := uint32(0)
	logger.Global().VI(1).Infof("Start consumers")
	for i := 0; i != readerCount; i++ {
		pid := i
		go func() {
			for {
				c := atomic.LoadUint32(&count)
				if c == writerCount*elementCount {
					break
				}

				// The timeout is required for termination.
				item, err := queue.Get(cancel)
				if err != nil {
					continue
				}
				atomic.AddUint32(&sum, uint32(item.(int)))
				atomic.AddUint32(&count, 1)
			}
			logger.Global().VI(1).Infof("Consumer %d done", pid)
			pending.Done()
		}()
	}

	// Generate the sequential ints.
	logger.Global().VI(1).Infof("Start producers")
	for i := 0; i != writerCount; i++ {
		pid := i
		go func() {
			for j := 0; j != elementCount; j++ {
				err := queue.Put(j)
				if err != nil {
					t.Errorf("Put: %v", err)
				}
			}
			logger.Global().VI(1).Infof("Producer %d done", pid)
			pending.Done()
		}()
	}

	logger.Global().VI(1).Infof("Start termination checker")
	go func() {
		pending.Wait()
		done <- struct{}{}
	}()

	logger.Global().VI(1).Infof("Wait for processes")
	stop := false
	for !stop {
		time.Sleep(100 * time.Millisecond)
		select {
		case <-done:
			stop = true
		default:
			cancel <- struct{}{}
		}
	}

	logger.Global().VI(1).Infof("Checking the sum")
	expected := writerCount * elementCount * (elementCount - 1) / 2
	s := atomic.LoadUint32(&sum)
	if s != uint32(expected) {
		t.Errorf("Expected sum %d, received %d", expected, sum)
	}
}
