// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"reflect"
	"runtime"
	"sync"
	"testing"
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

func listWQEntries(wq *writeq, p int) []*writer {
	var r []*writer
	for w := wq.activeWriters[p]; w != nil; w = w.next {
		r = append(r, w)
		if w.next == wq.activeWriters[p] {
			break
		}
	}
	return r
}

func addWriteq(wq *writeq, priority int, w ...*writeqEntry) {
	for i := range w {
		wq.addWriterLocked(wq.activeWriters[priority], &w[i].writer, priority)
	}
}

func cmpWriteqEntries(t *testing.T, wq *writeq, priority int, active *writeqEntry, w ...*writeqEntry) {
	_, _, line, _ := runtime.Caller(1)
	if active == nil {
		if got, want := wq.active, (*writer)(nil); got != want {
			t.Errorf("line %v: queue: got %v, want %v", line, got, want)
		}
	} else {
		if got, want := wq.active, &active.writer; got != want {
			t.Errorf("line %v: queue: got %v, want %v", line, got, want)
		}
	}

	var wl []*writer
	if len(w) > 0 {
		wl = make([]*writer, len(w))
		for i := range wl {
			wl[i] = &w[i].writer
		}
	}
	if got, want := listWQEntries(wq, priority), wl; !reflect.DeepEqual(got, want) {
		t.Errorf("line %v: queue: got %v, want %v", line, got, want)
	}
}

func cmpWriteqNext(t *testing.T, wq *writeq, w *writeqEntry) {
	_, _, line, _ := runtime.Caller(1)
	if got, want := wq.nextLocked(), &w.writer; got != want {
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

	wq.wait(nil, &fe1.writer, flowPriority)

	go func() {
		wg.Done()
		wq.wait(nil, &fe2.writer, flowPriority)
		ch <- fe2
	}()
	go func() {
		wg.Done()
		wq.wait(nil, &fe3.writer, expressPriority)
		ch <- fe3
	}()

	wg.Wait()
	wq.done(&fe1.writer)
	if got, want := <-ch, fe3; got != want {
		t.Errorf("got %p, want %p", got, want)
	}
	wq.done(&fe3.writer)
	if got, want := <-ch, fe2; got != want {
		t.Errorf("got %p, want %p", got, want)
	}

}

// test context cancelation.

/*
	ch := make(chan *writeqEntry, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	w1, w2, w3 := make(chan struct{}), make(chan struct{}), make(chan struct{})
	waiter := func(w *writeqEntry, p int, gate <-chan struct{}) {
		<-gate
		wq.wait(nil, &w.writer, p)
		ch <- w
		wg.Done()
	}

	go waiter(fe1, flowPriority, w1)
	go waiter(fe2, expressPriority, w2)
	go waiter(fe3, flowPriority, w3)

	close(w1)
	close(w2)
	close(w3)
	wg.Wait()
	close(ch)

	var done []*writeqEntry
	for w := range ch {
		done = append(done, w)
	}
	if got, want := done, []*writeqEntry{fe2, fe1, fe3}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}*/
//}

//	add(fe1, fe1)

/*
	cmp(fe1)
	rm(fe1)
	cmp()
	add(fe1)
	cmp(fe1)

	_, _ = fe2, fe3
*/
/*
	add(fe2)
	add(fe2)
	cmp(fe1, fe2)
	add(fe3)
	add(fe1)
	add(fe2)
	add(fe3)
	cmp(fe1, fe2, fe3)

	rm(fe2)
	cmp(fe1, fe3)
	rm(fe1)
	cmp(fe3)
	rm(fe3)
	cmp()
	add(fe1, fe2, fe3)
	cmp(fe1, fe2, fe3)
	rm(fe3)
	cmp(fe1, fe2)
	rm(fe2)
	cmp(fe1)
	rm(fe1)
	cmp()*/

/*
func TestWriteqNotification(t *testing.T) {
	wq := &writeq{}

	add := func(w ...*writeqEntry) {
		addWriteq(wq, flowPriority, w...)
	}

	rm := func(w ...*writeqEntry) {
		rmWriteq(wq, flowPriority, w...)
	}

	cmp := func(a *writeqEntry, w ...*writeqEntry) {
		cmpWriteqEntries(t, wq, flowPriority, a, w...)
	}

	addP0 := func(w ...*writeqEntry) {
		addWriteq(wq, expressPriority, w...)
	}

	rmP0 := func(w ...*writeqEntry) {
		rmWriteq(wq, expressPriority, w...)
	}

	cmpP0 := func(a *writeqEntry, w ...*writeqEntry) {
		cmpWriteqEntries(t, wq, expressPriority, a, w...)
	}

	notify := func(w *writeqEntry) {
		var wr *writer
		if w != nil {
			wr = &w.writer
		}
		wq.notifyNextWriter(wr)
	}

	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()

	notified := func(w *writeqEntry) {
		var got *writer
		select {
		case <-fe1.writer.notify:
			got = &fe1.writer
		case <-fe2.writer.notify:
			got = &fe2.writer
		case <-fe3.writer.notify:
			got = &fe3.writer
		}
		if want := &w.writer; got != want {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line %v: wrong writer notified: got %v, want %v", line, got, want)
		}
	}

	add(fe1)
	notify(fe1)
	<-fe1.notify
	cmp(fe1, fe1)
	rm(fe1)
	cmp(fe1)
	notify(fe1)
	cmp(nil)

	// iterate a few times to ensure that the select statement in
	// notified doesn't select from the expected channel by chance
	// when there are multiple channels ready - i.e. make sure that
	// there is exactly one writer ready to go.
	for i := 0; i < 100; i++ {
		add(fe1, fe2, fe3)
		cmp(nil, fe1, fe2, fe3)
		notify(fe1)
		notified(fe1)
		cmp(fe1, fe2, fe3, fe1)
		notify(fe1)
		notified(fe2)
		cmp(fe2, fe3, fe1, fe2)
		notify(fe2)
		notified(fe3)
		cmp(fe3, fe1, fe2, fe3)
		notify(fe3)
		notified(fe1)
		cmp(fe1, fe2, fe3, fe1)
		notify(fe1)
		notified(fe2)

		// reset to empty state.
		rm(fe1, fe2, fe3)
		notify(fe2)
		cmp(nil)
	}

	// test priorities
	for i := 0; i < 100; i++ {
		add(fe2, fe3)
		addP0(fe1)
		cmp(nil, fe2, fe3)
		cmpP0(nil, fe1)
		notify(fe2)
		notified(fe1) // the higher priority writer should get unblocked

		cmp(fe1, fe2, fe3)
		cmpP0(fe1, fe1)
		rmP0(fe1)
		// fe1 is done so remove it as the active writer and unblock fe2
		notify(fe1)
		cmp(fe2, fe3, fe2)
		notified(fe2)
		notify(fe2)
		notified(fe3)

		// reset to empty state.
		rm(fe2, fe3)
		notify(fe3)
		cmp(nil)
	}

	for i := 0; i < 100; i++ {
		wq.activateAndNotify(&fe2.writer, flowPriority)
		cmp(fe2, fe2)
		notified(fe2)
		cmp(fe2, fe2)
		wq.activateAndNotify(&fe1.writer, flowPriority)
		cmp(fe2, fe2, fe1)
		wq.deactivateAndNotify(&fe2.writer, flowPriority)
		cmp(fe1, fe1)
		notified(fe1)
		wq.deactivateAndNotify(&fe1.writer, flowPriority)
		cmp(nil)
	}

	wq.activateAndNotify(&fe1.writer, flowPriority)
	wq.activateAndNotify(nil, flowPriority)
}
*/
