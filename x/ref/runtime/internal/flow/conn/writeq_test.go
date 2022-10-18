// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/x/ref/test"
)

type writeqEntry struct {
	writer
}

func newEntry() *writeqEntry {
	wqe := &writeqEntry{
		writer: writer{notify: make(chan struct{}, 1)},
	}
	return wqe
}

func (q *writeq) addWriter(w *writer, p int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.addWriterLocked(w, p)
}

func addWriteq(wq *writeq, priority int, w ...*writeqEntry) {
	for i := range w {
		wq.addWriter(&w[i].writer, priority)
	}
}

func rmWriteq(wq *writeq, priority int, w ...*writeqEntry) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	for i := range w {
		wq.rmWriterLocked(&w[i].writer, priority)
	}
}

func (q *writeq) getActive() *writer {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.active
}

func listWQEntriesLocked(wq *writeq, p int) []*writer {
	var r []*writer
	for w := wq.activeWriters[p]; w != nil; w = w.next {
		r = append(r, w)
		if w.next == wq.activeWriters[p] {
			break
		}
	}
	return r
}

func cmpWriteqEntries(t *testing.T, wq *writeq, priority int, active *writeqEntry, w ...*writeqEntry) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	_, _, line, _ := runtime.Caller(1)
	if active == nil {
		if got, want := wq.active, (*writer)(nil); got != want {
			t.Fatalf("line %v: active: got %v, want %v", line, got, want)
		}
	} else {
		if got, want := wq.active, &active.writer; got != want {
			t.Fatalf("line %v: active: got %v, want %v", line, got, want)
		}
	}

	var wl []*writer
	if len(w) > 0 {
		wl = make([]*writer, len(w))
		for i := range wl {
			wl[i] = &w[i].writer
		}
	}
	if got, want := listWQEntriesLocked(wq, priority), wl; !reflect.DeepEqual(got, want) {
		t.Fatalf("line %v: queue: got %v, want %v", line, got, want)
	}
}

func cmpWriteqNext(t *testing.T, wq *writeq, w *writeqEntry) {
	_, _, line, _ := runtime.Caller(1)
	nw, _ := wq.nextLocked()
	if got, want := nw, &w.writer; got != want {
		t.Errorf("line %v: next: got %v, want %v", line, got, want)
	}
}

func TestWriteqLists(t *testing.T) {
	wq := &writeq{}

	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	addWriteq(wq, flowPriority, fe1, fe2)
	addWriteq(wq, expressPriority, fe3)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe1, fe2)
	cmpWriteqEntries(t, wq, expressPriority, nil, fe3)
	cmpWriteqNext(t, wq, fe3)
	cmpWriteqEntries(t, wq, expressPriority, nil)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe1, fe2)

	cmpWriteqNext(t, wq, fe1)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe2)
	cmpWriteqNext(t, wq, fe2)
	cmpWriteqEntries(t, wq, flowPriority, nil)

	fe1, fe2, fe3 = newEntry(), newEntry(), newEntry()
	fe4, fe5, fe6 := newEntry(), newEntry(), newEntry()
	addWriteq(wq, flowPriority, fe1, fe2)
	addWriteq(wq, expressPriority, fe3)
	addWriteq(wq, flowPriority, fe4)
	addWriteq(wq, expressPriority, fe5, fe6)

	cmpWriteqEntries(t, wq, expressPriority, nil, fe3, fe5, fe6)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe1, fe2, fe4)
	cmpWriteqNext(t, wq, fe3)
	cmpWriteqNext(t, wq, fe5)
	cmpWriteqEntries(t, wq, expressPriority, nil, fe6)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe1, fe2, fe4)
	addWriteq(wq, expressPriority, fe3)
	cmpWriteqEntries(t, wq, expressPriority, nil, fe6, fe3)

	rmWriteq(wq, flowPriority, fe2)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe1, fe4)
	rmWriteq(wq, flowPriority, fe1)
	rmWriteq(wq, flowPriority, fe1)
	cmpWriteqEntries(t, wq, flowPriority, nil, fe4)
	rmWriteq(wq, flowPriority, fe4)
	cmpWriteqEntries(t, wq, flowPriority, nil)

	// Make sure that removing the currently active writer works correctly.
	// This is used internally to remove a writer whose context is canceled.
	rmWriteq(wq, flowPriority, fe4)
	wq.nextLocked()
	cmpWriteqEntries(t, wq, expressPriority, nil, fe3)
	rmWriteq(wq, flowPriority, fe6)
	cmpWriteqEntries(t, wq, expressPriority, nil, fe3)

}

func TestWriteqErrors(t *testing.T) {
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()
	wq.wait(nil, &fe1.writer, expressPriority)

	var ready sync.WaitGroup
	ready.Add(2)
	go func() {
		ready.Done()
		wq.wait(nil, &fe2.writer, expressPriority)
	}()
	go func() {
		ready.Done()
		wq.wait(nil, &fe3.writer, expressPriority)
	}()

	ready.Wait()
	err := wq.wait(nil, &fe3.writer, expressPriority)
	if err == nil || !strings.Contains(err.Error(), "already exists in the writeq") {
		t.Fatalf("missing or unexpected error: %v", err)
	}

}

func TestWriteqNotifySerial(t *testing.T) {
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	// wait will return immediately if the writeq is empty and
	// there is no active writer.
	wq.wait(nil, &fe1.writer, expressPriority)
	cmpWriteqEntries(t, wq, flowPriority, fe1)
	cmpWriteqEntries(t, wq, expressPriority, fe1)
	// reset the active writer to allow the next wq.wait to run straight
	// through.
	wq.done(&fe1.writer)
	wq.wait(nil, &fe2.writer, flowPriority)
	cmpWriteqEntries(t, wq, flowPriority, fe2)
	cmpWriteqEntries(t, wq, expressPriority, fe2)
	wq.done(&fe2.writer)
	wq.wait(nil, &fe3.writer, flowPriority)
	cmpWriteqEntries(t, wq, flowPriority, fe3)
	cmpWriteqEntries(t, wq, expressPriority, fe3)
	wq.done(&fe3.writer)
	cmpWriteqEntries(t, wq, flowPriority, nil)
	cmpWriteqEntries(t, wq, expressPriority, nil)
}

func TestWriteqNotifyPriority(t *testing.T) {
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	ch := make(chan *writeqEntry, 3)
	var wg sync.WaitGroup
	wg.Add(2)

	first, second := make(chan struct{}), make(chan struct{})
	wq.wait(nil, &fe1.writer, flowPriority)

	go func() {
		<-first
		wq.wait(nil, &fe2.writer, flowPriority)
		ch <- fe2
	}()
	go func() {
		<-second
		wq.wait(nil, &fe3.writer, expressPriority)
		ch <- fe3
	}()

	close(first)

	waitFor(func() bool {
		wq.mu.Lock()
		defer wq.mu.Unlock()
		return wq.active == &fe1.writer &&
			wq.activeWriters[flowPriority] == &fe2.writer
	})

	cmpWriteqEntries(t, wq, flowPriority, fe1, fe2)
	close(second)

	waitFor(func() bool {
		wq.mu.Lock()
		defer wq.mu.Unlock()
		return wq.active == &fe1.writer &&
			wq.activeWriters[expressPriority] == &fe3.writer
	})
	cmpWriteqEntries(t, wq, expressPriority, fe1, fe3)

	// fe2 and fe3 are blocked until now.
	wq.done(&fe1.writer)

	// fe3 should run first because of its priority, even though
	// it called wait second.
	if got, want := <-ch, fe3; got != want {
		t.Errorf("got %p, want %p", got, want)
	}
	wq.done(&fe3.writer)
	if got, want := <-ch, fe2; got != want {
		t.Errorf("got %p, want %p", got, want)
	}
}

func TestWriteqSimpleOrdering(t *testing.T) {
	wq := &writeq{}
	start := newEntry()
	wq.wait(nil, &start.writer, flowPriority)

	nworkers := 10
	var wg sync.WaitGroup
	wg.Add(nworkers)
	numCh := make(chan int, 1)
	numChDone := make(chan struct{})
	doneCh := make(chan *writeqEntry, nworkers)
	errCh := make(chan error, nworkers)

	var writerMu sync.Mutex
	writers := make([]*writeqEntry, nworkers)

	// Test simple FIFO ordering and invariants.

	// Create nworkers+1 goroutines, the last of which will close the
	// channel so that the write queue can be examined.
	for i := 0; i <= nworkers; i++ {
		wr := newEntry()
		go func(w *writeqEntry, id int) {
			n := <-numCh
			if n >= nworkers {
				// All of the prior goroutines are blocked in their
				// invocation of wq.wait. Closing this channle allows
				// the code below to inspect the writeq.
				close(numChDone)
				return
			}
			writerMu.Lock()
			writers[n] = wr
			writerMu.Unlock()
			numCh <- n + 1
			wq.wait(nil, &w.writer, flowPriority)
			active := wq.getActive()
			if active != &w.writer {
				errCh <- fmt.Errorf("invariant violated: active: got %p, want %p", active, &w.writer)
			}
			time.Sleep(time.Duration(rand.Int31n(100)) * time.Millisecond)
			wq.done(&w.writer)
			doneCh <- wr
			wg.Done()
		}(wr, i+1)
	}

	numCh <- 0
	<-numChDone
	// All goroutines are now blocked in writeq.wait, waiting
	// in the order that they were created.
	writerMu.Lock()
	cmpWriteqEntries(t, wq, flowPriority, start, writers...)
	writerMu.Unlock()

	// Release the first writeq.wait
	wq.done(&start.writer)

	wg.Wait()
	cmpWriteqEntries(t, wq, flowPriority, nil)

	close(doneCh)
	close(errCh)
	i := 0
	// All of the goroutines should get their writeq turn in the
	// order that they were created.
	for w := range doneCh {
		if got, want := w, writers[i]; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		i++
	}

	for err := range errCh {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestWriteqSharedEntries(t *testing.T) {
	wq := &writeq{}

	nworkers := 10
	niterations := 1000
	shared := newEntry()
	var sharedMu sync.Mutex
	var done, ready sync.WaitGroup
	done.Add(nworkers)
	ready.Add(nworkers)
	goCh := make(chan struct{})

	ran := map[int]map[int]int{}
	for i := 0; i < nworkers; i++ {
		ran[i] = map[int]int{}
	}
	for i := 0; i < nworkers; i++ {
		go func(i int) {
			ready.Done()
			<-goCh
			defer done.Done()

			for j := 0; j < niterations; j++ {
				sharedMu.Lock()
				wq.wait(nil, &shared.writer, flowPriority)
				ran[i][j]++
				wq.done(&shared.writer)
				sharedMu.Unlock()
			}
		}(i)
	}
	ready.Wait()
	close(goCh)
	done.Wait()

	// Make sure they all got to run exactly once.
	if got, want := len(ran), nworkers; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	for i := 0; i < nworkers; i++ {
		if got, want := len(ran[i]), niterations; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		for j := 0; j < niterations; j++ {
			if got, want := ran[i][j], 1; got != want {
				t.Errorf("%v:%v: got %v, want %v", i, j, got, want)
			}
		}
	}
}

func TestWriteqConcurrency(t *testing.T) {
	wq := &writeq{}
	ctx, shutdown := test.V23Init()
	defer shutdown()

	nworkers := 100
	niterations := 1000
	var done sync.WaitGroup
	errCh := make(chan error, nworkers)
	done.Add(nworkers)

	for i := 0; i < nworkers; i++ {
		go func(i int) {
			defer done.Done()
			shared := newEntry()
			for j := 0; j < niterations; j++ {
				priority := expressPriority
				if j%2 == 0 {
					priority = flowPriority
				}
				if err := wq.wait(ctx, &shared.writer, priority); err != nil {
					errCh <- err
					return
				}
				active := wq.getActive()
				if active != &shared.writer {
					errCh <- fmt.Errorf("invariant violated: active: got %p, want %p", active, &shared.writer)
					return
				}
				time.Sleep(time.Duration(rand.Int31n(100)) * time.Nanosecond)
				wq.done(&shared.writer)
			}
		}(i)
	}
	done.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Error(err)
		}
	}
}

func nilPointers(w *writer) error {
	if w.prev != nil || w.next != nil {
		return fmt.Errorf("pointers should be nil: %p: %p <-> %p", w, w.prev, w.next)
	}
	return nil
}

func TestWriteqContextCancel(t *testing.T) {
	wq := &writeq{}
	rctx, shutdown := test.V23Init()

	defer shutdown()
	fe1, fe2 := newEntry(), newEntry()

	ctx, cancel := context.WithCancel(rctx)

	isNil := func(w *writeqEntry) {
		if err := nilPointers(&w.writer); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line %v: %v", line, err)
		}
	}

	wq.wait(ctx, &fe1.writer, expressPriority)
	cancel()
	err := wq.wait(ctx, &fe2.writer, expressPriority)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("missing or unexpected error: %v", err)
	}
	isNil(fe2)

	// fe2 will never make it into the queue since it was canceled.
	cmpWriteqEntries(t, wq, expressPriority, fe1)
	// fe1 is still the active writer since the cancel was issued after
	// it's wait had returned.
	wq.done(&fe1.writer)
	cmpWriteqEntries(t, wq, expressPriority, nil)

	nworkers := 100
	var done sync.WaitGroup
	var waiters sync.WaitGroup
	startCancel := make(chan struct{})

	errCh := make(chan error, nworkers)
	done.Add(nworkers * 2)
	waiters.Add(nworkers)

	wq.wait(ctx, &fe1.writer, expressPriority)

	// Need to use a largish number of goroutines to exercise all of the
	// paths in writeq.wait.
	for i := 0; i < nworkers; i++ {
		ctx, cancel := context.WithCancel(rctx)
		go func(i int) {
			waiters.Done()
			defer done.Done()
			shared := newEntry()
			if err := wq.wait(ctx, &shared.writer, flowPriority); err != nil {
				if perr := nilPointers(&shared.writer); perr != nil {
					errCh <- fmt.Errorf("%v: %v", err, perr)
					return
				}
				errCh <- err
				return
			}
		}(i)
		go func(cancel func()) {
			<-startCancel
			defer done.Done()
			time.Sleep(time.Duration(rand.Int31n(100)) * time.Nanosecond)
			cancel()
		}(cancel)
	}

	waiters.Wait()
	wq.done(&fe1.writer)
	isNil(fe1)
	close(startCancel)
	done.Wait()
	close(errCh)
	nerrors := 0
	for err := range errCh {
		if err == nil || !strings.Contains(err.Error(), "context canceled") {
			t.Fatalf("missing or unexpected error: %v", err)
		}
		nerrors++
	}

	if got, want := nerrors, nworkers/2; got < want {
		t.Errorf("got %v, want >= %v", got, want)
	}

}

func TestWriteqContextCancelSpecial(t *testing.T) {
	rctx, shutdown := test.V23Init()
	defer shutdown()

	wq := &writeq{}
	fe1 := newEntry()

	nWriters := 2
	nIterations := 100
	cancelations := 0

	writers := make([]*writeqEntry, nWriters)
	for i := 0; i < nWriters; i++ {
		writers[i] = newEntry()
		fmt.Printf("%v: %p\n", i, writers[i])
	}

	// This test is intended to catch the case where a writer that
	// has just become active and is waiting to receive its notification
	// is also canceled. In this case the notification and the cancelation
	// are racing, hence we need to run the loop below enough times to
	// to have the cancelation win the race. This race also occurs with
	// the RPC 'mux' benchmarks which run both streaming and non-streaming
	// RPCs with cancelations. See handleCancel in writeq.go.
	for i := 0; i < nIterations; i++ {

		ctx, cancel := context.WithCancel(rctx)

		var wg sync.WaitGroup

		errCh := make(chan error, nWriters*2)

		fmt.Printf("%v: \n", i)

		runner := func(ctx *context.T, id int, we *writeqEntry) {
			err := wq.wait(ctx, &we.writer, flowPriority)
			errCh <- err
			if err != nil {
				fmt.Printf("%v: --- error: %p: %v\n", id, &we.writer, err)
				if errors.Is(err, context.Canceled) {
					if err := nilPointers(&we.writer); err != nil {
						_, _, line, _ := runtime.Caller(1)
						errCh <- fmt.Errorf("line %v: %v: %v", line, id, err)
					}
				}
				return
			}
			fmt.Printf("%v: --- before: %p\n", id, &we.writer)
			wq.done(&we.writer)
			fmt.Printf("%v: --- after: %p\n", id, &we.writer)
		}

		if err := wq.wait(ctx, &fe1.writer, flowPriority); err != nil {
			t.Fatal(err)
		}

		fmt.Printf("fe1: %p\n", &fe1.writer)
		for j := 0; j < nWriters; j++ {
			fmt.Printf("%v: %p\n", j, writers[j])
		}

		wg.Add(nWriters)
		for j := 0; j < nWriters; j++ {
			go func(j int) {
				runner(ctx, j, writers[j])
				wg.Done()
			}(j)
		}

		cancel()
		fmt.Printf("calling done on fe1: %p\n", &fe1.writer)
		wq.done(&fe1.writer)
		fmt.Printf("done on fe1 done\n")

		wq.mu.Lock()
		if wq.active == &fe1.writer {
			t.Fatalf("%v: active should not anything but %p: %p %v\n", i, &fe1.writer, wq.active, wq.active)
		}
		wq.mu.Unlock()

		wg.Wait()

		wq.mu.Lock()
		if wq.active != nil {
			t.Fatalf("%v: active should be nil, not %p %v\n", i, wq.active, wq.active)
		}
		wq.mu.Unlock()
		cmpWriteqEntries(t, wq, flowPriority, nil)
		close(errCh)

		for err := range errCh {
			if err != nil {
				if errors.Is(err, context.Canceled) {
					cancelations++
				} else {
					t.Error(err)
				}
			}
		}

		/*		if cancelations > 0 {
				break
			}*/

	}

	if cancelations == 0 {
		t.Errorf("expected some cancelations")
	}

}
