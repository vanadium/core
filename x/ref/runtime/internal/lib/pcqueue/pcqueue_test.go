// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcqueue

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"v.io/x/ref/internal/logger"
)

const (
	queueSize    = 10
	elementCount = 100
	writerCount  = 10
	readerCount  = 10
)

// Test normal Put()/Get() combination.
func TestSimplePut(t *testing.T) {
	queue := New(0)
	done := make(chan struct{}, 1)
	go func() {
		queue.Put(1, nil) //nolint:errcheck
		done <- struct{}{}
	}()

	select {
	case <-done:
		t.Errorf("Unexpected completion")
	default:
	}

	item, err := queue.Get(nil)
	if err != nil {
		t.Errorf("Get: %v", err)
	}

	if item.(int) != 1 {
		t.Errorf("Expected 1, actual=%v", item)
	}

	<-done
}

// Test normal Put()/Get() combination.
func TestSimpleGet(t *testing.T) {
	queue := New(0)
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
	queue.Put(1, nil) //nolint:errcheck
	<-done
}

// Test normal queue operation with a single producer and single consumer.
func TestSequential(t *testing.T) {
	queue := New(queueSize)
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
		queue.Put(i, nil) //nolint:errcheck
	}

	// Wait for the consumer.
	logger.Global().VI(1).Infof("Waiting for consumer")
	<-done

	// Any subsequent read should timeout.
	logger.Global().VI(1).Infof("Start consumer")
	go func() {
		_, err := queue.Get(cancel)
		if err != ErrCancelled {
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

// Test timeouts for PutWithTimeout() when there is no consumer.
func TestSequentialPutCancel(t *testing.T) {
	queue := New(queueSize)
	done := make(chan struct{}, 1)
	cancel := make(chan struct{})

	logger.Global().VI(1).Infof("Put values")
	for i := 0; i != queueSize; i++ {
		err := queue.Put(i, nil)
		if err != nil {
			t.Errorf("Put: %v", err)
		}
	}

	logger.Global().VI(1).Infof("Start producer")
	go func() {
		err := queue.Put(0, cancel)
		if err != ErrCancelled {
			t.Errorf("Put: expected cancellation: %v", err)
		}
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

	logger.Global().VI(1).Infof("Wait for producer")
	<-done
}

// Test that Get() returns an error when the queue is closed.
func TestSequentialClose(t *testing.T) {
	queue := New(queueSize)
	err := queue.Put(0, nil)
	if err != nil {
		t.Errorf("Put: %v", err)
	}
	queue.Close()

	// Check that Get() returns the element.
	item, err := queue.Get(nil)
	if err != nil {
		t.Errorf("Get: %v", err)
	}
	if item.(int) != 0 {
		t.Errorf("Unexpected value: %v", item)
	}

	// Check that Get() returns an error.
	_, err = queue.Get(nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}

	// Check that Put() returns an error.
	err = queue.Put(0, nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}
}

// Test that concurrent Puts() may add values to the queue.
func TestConcurrentClose(t *testing.T) {
	queue := New(0)
	pending := &sync.WaitGroup{}
	pending.Add(2 * writerCount)
	for i := 0; i != writerCount; i++ {
		go func() {
			err := queue.Put(1, nil)
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
			err := queue.Put(2, nil)
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
	queue := New(queueSize)
	err := queue.Put(0, nil)
	if err != nil {
		t.Errorf("Put: %v", err)
	}
	queue.Shutdown()

	// Check that Get() returns an error.
	_, err = queue.Get(nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}

	// Check that Put() returns an error.
	err = queue.Put(0, nil)
	if err != ErrQueueIsClosed {
		t.Errorf("Expected queue to be closed: %v", err)
	}
}

// Test with concurrent producers, but a single consumer.
func TestConcurrentPutNoTimeouts(t *testing.T) {
	queue := New(queueSize)
	pending := &sync.WaitGroup{}

	// Generate the sequential ints.
	for i := 0; i != writerCount; i++ {
		pending.Add(1)
		go func() {
			for j := 0; j != elementCount; j++ {
				queue.Put(j, nil) //nolint:errcheck
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
	queue := New(queueSize)
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
				err := queue.Put(j, nil)
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

func TestSimpleTryPut(t *testing.T) {
	q := New(1)
	if err := q.TryPut(1); err != nil {
		t.Errorf("TryPut(1) got error: %q", err)
	}

	if err := q.TryPut(2); err != ErrTryAgain {
		t.Errorf("TryPut(2) got error: %q; want: %q", err, ErrTryAgain)
	}

	if item, err := q.Get(nil); err != nil {
		t.Errorf("Get() got error: %q", err)
	} else if item.(int) != 1 {
		t.Errorf("Get() = %v; want: %v", item, 1)
	}

	q.Close()
	if err := q.TryPut(3); err != ErrQueueIsClosed {
		t.Errorf("TryPut(3) got error: %q; want: %q", err, ErrQueueIsClosed)
	}
}

func TestSequentialTryPut(t *testing.T) {
	q := New(queueSize)
	const numIter = 5
	for i := 0; i < numIter; i++ {
		// All succeed.
		for j := i * queueSize; j < (i+1)*queueSize; j++ {
			if err := q.TryPut(j); err != nil {
				t.Errorf("TryPut(%v) returned error: %q", j, err)
			}
		}
		// All fail.
		for j := (i + 1) * queueSize; j < (i+2)*queueSize; j++ {
			if err := q.TryPut(j); err != ErrTryAgain {
				t.Errorf("TryPut(%v) returned error %q; want %q", j, err, ErrTryAgain)
			}
		}
		// Empty the queue.
		for j := i * queueSize; j < (i+1)*queueSize; j++ {
			item, err := q.Get(nil)
			if err != nil {
				t.Errorf("Get() returned error: %q", err)
			} else if item.(int) != j {
				t.Errorf("Get() = %v; want %v", item.(int), j)
			}
		}
	}

	q.Close()
	for i := numIter * queueSize; i < (numIter+1)*queueSize; i++ {
		if err := q.TryPut(i); err != ErrQueueIsClosed {
			t.Errorf("TryPut(%v) returned error %q; want %q", i, err, ErrQueueIsClosed)
		}
	}
}

func TestConcurrentTryPut(t *testing.T) {
	q := New(queueSize)
	pending := &sync.WaitGroup{}
	for i := 0; i != writerCount; i++ {
		pending.Add(1)
		go func(i int) {
			for j := 0; j != elementCount; j++ {
				// TryPut(j) until we succeed.
				for {
					err := q.TryPut(j)
					if err == nil {
						break
					}
					if err == ErrTryAgain {
						time.Sleep(1 * time.Millisecond)
					} else {
						t.Errorf("%v: TryPut(%v) returned error %q; want %q", i, j, err, ErrTryAgain)
					}
				}
			}
			pending.Done()
		}(i)
	}

	// Sum up the results and compare.
	sum := 0
	for i := 0; i != writerCount*elementCount; i++ {
		item, err := q.Get(nil)
		if err != nil {
			t.Errorf("Get() returned error: %q", err)
			continue
		}
		if item == nil {
			continue
		}
		sum += item.(int)
	}

	if expected := writerCount * elementCount * (elementCount - 1) / 2; sum != expected {
		t.Errorf("got sum %v, want %v", expected, sum)
	}

	pending.Wait()
}
