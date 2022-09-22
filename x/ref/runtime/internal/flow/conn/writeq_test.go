// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"fmt"
	"reflect"
	"runtime"
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

func addWriteq(wq *writeq, w ...*writeqEntry) {
	for i := range w {
		wq.activateWriterLocked(&w[i].writer, flowPriority)
	}
}

func rmWriteq(wq *writeq, w ...*writeqEntry) {
	for i := range w {
		wq.deactivateWriterLocked(&w[i].writer, flowPriority)
	}
}

func cmpWriteqEntries(t *testing.T, wq *writeq, w ...*writeqEntry) {
	var wl []*writer
	if len(w) > 0 {
		wl = make([]*writer, len(w))
		for i := range wl {
			wl[i] = &w[i].writer
		}
	}

	if got, want := listWQEntries(wq, flowPriority), wl; !reflect.DeepEqual(got, want) {
		_, _, line, _ := runtime.Caller(2)
		t.Errorf("line %v: got %#v, want %#v", line, got, want)
	}
}

func TestWriteqLists(t *testing.T) {
	wq := &writeq{}

	add := func(w ...*writeqEntry) {
		addWriteq(wq, w...)
	}

	rm := func(w ...*writeqEntry) {
		rmWriteq(wq, w...)
	}

	cmp := func(w ...*writeqEntry) {
		cmpWriteqEntries(t, wq, w...)
	}

	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()
	add(fe1, fe1)
	cmp(fe1)
	rm(fe1)
	cmp()
	add(fe1)
	cmp(fe1)

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
	cmp()
}

func TestWriteqNotification(t *testing.T) {
	wq := &writeq{}

	add := func(w ...*writeqEntry) {
		addWriteq(wq, w...)
	}

	rm := func(w ...*writeqEntry) {
		rmWriteq(wq, w...)
	}

	cmp := func(w ...*writeqEntry) {
		cmpWriteqEntries(t, wq, w...)
	}

	notify := func(w *writeqEntry) {
		wq.notifyNextWriterLocked(&w.writer)
	}

	active := func(w *writeqEntry) {
		if got, want := wq.writing, &w.writer; got != want {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line %v: got (%p)%v, want (%p)%v", line, got, got, want, want)
		}
	}

	fe1, fe2, fe3 := newEntry(), newEntry(), newEntry()
	add(fe1)
	go notify(fe1)
	<-fe1.notify
	cmp(fe1)
	rm(fe1)
	cmp()

	add(fe1, fe2, fe3)
	notify(fe1)
	notify(fe1)
	notify(fe1)

	<-fe1.notify
	cmp(fe2, fe3, fe1)
	active(fe1)
	notify(fe1)
	active(fe2)
	cmp(fe3, fe1, fe2)
	cmp()

	return

	notify(fe1)
	cmp(fe2, fe3, fe1)
	go notify(fe2)
	<-fe2.notify
	cmp(fe3, fe1, fe2)

	fmt.Printf("-------------\n")
	go notify(fe2)
	<-fe2.notify
	go notify(fe3)
	<-fe3.notify
	_, _, _ = rm, fe2, fe3
}
