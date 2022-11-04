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
	"v.io/x/ref/test/goroutines"
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
		t.Fatalf("line %v: queue:\n\tgot:  %v\n\twant: %v", line, got, want)
	}
}

func cmpWriteqNext(t *testing.T, wq *writeq, w *writeqEntry) {
	_, _, line, _ := runtime.Caller(1)
	nw, _ := wq.nextLocked()
	if got, want := nw, &w.writer; got != want {
		t.Errorf("line %v: next: got %v, want %v", line, got, want)
	}
}

func assertChanSize(t *testing.T, ch chan struct{}, size int) {
	if got, want := len(ch), size; got != want {
		_, _, line, _ := runtime.Caller(2)
		t.Fatalf("line: %v, got %v, want %v", line, got, want)
	}
}

func assertActive(t *testing.T, wq *writeq, w *writer) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if got, want := wq.active, w; got != want {
		_, _, line, _ := runtime.Caller(2)
		t.Fatalf("line: %v, got %v, want %v", line, got, want)
	}
}

func assertNotActive(t *testing.T, wq *writeq, w *writer) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if got, want := wq.active, w; got == want {
		_, _, line, _ := runtime.Caller(2)
		t.Fatalf("line: %v, got %v should not be the same as want", line, got)
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
	defer goroutines.NoLeaks(t, leakWaitTime)()
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()
	wq.wait(nil, &fe1.writer, expressPriority)

	var done sync.WaitGroup
	done.Add(2)
	go func() {
		wq.wait(nil, &fe2.writer, expressPriority)
		done.Done()
	}()
	go func() {
		wq.wait(nil, &fe3.writer, expressPriority)
		done.Done()
	}()

	waitForActiveAndQueued(t, wq, expressPriority, fe1, fe3)
	err := wq.wait(nil, &fe3.writer, expressPriority)
	if err == nil || !strings.Contains(err.Error(), "already exists in the writeq") {
		t.Fatalf("missing or unexpected error: %v", err)
	}
	wq.done(&fe1.writer)
	wq.done(&fe2.writer)
	wq.done(&fe3.writer)
	done.Wait()
}

func TestWriteqNotifySerial(t *testing.T) {
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

	// wait will return immediately if the writeq is empty and
	// there is no active writer.
	wq.wait(nil, &fe1.writer, expressPriority)
	assertWait(fe1)
	cmpWriteqEntries(t, wq, flowPriority, fe1)
	cmpWriteqEntries(t, wq, expressPriority, fe1)
	// reset the active writer to allow the next wq.wait to run straight
	// through.
	wq.done(&fe1.writer)
	wq.wait(nil, &fe2.writer, flowPriority)
	assertWait(fe2)
	cmpWriteqEntries(t, wq, flowPriority, fe2)
	cmpWriteqEntries(t, wq, expressPriority, fe2)
	wq.done(&fe2.writer)
	wq.wait(nil, &fe3.writer, flowPriority)
	assertWait(fe3)
	cmpWriteqEntries(t, wq, flowPriority, fe3)
	cmpWriteqEntries(t, wq, expressPriority, fe3)
	wq.done(&fe3.writer)
	cmpWriteqEntries(t, wq, flowPriority, nil)
	cmpWriteqEntries(t, wq, expressPriority, nil)
}

func waitForActiveAndQueued(t *testing.T, wq *writeq, priority int, a, b *writeqEntry) {
	err := waitFor(time.Minute, func() error {
		wq.mu.Lock()
		defer wq.mu.Unlock()
		if wq.active != &a.writer {
			return fmt.Errorf("%p is not active: %s", &a.writer, wq.stringLocked())
		}
		for w := wq.activeWriters[priority]; w != nil; w = w.next {
			if w == &b.writer {
				return nil
			}
		}
		return fmt.Errorf("%p is not queued: %s", &b.writer, wq.stringLocked())
	})
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		t.Errorf("line: %v, %v", line, err)
	}
}

func TestWriteqNotifyPriority(t *testing.T) {
	wq := &writeq{}
	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	ch := make(chan *writeqEntry, 3)
	var wg sync.WaitGroup
	wg.Add(2)

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

	first, second := make(chan struct{}), make(chan struct{})
	wq.wait(nil, &fe1.writer, flowPriority)
	assertWait(fe1)

	go func() {
		<-first
		wq.wait(nil, &fe2.writer, flowPriority)
		assertWait(fe2)
		ch <- fe2
	}()
	go func() {
		<-second
		wq.wait(nil, &fe3.writer, expressPriority)
		assertWait(fe3)
		ch <- fe3
	}()

	close(first)

	waitForActiveAndQueued(t, wq, flowPriority, fe1, fe2)
	cmpWriteqEntries(t, wq, flowPriority, fe1, fe2)
	close(second)

	waitForActiveAndQueued(t, wq, expressPriority, fe1, fe3)
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

type doneRecord struct {
	we *writeqEntry
	n  int
}

func TestWriteqFIFOOrdering(t *testing.T) {
	wq := &writeq{}
	start := newEntry()

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)

	}

	wq.wait(nil, &start.writer, flowPriority)
	assertWait(start)

	nworkers := 10
	var wg sync.WaitGroup
	wg.Add(nworkers)
	numCh := make(chan int, 1)
	numChDone := make(chan struct{})
	doneCh := make(chan doneRecord, nworkers)
	errCh := make(chan error, nworkers)

	var writerMu sync.Mutex
	writers := make([]*writeqEntry, nworkers)

	// Test FIFO ordering and invariants.

	// Create nworkers+1 goroutines, the last of which will close the
	// channel so that the write queue can be examined.
	for i := 0; i <= nworkers; i++ {
		wr := newEntry()
		go func(w *writeqEntry, id int) {
			n := <-numCh
			if n >= nworkers {
				// All of the prior goroutines should be blocked in their
				// invocation of wq.wait. Closing this channel allows
				// the code below to inspect the writeq. However, this
				// is still racy since the most recent goroutine may
				// still running through wait rather than having blocked,
				// hence the slight delay.
				time.Sleep(time.Millisecond * 100)
				close(numChDone)
				return
			}
			writerMu.Lock()
			writers[n] = w
			writerMu.Unlock()
			go func() {
				// Try and avoid the race between signaling the next goroutine
				// and calling wait below.
				time.Sleep(5 * time.Millisecond)
				select {
				case numCh <- n + 1:
				default:
				}
			}()
			wq.wait(nil, &w.writer, flowPriority)
			assertWait(w)
			if active := wq.getActive(); active != &w.writer {
				errCh <- fmt.Errorf("invariant violated: active: got %p, want %p", active, &w.writer)
			}
			time.Sleep(time.Duration(rand.Int31n(100)) * time.Millisecond)
			doneCh <- doneRecord{w, n}
			wq.done(&w.writer)
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

	for err := range errCh {
		if err != nil {
			t.Error(err)
		}
	}

	i := 0
	// All of the goroutines should get their writeq turn in the
	// order that they were created.
	for w := range doneCh {
		if got, want := w.we, writers[w.n]; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		i++
	}
}

func TestWriteqSharedEntries(t *testing.T) {
	wq := &writeq{}

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

	nworkers := 10
	niterations := 1000
	shared := newEntry()
	var sharedMu sync.Mutex
	var done, ready sync.WaitGroup
	done.Add(nworkers)
	ready.Add(nworkers)
	goCh := make(chan struct{})
	errCh := make(chan error, nworkers)

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
				assertWait(shared)
				if active := wq.getActive(); active != &shared.writer {
					errCh <- fmt.Errorf("invariant violated: active: got %p, want %p", active, &shared.writer)
				}
				ran[i][j]++
				wq.done(&shared.writer)
				sharedMu.Unlock()
			}
		}(i)
	}
	ready.Wait()
	close(goCh)
	done.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Error(err)
		}
	}

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

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

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
				assertWait(shared)
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

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

	assertCancel := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertNotActive(t, wq, &w.writer)
	}

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
	assertCancel(fe2)
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
				assertCancel(shared)
				if perr := nilPointers(&shared.writer); perr != nil {
					errCh <- fmt.Errorf("%v: %v", err, perr)
					return
				}
				errCh <- err
				return
			}
			assertWait(shared)
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

func TestWriteqContextCancelRace(t *testing.T) {
	wq := &writeq{}

	rctx, shutdown := test.V23Init()
	defer shutdown()

	assertWait := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertActive(t, wq, &w.writer)
	}

	assertCancel := func(w *writeqEntry) {
		assertChanSize(t, w.notify, 0)
		assertNotActive(t, wq, &w.writer)
	}

	fe1 := newEntry()
	nWriters := 4
	nIterations := 100
	cancelations := 0

	writers := make([]*writeqEntry, nWriters)
	for i := 0; i < nWriters; i++ {
		writers[i] = newEntry()
	}

	// This test is intended to catch the case where a writer that
	// has just become active and is waiting to receive its notification
	// is also canceled. In this case the notification and the cancelation
	// are racing, hence we need to run the loop below enough times to
	// to have the cancelation win the race. See handleCancel in writeq.go.
	// This race also occurs with the RPC 'mux' benchmarks which run both
	// streaming and non-streaming RPCs with cancelations.
	for cancelations == 0 {
		// Iterate until some cancelations are encountered or time out!
		for i := 0; i < nIterations; i++ {

			ctx, cancel := context.WithCancel(rctx)

			var wg sync.WaitGroup

			errCh := make(chan error, nWriters*2)

			runner := func(ctx *context.T, id int, we *writeqEntry) {
				err := wq.wait(ctx, &we.writer, flowPriority)
				errCh <- err
				if err != nil {
					if errors.Is(err, context.Canceled) {
						assertCancel(we)
						if err := nilPointers(&we.writer); err != nil {
							_, _, line, _ := runtime.Caller(1)
							errCh <- fmt.Errorf("line %v: %v: %v", line, id, err)
						}
					}
					return
				}
				assertWait(we)
				wq.done(&we.writer)
			}

			if err := wq.wait(ctx, &fe1.writer, flowPriority); err != nil {
				t.Fatal(err)
			}
			assertWait(fe1)

			wg.Add(nWriters)
			for j := 0; j < nWriters; j++ {
				go func(j int) {
					runner(ctx, j, writers[j])
					wg.Done()
				}(j)
			}

			wq.mu.Lock()
			if wq.active != &fe1.writer {
				t.Fatalf("%v: active should be %p: %p %v\n", i, &fe1.writer, wq.active, wq.active)
			}
			wq.mu.Unlock()

			cancel()
			wq.done(&fe1.writer)

			assertNotActive(t, wq, &fe1.writer)

			wg.Wait()

			assertActive(t, wq, nil)

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

			for j := 0; j < nWriters; j++ {
				if len(writers[j].notify) != 0 {
					t.Fatalf("writer %v: %p: has a stored notification", j, &writers[j].writer)
				}
			}

		}
	}
	t.Logf("#cancelations %v\n", cancelations)
}
